package redis

import (
	"context"
	"encoding/json"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/urizennnn/autostandup-reposcanner/config"

	"github.com/urizennnn/autostandup-reposcanner/parser/github"
)

type QueueMessage struct {
	Owner          string    `json:"owner"`
	Repo           string    `json:"repo"`
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	InstallationID int64     `json:"installation_id"`
	Branch         string    `json:"branch"`
	Format         string    `json:"format"`
}

func ConnectToRedis(addr, password string, db int, useTLS bool) (*redis.Client, error) {
    opts := &redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    }
    if useTLS {
        opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
    }
    rdb := redis.NewClient(opts)

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

func ConnectToRedisURL(url string) (*redis.Client, error) {
    opts, err := redis.ParseURL(url)
    if err != nil {
        return nil, fmt.Errorf("invalid redis url: %w", err)
    }
    rdb := redis.NewClient(opts)

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
				if err := handleAndParseMessageEvent(msg, rdb); err != nil {
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

func handleAndParseMessageEvent(msg redis.XMessage, rdb *redis.Client) error {
    log.Printf("Processing message ID: %s, Values: %v", msg.ID, msg.Values)

	githubPrivateKey, err := config.FetchSecretByName("APP_GITHUB_PRIVATE_KEY")
	if err != nil {
		fmt.Printf("An Error occured when fetching openai api key %v", err)
	}

	githubClientID, err := config.FetchSecretByName("APP_GITHUB_CLIENT_ID")
	if err != nil {
		fmt.Printf("An Error occured when fetching github client id %v", err)
	}

	marshalPayload, err := extractQueuePayload(msg)
	if err != nil {
		fmt.Printf("Error extracting queue payload: %v", err)
	}
	owner := marshalPayload.Owner
	repo := marshalPayload.Repo
	from := marshalPayload.From
	to := marshalPayload.To
	installationID := marshalPayload.InstallationID
	branch := marshalPayload.Branch
	format := marshalPayload.Format

	fmt.Print(installationID)
	client := github.CreateGithubClient([]byte(githubPrivateKey), githubClientID, installationID)
    res, err := github.ListCommits(client, owner, repo, branch, format, from, to)
    if err != nil {
        return fmt.Errorf("error listing commits for %s/%s: %w", owner, repo, err)
    }

    // Marshal the standup payload and publish to scan:results as a JSON string field
    payloadBytes, err := json.Marshal(res)
    if err != nil {
        return fmt.Errorf("failed to marshal summary payload: %w", err)
    }

    id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
        Stream:     "scan:results",
        MaxLen:     1000,
        Approx:     true,
        ID:         "*",
        NoMkStream: false,
        Values: map[string]any{
            "payload": string(payloadBytes),
            "repo":    res.Repo,
            "from":    from.UTC().Format(time.RFC3339),
            "to":      to.UTC().Format(time.RFC3339),
            "format":  format,
        },
    }).Result()
    if err != nil {
        return fmt.Errorf("failed to publish to scan:results: %w", err)
    }
    log.Printf("Published summary to scan:results with ID %s for %s/%s", id, owner, repo)

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
