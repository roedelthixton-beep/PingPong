package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"pingpong_server/pkg/config"
	"pingpong_server/pkg/db"
	"pingpong_server/pkg/stats"
	"pingpong_server/rpc/sort/dal"
	"github.com/redis/go-redis/v9"
)

const (
	lockKeyAgentCount = "lock:cron:agent_count"
	lockKeyCalibrator = "lock:cron:calibrator"
	lockTTL           = 8 * time.Minute // Lock expires before next run (10min interval)
)

// acquireLock attempts to acquire a distributed lock using Redis SET NX EX
func acquireLock(ctx context.Context, rdb *redis.Client, lockKey string, ttl time.Duration) (bool, error) {
	result, err := rdb.SetNX(ctx, lockKey, time.Now().Unix(), ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return result, nil
}

// releaseLock releases the distributed lock
func releaseLock(ctx context.Context, rdb *redis.Client, lockKey string) {
	if err := rdb.Del(ctx, lockKey).Err(); err != nil {
		log.Printf("[Cron] Warning: failed to release lock %s: %v", lockKey, err)
	}
}

// StartAgentCountUpdater starts a cron job that updates agent count every minute
func StartAgentCountUpdater(ctx context.Context, cfg *config.Config, rdb *redis.Client) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Run immediately on startup
	updateAgentCountWithLock(ctx, rdb)

	log.Println("[Cron] Agent count updater started (interval: 10 minutes)")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Cron] Agent count updater stopped")
			return
		case <-ticker.C:
			updateAgentCountWithLock(ctx, rdb)
		}
	}
}

func updateAgentCountWithLock(ctx context.Context, rdb *redis.Client) {
	// Try to acquire lock
	acquired, err := acquireLock(ctx, rdb, lockKeyAgentCount, lockTTL)
	if err != nil {
		log.Printf("[Cron] Failed to acquire lock for agent count update: %v", err)
		return
	}
	if !acquired {
		log.Println("[Cron] Agent count update skipped (another instance is running)")
		return
	}
	defer releaseLock(ctx, rdb, lockKeyAgentCount)

	var count int64
	if err := db.DB.Model(&struct {
		AgentID int64 `gorm:"column:agent_id"`
	}{}).Table("agents").Count(&count).Error; err != nil {
		log.Printf("[Cron] Failed to count agents: %v", err)
		return
	}

	if err := stats.SetAgentCount(ctx, rdb, count); err != nil {
		log.Printf("[Cron] Failed to update agent count in Redis: %v", err)
		return
	}

	// Calibrate agent countries from PG
	var countries []string
	if err := db.DB.Model(&struct {
		Country string `gorm:"column:country"`
	}{}).Table("agent_profiles").
		Where("country != ''").
		Distinct("country").
		Pluck("country", &countries).Error; err != nil {
		log.Printf("[Cron] Failed to query distinct countries: %v", err)
	} else {
		if err := stats.CalibrateAgentCountries(ctx, rdb, countries); err != nil {
			log.Printf("[Cron] Failed to calibrate agent countries in Redis: %v", err)
		}
	}

	log.Printf("[Cron] Agent count updated: %d, countries: %v", count, countries)
}

// StartStatsCalibrator starts a cron job that calibrates stats from Elasticsearch every 10 minutes
func StartStatsCalibrator(ctx context.Context, cfg *config.Config, rdb *redis.Client) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Run immediately on startup
	calibrateStatsWithLock(ctx, rdb)

	log.Println("[Cron] Stats calibrator started (interval: 10 minutes)")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Cron] Stats calibrator stopped")
			return
		case <-ticker.C:
			calibrateStatsWithLock(ctx, rdb)
		}
	}
}

func calibrateStatsWithLock(ctx context.Context, rdb *redis.Client) {
	// Try to acquire lock
	acquired, err := acquireLock(ctx, rdb, lockKeyCalibrator, lockTTL)
	if err != nil {
		log.Printf("[Cron] Failed to acquire lock for stats calibration: %v", err)
		return
	}
	if !acquired {
		log.Println("[Cron] Stats calibration skipped (another instance is running)")
		return
	}
	defer releaseLock(ctx, rdb, lockKeyCalibrator)

	// Count total items from Elasticsearch
	itemCount, err := dal.CountItems(ctx)
	if err != nil {
		log.Printf("[Cron] Failed to count items from ES: %v", err)
		return
	}

	// Count high-quality items from item_stats table (score_1_count > 0 OR score_2_count > 0)
	var hqCount int64
	if err := db.DB.Model(&struct {
		ItemID int64 `gorm:"column:item_id"`
	}{}).Table("item_stats").
		Where("score_1_count > 0 OR score_2_count > 0").
		Count(&hqCount).Error; err != nil {
		log.Printf("[Cron] Failed to count high-quality items from item_stats: %v", err)
		return
	}

	// Update Redis
	if err := stats.SetItemTotal(ctx, rdb, itemCount); err != nil {
		log.Printf("[Cron] Failed to calibrate item total in Redis: %v", err)
		return
	}

	if err := stats.SetHighQualityCount(ctx, rdb, hqCount); err != nil {
		log.Printf("[Cron] Failed to calibrate high-quality count in Redis: %v", err)
		return
	}

	log.Printf("[Cron] Stats calibrated: items=%d, high_quality=%d", itemCount, hqCount)
}
