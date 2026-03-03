#!/usr/bin/env python3
"""Benchmark Python vs Go LangSmith fetch CLIs on live data.

This script benchmarks speed, throughput, and concurrency scaling for the
shared command surface:
  - config show
  - trace
  - traces
  - thread
  - threads

It produces timestamped artifacts under:
  docs/benchmarks/gtm-agent/<timestamp>/
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import hashlib
import json
import math
import os
import pathlib
import shutil
import statistics
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict
from dataclasses import dataclass
from dataclasses import field
from typing import Any


DEFAULT_PROJECT_UUID = "0b60adb6-945a-4d38-9f8d-cff6e4437e4a"


@dataclass
class RunRecord:
    timestamp_utc: str
    profile: str
    cli: str
    phase: str
    iteration: int
    command: str
    exit_code: int
    wall_ms: float
    stdout_bytes: int
    stderr_bytes: int
    stdout_sha256: str
    stderr_sha256: str
    json_valid: bool
    json_root_type: str
    json_top_level_keys: str
    item_count: int | None
    message_count: int | None
    throughput_items_per_s: float | None
    throughput_messages_per_s: float | None
    error: str = ""


@dataclass
class ProfileResult:
    profile: str
    cli: str
    runs: list[RunRecord] = field(default_factory=list)


def run_checked(
    cmd: list[str],
    env: dict[str, str],
    timeout_s: int = 600,
    cwd: str | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        env=env,
        cwd=cwd,
        text=True,
        capture_output=True,
        timeout=timeout_s,
        check=False,
    )


def sh(cmd: str, cwd: str | None = None) -> str:
    proc = subprocess.run(
        ["/bin/zsh", "-lc", cmd],
        cwd=cwd,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        raise RuntimeError(
            f"command failed ({proc.returncode}): {cmd}\nstdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return proc.stdout.strip()


def load_env_from_file_with_shell(env_file: pathlib.Path) -> dict[str, str]:
    if not env_file.exists():
        raise FileNotFoundError(f"env file does not exist: {env_file}")
    cmd = (
        f"set -a; source {shell_quote(str(env_file))}; set +a; "
        "python3 - <<'PY'\n"
        "import os, json\n"
        "print(json.dumps(dict(os.environ)))\n"
        "PY"
    )
    out = sh(cmd)
    return json.loads(out)


def shell_quote(value: str) -> str:
    return "'" + value.replace("'", "'\"'\"'") + "'"


def ensure_python_fresh(py_repo: pathlib.Path) -> dict[str, str]:
    local = sh("git rev-parse HEAD", cwd=str(py_repo))
    local_short = sh("git rev-parse --short HEAD", cwd=str(py_repo))
    branch = sh("git branch --show-current", cwd=str(py_repo))
    remote = sh("git ls-remote origin refs/heads/main | awk '{print $1}'", cwd=str(py_repo))
    return {
        "branch": branch,
        "local_head": local,
        "local_head_short": local_short,
        "origin_main_head": remote,
        "is_latest_main": str(local == remote).lower(),
    }


def git_head(repo: pathlib.Path) -> dict[str, str]:
    return {
        "path": str(repo),
        "branch": sh("git branch --show-current", cwd=str(repo)),
        "head": sh("git rev-parse HEAD", cwd=str(repo)),
        "head_short": sh("git rev-parse --short HEAD", cwd=str(repo)),
    }


def build_go_binary(go_bin: pathlib.Path, target: str, env: dict[str, str]) -> None:
    go_bin.parent.mkdir(parents=True, exist_ok=True)
    cmd = ["go", "build", "-trimpath", "-o", str(go_bin), target]
    proc = run_checked(cmd, env=env, cwd=None, timeout_s=1200)
    if proc.returncode != 0:
        raise RuntimeError(f"go build failed\nstdout:\n{proc.stdout}\nstderr:\n{proc.stderr}")


def parse_json_signature(stdout: str) -> tuple[bool, str, str, Any | None]:
    stripped = stdout.strip()
    if not stripped:
        return False, "none", "", None
    try:
        parsed = json.loads(stripped)
    except Exception:
        return False, "invalid", "", None

    root_type = type(parsed).__name__
    keys = ""
    if isinstance(parsed, dict):
        keys = ",".join(sorted(str(k) for k in parsed.keys()))
    elif isinstance(parsed, list) and parsed and isinstance(parsed[0], dict):
        keys = ",".join(sorted(str(k) for k in parsed[0].keys()))
    return True, root_type, keys, parsed


def count_items_and_messages(profile: str, parsed: Any | None) -> tuple[int | None, int | None]:
    if parsed is None:
        return None, None

    if profile == "config_show_default":
        return None, None

    if profile.startswith("trace_"):
        if isinstance(parsed, list):
            return 1, len(parsed)
        if isinstance(parsed, dict):
            messages = parsed.get("messages")
            if isinstance(messages, list):
                return 1, len(messages)
            return 1, None
        return 1, None

    if profile.startswith("thread_default"):
        if isinstance(parsed, list):
            return 1, len(parsed)
        if isinstance(parsed, dict):
            messages = parsed.get("messages")
            if isinstance(messages, list):
                return 1, len(messages)
            return 1, None
        return 1, None

    if profile.startswith("traces_"):
        if isinstance(parsed, list):
            return len(parsed), None
        if isinstance(parsed, dict) and isinstance(parsed.get("messages"), list):
            return 1, len(parsed["messages"])
        return None, None

    if profile.startswith("threads_"):
        if isinstance(parsed, list):
            items = len(parsed)
            message_count = 0
            has_messages = False
            for item in parsed:
                if isinstance(item, dict) and isinstance(item.get("messages"), list):
                    message_count += len(item["messages"])
                    has_messages = True
            return items, (message_count if has_messages else None)
        if isinstance(parsed, dict) and isinstance(parsed.get("messages"), list):
            return 1, len(parsed["messages"])
        return None, None

    return None, None


def p_quantile(values: list[float], q: float) -> float:
    if not values:
        return float("nan")
    if len(values) == 1:
        return values[0]
    try:
        return statistics.quantiles(values, n=100, method="inclusive")[int(q * 100) - 1]
    except Exception:
        idx = max(0, min(len(values) - 1, int(math.ceil(q * len(values))) - 1))
        return sorted(values)[idx]


def summarize_profile(records: list[RunRecord]) -> dict[str, Any]:
    measured = [r for r in records if r.phase == "measured"]
    if not measured:
        return {
            "runs": 0,
            "success_runs": 0,
            "success_rate": 0.0,
        }

    success = [r for r in measured if r.exit_code == 0]
    latencies = [r.wall_ms for r in success]
    items_tp = [r.throughput_items_per_s for r in success if r.throughput_items_per_s is not None]
    msg_tp = [r.throughput_messages_per_s for r in success if r.throughput_messages_per_s is not None]
    return {
        "runs": len(measured),
        "success_runs": len(success),
        "success_rate": (len(success) / len(measured)) if measured else 0.0,
        "latency_ms": {
            "min": min(latencies) if latencies else None,
            "max": max(latencies) if latencies else None,
            "mean": statistics.fmean(latencies) if latencies else None,
            "median": statistics.median(latencies) if latencies else None,
            "p90": p_quantile(latencies, 0.90) if latencies else None,
            "p95": p_quantile(latencies, 0.95) if latencies else None,
            "stddev": statistics.pstdev(latencies) if len(latencies) > 1 else (0.0 if latencies else None),
        },
        "throughput": {
            "items_per_s_median": statistics.median(items_tp) if items_tp else None,
            "messages_per_s_median": statistics.median(msg_tp) if msg_tp else None,
        },
        "json_signatures": sorted(
            {
                f"{r.json_root_type}|{r.json_top_level_keys}"
                for r in success
                if r.json_valid
            }
        ),
    }


def normalize_payload_for_parity(profile: str, parsed: Any) -> tuple[str, Any]:
    if profile.startswith("thread_default"):
        if isinstance(parsed, dict) and isinstance(parsed.get("messages"), list):
            return "compatible_with_normalization", parsed["messages"]
        if isinstance(parsed, list):
            return "compatible", parsed
    if profile.startswith("trace_"):
        if isinstance(parsed, dict) and isinstance(parsed.get("messages"), list):
            return "compatible_with_normalization", parsed["messages"]
        if isinstance(parsed, list):
            return "compatible", parsed
    if profile.startswith("traces_"):
        # Known behavioral mismatch between Python and Go list payload semantics.
        return "behavioral_mismatch", parsed
    if profile.startswith("threads_"):
        if isinstance(parsed, list):
            return "compatible", parsed
        if isinstance(parsed, dict) and isinstance(parsed.get("messages"), list):
            return "compatible_with_normalization", [{"thread_id": parsed.get("thread_id"), "messages": parsed.get("messages")}]
    return "compatible", parsed


def parity_classification(
    profile: str,
    py_records: list[RunRecord],
    go_records: list[RunRecord],
) -> dict[str, Any]:
    py_success = [r for r in py_records if r.phase == "measured" and r.exit_code == 0 and r.json_valid]
    go_success = [r for r in go_records if r.phase == "measured" and r.exit_code == 0 and r.json_valid]
    if not py_success or not go_success:
        return {
            "classification": "insufficient_data",
            "reason": "missing successful json outputs",
        }

    py_parsed = json.loads(py_success[0].command_output_cache) if hasattr(py_success[0], "command_output_cache") else None
    go_parsed = json.loads(go_success[0].command_output_cache) if hasattr(go_success[0], "command_output_cache") else None
    if py_parsed is None or go_parsed is None:
        return {
            "classification": "insufficient_data",
            "reason": "normalized output cache unavailable",
        }

    py_norm_class, py_norm = normalize_payload_for_parity(profile, py_parsed)
    go_norm_class, go_norm = normalize_payload_for_parity(profile, go_parsed)

    if profile.startswith("traces_"):
        return {
            "classification": "behavioral_mismatch",
            "reason": "python traces returns full payload while go traces returns summaries",
            "py_normalization": py_norm_class,
            "go_normalization": go_norm_class,
        }

    if type(py_norm).__name__ != type(go_norm).__name__:
        return {
            "classification": "behavioral_mismatch",
            "reason": f"normalized root type differs: py={type(py_norm).__name__} go={type(go_norm).__name__}",
            "py_normalization": py_norm_class,
            "go_normalization": go_norm_class,
        }

    if profile.startswith("trace_") or profile.startswith("thread_default"):
        py_len = len(py_norm) if isinstance(py_norm, list) else None
        go_len = len(go_norm) if isinstance(go_norm, list) else None
        if py_len is not None and go_len is not None and py_len == go_len:
            cls = "compatible_with_normalization" if (
                "compatible_with_normalization" in (py_norm_class, go_norm_class)
            ) else "compatible"
            return {
                "classification": cls,
                "reason": "message counts align after normalization",
                "py_messages": py_len,
                "go_messages": go_len,
                "py_normalization": py_norm_class,
                "go_normalization": go_norm_class,
            }
        return {
            "classification": "behavioral_mismatch",
            "reason": f"message counts differ: py={py_len} go={go_len}",
            "py_normalization": py_norm_class,
            "go_normalization": go_norm_class,
        }

    if profile.startswith("threads_"):
        py_items = len(py_norm) if isinstance(py_norm, list) else None
        go_items = len(go_norm) if isinstance(go_norm, list) else None
        if py_items == go_items:
            return {
                "classification": "compatible",
                "reason": "thread item counts align",
                "py_items": py_items,
                "go_items": go_items,
            }
        return {
            "classification": "behavioral_mismatch",
            "reason": f"thread item counts differ: py={py_items} go={go_items}",
        }

    return {
        "classification": "compatible",
        "reason": "no known parity divergence for profile",
    }


def parse_level_list(text: str) -> list[int]:
    vals = []
    for part in text.split(","):
        stripped = part.strip()
        if not stripped:
            continue
        vals.append(int(stripped))
    if not vals:
        raise ValueError("concurrency levels cannot be empty")
    return vals


def parse_list_arg(text: str) -> list[str]:
    parts: list[str] = []
    for piece in text.split(","):
        value = piece.strip()
        if value:
            parts.append(value)
    return parts


def resolve_profile_tokens(tokens: list[str], all_names: list[str]) -> set[str]:
    all_set = set(all_names)
    if not tokens:
        return all_set

    resolved: set[str] = set()
    for token in tokens:
        if token == "all":
            resolved.update(all_set)
            continue
        if token == "core":
            resolved.update(
                {
                    "config_show_default",
                    "trace_default",
                    "trace_enriched",
                    "thread_default",
                    "traces_default",
                    "threads_default",
                }
            )
            continue
        if token == "bulk":
            resolved.update({name for name in all_names if name.startswith("traces_bulk_c")})
            resolved.update({name for name in all_names if name.startswith("threads_bulk_c")})
            continue
        if token.endswith("*"):
            prefix = token[:-1]
            matched = {name for name in all_names if name.startswith(prefix)}
            if not matched:
                raise ValueError(f"profile token matched no profiles: {token}")
            resolved.update(matched)
            continue
        if token in all_set:
            resolved.add(token)
            continue
        raise ValueError(f"unknown profile token: {token}")

    unknown = sorted(resolved - all_set)
    if unknown:
        raise ValueError(f"profile resolution produced unknown profiles: {', '.join(unknown)}")
    return resolved


def filter_profiles(
    profiles: list[tuple[str, list[str], list[str]]],
    include_tokens: list[str],
    skip_tokens: list[str],
) -> list[tuple[str, list[str], list[str]]]:
    all_names = [name for name, _, _ in profiles]
    include_set = resolve_profile_tokens(include_tokens, all_names)
    skip_set = resolve_profile_tokens(skip_tokens, all_names) if skip_tokens else set()
    selected: list[tuple[str, list[str], list[str]]] = []
    for profile in profiles:
        name = profile[0]
        if name not in include_set:
            continue
        if name in skip_set:
            continue
        selected.append(profile)
    if not selected:
        raise ValueError("profile selection is empty after applying include/skip filters")
    return selected


def discover_ids(
    go_bin: pathlib.Path,
    env: dict[str, str],
    project_uuid: str,
    workdir: pathlib.Path,
) -> tuple[str, str]:
    temp_dir = pathlib.Path("/tmp/langsmith-fetch-go-bench-discovery")
    if temp_dir.exists():
        shutil.rmtree(temp_dir)
    temp_dir.mkdir(parents=True, exist_ok=True)

    trace_dir = temp_dir / "traces"
    trace_dir.mkdir(parents=True, exist_ok=True)

    trace_cmd = [
        str(go_bin),
        "traces",
        "--project-uuid",
        project_uuid,
        "--limit",
        "1",
        "--dir",
        str(trace_dir),
        "--no-progress",
        "--format",
        "raw",
    ]
    trace_proc = run_checked(trace_cmd, env=env, cwd=str(workdir))
    if trace_proc.returncode != 0:
        raise RuntimeError(f"failed to discover trace id: {trace_proc.stderr}")
    trace_files = sorted(trace_dir.glob("*.json"))
    if not trace_files:
        raise RuntimeError("failed to discover trace id: no files created")
    trace_id = trace_files[0].stem

    thread_id = discover_working_thread_id_via_api(env=env, project_uuid=project_uuid)

    return trace_id, thread_id


def discover_working_thread_id_via_api(env: dict[str, str], project_uuid: str) -> str:
    api_key = env.get("LANGSMITH_API_KEY") or env.get("LANGCHAIN_API_KEY")
    if not api_key:
        raise RuntimeError("cannot discover thread id: missing LANGSMITH_API_KEY/LANGCHAIN_API_KEY")

    endpoint = env.get("LANGSMITH_ENDPOINT") or env.get("LANGCHAIN_ENDPOINT") or "https://api.smith.langchain.com"
    endpoint = endpoint.rstrip("/")

    query_url = f"{endpoint}/runs/query"
    payload = {
        "session": [project_uuid],
        "is_root": True,
        "limit": 100,
    }
    req = urllib.request.Request(
        query_url,
        data=json.dumps(payload).encode("utf-8"),
        method="POST",
        headers={
            "content-type": "application/json",
            "x-api-key": api_key,
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = ""
        try:
            body = exc.read().decode("utf-8", "replace")
        except Exception:
            body = "<unreadable>"
        raise RuntimeError(
            f"cannot discover thread id: runs/query failed: HTTP {exc.code} {exc.reason}; body={body}"
        ) from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"cannot discover thread id: runs/query failed: {exc}") from exc

    runs = data.get("runs") if isinstance(data, dict) else None
    if not isinstance(runs, list):
        raise RuntimeError("cannot discover thread id: invalid runs/query response shape")

    candidates: list[str] = []
    seen: set[str] = set()
    for run in runs:
        if not isinstance(run, dict):
            continue
        extra = run.get("extra")
        if not isinstance(extra, dict):
            continue
        meta = extra.get("metadata")
        if not isinstance(meta, dict):
            continue
        thread_id = meta.get("thread_id")
        if not isinstance(thread_id, str) or not thread_id:
            continue
        if thread_id in seen:
            continue
        seen.add(thread_id)
        candidates.append(thread_id)

    if not candidates:
        raise RuntimeError("cannot discover thread id: no thread_id values in root runs metadata")

    for thread_id in candidates:
        q = urllib.parse.urlencode({"select": "all_messages", "session_id": project_uuid})
        url = f"{endpoint}/runs/threads/{urllib.parse.quote(thread_id, safe='')}?{q}"
        probe_req = urllib.request.Request(
            url,
            method="GET",
            headers={
                "x-api-key": api_key,
            },
        )
        try:
            with urllib.request.urlopen(probe_req, timeout=60) as resp:
                payload = json.loads(resp.read().decode("utf-8"))
        except Exception:
            continue
        if not isinstance(payload, dict):
            continue
        previews = payload.get("previews")
        if not isinstance(previews, dict):
            continue
        all_messages = previews.get("all_messages")
        if isinstance(all_messages, str) and all_messages.strip():
            return thread_id

    raise RuntimeError("cannot discover thread id: no candidate thread has previews.all_messages")


def cmd_to_str(cmd: list[str]) -> str:
    return " ".join(shell_quote(part) for part in cmd)


def build_profiles(
    py_cli: pathlib.Path,
    go_bin: pathlib.Path,
    project_uuid: str,
    trace_id: str,
    thread_id: str,
    bulk_limit: int,
    concurrency_levels: list[int],
) -> list[tuple[str, list[str], list[str]]]:
    profiles: list[tuple[str, list[str], list[str]]] = []

    profiles.append(
        (
            "config_show_default",
            [str(py_cli), "config", "show"],
            [str(go_bin), "config", "show"],
        )
    )
    profiles.append(
        (
            "trace_default",
            [str(py_cli), "trace", trace_id, "--format", "raw"],
            [str(go_bin), "trace", trace_id, "--format", "raw"],
        )
    )
    profiles.append(
        (
            "trace_enriched",
            [
                str(py_cli),
                "trace",
                trace_id,
                "--format",
                "raw",
                "--include-metadata",
                "--include-feedback",
            ],
            [
                str(go_bin),
                "trace",
                trace_id,
                "--format",
                "raw",
                "--include-metadata",
                "--include-feedback",
            ],
        )
    )
    profiles.append(
        (
            "thread_default",
            [
                str(py_cli),
                "thread",
                thread_id,
                "--project-uuid",
                project_uuid,
                "--format",
                "raw",
            ],
            [
                str(go_bin),
                "thread",
                thread_id,
                "--project-uuid",
                project_uuid,
                "--format",
                "raw",
            ],
        )
    )
    profiles.append(
        (
            "traces_default",
            [
                str(py_cli),
                "traces",
                "--project-uuid",
                project_uuid,
                "--limit",
                "1",
                "--format",
                "raw",
                "--no-progress",
            ],
            [
                str(go_bin),
                "traces",
                "--project-uuid",
                project_uuid,
                "--limit",
                "1",
                "--format",
                "raw",
                "--no-progress",
            ],
        )
    )
    profiles.append(
        (
            "threads_default",
            [
                str(py_cli),
                "threads",
                "--project-uuid",
                project_uuid,
                "--limit",
                "1",
                "--format",
                "raw",
                "--no-progress",
            ],
            [
                str(go_bin),
                "threads",
                "--project-uuid",
                project_uuid,
                "--limit",
                "1",
                "--format",
                "raw",
                "--no-progress",
            ],
        )
    )

    for c in concurrency_levels:
        c_str = str(c)
        profiles.append(
            (
                f"traces_bulk_c{c_str}",
                [
                    str(py_cli),
                    "traces",
                    "--project-uuid",
                    project_uuid,
                    "--limit",
                    str(bulk_limit),
                    "--format",
                    "raw",
                    "--no-progress",
                    "--max-concurrent",
                    c_str,
                ],
                [
                    str(go_bin),
                    "traces",
                    "--project-uuid",
                    project_uuid,
                    "--limit",
                    str(bulk_limit),
                    "--format",
                    "raw",
                    "--no-progress",
                    "--max-concurrent",
                    c_str,
                ],
            )
        )
        profiles.append(
            (
                f"threads_bulk_c{c_str}",
                [
                    str(py_cli),
                    "threads",
                    "--project-uuid",
                    project_uuid,
                    "--limit",
                    str(bulk_limit),
                    "--format",
                    "raw",
                    "--no-progress",
                    "--max-concurrent",
                    c_str,
                ],
                [
                    str(go_bin),
                    "threads",
                    "--project-uuid",
                    project_uuid,
                    "--limit",
                    str(bulk_limit),
                    "--format",
                    "raw",
                    "--no-progress",
                    "--max-concurrent",
                    c_str,
                ],
            )
        )

    return profiles


def write_runs_csv(path: pathlib.Path, records: list[RunRecord]) -> None:
    if not records:
        path.write_text("", encoding="utf-8")
        return
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=list(asdict(records[0]).keys()))
        writer.writeheader()
        for record in records:
            writer.writerow(asdict(record))


def speedup(py_median: float | None, go_median: float | None) -> float | None:
    if py_median is None or go_median is None or go_median == 0:
        return None
    return py_median / go_median


def extract_scaling(summary: dict[str, Any], prefix: str) -> dict[str, Any]:
    rows: dict[str, Any] = {}
    for profile, data in summary.items():
        if not profile.startswith(prefix):
            continue
        level = int(profile.split("c")[-1])
        py_med = data.get("python", {}).get("latency_ms", {}).get("median")
        go_med = data.get("go", {}).get("latency_ms", {}).get("median")
        rows[level] = {
            "python_median_ms": py_med,
            "go_median_ms": go_med,
        }
    if 1 in rows:
        base_py = rows[1]["python_median_ms"]
        base_go = rows[1]["go_median_ms"]
        for level, row in rows.items():
            py_med = row["python_median_ms"]
            go_med = row["go_median_ms"]
            row["python_scaling_efficiency"] = (
                ((base_py / py_med) / level) if (base_py and py_med and level > 0) else None
            )
            row["go_scaling_efficiency"] = (
                ((base_go / go_med) / level) if (base_go and go_med and level > 0) else None
            )
    return dict(sorted(rows.items(), key=lambda x: x[0]))


def write_summary_markdown(
    path: pathlib.Path,
    environment: dict[str, Any],
    summary: dict[str, Any],
    scaling: dict[str, Any],
) -> None:
    lines: list[str] = []
    lines.append("# Python vs Go Benchmark Summary (gtm-agent)")
    lines.append("")
    lines.append(f"- Timestamp (UTC): `{environment['timestamp_utc']}`")
    lines.append(f"- Project UUID: `{environment['project_uuid']}`")
    lines.append(f"- Trace ID sample: `{environment['trace_id']}`")
    lines.append(f"- Thread ID sample: `{environment['thread_id']}`")
    lines.append("")
    lines.append("## Versions")
    lines.append("")
    lines.append(f"- Go repo: `{environment['go_repo']['head_short']}`")
    lines.append(f"- Python repo: `{environment['python_repo']['head_short']}`")
    lines.append(f"- Python latest main aligned: `{environment['python_freshness']['is_latest_main']}`")
    lines.append("")
    lines.append("## Latency And Throughput")
    lines.append("")
    lines.append("| Profile | Py median ms | Go median ms | Go speedup (x) | Py items/s | Go items/s | Parity |")
    lines.append("|---|---:|---:|---:|---:|---:|---|")
    for profile in sorted(summary.keys()):
        row = summary[profile]
        py = row.get("python", {})
        go = row.get("go", {})
        py_med = py.get("latency_ms", {}).get("median")
        go_med = go.get("latency_ms", {}).get("median")
        py_tp = py.get("throughput", {}).get("items_per_s_median")
        go_tp = go.get("throughput", {}).get("items_per_s_median")
        su = speedup(py_med, go_med)
        parity_cls = row.get("parity", {}).get("classification", "unknown")
        lines.append(
            f"| `{profile}` | "
            f"{fmt_num(py_med)} | {fmt_num(go_med)} | {fmt_num(su)} | "
            f"{fmt_num(py_tp)} | {fmt_num(go_tp)} | `{parity_cls}` |"
        )
    lines.append("")
    lines.append("## Concurrency Scaling")
    lines.append("")
    for family, rows in scaling.items():
        lines.append(f"### {family}")
        lines.append("")
        lines.append("| Concurrency | Py median ms | Go median ms | Py efficiency | Go efficiency |")
        lines.append("|---:|---:|---:|---:|---:|")
        for level, vals in rows.items():
            lines.append(
                f"| {level} | {fmt_num(vals.get('python_median_ms'))} | {fmt_num(vals.get('go_median_ms'))} | "
                f"{fmt_num(vals.get('python_scaling_efficiency'))} | {fmt_num(vals.get('go_scaling_efficiency'))} |"
            )
        lines.append("")
    lines.append("## Notes")
    lines.append("")
    lines.append("- `traces_*` is expected to show a behavioral mismatch: Python returns full payload, Go returns summaries.")
    lines.append("- `thread_default` may classify as normalization-compatible depending on wrapper shape from Python output.")
    lines.append("")
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def fmt_num(value: float | None) -> str:
    if value is None:
        return "-"
    return f"{value:.3f}"


def now_utc_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def now_stamp() -> str:
    return dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--python-cli", default="../langsmith-fetch/.venv/bin/langsmith-fetch")
    parser.add_argument("--go-build-target", default="./cmd/langsmith-fetch")
    parser.add_argument("--go-bin-out", default="/tmp/langsmith-fetch-go-bench/langsmith-fetch")
    parser.add_argument("--go-cache", default="/tmp/go-build-cache")
    parser.add_argument("--project-uuid", default=DEFAULT_PROJECT_UUID)
    parser.add_argument("--trace-id", default="")
    parser.add_argument("--thread-id", default="")
    parser.add_argument("--source-env", default="../ai-sdr/.env")
    parser.add_argument("--warmup-runs", type=int, default=3)
    parser.add_argument("--measured-runs", type=int, default=10)
    parser.add_argument("--bulk-limit", type=int, default=50)
    parser.add_argument("--concurrency-levels", default="1,2,5,10,20")
    parser.add_argument("--profiles", default="all")
    parser.add_argument("--skip-profiles", default="")
    parser.add_argument("--timeout-seconds", type=int, default=300)
    parser.add_argument("--out-dir", default="docs/benchmarks/gtm-agent")
    parser.add_argument("--fail-fast", action="store_true")
    args = parser.parse_args()

    repo_root = pathlib.Path(__file__).resolve().parents[2]
    py_cli = pathlib.Path(args.python_cli).resolve()
    go_bin = pathlib.Path(args.go_bin_out).resolve()
    env_file = pathlib.Path(args.source_env).resolve()
    out_root = (repo_root / args.out_dir).resolve()
    out_dir = out_root / now_stamp()
    out_dir.mkdir(parents=True, exist_ok=True)

    base_env = os.environ.copy()
    source_env = load_env_from_file_with_shell(env_file)
    bench_env = base_env.copy()
    bench_env.update(source_env)
    bench_env["LANGSMITH_PROJECT_UUID"] = args.project_uuid
    bench_env.pop("LANGSMITH_ENDPOINT", None)
    bench_env.pop("LANGCHAIN_ENDPOINT", None)
    bench_env["NO_COLOR"] = "1"
    bench_env["PYTHONWARNINGS"] = "ignore"
    bench_env["GOCACHE"] = args.go_cache

    py_repo = py_cli.parents[2]
    freshness = ensure_python_fresh(py_repo)
    if freshness["is_latest_main"] != "true":
        print(
            "python repo is not at latest origin/main; benchmark continuing with current checkout",
            file=sys.stderr,
        )

    build_go_binary(go_bin, args.go_build_target, env=bench_env)

    trace_id = args.trace_id.strip()
    thread_id = args.thread_id.strip()
    if not trace_id or not thread_id:
        discovered_trace, discovered_thread = discover_ids(
            go_bin=go_bin,
            env=bench_env,
            project_uuid=args.project_uuid,
            workdir=repo_root,
        )
        if not trace_id:
            trace_id = discovered_trace
        if not thread_id:
            thread_id = discovered_thread

    concurrency_levels = parse_level_list(args.concurrency_levels)
    profiles = build_profiles(
        py_cli=py_cli,
        go_bin=go_bin,
        project_uuid=args.project_uuid,
        trace_id=trace_id,
        thread_id=thread_id,
        bulk_limit=args.bulk_limit,
        concurrency_levels=concurrency_levels,
    )
    profiles = filter_profiles(
        profiles=profiles,
        include_tokens=parse_list_arg(args.profiles),
        skip_tokens=parse_list_arg(args.skip_profiles),
    )

    all_records: list[RunRecord] = []
    grouped: dict[str, dict[str, ProfileResult]] = {}

    for profile_name, py_cmd, go_cmd in profiles:
        grouped.setdefault(profile_name, {})
        grouped[profile_name]["python"] = ProfileResult(profile=profile_name, cli="python")
        grouped[profile_name]["go"] = ProfileResult(profile=profile_name, cli="go")
        for cli_name, cmd in (("python", py_cmd), ("go", go_cmd)):
            total_runs = args.warmup_runs + args.measured_runs
            for i in range(total_runs):
                phase = "warmup" if i < args.warmup_runs else "measured"
                iteration = i + 1
                started = time.perf_counter()
                timeout_error = ""
                try:
                    proc = run_checked(
                        cmd,
                        env=bench_env,
                        cwd=str(repo_root),
                        timeout_s=args.timeout_seconds,
                    )
                except subprocess.TimeoutExpired as exc:
                    timeout_error = f"command timed out after {args.timeout_seconds}s"
                    proc = subprocess.CompletedProcess(
                        cmd,
                        124,
                        exc.stdout or "",
                        f"{exc.stderr or ''}\n{timeout_error}".strip(),
                    )
                ended = time.perf_counter()

                stdout = proc.stdout or ""
                stderr = proc.stderr or ""
                json_valid, root_type, top_keys, parsed = parse_json_signature(stdout)
                item_count, message_count = count_items_and_messages(profile_name, parsed)
                wall_ms = (ended - started) * 1000.0
                tp_items = (item_count / (wall_ms / 1000.0)) if (item_count is not None and wall_ms > 0) else None
                tp_messages = (message_count / (wall_ms / 1000.0)) if (message_count is not None and wall_ms > 0) else None

                record = RunRecord(
                    timestamp_utc=now_utc_iso(),
                    profile=profile_name,
                    cli=cli_name,
                    phase=phase,
                    iteration=iteration,
                    command=cmd_to_str(cmd),
                    exit_code=proc.returncode,
                    wall_ms=wall_ms,
                    stdout_bytes=len(stdout.encode("utf-8", "replace")),
                    stderr_bytes=len(stderr.encode("utf-8", "replace")),
                    stdout_sha256=hashlib.sha256(stdout.encode("utf-8", "replace")).hexdigest(),
                    stderr_sha256=hashlib.sha256(stderr.encode("utf-8", "replace")).hexdigest(),
                    json_valid=json_valid,
                    json_root_type=root_type,
                    json_top_level_keys=top_keys,
                    item_count=item_count,
                    message_count=message_count,
                    throughput_items_per_s=tp_items,
                    throughput_messages_per_s=tp_messages,
                    error=(
                        timeout_error
                        if timeout_error
                        else (stderr.strip().splitlines()[-1] if proc.returncode != 0 and stderr.strip() else "")
                    ),
                )
                # Cache parsed output for parity on first measured success row.
                record.command_output_cache = stdout.strip()  # type: ignore[attr-defined]

                all_records.append(record)
                grouped[profile_name][cli_name].runs.append(record)

                print(
                    f"[{profile_name}] {cli_name} {phase} {iteration}/{total_runs} "
                    f"exit={proc.returncode} wall_ms={wall_ms:.2f}"
                )
                if proc.returncode != 0 and args.fail_fast:
                    raise RuntimeError(
                        f"fail-fast enabled: command failed\nprofile={profile_name}\ncli={cli_name}\ncmd={cmd_to_str(cmd)}\n"
                        f"stderr:\n{stderr}\nstdout:\n{stdout}"
                    )

    summary: dict[str, Any] = {}
    for profile_name in sorted(grouped.keys()):
        py_runs = grouped[profile_name]["python"].runs
        go_runs = grouped[profile_name]["go"].runs
        summary[profile_name] = {
            "python": summarize_profile(py_runs),
            "go": summarize_profile(go_runs),
            "parity": parity_classification(profile_name, py_runs, go_runs),
        }

    scaling = {
        "traces_bulk": extract_scaling(summary, "traces_bulk_c"),
        "threads_bulk": extract_scaling(summary, "threads_bulk_c"),
    }

    environment = {
        "timestamp_utc": now_utc_iso(),
        "project_uuid": args.project_uuid,
        "trace_id": trace_id,
        "thread_id": thread_id,
        "warmup_runs": args.warmup_runs,
        "measured_runs": args.measured_runs,
        "bulk_limit": args.bulk_limit,
        "concurrency_levels": concurrency_levels,
        "python_cli": str(py_cli),
        "go_bin": str(go_bin),
        "source_env": str(env_file),
        "go_repo": git_head(repo_root),
        "python_repo": git_head(py_repo),
        "python_freshness": freshness,
        "tool_versions": {
            "python": sh(f"{shell_quote(str(py_cli.parent / 'python'))} --version || python --version"),
            "go": sh("go version"),
        },
    }

    (out_dir / "environment.json").write_text(json.dumps(environment, indent=2) + "\n", encoding="utf-8")
    write_runs_csv(out_dir / "runs.csv", all_records)
    (out_dir / "summary.json").write_text(
        json.dumps(
            {
                "environment": environment,
                "summary": summary,
                "scaling": scaling,
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )
    write_summary_markdown(
        path=out_dir / "summary.md",
        environment=environment,
        summary=summary,
        scaling=scaling,
    )

    print(f"benchmark complete: {out_dir}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
