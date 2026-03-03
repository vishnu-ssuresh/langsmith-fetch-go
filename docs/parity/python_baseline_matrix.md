# Python Baseline Parity Matrix

Baseline snapshot:

- Python repo: `../langsmith-fetch`
- Python commit: `52fe117`
- Snapshot UTC: `2026-03-03T07:14:05Z`

Status legend:

- `PARITY`: behavior matches Python baseline.
- `PARITY+`: behavior matches and adds safer handling.
- `DELTA`: intentional behavior change from Python baseline.

| ID | Behavior | Python baseline evidence | Go evidence | Status |
|---|---|---|---|---|
| P1 | `trace` fetch uses `GET /runs/{id}?include_messages=true` | `src/langsmith_cli/fetchers.py` (`fetch_trace`) | `internal/langsmith/runs/accessor.go` (`GetRun`), `internal/cmd/integration_test.go` (`TestExecute_Trace_Integration`) | PARITY |
| P2 | Trace message extraction order: `messages` first, then `outputs.messages` | `src/langsmith_cli/fetchers.py` (`fetch_trace`) | `internal/core/single/trace.go` (`GetMessages`) | PARITY |
| P3 | `thread` fetch uses `GET /runs/threads/{id}` with `select=all_messages` and `session_id` | `src/langsmith_cli/fetchers.py` (`fetch_thread`) and `tests/test_fetchers.py` (`test_fetch_thread_params_sent`) | `internal/langsmith/threads/accessor.go`, `internal/cmd/integration_test.go` (`TestExecute_Thread_Integration`) | PARITY |
| P4 | Thread parsing uses newline-separated JSON from `previews.all_messages` | `src/langsmith_cli/fetchers.py` (`fetch_thread`) and `tests/test_fetchers.py` (`test_fetch_thread_parses_multiline_json`) | `internal/langsmith/threads/accessor.go`, `internal/langsmith/threads/accessor_test.go` | PARITY |
| P5 | `threads` list starts from root runs query and then fetches each unique thread | `src/langsmith_cli/fetchers.py` (`fetch_recent_threads`) and `tests/test_cli.py` (`TestThreadsCommand.test_threads_default_limit`) | `internal/core/threads/service.go`, `internal/cmd/integration_test.go` (`TestExecute_Threads_Integration`) | PARITY |
| P6 | `traces` list starts from root runs query | `tests/test_cli.py` (`TestTracesCommand.test_traces_default_no_metadata`) | `internal/langsmith/runs/accessor.go` (`QueryRoot`), `internal/cmd/integration_test.go` (`TestExecute_Traces_Integration`) | PARITY |
| P7 | `--include-metadata` and `--include-feedback` enrich trace output | `tests/test_cli.py` (`test_trace_with_metadata_flag`, traces metadata tests) | `internal/cmd/trace.go`, `internal/cmd/traces.go`, `internal/core/traces/service.go`, `internal/cmd/traces_test.go` | PARITY |
| P8 | `--last-n-minutes` and `--since` are mutually exclusive | Python fetchers support both filters (CLI validates separately) | `internal/cmd/time_filter.go`, `internal/cmd/time_filter_test.go` | PARITY+ |
| P9 | API key masking in `config show` | `tests/test_config.py` (`test_show_with_api_key_masked`) | `internal/cmd/config_show.go` (`maskSecret`), `internal/cmd/config_show_test.go` | PARITY |
| P10 | HTTP status mapping and retries (`429`, `5xx`) | Python uses `requests.raise_for_status` and does not provide typed errors/retry policy in fetchers | `internal/langsmith/statuserr/statuserr.go`, accessor typed-error tests, `internal/cmd/integration_test.go` retry tests | PARITY+ |
| P11 | Project-name based UUID resolution exists | `src/langsmith_cli/config.py` (`get_project_uuid`, `_lookup_project_uuid_by_name`) | `internal/langsmith/projects/accessor.go`, `internal/cmd/project.go` | PARITY |
| D1 | Config precedence for API key (`env` vs `config`) | Python `get_api_key`: config first, then env | Go `internal/config/file.go`: env-over-file precedence | DELTA |
| D2 | Directory-mode CLI shape for bulk commands | Python uses positional output directory (`threads <dir>`, `traces <dir>`) | Go uses optional `--dir` flag (and supports stdout mode) | DELTA |

## Captured Fixtures

- `fixtures/sample_trace_response.json`
- `fixtures/sample_thread_response.json`
- `fixtures/runs_query_threads_response.json`
- `fixtures/runs_query_traces_response.json`
- `fixtures/error_not_found_response.json`
