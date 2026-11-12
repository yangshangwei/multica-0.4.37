"""Run existing pool-sizing tests by exact files, excluding the DB TestMain."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path("/Volumes/artisan/code/2026/multica-0.4.37")
OUT = Path(__file__).resolve().parent
CANDIDATE = Path("/Volumes/artisan/code/2026/mc-collaboration-final5-20261102")
PRIVATE = OUT / "pool-precedence"
PRIVATE.mkdir(exist_ok=True, mode=0o700)
HOME_DIR = PRIVATE / "home"
HOME_DIR.mkdir(exist_ok=True, mode=0o700)
ALLOCATIONS = PRIVATE / "allocations"
ALLOCATIONS.mkdir(exist_ok=True, mode=0o700)
DATABASE = "ccheck_20261102130000000000_abcdef123456"
(ALLOCATIONS / (DATABASE + ".json")).write_text(json.dumps({
    "database": DATABASE, "state": "running", "template": "multica_check_fixture_go",
    "fixture_only": True, "actual_database_allocated": False,
}) + "\n")
SOURCES = [CANDIDATE / "server/cmd/server/dbstats.go", CANDIDATE / "server/cmd/server/dbstats_test.go"]
HASHES = {str(path.relative_to(CANDIDATE)): hashlib.sha256(path.read_bytes()).hexdigest() for path in SOURCES}
environment = {**os.environ, "PATH": "/opt/homebrew/bin:/usr/bin:/bin", "HOME": str(HOME_DIR),
               "GOCACHE": "/Volumes/artisan/dev-cache/go-build", "GOMODCACHE": "/Users/artisan/go/pkg/mod",
               "GOPATH": "/Users/artisan/go", "GOPROXY": "file:///Users/artisan/go/pkg/mod/cache/download",
               "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOMAXPROCS": "2",
               "DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1",
               "DATABASE_URL": "postgres://fixture@127.0.0.1/" + DATABASE,
               "MULTICA_TASK_REAL_GO": "/opt/homebrew/bin/go", "MULTICA_TASK_PACKAGE_DB_MODE": "0",
               "MULTICA_TASK_PACKAGE_DB_EVIDENCE": str(ALLOCATIONS), "PYTHONDONTWRITEBYTECODE": "1"}
for key in ("MULTICA_TASK_CONFIG_ROOT", "MULTICA_TASK_WORKSPACES_ROOT", "MULTICA_RUN_REAL_AGENT_SMOKE"):
    environment.pop(key, None)
results = []
for version, wrapper, expected in [
    ("v5", ROOT / ".omx/private/collaboration-resume-20261101/adapter-v5/check-bin/go", 1),
    ("v6", OUT / "check-bin/go", 0),
]:
    command = ["/usr/bin/sandbox-exec", "-p", "(version 1) (allow default) (deny network*)", str(wrapper),
               "-C", str(CANDIDATE / "server"), "test", "-p", "2", "./cmd/server/dbstats.go",
               "./cmd/server/dbstats_test.go", "-count=1", "-json"]
    log = PRIVATE / (version + ".jsonl")
    if log.exists():
        raise RuntimeError("Refusing to overwrite a prior pool-precedence proof")
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=120)
    log.write_text(result.stdout + result.stderr)
    counts = {key: 0 for key in ("run", "pass", "fail", "skip")}
    for line in result.stdout.splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get("Test") and event.get("Action") in counts:
            counts[event["Action"]] += 1
    record = {"version": version, "command": command, "exit_code": result.returncode,
              "counts": counts, "started_at": started,
              "finished_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "log_sha256": hashlib.sha256(log.read_bytes()).hexdigest(),
              "wrapper_sha256": hashlib.sha256(wrapper.read_bytes()).hexdigest(),
              "sources_sha256": HASHES, "exact_file_list_excludes_DB_TestMain": True,
              "network_denied": True, "database_connections": 0, "actual_database_allocated": False,
              "real_provider_calls": 0}
    (PRIVATE / (version + ".json")).write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({key: record[key] for key in ("version", "exit_code", "counts", "log_sha256")}))
    if result.returncode != expected or counts["run"] != 7 or counts["skip"] != 0:
        raise RuntimeError("Existing pool-sizing tests did not produce the expected bounded proof")
    results.append(record)
for path in SOURCES:
    if hashlib.sha256(path.read_bytes()).hexdigest() != HASHES[str(path.relative_to(CANDIDATE))]:
        raise RuntimeError("Source changed during the connection-free test run")
(PRIVATE / "summary.json").write_text(json.dumps({"results": results, "source_files_unchanged": True}, indent=2) + "\n")
