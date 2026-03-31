package feedcache

import (
	"context"
	"testing"

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

func TestFeedCache_PushAndPop(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	fc := NewFeedCache(rdb)
	ctx := context.Background()

	agentID := int64(1001)
	groupIDs := []int64{100001, 100002, 100003, 100004, 100005}

	// Push items
	err := fc.Push(ctx, agentID, groupIDs)
	assert.NoError(t, err)

	// Pop 2 items
	popped, err := fc.Pop(ctx, agentID, 2)
	assert.NoError(t, err)
	assert.Equal(t, []int64{100001, 100002}, popped)

	// Check remaining length
	length, err := fc.Len(ctx, agentID)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), length)

	// Pop remaining items
	popped, err = fc.Pop(ctx, agentID, 10)
	assert.NoError(t, err)
	assert.Equal(t, []int64{100003, 100004, 100005}, popped)

	// Cache should be empty now
	length, err = fc.Len(ctx, agentID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), length)
}

func TestFeedCache_Clear(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	fc := NewFeedCache(rdb)
	ctx := context.Background()

	agentID := int64(1001)
	groupIDs := []int64{100001, 100002, 100003}

	// Push items
	err := fc.Push(ctx, agentID, groupIDs)
	require.NoError(t, err)

	// Verify items exist
	length, err := fc.Len(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), length)

	// Clear cache
	err = fc.Clear(ctx, agentID)
	assert.NoError(t, err)

	// Verify cache is empty
	length, err = fc.Len(ctx, agentID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), length)
}

func TestFeedCache_MultipleAgents(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	fc := NewFeedCache(rdb)
	ctx := context.Background()

	// Agent 1 cache
	err := fc.Push(ctx, 1001, []int64{100001, 100002})
	require.NoError(t, err)

	// Agent 2 cache
	err = fc.Push(ctx, 1002, []int64{100003, 100004})
	require.NoError(t, err)

	// Pop from agent 1
	popped1, err := fc.Pop(ctx, 1001, 1)
	assert.NoError(t, err)
	assert.Equal(t, []int64{100001}, popped1)

	// Pop from agent 2
	popped2, err := fc.Pop(ctx, 1002, 1)
	assert.NoError(t, err)
	assert.Equal(t, []int64{100003}, popped2)

	// Verify remaining lengths
	len1, err := fc.Len(ctx, 1001)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), len1)

	len2, err := fc.Len(ctx, 1002)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), len2)
}

func TestFeedCache_EmptyPop(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	fc := NewFeedCache(rdb)
	ctx := context.Background()

	agentID := int64(1001)

	// Pop from empty cache
	popped, err := fc.Pop(ctx, agentID, 5)
	assert.NoError(t, err)
	assert.Empty(t, popped)
}

func TestFeedCache_EmptyPush(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	fc := NewFeedCache(rdb)
	ctx := context.Background()

	agentID := int64(1001)

	// Push empty list
	err := fc.Push(ctx, agentID, []int64{})
	assert.NoError(t, err)

	// Verify cache is still empty
	length, err := fc.Len(ctx, agentID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), length)
}

func TestFeedCache_KeyFormat(t *testing.T) {
	key := GetKey(1001)
	assert.Equal(t, "feed:cache:1001", key)
}
