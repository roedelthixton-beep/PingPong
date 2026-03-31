package bloomfilter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestBloomFilter_Add(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	bf := NewBloomFilter(rdb)
	ctx := context.Background()

	agentID := int64(1001)
	groupIDs := []int64{100001, 100002, 100003}

	err := bf.Add(ctx, agentID, groupIDs)
	assert.NoError(t, err)

	// Verify items were added
	key := GetKeyForDate(time.Now())
	for _, gid := range groupIDs {
		value := fmt.Sprintf("1001:%d", gid)
		exists, err := rdb.SIsMember(ctx, key, value).Result()
		assert.NoError(t, err)
		assert.True(t, exists, "Expected %s to exist in bloom filter", value)
	}
}

func TestBloomFilter_CheckExists(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	bf := NewBloomFilter(rdb)
	ctx := context.Background()

	agentID := int64(1001)

	// Add some items
	addedGroups := []int64{100001, 100002}
	err := bf.Add(ctx, agentID, addedGroups)
	require.NoError(t, err)

	// Check existence
	checkGroups := []int64{100001, 100002, 100003}
	result, err := bf.CheckExists(ctx, agentID, checkGroups)
	assert.NoError(t, err)

	assert.True(t, result[100001], "100001 should exist")
	assert.True(t, result[100002], "100002 should exist")
	assert.False(t, result[100003], "100003 should not exist")
}

func TestBloomFilter_CheckExists_MultipleAgents(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	bf := NewBloomFilter(rdb)
	ctx := context.Background()

	// Agent 1 sees group 100001
	err := bf.Add(ctx, 1001, []int64{100001})
	require.NoError(t, err)

	// Agent 2 sees group 100002
	err = bf.Add(ctx, 1002, []int64{100002})
	require.NoError(t, err)

	// Agent 1 should only see 100001
	result1, err := bf.CheckExists(ctx, 1001, []int64{100001, 100002})
	assert.NoError(t, err)
	assert.True(t, result1[100001])
	assert.False(t, result1[100002])

	// Agent 2 should only see 100002
	result2, err := bf.CheckExists(ctx, 1002, []int64{100001, 100002})
	assert.NoError(t, err)
	assert.False(t, result2[100001])
	assert.True(t, result2[100002])
}

func TestBloomFilter_EmptyInput(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	bf := NewBloomFilter(rdb)
	ctx := context.Background()

	// Add empty list
	err := bf.Add(ctx, 1001, []int64{})
	assert.NoError(t, err)

	// Check empty list
	result, err := bf.CheckExists(ctx, 1001, []int64{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestBloomFilter_KeyFormat(t *testing.T) {
	date := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	key := GetKeyForDate(date)
	assert.Equal(t, "bf:global:20260306", key)
}
