"""Capture bounded, private-account-free IM verification receipts."""

from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
import argparse
import hashlib
import json
import os
import subprocess
import time


parser = argparse.ArgumentParser()
parser.add_argument("label")
parser.add_argument("--database", default="service_s6_other")
parser.add_argument("command", nargs=argparse.REMAINDER)
args = parser.parse_args()
command = args.command
if command and command[0] == "--":
    command = command[1:]
if not command:
    raise SystemExit("a verification command is required")

evidence = Path(__file__).resolve().parent
repo = evidence.parents[4]
cwd = repo / "server"
prefix = evidence / ("outbound-tail-" + args.label)
launcher = Path("/tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py")
invocation = ["python3", str(launcher), args.database, *command]
env = os.environ.copy()
env["GOTOOLCHAIN"] = "go1.26.6"


def now():
    return datetime.now(timezone.utc).isoformat()


def hashes():
    paths = {
        repo / "server/go.mod",
        repo / "server/go.sum",
        repo / "scripts/go-test-with-agent-cli-guard.sh",
        launcher,
        Path(__file__).resolve(),
    }
    for package in ("lark", "dingtalk", "telegram", "wecom", "slack"):
        paths.update((repo / "server/internal/integrations" / package).glob("*.go"))
    for relative in (
        "server/internal/integrations/channel",
        "server/internal/storage",
        "server/internal/testutil",
        "server/pkg/protocol",
        "server/pkg/db/generated",
    ):
        paths.update((repo / relative).rglob("*.go"))
    paths.update((repo / "server/internal/service").glob("collaboration*.go"))
    result = {}
    for path in sorted(paths):
        if path.is_file():
            key = str(path.relative_to(repo)) if path.is_relative_to(repo) else str(path)
            result[key] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result


started_at = now()
head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip()
before = hashes()
receipt = {
    "command": invocation,
    "cwd": str(cwd),
    "started_at": started_at,
    "head": head,
    "database_alias": args.database,
    "database_schema_prerequisite": "737_collaboration_runtime_preparation",
    "source_sha256": before,
    "real_provider_account_access": False,
    "external_im": False,
    "production_gate_change": False,
}
prefix.with_name(prefix.name + "-command.json").write_text(json.dumps(receipt, indent=2) + "\n")
started = time.monotonic()
stdout_path = prefix.with_suffix(".jsonl")
stderr_path = prefix.with_suffix(".stderr")
with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
    process = subprocess.run(invocation, cwd=cwd, env=env, stdout=stdout, stderr=stderr)
after = hashes()
actions = Counter()
tests = {"passed": [], "failed": [], "skipped": []}
packages = {}
for line in stdout_path.read_text(errors="replace").splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    action = event.get("Action")
    if action not in ("pass", "fail", "skip"):
        continue
    actions[action] += 1
    package = event.get("Package")
    if event.get("Test"):
        status = {"pass": "passed", "fail": "failed", "skip": "skipped"}[action]
        tests[status].append({"package": package, "test": event["Test"]})
    else:
        packages[package] = action
result = {
    "exit_code": process.returncode,
    "started_at": started_at,
    "finished_at": now(),
    "duration_seconds": round(time.monotonic() - started, 3),
    "head_before": head,
    "head_after": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
    "counts": {status: len(items) for status, items in tests.items()},
    "top_level_counts": {
        status: sum("/" not in item["test"] for item in items)
        for status, items in tests.items()
    },
    "tests": tests,
    "packages": packages,
    "go_event_actions": dict(actions),
    "stdout_sha256": hashlib.sha256(stdout_path.read_bytes()).hexdigest(),
    "stderr_sha256": hashlib.sha256(stderr_path.read_bytes()).hexdigest(),
    "source_changed_during_run": [
        path for path in sorted(set(before) | set(after)) if before.get(path) != after.get(path)
    ],
    "source_sha256_after": after,
    "real_provider_account_access": False,
    "external_im": False,
    "production_gate_change": False,
}
prefix.with_name(prefix.name + "-result.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps({key: result[key] for key in ("exit_code", "duration_seconds", "counts", "top_level_counts", "source_changed_during_run")}), flush=True)
raise SystemExit(process.returncode)
