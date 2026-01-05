# Configuration

Configuration is loaded from environment variables with the `APP_` prefix.
The loader also attempts to read `.env`, `.env.<APP_ENV>`, and `.env.<GO_ENV>`
(if present) using `godotenv` with overwrite semantics.

Duration values use Go's `time.ParseDuration` format (examples: `100ms`, `1s`,
`5m`, `1h`).

## Required variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_REDIS_URL` | none | Redis URL for streams and pub/sub. |
| `APP_GITHUB_PRIVATE_KEY` | none | GitHub App private key (PEM). Literal `\n` is converted to newlines. |
| `APP_GITHUB_CLIENT_ID` | none | GitHub App client ID. |
| `APP_OPENAI_API_KEY` | none | OpenAI API key. |

## App behavior

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_ENV` | `prod` | Environment name; used for `.env` selection. |
| `APP_LOG_LEVEL` | `info` | Log level string (not currently used to filter logs). |
| `APP_SHUTDOWN_GRACE` | `15s` | Intended shutdown grace period (not currently used). |

## Performance and concurrency

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_WORKER_COUNT` | `5` | Number of concurrent job workers. |
| `APP_GITHUB_CONCURRENCY` | `10` | Concurrent GitHub commit stat fetches per job. |
| `APP_GITHUB_RATE_LIMIT` | `80` | GitHub API requests per minute. |
| `APP_OPENAI_RATE_LIMIT` | `50` | OpenAI requests per minute (defined but not enforced in code). |
| `APP_CACHE_SIZE` | `1000` | In-memory LRU size for commit stats. |
| `APP_MESSAGE_TIMEOUT` | `5m` | Per-job processing timeout. |

## Redis and IO tuning

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_REDIS_STREAM_MAX_LEN` | `1000` | Approximate max length for `scan:results`. |
| `APP_REDIS_BLOCK_TIMEOUT` | `1s` | XREADGROUP block time. |
| `APP_REDIS_BATCH_SIZE` | `10` | Max messages per XREADGROUP call. |
| `APP_BACKOFF_MIN` | `100ms` | Initial backoff on Redis read errors. |
| `APP_BACKOFF_MAX` | `3s` | Maximum backoff on Redis read errors. |
| `APP_HTTP_CLIENT_TIMEOUT` | `30s` | GitHub HTTP client timeout. |
| `APP_REDIS_CONN_TIMEOUT` | `3s` | Redis connection ping timeout. |
| `APP_MAX_RETRIES` | `3` | Max retries for transient job failures. |
