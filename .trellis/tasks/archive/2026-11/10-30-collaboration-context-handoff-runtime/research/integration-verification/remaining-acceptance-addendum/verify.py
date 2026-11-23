"""Capture the bounded acceptance suites without recording database credentials."""

import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
from datetime import datetime, timezone
from urllib.parse import parse_qs, unquote, urlparse


ROOT = Path.cwd()
OUT = Path(__file__).resolve().parent
OWNED = [
    "server/internal/service/collaboration_squad_budget_matrix_test.go",
    "server/internal/service/collaboration_memory_merge_acceptance_test.go",
    "server/internal/handler/collaboration_runtime_credential_matrix_test.go",
]
REVIEW_FOLLOWUP = sys.argv[1:] == ["--review-followup"]
PREFIX = "review-" if REVIEW_FOLLOWUP else ""


def read_command(*command):
    return subprocess.check_output(command, text=True).strip()


def write_json(name, value):
    (OUT / (PREFIX + name)).write_text(json.dumps(value, indent=2) + "\n")


def digest_file(path):
    return hashlib.sha256((ROOT / path).read_bytes()).hexdigest()


def source():
    tracked = read_command("git", "ls-files", "--cached", "--others", "--exclude-standard", "server").splitlines()
    sources = {
        path: digest_file(path)
        for path in sorted(set(tracked))
        if Path(path).suffix in {".go", ".sql", ".mod", ".sum"}
    }
    return {
        "head": read_command("git", "rev-parse", "HEAD"),
        "server_status": read_command("git", "status", "--short", "--", "server"),
        "owned_files": {path: digest_file(path) for path in OWNED},
        "server_source_aggregate_sha256": hashlib.sha256(json.dumps(sources, sort_keys=True).encode()).hexdigest(),
        "server_source_files": sources,
    }


database_uri = urlparse(os.environ.get("DATABASE_URL", ""))
database = database_uri.path.lstrip("/")
if database != "multica_runtime_final2_858f011e":
    sys.exit("Refusing an absent or different assigned database")

baseline = source()
write_json("source-before.json", baseline)
postgres_environment = {
    **os.environ,
    "PGDATABASE": database,
    "PGHOST": database_uri.hostname or "",
    "PGPORT": str(database_uri.port or 5432),
    "PGUSER": unquote(database_uri.username or ""),
    "PGPASSWORD": unquote(database_uri.password or ""),
    "PGSSLMODE": parse_qs(database_uri.query).get("sslmode", ["prefer"])[0],
}
postgres = subprocess.run(
    ["psql", "-XAt", "-c", "SELECT current_database(), current_setting('server_version')"],
    env=postgres_environment,
    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
)
if postgres.returncode:
    sys.exit("Unable to read the assigned PostgreSQL version")
write_json("environment.json", {
    "database": database,
    "platform": platform.platform(),
    "machine": platform.machine(),
    "go": read_command("go", "version"),
    "postgres": postgres.stdout.strip(),
    "provider_execution": False,
    "production_changes": False,
})

commands = [
    ("service-final", ["go", "-C", "server", "test", "./internal/service", "-run", "^TestCollaboration(SquadPreparationBudgetMatrix|MemoryConflictRefreshMergeRetainsBothFacts)$", "-count=1", "-timeout", "10m", "-race", "-json"]),
    ("handler-final", ["go", "-C", "server", "test", "./internal/handler", "-run", "^TestCollaborationRuntimeCredentialMatrix$", "-count=1", "-timeout", "10m", "-json"]),
    ("vet", ["go", "-C", "server", "vet", "./internal/service", "./internal/handler"]),
    ("gofmt", ["gofmt", "-l", *OWNED]),
]
if REVIEW_FOLLOWUP:
    commands = [(PREFIX + name, command) for name, command in commands if name != "service-final"]
results = []
for name, command in commands:
    started = datetime.now(timezone.utc).isoformat()
    completed = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    (OUT / (name + ".stderr")).write_bytes(completed.stderr)
    (OUT / (name + ".exit")).write_text(str(completed.returncode) + "\n")
    result = {"name": name, "command": command, "started_at": started,
              "ended_at": datetime.now(timezone.utc).isoformat(), "exit_code": completed.returncode}
    if name.endswith("-final"):
        (OUT / (name + ".jsonl")).write_bytes(completed.stdout)
        events = [json.loads(line) for line in completed.stdout.splitlines() if line]
        (OUT / (name + ".log")).write_text("".join(event.get("Output", "") for event in events))
        runs = [event["Test"] for event in events if event.get("Action") == "run" and "Test" in event]
        passes = [event["Test"] for event in events if event.get("Action") == "pass" and "Test" in event]
        failures = [event.get("Test", "package") for event in events if event.get("Action") in {"fail", "build-fail"}]
        skips = [event.get("Test", "package") for event in events if event.get("Action") == "skip"]
        result.update({"tests_run": runs, "tests_passed": passes, "failures": failures, "skips": skips,
                       "leaf_tests": [test for test in passes if not any(other.startswith(test + "/") for other in passes)]})
        result["nonempty_no_skip_gate"] = bool(runs) and sorted(runs) == sorted(passes) and not failures and not skips
    else:
        (OUT / (name + ".log")).write_bytes(completed.stdout)
        result["clean_output"] = not completed.stdout and not completed.stderr
    results.append(result)
    write_json("commands.json", results)
    print(name, "exit", completed.returncode, flush=True)
    if completed.returncode or result.get("nonempty_no_skip_gate") is False or result.get("clean_output") is False:
        sys.exit(1)

after = source()
write_json("source-after.json", after)
write_json("source-stability.json", {"identical": baseline == after})
if baseline != after:
    sys.exit("Source changed during acceptance verification")
