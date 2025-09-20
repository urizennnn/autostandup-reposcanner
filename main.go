package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	goredis "github.com/redis/go-redis/v9"
	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/redis"
)

var consumerName = fmt.Sprintf("%s-%d", "auto-standup-repo-scanner-1", os.Getpid())

func main() {
	fmt.Println("Starting Repo Scanner Service...")
	cfg, err := config.NewLoader("APP").Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var rdbClient *goredis.Client
	if cfg.RedisURL != "" {
		rdbClient, err = redis.ConnectToRedisURL(cfg.RedisURL)
	} else {
		rdbClient, err = redis.ConnectToRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisUseTLS)
	}
	if err != nil {
		log.Fatalf("Redis connection error: %v", err)
	}
	err = redis.WatchStreams(ctx, rdbClient, "scan:jobs", "scanners", consumerName)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("WatchStreams %v", err)
	}
}
