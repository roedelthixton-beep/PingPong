package db

import (
	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis(addr, password string) {
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
}
