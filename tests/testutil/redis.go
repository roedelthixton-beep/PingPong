package testutil

import (
	"testing"

	"github.com/redis/go-redis/v9"

	"pingpong_server/pkg/config"
)

var testRedis *redis.Client

func GetTestRedis() *redis.Client {
	if testRedis == nil {
		cfg := config.Load()
		testRedis = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	}
	return testRedis
}

// GetRedisClient returns Redis client for testing
func GetRedisClient(t *testing.T) *redis.Client {
	return GetTestRedis()
}
