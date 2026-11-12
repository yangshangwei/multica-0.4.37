"""Task-local database allocation and bounded Go test execution; no provider calls."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import urllib.parse
import uuid

ROOT = Path(__file__).resolve().parents[4]
EVIDENCE = Path(__file__).resolve().parent / "test-environments"
PSQL = "/opt/homebrew/opt/libpq/bin/psql"
PACKAGES = {"./internal/handler", "./internal/service", "./cmd/server", "./cmd/multica"}


def write_json(path, value):
    with path.open("x") as f:
        json.dump(value, f, indent=2)
        f.write("\n")


def connection_env(url):
    u = urllib.parse.urlsplit(url)
    assert u.hostname in {"localhost", "127.0.0.1", "::1"}
    return dict(PGHOST=u.hostname, PGPORT=str(u.port or 5432),
                PGUSER=urllib.parse.unquote(u.username or ""),
                PGPASSWORD=urllib.parse.unquote(u.password or ""),
                PGDATABASE=u.path.removeprefix("/"), PGCONNECT_TIMEOUT="5",
                PGSSLMODE="disable")


def query(url, sql):
    env = {"PATH": os.environ["PATH"], **connection_env(url)}
    result = subprocess.run([PSQL, "-X", "-At", "-v", "ON_ERROR_STOP=1", "-c", sql],
                            env=env, capture_output=True, text=True, timeout=15)
    if result.returncode:
        raise RuntimeError("Owned database query failed; no connection details emitted")
    return result.stdout.strip()


def allocate(label, package):
    assert package in PACKAGES and re.fullmatch(r"[a-z0-9_-]+", label)
    EVIDENCE.mkdir(exist_ok=True)
    record_path = EVIDENCE / (label + ".json")
    assert not record_path.exists(), "Allocation label already used"
    line = next(line for line in (ROOT / ".env").read_text().splitlines()
                if line.startswith("DATABASE_URL="))
    url = " ".join(shlex.split(line.split("=", 1)[1], comments=True))
    assert "$" not in url
    parsed = urllib.parse.urlsplit(url)
    admin = urllib.parse.urlunsplit(parsed._replace(path="/postgres"))
    before = query(admin, "SELECT current_database(),current_setting('server_version'),"
                   "current_setting('max_connections'),(SELECT count(*) FROM pg_stat_activity)")
    database = "m1s1_" + label.replace("-", "_")[:25] + "_" + uuid.uuid4().hex[:8]
    assert re.fullmatch(r"[a-z0-9_]+", database)
    assert query(admin, "SELECT count(*) FROM pg_database WHERE datname='" + database + "'") == "0"
    private = Path(tempfile.mkdtemp(prefix="multica-m1s1-")).resolve()
    os.chmod(private, 0o700)
    probe = subprocess.run(["git", "-C", str(private), "rev-parse", "--show-toplevel"],
                           capture_output=True, text=True)
    assert probe.returncode and not probe.stdout.strip(), "Test state inherits a Git repository"
    caches = json.loads(subprocess.check_output(["go", "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"], text=True))
    for name in ["home", "tmp"]:
        (private / name).mkdir(mode=0o700)
    owned_url = urllib.parse.urlunsplit(parsed._replace(path="/" + database))
    env = {"PATH": os.environ["PATH"], "HOME": str(private / "home"),
           "TMPDIR": str(private / "tmp"), "TMP": str(private / "tmp"), "TEMP": str(private / "tmp"),
           **caches, "DATABASE_URL": owned_url, "GOTOOLCHAIN": "auto"}
    env_path = private / "environment.json"
    write_json(env_path, env)
    os.chmod(env_path, 0o600)
    record = {"label": label, "package": package, "database": database,
              "private_environment": str(env_path), "state": "allocated",
              "cluster_before": before, "source_head": subprocess.check_output(
                  ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
              "runner_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
              "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    write_json(record_path, record)
    query(admin, 'CREATE DATABASE "' + database + '" TEMPLATE template0')
    assert query(owned_url, "SELECT current_database()") == database
    result = subprocess.run(["go", "-C", "server", "run", "./cmd/migrate", "up"],
                            cwd=ROOT, env=env, capture_output=True, text=True)
    log = result.stdout + result.stderr
    for secret in [owned_url, parsed.password or "", urllib.parse.unquote(parsed.password or "")]:
        if secret:
            log = log.replace(secret, "[redacted]")
    (EVIDENCE / (label + "-migration.log")).write_text(log)
    record["migration_exit"] = result.returncode
    record["state"] = "ready" if result.returncode == 0 else "migration_failed"
    if result.returncode == 0:
        record["schema_migrations_count"] = query(owned_url, "SELECT count(*) FROM schema_migrations")
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({k: record[k] for k in ["label", "package", "database", "state", "migration_exit"]}))
    return result.returncode


def run(label, regex, race):
    record_path = EVIDENCE / (label + ".json")
    record = json.loads(record_path.read_text())
    assert record["state"] == "ready", "Allocate a fresh database for each run"
    assert record["package"] in PACKAGES
    env = json.loads(Path(record["private_environment"]).read_text())
    assert query(env["DATABASE_URL"], "SELECT current_database()") == record["database"]
    argv = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test",
            record["package"], "-count=1", "-json", "-timeout=15m", "-run", regex]
    if race:
        argv.append("-race")
    record.update(state="running", command=argv)
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    log_path = EVIDENCE / (label + "-tests.jsonl")
    with log_path.open("x") as log:
        result = subprocess.run(argv, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
    events = []
    for line in log_path.read_text().splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if event.get("Test") and event.get("Action") in {"pass", "fail", "skip"}:
            events.append({"test": event["Test"], "action": event["Action"]})
    record.update(state="complete", exit_code=result.returncode, named_test_results=events,
                  log_sha256=hashlib.sha256(log_path.read_bytes()).hexdigest())
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({"label": label, "exit": result.returncode,
                      "counts": {s: sum(e["action"] == s for e in events) for s in ["pass", "fail", "skip"]},
                      "failed": [e["test"] for e in events if e["action"] == "fail"], "log": str(log_path)}))
    return result.returncode


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="action", required=True)
    a = sub.add_parser("allocate")
    a.add_argument("label")
    a.add_argument("package", choices=sorted(PACKAGES))
    r = sub.add_parser("run")
    r.add_argument("label")
    r.add_argument("--regex", required=True)
    r.add_argument("--race", action="store_true")
    args = parser.parse_args()
    raise SystemExit(allocate(args.label, args.package) if args.action == "allocate" else run(args.label, args.regex, args.race))
