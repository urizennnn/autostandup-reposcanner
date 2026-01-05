# Message Contracts

This service uses Redis Streams for inputs and outputs. The input stream and
consumer group are fixed in code:

- Input stream: `scan:jobs`
- Input consumer group: `scanners`
- Output stream: `scan:results`
- Output consumer group: `workers` (created automatically on startup)

## Input: scan:jobs

Each stream entry must include a `queuePayload` field. The value can be a JSON
string, a raw JSON blob, or a Redis hash that can be marshaled to JSON.

### queuePayload schema

```json
{
  "owner": "org-or-user",
  "repo": "repo-name",
  "from": "2024-03-01T00:00:00Z",
  "to": "2024-03-31T23:59:59Z",
  "installation_id": 123456,
  "branch": "main",
  "format": "technical"
}
```

Field details:

- `owner` (string): GitHub org or user.
- `repo` (string): GitHub repository name.
- `from` (string): RFC3339 timestamp (inclusive).
- `to` (string): RFC3339 timestamp (inclusive).
- `installation_id` (number): GitHub App installation ID for the repo.
- `branch` (string): Branch name or SHA. Empty uses the default branch.
- `format` (string): Output format. Accepted values are `technical`,
  `mildly-technical`, or `layman` (case-insensitive, hyphens or underscores are
  allowed).

### Example XADD

```bash
XADD scan:jobs * queuePayload '{"owner":"acme","repo":"payments","from":"2024-03-01T00:00:00Z","to":"2024-03-31T23:59:59Z","installation_id":123456,"branch":"main","format":"technical"}'
```

## Output: scan:results

Each output entry includes:

- `payload`: JSON string of the standup payload.
- `repo`: repository identifier from the payload (usually `owner/repo`).
- `from`: RFC3339 `from` timestamp (from input).
- `to`: RFC3339 `to` timestamp (from input).
- `format`: the format requested by the job.

### Standup payload structure (technical format example)

```json
{
  "repo": "acme/payments",
  "window": {
    "since": "2024-03-01",
    "until": "2024-03-31"
  },
  "technical": {
    "header": "Daily standup header",
    "whatWorkedOn": ["bullet 1", "bullet 2"],
    "filesChanged": { "files": 12, "additions": 340, "deletions": 120 },
    "commits": ["feat: add ...", "fix: correct ..."]
  },
  "contributors": [
    { "name": "Jane Doe", "email": "jane@example.com", "commits": 5 }
  ]
}
```

Notes:

- Only one of `technical`, `mildlyTechnical`, or `layman` is populated per job,
  depending on `format`.
- If the time window returns no commits, the service currently publishes a
  zero-value payload rather than skipping the result.
