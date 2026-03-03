# Python vs Go Benchmark Harness

This directory contains a reusable benchmark harness for comparing:

- Python `langsmith-fetch` (`../langsmith-fetch`)
- Go `langsmith-fetch-go` (this repo)

against the `gtm-agent` LangSmith project.

## What It Measures

- Per-command latency (`median`, `p90`, `p95`, etc.)
- Throughput (`items/s`, `messages/s` where applicable)
- Concurrency scaling efficiency for bulk commands
- Output parity classification (`compatible`, `compatible_with_normalization`, `behavioral_mismatch`)

## Default Scope

Commands:

- `config show`
- `trace`
- `traces`
- `thread`
- `threads`

Profiles include:

- Default single-item flows
- Enriched trace flow (`--include-metadata --include-feedback`)
- Bulk concurrency sweeps for `traces` and `threads` across `1,2,5,10,20`

## Usage

Run from repository root:

```bash
python scripts/bench/benchmark_py_vs_go.py \
  --source-env ../ai-sdr/.env \
  --project-uuid 0b60adb6-945a-4d38-9f8d-cff6e4437e4a
```

Optional quick smoke run:

```bash
python scripts/bench/benchmark_py_vs_go.py \
  --source-env ../ai-sdr/.env \
  --profiles core \
  --warmup-runs 0 \
  --measured-runs 1 \
  --bulk-limit 5 \
  --concurrency-levels 1 \
  --timeout-seconds 120
```

Full command + concurrency benchmark (recommended):

```bash
python scripts/bench/benchmark_py_vs_go.py \
  --source-env ../ai-sdr/.env \
  --project-uuid 0b60adb6-945a-4d38-9f8d-cff6e4437e4a \
  --warmup-runs 1 \
  --measured-runs 5 \
  --bulk-limit 50 \
  --concurrency-levels 1,2,5,10,20 \
  --profiles all \
  --timeout-seconds 300
```

Notes:

- `--profiles` accepts profile names, prefixes (`traces_*`), and groups: `core`, `bulk`, `all`.
- `--skip-profiles` excludes profiles after include selection.
- `--timeout-seconds` bounds each command execution to avoid stuck runs.

## Artifacts

Each run writes to:

```text
docs/benchmarks/gtm-agent/<timestamp>/
```

Artifacts:

- `environment.json`
- `runs.csv`
- `summary.json`
- `summary.md`
