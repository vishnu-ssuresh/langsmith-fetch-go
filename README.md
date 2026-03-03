# langsmith-fetch-go

[![CLI CI](https://github.com/vishnu-ssuresh/langsmith-fetch-go/actions/workflows/cli-ci.yml/badge.svg)](https://github.com/vishnu-ssuresh/langsmith-fetch-go/actions/workflows/cli-ci.yml)

Go CLI for fetching LangSmith traces and threads with deterministic output, strong tests, and SDK-backed auth/transport behavior.

Originally built as a personal project to learn Go while shipping a production-style CLI.

## Highlights

- Focused CLI commands: `trace`, `traces`, `thread`, `threads`, `config`
- Shared auth and HTTP transport via `langsmith-sdk/go`
- Supports LangSmith cloud and self-hosted endpoints
- Output modes: `pretty`, `json`, `raw`
- File and directory export modes with filename templating
- Concurrent bulk fetching with bounded workers and stable ordering
- Integration + unit coverage for request shape, parsing, retries, and error mapping

## Table of Contents

- [Project Status](#project-status)
- [Requirements](#requirements)
- [Repository Layout](#repository-layout)
- [Quick Start](#quick-start)
- [Setup Notes](#setup-notes)
- [Authentication And Configuration](#authentication-and-configuration)
- [Command Reference](#command-reference)
- [Output And File Modes](#output-and-file-modes)
- [Self-Hosted LangSmith](#self-hosted-langsmith)
- [Error Handling And Retries](#error-handling-and-retries)
- [Parity Tracking](#parity-tracking)
- [Development](#development)
- [CI Pipeline](#ci-pipeline)
- [Troubleshooting](#troubleshooting)

## Project Status

`langsmith-fetch-go` is an actively developed Go migration of the Python `langsmith-fetch` tool.

Current capabilities:

- End-to-end CLI flows for `trace`, `traces`, `thread`, `threads`, and `config show`
- Config file read/write (`~/.langsmith-cli/config.yaml`)
- Project UUID resolution by project name
- Metadata + feedback enrichment for traces
- Retry-aware transport and typed status-error mapping

## Requirements

- Go `1.22+`
- A LangSmith API key
- Access to a compatible `langsmith-sdk/go` checkout (see Quick Start)

## Repository Layout

```text
cmd/langsmith-fetch/              # CLI entrypoint
internal/cmd/                     # command parsing + wiring
internal/core/                    # single + bulk orchestration
internal/langsmith/               # domain accessors (runs/threads/feedback/projects)
internal/config/                  # env + file config loading/saving
internal/output/                  # pretty/json/raw rendering
internal/files/                   # safe filename + file IO helpers
docs/parity/                      # Python parity matrix
```

## Quick Start

### 1) Clone fetch-go and sdk side-by-side

This repo currently uses a local Go module replace:

```go
replace langsmith-sdk/go => ../langsmith-sdk/go
```

So keep both repos as siblings:

```bash
cd /path/to/workspace
git clone https://github.com/vishnu-ssuresh/langsmith-fetch-go.git
git clone https://github.com/vishnu-ssuresh/langsmith-sdk.git
```

### 2) Build the CLI

```bash
cd langsmith-fetch-go
go build -o ./bin/langsmith-fetch ./cmd/langsmith-fetch
```

### 3) Configure credentials

Using environment variables:

```bash
export LANGSMITH_API_KEY="lsv2_..."
```

Or using config:

```bash
./bin/langsmith-fetch config set api-key "lsv2_..."
```

### 4) Run a command

```bash
./bin/langsmith-fetch trace <trace-id> --format pretty
```

## Setup Notes

### Using a custom `langsmith-sdk/go` checkout

This repo depends on `langsmith-sdk/go` via a local replace.
Default in `go.mod`:

```go
replace langsmith-sdk/go => ../langsmith-sdk/go
```

If your SDK checkout is elsewhere (or you are testing a custom SDK branch), update the replace target:

```bash
go mod edit -replace=langsmith-sdk/go=/absolute/path/to/langsmith-sdk/go
go mod tidy
```

Verify the module resolution path:

```bash
go list -m -f '{{.Path}} => {{.Dir}}' langsmith-sdk/go
```

### Environment compatibility notes

`langsmith-fetch-go` reads:

- `LANGSMITH_ENDPOINT` (or `LANGCHAIN_ENDPOINT`)

Some repos use `LANGSMITH_HOST_API_URL` instead. If your env file uses that key, map it explicitly:

```bash
export LANGSMITH_ENDPOINT="${LANGSMITH_HOST_API_URL}"
```

If you want to force default cloud behavior, unset endpoint overrides:

```bash
unset LANGSMITH_ENDPOINT LANGCHAIN_ENDPOINT
```

### Smoke test with an existing `.env` file

Example flow when loading env vars from a sibling repo:

```bash
set -a
source ../ai-sdr/.env
set +a

# required when project is not in the env file
export LANGSMITH_PROJECT="gtm-agent"

# optional: use endpoint from host-url style env var
# export LANGSMITH_ENDPOINT="${LANGSMITH_HOST_API_URL}"

go run ./cmd/langsmith-fetch traces --limit 3 --format pretty --no-progress
```

## Authentication And Configuration

### Precedence

- Command flags (for fields that have flags, such as `--project-uuid` or `--format`)
- Environment variables
- Config file (`~/.langsmith-cli/config.yaml`)

For API key specifically, runtime resolution is environment first, then config file.

### Supported environment variables

Primary:

- `LANGSMITH_API_KEY`
- `LANGSMITH_ENDPOINT`
- `LANGSMITH_WORKSPACE_ID`
- `LANGSMITH_PROJECT`
- `LANGSMITH_PROJECT_UUID`

Compat aliases:

- `LANGCHAIN_API_KEY`
- `LANGCHAIN_ENDPOINT`
- `LANGCHAIN_WORKSPACE_ID`
- `LANGCHAIN_PROJECT`
- `LANGCHAIN_PROJECT_UUID`

### Config commands

```bash
langsmith-fetch config show
langsmith-fetch config set api-key <value>
langsmith-fetch config set workspace-id <value>
langsmith-fetch config set endpoint <value>
langsmith-fetch config set project-uuid <value>
langsmith-fetch config set project-name <value>
langsmith-fetch config set default-format <pretty|json|raw>
```

Config file path:

- `~/.langsmith-cli/config.yaml`

## Command Reference

All commands:

- `trace` fetch one trace by ID
- `traces` list traces
- `thread` fetch one thread by ID
- `threads` list threads
- `config` show or set configuration

### `trace`

```bash
langsmith-fetch trace <trace-id> [flags]
langsmith-fetch trace --trace-id <trace-id> [flags]
```

Flags:

- `--format pretty|json|raw`
- `--file <path>`
- `--include-metadata`
- `--include-feedback`

Examples:

```bash
langsmith-fetch trace 3b0b15fe-... --format json
langsmith-fetch trace 3b0b15fe-... --include-metadata --include-feedback
langsmith-fetch trace 3b0b15fe-... --file ./out/trace.json
```

### `traces`

```bash
langsmith-fetch traces [flags]
```

Flags:

- `--project-id <uuid>` (alias: `--project-uuid`)
- `--limit <n>` (alias: `-n`)
- `--last-n-minutes <n>`
- `--since <RFC3339>`
- `--max-concurrent <n>`
- `--no-progress`
- `--include-metadata`
- `--include-feedback`
- `--format pretty|json|raw`
- `--file <path>`
- `--dir <path>`
- `--filename-pattern <pattern>` (default: `{trace_id}.json`)

Rules:

- `--last-n-minutes` and `--since` are mutually exclusive
- `--file` and `--dir` are mutually exclusive

Examples:

```bash
langsmith-fetch traces --project-uuid <uuid> --limit 20 --format pretty
langsmith-fetch traces --project-uuid <uuid> --since 2025-12-09T10:00:00Z --format json
langsmith-fetch traces --project-uuid <uuid> --dir ./out/traces --filename-pattern "{index}_{trace_id}"
```

### `thread`

```bash
langsmith-fetch thread <thread-id> [flags]
langsmith-fetch thread --thread-id <thread-id> [flags]
```

Flags:

- `--project-id <uuid>` (alias: `--project-uuid`)
- `--format pretty|json|raw`
- `--file <path>`

Examples:

```bash
langsmith-fetch thread test-thread --project-uuid <uuid>
langsmith-fetch thread test-thread --project-uuid <uuid> --format json
```

### `threads`

```bash
langsmith-fetch threads [flags]
```

Flags:

- `--project-id <uuid>` (alias: `--project-uuid`)
- `--limit <n>` (alias: `-n`)
- `--last-n-minutes <n>`
- `--since <RFC3339>`
- `--max-concurrent <n>`
- `--no-progress`
- `--format pretty|json|raw`
- `--file <path>`
- `--dir <path>`
- `--filename-pattern <pattern>` (default: `{thread_id}.json`)

Rules:

- `--last-n-minutes` and `--since` are mutually exclusive
- `--file` and `--dir` are mutually exclusive

Examples:

```bash
langsmith-fetch threads --project-uuid <uuid> --limit 10 --format pretty
langsmith-fetch threads --project-uuid <uuid> --dir ./out/threads --filename-pattern "thread_{index}"
```

## Output And File Modes

Output formats:

- `pretty`: human-readable, line-oriented
- `json`: indented JSON
- `raw`: compact JSON

File writing:

- `--file`: write one aggregate output file
- `--dir`: write one file per item (bulk commands)
- `--filename-pattern` placeholders:
  - `{id}`
  - `{trace_id}`
  - `{thread_id}`
  - `{index}`

## Self-Hosted LangSmith

Set endpoint via env or config:

```bash
export LANGSMITH_ENDPOINT="https://smith.example.com"
langsmith-fetch trace <trace-id>
```

or

```bash
langsmith-fetch config set endpoint "https://smith.example.com"
```

## Error Handling And Retries

- Transport retries retryable statuses (`429`, `5xx`) and transient network failures per SDK policy.
- Domain accessors map status codes to typed SDK errors:
  - `ErrUnauthorized` (`401`)
  - `ErrForbidden` (`403`)
  - `ErrNotFound` (`404`)
  - `ErrRateLimited` (`429`)
  - `ErrTransient` (`5xx`)

## Parity Tracking

Python migration parity is tracked in:

- [`docs/parity/python_baseline_matrix.md`](docs/parity/python_baseline_matrix.md)

## Development

Run tests:

```bash
go test ./...
go test -race ./...
```

Run vet:

```bash
go vet ./...
```

Build:

```bash
go build -trimpath ./cmd/langsmith-fetch
```

## CI Pipeline

GitHub Actions workflow:

- [`.github/workflows/cli-ci.yml`](.github/workflows/cli-ci.yml)

Gates:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/langsmith-fetch`
- Cross-platform binary builds:
  - linux (`amd64`, `arm64`)
  - darwin (`amd64`, `arm64`)
  - windows (`amd64`, `arm64`)

## Troubleshooting

### `LANGSMITH_API_KEY ... is required`

Set an API key via env or config:

```bash
export LANGSMITH_API_KEY="lsv2_..."
# or
langsmith-fetch config set api-key "lsv2_..."
```

### `--project-id is required`

For thread-based commands, provide one of:

- `--project-uuid <uuid>`
- `LANGSMITH_PROJECT_UUID`
- `LANGSMITH_PROJECT` (name lookup path)

### Time filter validation errors

Use either `--since` or `--last-n-minutes`, not both.

### Local module replace issues

If build fails resolving `langsmith-sdk/go`, ensure the sibling checkout exists:

```text
../langsmith-sdk/go
```

Or repoint the replace target to your custom SDK checkout:

```bash
go mod edit -replace=langsmith-sdk/go=/absolute/path/to/langsmith-sdk/go
go mod tidy
```
