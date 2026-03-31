package impr

import (
	"context"
	"fmt"
	"testing"

	"pingpong_server/pkg/config"
	"github.com/redis/go-redis/v9"
)

func newTestRedisClient() *redis.Client {
	cfg := config.Load()
	return redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
}

func TestRecordAndGetImpressions(t *testing.T) {
	rdb := newTestRedisClient()
	ctx := context.Background()

	agentID := int64(999999)
	itemKey := fmt.Sprintf(KeyItemIDs, agentID)
	groupKey := fmt.Sprintf(KeyGroupIDs, agentID)
	urlKey := fmt.Sprintf(KeyURLs, agentID)

	// Cleanup before test
	rdb.Del(ctx, itemKey, groupKey, urlKey)
	defer rdb.Del(ctx, itemKey, groupKey, urlKey)

	// Record impressions
	err := RecordImpressions(ctx, rdb, agentID, []ImprItem{
		{ItemID: 101, GroupID: 12345, URL: "https://example.com/article"},
		{ItemID: 102},
		{ItemID: 103},
	})
	if err != nil {
		t.Fatalf("RecordImpressions failed: %v", err)
	}

	// Get seen items
	seen, err := GetSeenItems(ctx, rdb, agentID)
	if err != nil {
		t.Fatalf("GetSeenItems failed: %v", err)
	}

	// Verify item IDs
	if len(seen.ItemIDs) != 3 {
		t.Errorf("expected 3 item IDs, got %d", len(seen.ItemIDs))
	}
	itemIDSet := make(map[int64]bool)
	for _, id := range seen.ItemIDs {
		itemIDSet[id] = true
	}
	for _, expectedID := range []int64{101, 102, 103} {
		if !itemIDSet[expectedID] {
			t.Errorf("expected item_id %d in seen items, but not found", expectedID)
		}
	}

	// Verify group IDs
	if len(seen.GroupIDs) != 1 {
		t.Errorf("expected 1 group ID, got %d: %v", len(seen.GroupIDs), seen.GroupIDs)
	} else if seen.GroupIDs[0] != 12345 {
		t.Errorf("expected group_id %d, got %d", 12345, seen.GroupIDs[0])
	}

	// Verify URLs
	if len(seen.URLs) != 1 {
		t.Errorf("expected 1 URL, got %d: %v", len(seen.URLs), seen.URLs)
	} else if seen.URLs[0] != "https://example.com/article" {
		t.Errorf("expected url %q, got %q", "https://example.com/article", seen.URLs[0])
	}

	t.Logf("impr test passed: %d item_ids, %d group_ids, %d urls",
		len(seen.ItemIDs), len(seen.GroupIDs), len(seen.URLs))
}

func TestRecordImpressions_Empty(t *testing.T) {
	rdb := newTestRedisClient()
	ctx := context.Background()

	// Recording empty items should succeed (no-op)
	err := RecordImpressions(ctx, rdb, 888888, []ImprItem{})
	if err != nil {
		t.Fatalf("RecordImpressions with empty items failed: %v", err)
	}
}

func TestGetSeenItems_NewAgent(t *testing.T) {
	rdb := newTestRedisClient()
	ctx := context.Background()

	agentID := int64(777777)
	itemKey := fmt.Sprintf(KeyItemIDs, agentID)
	groupKey := fmt.Sprintf(KeyGroupIDs, agentID)
	urlKey := fmt.Sprintf(KeyURLs, agentID)
	rdb.Del(ctx, itemKey, groupKey, urlKey)

	seen, err := GetSeenItems(ctx, rdb, agentID)
	if err != nil {
		t.Fatalf("GetSeenItems failed: %v", err)
	}
	if len(seen.ItemIDs) != 0 {
		t.Errorf("expected 0 item_ids for new agent, got %d", len(seen.ItemIDs))
	}
	if len(seen.GroupIDs) != 0 {
		t.Errorf("expected 0 group_ids for new agent, got %d", len(seen.GroupIDs))
	}
	if len(seen.URLs) != 0 {
		t.Errorf("expected 0 urls for new agent, got %d", len(seen.URLs))
	}
}
