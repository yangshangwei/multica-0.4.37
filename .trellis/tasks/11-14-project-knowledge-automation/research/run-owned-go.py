#!/usr/bin/env python3
"""Run one explicitly selected package against a new task-owned database.

This is task evidence tooling, not a replacement for the repository test entry
point. It never drops a database or reads model/account configuration.
"""

import argparse
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.parse
import uuid


ROOT = Path(__file__).resolve().parents[4]
RESEARCH = Path(__file__).resolve().parent
STATE = (Path(tempfile.gettempdir()) / "multica-knowledge-automation-20261114").resolve()
PSQL = Path("/opt/homebrew/opt/libpq/bin/psql")
PACKAGES = {
    "./cmd/server": "router",
    "./internal/handler": "handler",
    "./internal/service": "service",
    "./internal/migrations": "migrations",
    "./internal/testutil": "testutil",
}


def save(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")
    path.chmod(0o600)


def source_fingerprint():
    digest = hashlib.sha256()
    for path in sorted((ROOT / "server").rglob("*")):
        if path.is_file() and (
            path.suffix in {".go", ".sql"} or path.name in {"go.mod", "go.sum", "sqlc.yaml"}
        ):
            digest.update(str(path.relative_to(ROOT)).encode())
            digest.update(b"\0")
            digest.update(path.read_bytes())
    return digest.hexdigest()


def database_url():
    for line in (ROOT / ".env").read_text().splitlines():
        key, separator, value = line.strip().partition("=")
        if key != "DATABASE_URL" or not separator:
            continue
        value = value.strip()
        if value[:1] in {"\"", "'"} and value[-1:] == value[:1]:
            value = value[1:-1]
        if "$" in value or any(ord(char) < 32 for char in value):
            raise RuntimeError("DATABASE_URL must be literal configuration data")
        parsed = urllib.parse.urlparse(value)
        if parsed.scheme not in {"postgres", "postgresql"} or parsed.hostname not in {
            "localhost", "127.0.0.1", "::1"
        }:
            raise RuntimeError("task launcher permits only the recorded local cluster")
        return parsed
    raise RuntimeError("explicit local DATABASE_URL unavailable")


def redis_allocation(allocation_file, run_dir):
    """Admit only the immutable, authenticated fixture owned by this task."""
    path = Path(allocation_file).resolve(strict=True)
    if path.parent != RESEARCH or path.name != "p6-redis-allocation.json":
        raise RuntimeError("Redis requires this task's explicit allocation pointer")
    if path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o777 != 0o600:
        raise RuntimeError("Redis allocation is not private and owned")
    data = path.read_bytes()
    allocation = json.loads(data)
    if allocation.get("schema") != "multica.p6.redis-allocation.v2" or allocation.get("state") != "running" or allocation.get("safe_to_pass_to_go_launcher") is not True or allocation.get("go_launcher_admission", {}).get("permitted") is not True:
        raise RuntimeError("Redis allocation has no current execution permit")
    if allocation.get("task") != {"approved_version": "2026-11-14-v1", "name": "11-14-project-knowledge-automation", "path": str(RESEARCH.parent)}:
        raise RuntimeError("Redis allocation belongs to a different task")
    connection = allocation.get("connection", {})
    url = allocation.get("redis_test_url")
    if not isinstance(url, str) or "$" in url or any(ord(char) < 32 for char in url):
        raise RuntimeError("Redis URL must be explicit literal fixture data")
    parsed = urllib.parse.urlparse(url)
    try:
        port = parsed.port
    except ValueError:
        raise RuntimeError("Redis fixture port is invalid") from None
    if parsed.scheme != "redis" or parsed.hostname != "127.0.0.1" or not port or not 1024 < port < 65536 or 55749 <= port <= 55752 or parsed.path != "/12" or parsed.query or parsed.fragment or connection.get("scheme") != "redis" or connection.get("host") != "127.0.0.1" or connection.get("port") != port or type(connection.get("db")) is not int or connection["db"] != 12 or connection.get("tls") is not False:
        raise RuntimeError("Redis URL differs from its dedicated loopback allocation")
    password = connection.get("password")
    if parsed.username not in {None, "", "default"} or not isinstance(password, str) or len(password) < 32 or urllib.parse.unquote(parsed.password or "") != password or allocation.get("env") != {"REDIS_TEST_URL": url} or allocation.get("credentials", {}).get("generated_for_this_fixture_only") is not True:
        raise RuntimeError("Redis credentials differ from the synthetic allocation")
    evidence = allocation["evidence"]
    receipt_path = Path(evidence["immutable_receipt"]).resolve(strict=True)
    if receipt_path.parent != RESEARCH or not receipt_path.name.startswith("p6-redis-allocation-receipt-") or receipt_path.stat().st_uid != os.getuid() or receipt_path.stat().st_mode & 0o777 != 0o600:
        raise RuntimeError("Redis immutable receipt is not private and owned")
    receipt = json.loads(receipt_path.read_text())
    allocation_hash = hashlib.sha256(data).hexdigest()
    if receipt.get("allocation_path") != str(path) or receipt.get("allocation_sha256") != allocation_hash or receipt.get("safe_to_pass_to_go_launcher") is not True:
        raise RuntimeError("Redis allocation differs from its immutable receipt")
    state = Path(allocation["state_dir"]).resolve(strict=True)
    helper = Path(allocation["teardown"]["script"]).resolve(strict=True)
    proof = Path(evidence["owned_binding_proof"]).resolve(strict=True)
    if not state.is_relative_to(STATE) or state.is_relative_to(ROOT) or helper.parent != state or proof.parent != state or any(file.stat().st_uid != os.getuid() or file.stat().st_mode & 0o077 for file in [state, helper, proof]):
        raise RuntimeError("Redis ownership tooling is outside its private allocation")
    helper_hash = hashlib.sha256(helper.read_bytes()).hexdigest()
    if helper_hash != allocation["teardown"]["script_sha256"] or hashlib.sha256(proof.read_bytes()).hexdigest() != evidence["owned_binding_sha256"]:
        raise RuntimeError("Redis ownership proof or verifier changed")
    command = [sys.executable, "-I", str(helper), "--allocation", str(path),
               "--expected-allocation-sha256", allocation_hash, "--verify-only"]
    result = subprocess.run(command, env={"PATH": os.environ["PATH"], "HOME": str(run_dir / "home")},
                            capture_output=True, text=True, timeout=45)
    diagnostics = run_dir / ("redis-ownership-" + uuid.uuid4().hex[:8] + ".log")
    diagnostics.write_text(result.stdout + result.stderr)
    diagnostics.chmod(0o600)
    if result.returncode:
        raise RuntimeError("Current Redis ownership or authenticated reachability failed")
    verified = json.loads(result.stdout)
    if verified.get("status") != "verified_owned_running" or verified.get("run_id") != allocation["allocation_id"] or verified.get("container_id") != allocation["container"]["id"] or verified.get("network_id") != allocation["network"]["id"]:
        raise RuntimeError("Redis runtime proof differs from the recorded resources")
    return url, {"allocation_file": str(path), "allocation_sha256": allocation_hash,
                 "immutable_receipt": str(receipt_path),
                 "immutable_receipt_sha256": hashlib.sha256(receipt_path.read_bytes()).hexdigest(),
                 "verifier_sha256": helper_hash, "proof": verified, "diagnostics": str(diagnostics)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--package", required=True, choices=PACKAGES)
    parser.add_argument("--run", required=True, help="explicit Go test name expression")
    parser.add_argument("--label", required=True)
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--redis-allocation", help="Explicit verified task-owned Redis; never inherit a default URL")
    parser.add_argument("--timeout", default="15m")
    args = parser.parse_args()
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", args.label):
        parser.error("label must be a short lowercase run identity")
    if not re.fullmatch(r"[1-9][0-9]*[ms]", args.timeout):
        parser.error("timeout must be positive minutes or seconds")
    if not args.run or args.run == "^$":
        parser.error("select real tests; empty execution is not a compile check")
    go = shutil.which("go")
    if go is None or not PSQL.is_file():
        raise RuntimeError("required Go/PostgreSQL client unavailable")
    STATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    STATE.chmod(0o700)
    probe = subprocess.run(
        ["git", "-C", str(STATE), "rev-parse", "--show-toplevel"],
        capture_output=True, text=True,
    )
    if probe.returncode == 0 or probe.stdout.strip():
        raise RuntimeError("execution state inherits a Git repository")
    lock = (STATE / "package.lock").open("a")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        raise RuntimeError("another owned database suite is running; serialize packages") from None

    run_id = args.label + "-" + uuid.uuid4().hex[:8]
    run_dir = STATE / run_id
    run_dir.mkdir(mode=0o700)
    home = run_dir / "home"
    temporary = run_dir / "tmp"
    home.mkdir(mode=0o700)
    temporary.mkdir(mode=0o700)
    cache = subprocess.check_output(
        [go, "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"], text=True,
    )
    env = {
        "PATH": os.environ["PATH"], "HOME": str(home),
        "TMPDIR": str(temporary), "TMP": str(temporary), "TEMP": str(temporary),
        "LANG": "en_US.UTF-8", "APP_ENV": "test", "ANALYTICS_DISABLED": "true",
        "JWT_SECRET": "knowledge-automation-isolated-test-signing-secret",
        "MULTICA_KNOWLEDGE_CONTEXT_ENABLED": "false",
        "MULTICA_MESSAGING_INTEGRATIONS_ENABLED": "false",
        "MULTICA_VCS_INTEGRATION_ENABLED": "false",
        **json.loads(cache),
    }
    redis_proof = None
    if args.redis_allocation:
        env["REDIS_TEST_URL"], redis_proof = redis_allocation(args.redis_allocation, run_dir)
    parsed = database_url()
    pg_env = {
        **env, "PGHOST": parsed.hostname, "PGPORT": str(parsed.port or 5432),
        "PGUSER": urllib.parse.unquote(parsed.username or ""),
        "PGPASSWORD": urllib.parse.unquote(parsed.password or ""),
        "PGCONNECT_TIMEOUT": "5", "PGDATABASE": "postgres",
    }
    for key, value in urllib.parse.parse_qsl(parsed.query):
        if key == "sslmode":
            pg_env["PGSSLMODE"] = value

    def sql(statement, database="postgres"):
        result = subprocess.run(
            [str(PSQL), "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", statement],
            env={**pg_env, "PGDATABASE": database}, capture_output=True, text=True, timeout=30,
        )
        if result.returncode:
            error_path = run_dir / "database-error.log"
            error_path.write_text(result.stderr)
            error_path.chmod(0o600)
            raise RuntimeError("database command failed; private diagnostics retained")
        return result.stdout.strip()

    cluster = json.loads(sql("SELECT json_build_object('database',current_database(),"
        "'version',version(),'max_connections',current_setting('max_connections')::int,"
        "'connections',(SELECT count(*) FROM pg_stat_activity))"))
    if cluster["database"] != "postgres" or cluster["max_connections"] - cluster["connections"] < 30:
        raise RuntimeError("unexpected cluster identity or insufficient shared connection headroom")
    name = "mca_knowledge_" + PACKAGES[args.package] + "_" + uuid.uuid4().hex[:12]
    sql('CREATE DATABASE "' + name + '"')
    if sql("SELECT current_database()", name) != name:
        raise RuntimeError("new database identity mismatch")
    query = urllib.parse.urlencode([(k, v) for k, v in urllib.parse.parse_qsl(parsed.query)
                                  if k == "sslmode"])
    env["DATABASE_URL"] = urllib.parse.urlunparse(parsed._replace(path="/" + name, query=query))
    env["MULTICA_KNOWLEDGE_MATRIX_DATABASE"] = name
    save(run_dir / "environment.json", env)
    record = {
        "run_id": run_id, "database": name, "package": args.package,
        "test_pattern": args.run, "run_directory": str(run_dir),
        "state": "allocated", "cluster_before": cluster,
        "go_version": subprocess.check_output([go, "version"], text=True).strip(),
        "source_sha256_before": source_fingerprint(),
        "head": subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"], text=True).strip(),
        "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "real_model_credentials_inherited": False, "agent_cli_guard": True,
        "launcher_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "redis_ownership_before": redis_proof, "ambient_redis_inherited": False,
        "ambient_redis_present": "REDIS_TEST_URL" in os.environ,
    }
    evidence = RESEARCH / (run_id + ".json")
    save(evidence, record)
    migration_command = [go, "-C", str(ROOT / "server"), "run", "./cmd/migrate", "up"]
    with (run_dir / "migrate.log").open("w") as log:
        migration = subprocess.run(migration_command, env=env, stdout=log, stderr=subprocess.STDOUT)
    record["migration_exit"] = migration.returncode
    if migration.returncode:
        record["state"] = "migration_failed"
        save(evidence, record)
        print(json.dumps({"evidence": str(evidence), "state": record["state"]}))
        return migration.returncode
    record["migration_ledger"] = sql("SELECT count(*)::text || ':' || max(version)::text FROM schema_migrations", name)
    if sql("SELECT current_database()", name) != name:
        raise RuntimeError("test database identity mismatch")
    command = ["bash", str(ROOT / "scripts/go-test-with-agent-cli-guard.sh"), "--", go,
               "-C", str(ROOT / "server"), "test", "-json", "-count=1", "-p", "1",
               "-parallel", "2", "-timeout=" + args.timeout]
    if args.race:
        command.append("-race")
    command.extend([args.package, "-run", args.run])
    record.update(state="running", command=command)
    save(evidence, record)
    print(json.dumps({"evidence": str(evidence), "state": "running", "database": name}), flush=True)
    with (run_dir / "tests.jsonl").open("w") as log:
        process = subprocess.run(command, env=env, stdout=log, stderr=subprocess.STDOUT)
    inventory = {"run": [], "pass": [], "fail": [], "skip": []}
    for line in (run_dir / "tests.jsonl").read_text().splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("Test") and event.get("Action") in inventory:
            inventory[event["Action"]].append(event["Test"])
    record.update(
        state="complete" if process.returncode == 0 and inventory["run"] else "failed",
        test_exit=process.returncode, tests=inventory,
        source_sha256_after=source_fingerprint(),
        connections_after=int(sql("SELECT count(*) FROM pg_stat_activity WHERE datname='" + name + "'")),
        finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
    )
    record["source_stable"] = record["source_sha256_before"] == record["source_sha256_after"]
    if args.redis_allocation:
        try:
            _, record["redis_ownership_after"] = redis_allocation(args.redis_allocation, run_dir)
        except (RuntimeError, subprocess.SubprocessError) as error:
            record["state"] = "failed"
            record["redis_ownership_error"] = str(error)
            save(evidence, record)
            raise
    save(evidence, record)
    print(json.dumps({"evidence": str(evidence), "state": record["state"],
                      "test_counts": {key: len(value) for key, value in inventory.items()},
                      "source_stable": record["source_stable"]}))
    return process.returncode or (0 if inventory["run"] else 2)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(2)
