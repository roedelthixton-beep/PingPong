package main

import (
	"log"
	"net"
	"strings"

	etcd "github.com/kitex-contrib/registry-etcd"

	"pingpong_server/kitex_gen/pingpong/feed/feedservice"
	"pingpong_server/kitex_gen/pingpong/item/itemservice"
	"pingpong_server/kitex_gen/pingpong/sort/sortservice"
	"pingpong_server/pkg/config"
	"pingpong_server/pkg/db"
	"pingpong_server/pkg/logger"
	"pingpong_server/pkg/mq"
	"pingpong_server/pkg/rpcx"
)

var (
	sortClient sortservice.Client
	itemClient itemservice.Client
)

func main() {
	cfg := config.Load()
	logger.Init("rpc/feed/.log")

	db.Init(cfg.PgDSN)
	db.InitRedis(cfg.RedisAddr, cfg.RedisPassword)
	mq.Init(cfg.RedisAddr, cfg.RedisPassword)

	etcdEndpoints := splitEtcdEndpoints(cfg.EtcdAddr)

	resolver, err := etcd.NewEtcdResolver(etcdEndpoints)
	if err != nil {
		log.Fatalf("failed to create etcd resolver: %v", err)
	}

	sortClient, err = sortservice.NewClient("SortService", rpcx.ClientOptions(resolver)...)
	if err != nil {
		log.Fatalf("failed to create sort client: %v", err)
	}

	itemClient, err = itemservice.NewClient("ItemService", rpcx.ClientOptions(resolver)...)
	if err != nil {
		log.Fatalf("failed to create item client: %v", err)
	}

	registry, err := etcd.NewEtcdRegistry(etcdEndpoints)
	if err != nil {
		log.Fatalf("failed to create etcd registry: %v", err)
	}

	listenAddr := cfg.ListenAddr(cfg.FeedRPCPort)
	addr, _ := net.ResolveTCPAddr("tcp", listenAddr)
	svr := feedservice.NewServer(
		NewFeedServiceImpl(cfg),
		rpcx.ServerOptions(addr, registry, "FeedService")...,
	)

	if err := svr.Run(); err != nil {
		log.Fatalf("feed service failed: %v", err)
	}
}

func splitEtcdEndpoints(raw string) []string {
	parts := strings.Split(raw, ",")
	endpoints := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			endpoints = append(endpoints, p)
		}
	}
	if len(endpoints) == 0 {
		return []string{"localhost:2379"}
	}
	return endpoints
}
