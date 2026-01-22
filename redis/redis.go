package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/urizennnn/autostandup-reposcanner/ai"
	"github.com/urizennnn/autostandup-reposcanner/config"
	"github.com/urizennnn/autostandup-reposcanner/parser/github"
	"golang.org/x/sync/errgroup"
)

func ConnectToRedisURL(url string, connTimeout time.Duration) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
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

func WatchStreams(ctx context.Context, rdb *redis.Client, stream, group, consumer string, cfg *config.Config) error {
	log.Printf("[INFO] watching stream=%s group=%s consumer=%s workers=%d", stream, group, consumer, cfg.WorkerCount)

	jobs := make(chan redis.XMessage, cfg.WorkerCount*2)
	g, ctx := errgroup.WithContext(ctx)

	for i := 0; i < cfg.WorkerCount; i++ {
		workerID := i
		g.Go(func() error {
			for msg := range jobs {
				if err := processMessage(ctx, msg, rdb, stream, group, cfg); err != nil {
					log.Printf("[ERROR] worker %d processing %s: %v", workerID, msg.ID, err)
				}
			}
			return nil
		})
	}

	g.Go(func() error {
		defer close(jobs)
		backoff := cfg.BackoffMin
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			res, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumer,
				Streams:  []string{stream, ">"},
				Count:    int64(cfg.RedisBatchSize),
				Block:    cfg.RedisBlockTimeout,
				NoAck:    false,
			}).Result()

			switch {
			case err == redis.Nil:
				continue
			case err != nil:
				log.Printf("[ERROR] reading from stream: %v", err)
				select {
				case <-time.After(backoff):
					if backoff < cfg.BackoffMax {
						backoff *= 2
					}
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			default:
				backoff = cfg.BackoffMin
			}

			for _, incomingStream := range res {
				for _, msg := range incomingStream.Messages {
					select {
					case jobs <- msg:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	})

	return g.Wait()
}

func processMessage(ctx context.Context, msg redis.XMessage, rdb *redis.Client, stream, group string, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.MessageTimeout)
	defer cancel()

	defer func() {
		if err := rdb.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
			log.Printf("[ERROR] acking message %s: %v", msg.ID, err)
		}
	}()

	maxRetries := cfg.MaxRetries
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := handleAndParseMessageEvent(ctx, msg, rdb, cfg)
		if err == nil {
			return nil
		}

		if isTransient(err) && attempt < maxRetries {
			backoff := time.Duration(attempt) * time.Second
			log.Printf("[WARN] retry %d/%d for %s after %v: %v", attempt, maxRetries, msg.ID, backoff, err)
			time.Sleep(backoff)
			continue
		}

		log.Printf("[ERROR] failed %s: %v", msg.ID, err)
		return err
	}
	return nil
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "temporary")
}

func handleAndParseMessageEvent(ctx context.Context, msg redis.XMessage, rdb *redis.Client, cfg *config.Config) error {
	log.Printf("[INFO] processing message %s", msg.ID)

	payload, err := extractAndValidatePayload(msg)
	isTestStandupFlag := contextKey("isTestStandup")
	ctx = context.WithValue(ctx, isTestStandupFlag, payload.IsTestStandup)
	if payload.IsTestStandup {
		log.Printf("[INFO] test standup received repo=%s/%s", payload.Owner, payload.Repo)
	}
	if err != nil {
		return err
	}

	result, err := processRepoScan(ctx, payload, cfg)
	if err != nil {
		return err
	}

	return publishResult(ctx, rdb, result, payload, cfg)
}

func extractAndValidatePayload(msg redis.XMessage) (QueueMessage, error) {
	payload, err := extractQueuePayload(msg)
	if err != nil {
		return QueueMessage{}, fmt.Errorf("extracting queue payload: %w", err)
	}
	return payload, nil
}

func processRepoScan(ctx context.Context, payload QueueMessage, cfg *config.Config) (ai.SummarizeResult, error) {
	githubPrivateKey, err := config.FetchSecretByName("APP_GITHUB_PRIVATE_KEY")
	if err != nil {
		return ai.SummarizeResult{}, fmt.Errorf("fetching github private key: %w", err)
	}

	githubClientID, err := config.FetchSecretByName("APP_GITHUB_CLIENT_ID")
	if err != nil {
		return ai.SummarizeResult{}, fmt.Errorf("fetching github client id: %w", err)
	}

	client, err := github.NewClient(cfg, []byte(githubPrivateKey), githubClientID, payload.InstallationID)
	if err != nil {
		return ai.SummarizeResult{}, fmt.Errorf("creating github client: %w", err)
	}

	return client.ListCommits(ctx, payload.Owner, payload.Repo, payload.Branch, payload.Format, payload.From, payload.To)
}

func publishResult(ctx context.Context, rdb *redis.Client, result ai.SummarizeResult, payload QueueMessage, cfg *config.Config) error {
	isTestStandupFlag := ctx.Value(contextKey("isTestStandup")).(bool)

	if isTestStandupFlag {
		testPayload := map[string]any{
			"payload":       result.Payload,
			"details":       result.Details,
			"isTestStandup": true,
		}
		testPayloadBytes, err := json.Marshal(testPayload)
		if err != nil {
			return fmt.Errorf("marshal test payload: %w", err)
		}
		id, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream:     "scan:results",
			MaxLen:     int64(cfg.RedisStreamMaxLen),
			Approx:     true,
			ID:         "*",
			NoMkStream: false,
			Values: map[string]any{
				"payload":       string(testPayloadBytes),
				"repo":          result.Payload.Repo,
				"from":          payload.From.UTC().Format(time.RFC3339),
				"to":            payload.To.UTC().Format(time.RFC3339),
				"format":        payload.Format,
				"isTestStandup": true,
			},
		}).Result()
		if err != nil {
			return fmt.Errorf("publish test to scan:results: %w", err)
		}
		log.Printf("[INFO] published test summary id=%s repo=%s/%s", id, payload.Owner, payload.Repo)
		return nil
	}

	payloadBytes, err := json.Marshal(result.Payload)
	if err != nil {
		return fmt.Errorf("marshal summary payload: %w", err)
	}

	id, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream:     "scan:results",
		MaxLen:     int64(cfg.RedisStreamMaxLen),
		Approx:     true,
		ID:         "*",
		NoMkStream: false,
		Values: map[string]any{
			"payload": string(payloadBytes),
			"repo":    result.Payload.Repo,
			"from":    payload.From.UTC().Format(time.RFC3339),
			"to":      payload.To.UTC().Format(time.RFC3339),
			"format":  payload.Format,
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("publish to scan:results: %w", err)
	}

	log.Printf("[INFO] published summary id=%s repo=%s/%s", id, payload.Owner, payload.Repo)
	return nil
}

func extractQueuePayload(msg redis.XMessage) (QueueMessage, error) {
	v, ok := msg.Values["queuePayload"]
	if !ok || v == nil {
		return QueueMessage{}, fmt.Errorf("missing queuePayload")
	}

	var b []byte
	switch t := v.(type) {
	case string:
		b = []byte(t)
	case []byte:
		b = t
	case map[string]any:
		var err error
		b, err = json.Marshal(t)
		if err != nil {
			return QueueMessage{}, err
		}
	default:
		return QueueMessage{}, fmt.Errorf("unexpected type for queuePayload: %T", v)
	}

	var p QueueMessage
	if err := json.Unmarshal(b, &p); err != nil {
		return QueueMessage{}, err
	}
	return p, nil
}
