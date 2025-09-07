package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectToRedis(addr, password string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	if err := rdb.
		XGroupCreateMkStream(ctx, "scan:results", "workers", "$").
		Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		_ = rdb.Close()
		return nil, fmt.Errorf("xgroup create scan:results/workers: %w", err)
	}

	return rdb, nil
}

func WatchStreams(ctx context.Context, rdb *redis.Client, stream, group, consumer string) error {
	backoff := 100 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    int64(10),
			Block:    5 * time.Second,
			NoAck:    false,
		}).Result()
		switch {
		case err == redis.Nil:
			continue
		case err != nil:
			log.Printf("Error reading from stream: %v", err)
			select {
			case <-time.After(backoff):
				if backoff < 3*time.Second {
					backoff *= 2
				}
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			backoff = 100 * time.Millisecond
		}
		for _, incomingStream := range res {
			for _, msg := range incomingStream.Messages {
				if err := handleAndParseMessageEvent(msg); err != nil {
					log.Printf("Error handling message %s: %v", msg.ID, err)
					continue
				}
				if err := rdb.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
					log.Printf("Error acknowledging message %s: %v", msg.ID, err)
				}
			}
		}

	}
}

func handleAndParseMessageEvent(msg redis.XMessage) error {
	log.Printf("Processing message ID: %s, Values: %v", msg.ID, msg.Values)
	return nil
}
