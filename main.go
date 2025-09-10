package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/parser/github"
	"github.com/urizennnn/autostandup-reposcanner/redis"
)

var consumerName = fmt.Sprintf("%s-%d", "auto-standup-repo-scanner-1", os.Getpid())

func main() {
	fmt.Println("Starting Repo Scanner Service...")
	cfg, err := config.NewLoader("APP").Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	_ = cfg

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rdb, err := redis.ConnectToRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Redis connection error: %v", err)
	}
	client := github.CreateGithubClient([]byte(cfg.GithubPrivateKey), "Iv23liz8OgaUIWul4HBe", 84821041)
	sha := github.ListCommits(client)
	for _, s := range sha {
		github.GetCommit(client, s)
	}
	err = redis.WatchStreams(ctx, rdb, "scan:jobs", "scanners", consumerName)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("WatchStreams %v", err)
	}
}
