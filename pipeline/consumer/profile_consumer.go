package consumer

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"pingpong_server/pipeline/llm"
	"pingpong_server/pkg/config"
	"pingpong_server/pkg/db"
	"pingpong_server/pkg/mq"
	"pingpong_server/pkg/stats"
	"pingpong_server/rpc/profile/dal"
)

const (
	profileStream = "stream:profile:update"
	profileGroup  = "cg:profile:update"
	maxRetries    = 3
)

type ProfileConsumer struct {
	llmClient  *llm.Client
	maxWorkers int
}

func NewProfileConsumer(cfg *config.Config) *ProfileConsumer {
	return &ProfileConsumer{
		llmClient:  llm.NewClient(cfg),
		maxWorkers: 10, // Fixed concurrency level
	}
}

func (c *ProfileConsumer) Start(ctx context.Context) {
	log.Printf("[ProfileConsumer] starting with %d workers", c.maxWorkers)

	if err := mq.EnsureConsumerGroup(ctx, profileStream, profileGroup); err != nil {
		log.Fatalf("[ProfileConsumer] failed to create consumer group: %v", err)
	}

	// Create message channel for worker pool
	type msgTask struct {
		id     string
		values map[string]interface{}
	}
	msgChan := make(chan msgTask, c.maxWorkers*2)
	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < c.maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("[ProfileConsumer] worker %d started", workerID)
			for task := range msgChan {
				c.processMessage(ctx, task.id, task.values)
			}
			log.Printf("[ProfileConsumer] worker %d stopped", workerID)
		}(i)
	}

	// Main loop: fetch messages and distribute to workers
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("[ProfileConsumer] context cancelled, closing message channel")
				close(msgChan)
				return
			default:
			}

			msgs, err := mq.Consume(ctx, profileStream, profileGroup, "profile-worker-1", 10)
			if err != nil {
				log.Printf("[ProfileConsumer] consume error: %v", err)
				time.Sleep(time.Second)
				continue
			}

			for _, msg := range msgs {
				task := msgTask{
					id:     msg.ID,
					values: msg.Values,
				}
				select {
				case msgChan <- task:
					// Message sent to worker
				case <-ctx.Done():
					log.Println("[ProfileConsumer] context cancelled while sending message")
					close(msgChan)
					return
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("[ProfileConsumer] shutting down, waiting for workers to finish...")
	wg.Wait()
	log.Println("[ProfileConsumer] all workers stopped")
}

func (c *ProfileConsumer) processMessage(ctx context.Context, msgID string, values map[string]interface{}) {
	agentIDStr, ok := values["agent_id"].(string)
	if !ok {
		log.Printf("[ProfileConsumer] invalid message: missing agent_id")
		mq.Ack(ctx, profileStream, profileGroup, msgID)
		return
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		log.Printf("[ProfileConsumer] invalid agent_id: %s", agentIDStr)
		mq.Ack(ctx, profileStream, profileGroup, msgID)
		return
	}

	log.Printf("[ProfileConsumer] processing agent_id=%d", agentID)

	// Set status to processing (1)
	dal.UpdateAgentProfileStatus(db.DB, agentID, 1)

	// Get agent bio
	agent, err := dal.GetAgentByID(db.DB, agentID)
	if err != nil {
		log.Printf("[ProfileConsumer] agent not found: %d, err: %v", agentID, err)
		dal.UpdateAgentProfileStatus(db.DB, agentID, 2) // failed
		mq.Ack(ctx, profileStream, profileGroup, msgID)
		return
	}

	if agent.Bio == "" {
		log.Printf("[ProfileConsumer] agent %d has empty bio, skipping", agentID)
		dal.UpdateAgentProfileStatus(db.DB, agentID, 3) // done with no keywords
		mq.Ack(ctx, profileStream, profileGroup, msgID)
		return
	}

	// Call LLM to extract keywords with retries
	var keywords []string
	var country string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		keywords, country, err = c.llmClient.ExtractKeywords(ctx, agent.Bio)
		if err == nil {
			break
		}
		log.Printf("[ProfileConsumer] LLM attempt %d/%d failed for agent %d: %v", attempt, maxRetries, agentID, err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	if err != nil {
		log.Printf("[ProfileConsumer] all retries failed for agent %d: %v", agentID, err)
		dal.UpdateAgentProfileStatus(db.DB, agentID, 2) // failed
		mq.Ack(ctx, profileStream, profileGroup, msgID)
		return
	}

	// Update keywords, country and status to done (3)
	dal.UpdateAgentProfileKeywords(db.DB, agentID, keywords, country, 3)
	log.Printf("[ProfileConsumer] agent %d keywords updated: %v, country: %s", agentID, keywords, country)

	// Incremental sync: add country to stats set
	if country != "" {
		if err := stats.AddAgentCountry(ctx, mq.RDB, country); err != nil {
			log.Printf("[ProfileConsumer] failed to sync country to stats: %v", err)
		}
	}

	mq.Ack(ctx, profileStream, profileGroup, msgID)
}
