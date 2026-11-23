#!/usr/bin/env python3
"""Bound the task-owned production journey and preserve generated user files."""

import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time

repo = Path("/Volumes/artisan/code/2026/multica-0.4.37")
research = repo / ".trellis/tasks/11-22-collaboration-sidebar-beta/research"
source_report = research / "isolated-source.json"
source_root = Path(json.loads(source_report.read_text())["source_root"]) if source_report.is_file() else repo
next_env = source_root / "apps/web/next-env.d.ts"
original_next_env = next_env.read_bytes()
original_next_env_mode = next_env.stat().st_mode
environment_paths = [repo / name for name in [
    ".env", ".env.worktree", ".env.check", "apps/desktop/.env.development.local",
] if (repo / name).is_file()]
environment_hashes = {path: hashlib.sha256(path.read_bytes()).hexdigest() for path in environment_paths}
root_next_env_hash = hashlib.sha256((repo / "apps/web/next-env.d.ts").read_bytes()).hexdigest()
run_env = os.environ.copy()
run_env["SIDEBAR_VERIFY_SOURCE_ROOT"] = str(source_root)
source_env = repo / ".env"
if not source_env.is_file():
    source_env = repo / ".env.worktree"
run_env["SIDEBAR_VERIFY_SOURCE_ENV"] = str(source_env)
started = time.monotonic()
deadline_exceeded = False
print(f"Running isolated Web/Desktop navigation verification; log: {research / 'production-run.log'}", flush=True)
with (research / "production-run.log").open("wb") as log:
    process = subprocess.Popen(
        ["bash", str(research / "verify-production-navigation.sh")],
        cwd=source_root,
        env=run_env,
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    try:
        exit_code = process.wait(timeout=600)
    except subprocess.TimeoutExpired:
        deadline_exceeded = True
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=30)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
        exit_code = 124
    finally:
        if next_env.read_bytes() != original_next_env:
            next_env.write_bytes(original_next_env)
        os.chmod(next_env, original_next_env_mode)

report = {
    "source_root": str(source_root),
    "exit_code": exit_code,
    "duration_seconds": round(time.monotonic() - started, 1),
    "deadline_seconds": 600,
    "deadline_exceeded": deadline_exceeded,
    "next_env_preserved": next_env.read_bytes() == original_next_env,
    "original_checkout_next_env_preserved": hashlib.sha256((repo / "apps/web/next-env.d.ts").read_bytes()).hexdigest() == root_next_env_hash,
    "environment_files_preserved": {
        str(path.relative_to(repo)): hashlib.sha256(path.read_bytes()).hexdigest() == checksum
        for path, checksum in environment_hashes.items()
    },
}
(research / "production-run.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps(report), flush=True)
raise SystemExit(exit_code)
