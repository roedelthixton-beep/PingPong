package mq

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		mr.Close()
	})

	RDB = client
	t.Cleanup(func() {
		RDB = nil
	})

	return mr, client
}

func TestConsumePendingClaimsExistingPendingMessages(t *testing.T) {
	_, client := setupTestRedis(t)
	ctx := context.Background()

	stream := "stream:test:pending"
	group := "cg:test:pending"

	require.NoError(t, EnsureConsumerGroup(ctx, stream, group))

	msgID, err := Publish(ctx, stream, map[string]interface{}{"foo": "bar"})
	require.NoError(t, err)

	messages, err := Consume(ctx, stream, group, "consumer-a", 1)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, msgID, messages[0].ID)

	pendingBefore, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pendingBefore, 1)
	assert.Equal(t, int64(1), pendingBefore[0].RetryCount)

	claimed, err := ConsumePending(ctx, stream, group, "consumer-b", 10, 0)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, msgID, claimed[0].Message.ID)
	assert.Equal(t, int64(1), claimed[0].RetryCount)
	assert.Equal(t, "consumer-a", claimed[0].Consumer)

	pendingAfter, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pendingAfter, 1)
	assert.Equal(t, int64(2), pendingAfter[0].RetryCount)
	assert.Equal(t, "consumer-b", pendingAfter[0].Consumer)
}
