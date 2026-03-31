package cache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pingpong_server/pkg/config"
	"pingpong_server/tests/testutil"
)

func TestMain(m *testing.M) {
	testutil.RunTestMain(m)
}

// TestCacheE2E tests the multi-level caching system end-to-end
func TestCacheE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E cache test in short mode")
	}

	testCfg := config.Load()
	if !testCfg.EnableSearchCache {
		t.Skip("Cache is not enabled (ENABLE_SEARCH_CACHE=false), skipping cache E2E test")
	}

	testutil.WaitForAPI(t)
	testutil.CleanTestData(t)

	rdb := redis.NewClient(&redis.Options{
		Addr: testCfg.RedisAddr,
	})
	defer rdb.Close()

	ctx := context.Background()

	tt := time.Now().Unix()
	authorEmail := fmt.Sprintf("cache_author%d@test.com", tt)
	userEmail := fmt.Sprintf("cache_user%d@test.com", tt)

	t.Log("=== Step 1: Setup test agents ===")
	authorResp := testutil.RegisterAgent(t, authorEmail, "CacheAuthor", "Tech writer")
	authorToken := authorResp["token"].(string)

	userResp := testutil.RegisterAgent(t, userEmail, "CacheUser", "")
	userToken := userResp["token"].(string)
	userID := testutil.MustID(t, userResp["agent_id"], "agent_id")

	t.Log("=== Step 2: Update user profile ===")
	testutil.UpdateProfile(t, userToken, "I am interested in artificial intelligence and machine learning")
	testutil.WaitForProfileProcessed(t, userID)

	t.Log("=== Step 3: Publish test items ===")
	item1 := testutil.PublishItem(t, authorToken,
		"Google DeepMind announced a breakthrough in protein structure prediction using a next-generation AlphaFold model. The system achieves 95% accuracy on previously unsolvable protein complexes and reduces computation time by 80%. Researchers demonstrated successful predictions for over 200 million protein structures, opening new possibilities for drug discovery and synthetic biology applications.",
		"AlphaFold breakthrough in protein structure prediction",
		"https://example.com/ai")
	item1ID := testutil.MustID(t, item1["item_id"], "item_id")

	item2 := testutil.PublishItem(t, authorToken,
		"A team at Stanford University published a comprehensive benchmark comparing 15 open-source large language models on mathematical reasoning, code generation, and multilingual understanding tasks. The study finds that models with mixture-of-experts architectures achieve 40% better performance per compute unit compared to dense transformer models of equivalent parameter count.",
		"Comprehensive LLM benchmark study from Stanford",
		"https://example.com/ml")
	item2ID := testutil.MustID(t, item2["item_id"], "item_id")

	testutil.WaitForItemsProcessed(t, []int64{item1ID, item2ID})

	t.Log("=== Step 4: Test profile cache ===")
	profileCacheKey := fmt.Sprintf("cache:profile:%d", userID)

	_ = testutil.FetchFeedRefresh(t, userToken, 20)
	time.Sleep(100 * time.Millisecond)

	exists, err := rdb.Exists(ctx, profileCacheKey).Result()
	require.NoError(t, err)
	assert.Greater(t, exists, int64(0), "Profile should be cached after first fetch")

	ttl, err := rdb.TTL(ctx, profileCacheKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl.Seconds(), float64(0), "Profile cache should have TTL")
	assert.LessOrEqual(t, ttl.Seconds(), float64(60), "Profile cache TTL should be <= 60s")
	t.Logf("Profile cache TTL: %.1fs", ttl.Seconds())

	t.Log("=== Step 5: Test search cache ===")
	searchKeys, err := rdb.Keys(ctx, "cache:search:*").Result()
	require.NoError(t, err)
	t.Logf("Found %d search cache keys", len(searchKeys))

	if len(searchKeys) > 0 {
		searchTTL, err := rdb.TTL(ctx, searchKeys[0]).Result()
		require.NoError(t, err)
		assert.Greater(t, searchTTL.Seconds(), float64(0), "Search cache should have TTL")
		assert.LessOrEqual(t, searchTTL.Seconds(), float64(2), "Search cache TTL should be <= 2s")
		t.Logf("Search cache TTL: %.1fs", searchTTL.Seconds())
	}

	t.Log("=== Step 6: Test concurrent requests (SingleFlight) ===")
	concurrency := 10
	var wg sync.WaitGroup
	results := make([]map[string]interface{}, concurrency)

	startTime := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = testutil.FetchFeedRefresh(t, userToken, 20)
		}(i)
	}
	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Concurrent requests completed in %v", duration)

	firstResult := results[0]["items"].([]interface{})
	for i := 1; i < concurrency; i++ {
		items := results[i]["items"].([]interface{})
		assert.Equal(t, len(firstResult), len(items), "All concurrent requests should return same number of items")
	}

	assert.Less(t, duration.Milliseconds(), int64(1000), "Concurrent requests should complete quickly with caching")

	t.Log("=== Step 7: Test cache expiration ===")
	t.Log("Waiting for search cache to expire (2s)...")
	time.Sleep(3 * time.Second)

	searchKeysAfter, err := rdb.Keys(ctx, "cache:search:*").Result()
	require.NoError(t, err)
	t.Logf("Search cache keys after expiration: %d", len(searchKeysAfter))

	feed2 := testutil.FetchFeedRefresh(t, userToken, 20)
	items2 := feed2["items"].([]interface{})
	t.Logf("Feed after cache expiration returned %d items", len(items2))

	t.Log("=== Step 8: Test profile cache invalidation ===")
	t.Log("Clearing impression records...")
	imprKey := fmt.Sprintf("impr:agent:%d:items", userID)
	rdb.Del(ctx, imprKey)

	testutil.UpdateProfile(t, userToken, "I am interested in deep learning and neural networks")
	testutil.WaitForProfileProcessed(t, userID)

	time.Sleep(2 * time.Second)

	feed3 := testutil.FetchFeedRefresh(t, userToken, 20)
	items3 := feed3["items"].([]interface{})
	t.Logf("Feed with updated profile returned %d items", len(items3))

	t.Log("=== Step 9: Test load_more action ===")
	if len(items3) > 0 {
		feed4 := testutil.FetchFeedLoadMore(t, userToken, 20)
		items4 := feed4["items"].([]interface{})
		t.Logf("Load more returned %d items", len(items4))
	}

	t.Log("=== Step 10: Test cache graceful degradation ===")
	rdb.Del(ctx, fmt.Sprintf("impr:agent:%d:items", userID))

	feed5 := testutil.FetchFeedRefresh(t, userToken, 20)
	items5 := feed5["items"].([]interface{})
	t.Logf("Feed returned %d items (graceful degradation test)", len(items5))

	t.Log("=== Cache E2E test completed successfully ===")
}

func TestCachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testutil.WaitForAPI(t)

	userResp := testutil.RegisterAgent(t, "perf_user@test.com", "PerfUser", "")
	userToken := userResp["token"].(string)
	userID := testutil.MustID(t, userResp["agent_id"], "agent_id")

	testutil.UpdateProfile(t, userToken, "I am interested in technology and software engineering")
	testutil.WaitForProfileProcessed(t, userID)

	testutil.FetchFeedRefresh(t, userToken, 20)
	time.Sleep(100 * time.Millisecond)

	t.Log("=== Measuring cache hit performance ===")
	iterations := 50
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		testutil.FetchFeedRefresh(t, userToken, 20)
		duration := time.Since(start)
		totalDuration += duration
	}

	avgDuration := totalDuration / time.Duration(iterations)
	t.Logf("Average request duration (with cache): %v", avgDuration)
	t.Logf("Requests per second: %.2f", float64(iterations)/totalDuration.Seconds())

	assert.Less(t, avgDuration.Milliseconds(), int64(100),
		"Average request with cache should be < 100ms")
}

func TestCacheConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	testutil.WaitForAPI(t)

	userResp := testutil.RegisterAgent(t, "concurrent_user@test.com", "ConcurrentUser", "")
	userToken := userResp["token"].(string)
	userID := testutil.MustID(t, userResp["agent_id"], "agent_id")

	testutil.UpdateProfile(t, userToken, "I am interested in distributed systems and databases")
	testutil.WaitForProfileProcessed(t, userID)

	t.Log("=== Testing high concurrency ===")

	concurrency := 100
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	startTime := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Request %d panicked: %v", idx, r)
				}
			}()

			result := testutil.FetchFeedRefresh(t, userToken, 20)
			if result != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Completed %d/%d requests in %v", successCount, concurrency, duration)
	t.Logf("Throughput: %.2f req/s", float64(concurrency)/duration.Seconds())

	assert.Equal(t, concurrency, successCount, "All concurrent requests should succeed")
	assert.Less(t, duration.Seconds(), float64(5),
		"100 concurrent requests should complete in < 5s with caching")
}
