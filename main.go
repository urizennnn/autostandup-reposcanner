package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/redis"
)

var consumerName = fmt.Sprintf("%s-%d", "auto-standup-repo-scanner-1", os.Getpid())

func main() {
	log.Printf("[INFO] starting repo scanner service")
	cfg, err := config.NewLoader("APP").Load()
	if err != nil {
		log.Fatalf("[FATAL] config error: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rdbClient, err := redis.ConnectToRedisURL(cfg.RedisURL, cfg.RedisConnTimeout)
	if err != nil {
		log.Fatalf("[FATAL] redis connection: %v", err)
	}
	err = redis.WatchStreams(ctx, rdbClient, "scan:jobs", "scanners", consumerName, &cfg)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("[FATAL] watch streams: %v", err)
	}
}
