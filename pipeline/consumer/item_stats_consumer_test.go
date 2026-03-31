package consumer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"pingpong_server/pkg/config"
	"pingpong_server/pkg/itemstats"
	"pingpong_server/pkg/mq"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupItemStatsConsumerRedis(t *testing.T) {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		mr.Close()
	})

	mq.RDB = client
	t.Cleanup(func() {
		mq.RDB = nil
	})
}

func TestItemStatsConsumerRetriesPendingMessageUntilSuccess(t *testing.T) {
	setupItemStatsConsumerRedis(t)

	cfg := &config.Config{FeedbackConsumerWorkers: 1}
	consumer := NewItemStatsConsumer(cfg, nil)
	consumer.consumerName = "test-item-stats-success"
	consumer.readBlock = 10 * time.Millisecond
	consumer.retryMinIdle = 5 * time.Millisecond
	consumer.maxRetries = 3

	var attempts atomic.Int64
	consumer.handleEvent = func(ctx context.Context, event itemstats.Event) error {
		if attempts.Add(1) < 3 {
			return errors.New("transient failure")
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		consumer.Start(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	_, err := itemstats.PublishConsumed(context.Background(), 101, 202)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		pending, pendingErr := mq.PendingCount(context.Background(), itemstats.StreamName, itemstats.GroupName)
		return pendingErr == nil && attempts.Load() == 3 && pending == 0
	}, 2*time.Second, 20*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int64(3), attempts.Load())
}

func TestItemStatsConsumerDropsMessageAfterMaxRetries(t *testing.T) {
	setupItemStatsConsumerRedis(t)

	cfg := &config.Config{FeedbackConsumerWorkers: 1}
	consumer := NewItemStatsConsumer(cfg, nil)
	consumer.consumerName = "test-item-stats-drop"
	consumer.readBlock = 10 * time.Millisecond
	consumer.retryMinIdle = 5 * time.Millisecond
	consumer.maxRetries = 3

	var attempts atomic.Int64
	consumer.handleEvent = func(ctx context.Context, event itemstats.Event) error {
		attempts.Add(1)
		return errors.New("persistent failure")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		consumer.Start(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	_, err := itemstats.PublishConsumed(context.Background(), 303, 404)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		pending, pendingErr := mq.PendingCount(context.Background(), itemstats.StreamName, itemstats.GroupName)
		return pendingErr == nil && attempts.Load() == 3 && pending == 0
	}, 2*time.Second, 20*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int64(3), attempts.Load())
}
