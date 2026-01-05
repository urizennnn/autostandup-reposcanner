# Operations

This section describes runtime behavior, concurrency, and failure handling.

## Startup and stream setup

- On startup, the service connects to Redis and creates the output consumer
  group `workers` on `scan:results` (if it does not exist).
- The input consumer group `scanners` for `scan:jobs` is not created by this
  service and must exist before startup.

## Worker model

- A single reader goroutine performs `XREADGROUP` on `scan:jobs` and pushes
  messages into a buffered channel.
- `APP_WORKER_COUNT` worker goroutines process messages concurrently.
- Each message has its own context with `APP_MESSAGE_TIMEOUT`.

## Message lifecycle

- Each message is acknowledged (`XACK`) after processing, even if processing
  fails.
- Job processing retries up to `APP_MAX_RETRIES` for transient failures
  (string match for "rate limit", "timeout", "connection", or "temporary").
- Retry backoff is linear per attempt (1s, 2s, 3s).

## Redis read backoff

- If `XREADGROUP` returns an error, the reader backs off exponentially from
  `APP_BACKOFF_MIN` up to `APP_BACKOFF_MAX`, then resumes.

## GitHub access

- Commit lists are fetched with `Repositories.ListCommits` using `since`, `until`,
  and `branch`.
- Per-commit file stats are fetched concurrently, limited by
  `APP_GITHUB_CONCURRENCY`.
- A local rate limiter enforces `APP_GITHUB_RATE_LIMIT` requests per minute.

## OpenAI summarization

- Commit summaries are generated with the OpenAI GPT-4o model using function
  calling for a strict JSON schema.
- `APP_OPENAI_RATE_LIMIT` is defined but not currently enforced in code.

## Caching

- Commit stats are cached in an in-memory LRU cache with size
  `APP_CACHE_SIZE` and a 1-hour TTL.
- Cache is process-local and resets on restart.

## Output stream trimming

- Results are appended to `scan:results` with approximate trimming to
  `APP_REDIS_STREAM_MAX_LEN`.

## Logging

- The service uses `log.Printf` and logs startup, job processing, and
  publish events with basic metadata.
