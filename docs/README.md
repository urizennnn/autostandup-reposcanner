# AutoStandup Repo Scanner

AutoStandup Repo Scanner is a long-running worker that scans GitHub repositories
and publishes structured standup summaries. It is designed to sit behind a Redis
stream, consume scan jobs, and emit results to another stream for downstream
services.

## Responsibilities

- Consume jobs from Redis stream `scan:jobs` using consumer group `scanners`.
- Fetch commits from GitHub using a GitHub App installation token.
- Summarize commit activity with OpenAI into a structured standup payload.
- Publish results to Redis stream `scan:results`.

This service does not create jobs or deliver results to end users; it only scans
and summarizes.

## High-level flow

1. Join Redis consumer group `scanners` and block on `scan:jobs`.
2. Parse each job's `queuePayload` JSON for repo, branch, time window, and
   installation ID.
3. Query GitHub for commits in the time window and enrich each commit with file
   stats.
4. Call OpenAI to produce a structured standup payload.
5. Write the payload to `scan:results`.

## Running locally

1. Set required environment variables (see Configuration).
2. Ensure the Redis consumer group `scanners` exists for `scan:jobs`.
3. Start the worker:

```bash
go run .
```

## Dependencies

- Redis with Streams enabled.
- GitHub App credentials and an installation on the target repo(s).
- OpenAI API key.

For full details, see:

- [Configuration](Configuration.md)
- [Message Contracts](Message-Contracts.md)
- [Operations](Operations.md)
