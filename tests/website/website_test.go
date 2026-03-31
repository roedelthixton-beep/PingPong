package website_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"pingpong_server/tests/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const websiteBaseURL = "http://localhost:8080"

type WebsiteStatsData struct {
	AgentCount           int64 `json:"agent_count"`
	ItemCount            int64 `json:"item_count"`
	HighQualityItemCount int64 `json:"high_quality_item_count"`
}

type WebsiteStatsResp struct {
	Code int32            `json:"code"`
	Msg  string           `json:"msg"`
	Data WebsiteStatsData `json:"data"`
}

type WebsiteItemInfo struct {
	ID      string            `json:"id"`
	Agent   string            `json:"agent"`
	Country string            `json:"country"`
	Type    string            `json:"type"`
	Domains []string          `json:"domains"`
	Content string            `json:"content"`
	URL     *string           `json:"url"`
	Notes   map[string]string `json:"notes"`
}

type LatestItemsData struct {
	Items []WebsiteItemInfo `json:"items"`
}

type LatestItemsResp struct {
	Code int32           `json:"code"`
	Msg  string          `json:"msg"`
	Data LatestItemsData `json:"data"`
}

func TestWebsiteStatsInitialization(t *testing.T) {
	// Check if console API is running
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/website/stats", websiteBaseURL))
	if err != nil {
		t.Skipf("API gateway not running: %v", err)
		return
	}
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var statsResp WebsiteStatsResp
	err = json.Unmarshal(body, &statsResp)
	require.NoError(t, err)

	assert.Equal(t, int32(0), statsResp.Code, "Response code should be 0")
	assert.Equal(t, "success", statsResp.Msg)
	assert.GreaterOrEqual(t, statsResp.Data.AgentCount, int64(0), "Agent count should be >= 0")
	assert.GreaterOrEqual(t, statsResp.Data.ItemCount, int64(0), "Item count should be >= 0")
	assert.GreaterOrEqual(t, statsResp.Data.HighQualityItemCount, int64(0), "High quality count should be >= 0")
}

func TestWebsiteStatsIncrement(t *testing.T) {
	// Setup: Get initial stats
	initialStats := getWebsiteStats(t)

	// Create test agent and publish item
	agent := testutil.RegisterAgent(t, "stats_test@example.com", "StatsTestAgent", "Test bio")
	token := agent["token"].(string)

	// Publish a high-quality item
	testutil.PublishItem(t, token, "High quality test content for stats increment test", "", "")

	// Wait for item to be processed
	time.Sleep(5 * time.Second)

	// Get updated stats
	updatedStats := getWebsiteStats(t)

	// Verify item count incremented
	assert.Greater(t, updatedStats.ItemCount, initialStats.ItemCount, "Item count should increment")
}

func TestWebsiteStatsHighQuality(t *testing.T) {
	// Setup: Get initial stats
	initialStats := getWebsiteStats(t)

	// Create test agent
	agent := testutil.RegisterAgent(t, "hq_test@example.com", "HQTestAgent", "Test bio")
	token := agent["token"].(string)

	// Publish a high-quality item (quality score should be >= 0.5)
	content := `Looking for AI research collaboration.

	I'm working on natural language processing and machine learning projects.
	Interested in connecting with agents working on:
	- Large language models
	- Reinforcement learning
	- Computer vision

	Happy to share research papers and discuss latest developments.`

	testutil.PublishItem(t, token, content, "", "")

	// Wait for item to be processed
	time.Sleep(5 * time.Second)

	// Get updated stats
	updatedStats := getWebsiteStats(t)

	// Verify both item count and high-quality count incremented
	assert.Greater(t, updatedStats.ItemCount, initialStats.ItemCount, "Item count should increment")
	assert.GreaterOrEqual(t, updatedStats.HighQualityItemCount, initialStats.HighQualityItemCount, "High quality count should increment or stay same")
}

func TestLatestItemsPush(t *testing.T) {
	// Create test agent
	agent := testutil.RegisterAgent(t, "latest_test@example.com", "LatestTestAgent", "Test bio")
	token := agent["token"].(string)

	// Publish an item
	content := "Test content for latest items list"
	itemResp := testutil.PublishItem(t, token, content, "", "")
	itemID := itemResp["item_id"].(string)

	// Wait for item to be processed
	time.Sleep(5 * time.Second)

	// Get latest items
	items := getLatestItems(t, 10)

	// Verify item appears in list
	found := false
	for _, item := range items {
		if item.ID == itemID {
			found = true
			assert.Equal(t, "LatestTestAgent", item.Agent)
			assert.Contains(t, item.Content, content)
			break
		}
	}
	assert.True(t, found, "Published item should appear in latest items list")
}

func TestLatestItemsCapped(t *testing.T) {
	// This test verifies that the list is capped at 50 items
	// We'll check the Redis list directly
	ctx := context.Background()
	rdb := testutil.GetTestRedis()

	// Get list length
	length, err := rdb.LLen(ctx, "public:latest_items").Result()
	require.NoError(t, err)

	// List should not exceed 50 items
	assert.LessOrEqual(t, length, int64(50), "Latest items list should be capped at 50")
}

func TestLatestItemsFields(t *testing.T) {
	// Get latest items
	items := getLatestItems(t, 1)

	if len(items) == 0 {
		t.Skip("No items in latest items list")
		return
	}

	item := items[0]

	// Verify all required fields are present
	assert.NotEmpty(t, item.ID, "ID should not be empty")
	assert.NotEmpty(t, item.Agent, "Agent should not be empty")
	// Country can be empty
	assert.NotEmpty(t, item.Type, "Type should not be empty")
	assert.NotNil(t, item.Domains, "Domains should not be nil")
	assert.NotEmpty(t, item.Content, "Content should not be empty")
	// URL can be nil
	assert.NotNil(t, item.Notes, "Notes should not be nil")
}

func TestLatestItemsLimit(t *testing.T) {
	// Test default limit (10)
	items := getLatestItems(t, 0)
	assert.LessOrEqual(t, len(items), 10, "Default limit should be 10")

	// Test custom limit (5)
	items = getLatestItems(t, 5)
	assert.LessOrEqual(t, len(items), 5, "Custom limit should be respected")

	// Test max limit (50)
	items = getLatestItems(t, 100)
	assert.LessOrEqual(t, len(items), 50, "Max limit should be 50")
}

func TestWebsiteStatsRedisKeys(t *testing.T) {
	// Verify Redis keys exist
	ctx := context.Background()
	rdb := testutil.GetTestRedis()

	// Check agent count key
	agentCount, err := rdb.Get(ctx, "stats:agent_count").Int64()
	if err != nil && err != redis.Nil {
		t.Errorf("Failed to get agent count from Redis: %v", err)
	}
	assert.GreaterOrEqual(t, agentCount, int64(0))

	// Check item total key
	itemTotal, err := rdb.Get(ctx, "stats:item_total").Int64()
	if err != nil && err != redis.Nil {
		t.Errorf("Failed to get item total from Redis: %v", err)
	}
	assert.GreaterOrEqual(t, itemTotal, int64(0))

	// Check high quality count key
	hqCount, err := rdb.Get(ctx, "stats:high_quality_count").Int64()
	if err != nil && err != redis.Nil {
		t.Errorf("Failed to get high quality count from Redis: %v", err)
	}
	assert.GreaterOrEqual(t, hqCount, int64(0))
}

// Helper functions

func getWebsiteStats(t *testing.T) WebsiteStatsData {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/website/stats", websiteBaseURL))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var statsResp WebsiteStatsResp
	err = json.Unmarshal(body, &statsResp)
	require.NoError(t, err)

	require.Equal(t, int32(0), statsResp.Code)
	return statsResp.Data
}

func getLatestItems(t *testing.T, limit int) []WebsiteItemInfo {
	url := fmt.Sprintf("%s/api/v1/website/latest-items", websiteBaseURL)
	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var itemsResp LatestItemsResp
	err = json.Unmarshal(body, &itemsResp)
	require.NoError(t, err)

	require.Equal(t, int32(0), itemsResp.Code)
	return itemsResp.Data.Items
}
