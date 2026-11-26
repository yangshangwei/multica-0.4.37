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
EVIDENCE = Path(__file__).resolve().parent / "f2-test-environments"
MANIFEST = Path(__file__).with_name("f2-owned-files.json")
PSQL = "/opt/homebrew/opt/libpq/bin/psql"
PACKAGES = {"./internal/handler", "./internal/service", "./cmd/server", "./cmd/multica"}


def read_json(path, error):
    try:
        return json.loads(Path(path).read_text())
    except (OSError, ValueError, TypeError):
        raise RuntimeError(error) from None


def source_hashes():
    manifest = read_json(MANIFEST, "F2 source ownership manifest required")
    paths = manifest.get("files") if isinstance(manifest, dict) else None
    if not isinstance(paths, list) or not paths:
        raise RuntimeError("F2 source inventory must be nonempty")
    hashes, resolved = {}, set()
    for name in paths:
        if (not isinstance(name, str) or not name or "\\" in name or
                any(ord(char) < 32 for char in name) or
                Path(name).is_absolute() or ".." in Path(name).parts or
                Path(name).as_posix() != name or name in hashes):
            raise RuntimeError("F2 source inventory has an invalid or duplicate path")
        try:
            path = (ROOT / name).resolve(strict=True)
            path.relative_to(ROOT.resolve())
            if not path.is_file() or path in resolved:
                raise ValueError()
            hashes[name] = hashlib.sha256(path.read_bytes()).hexdigest()
            resolved.add(path)
        except (OSError, ValueError):
            raise RuntimeError("F2 source inventory file is missing, duplicated or outside root") from None
    return hashes


def write_json(path, value):
    with path.open("x") as f:
        json.dump(value, f, indent=2)
        f.write("\n")


def connection_env(url):
    try:
        u = urllib.parse.urlsplit(url)
        if (u.scheme not in {"postgres", "postgresql"} or
                u.hostname not in {"localhost", "127.0.0.1", "::1"}):
            raise ValueError()
        return dict(PGHOST=u.hostname, PGPORT=str(u.port or 5432),
                    PGUSER=urllib.parse.unquote(u.username or ""),
                    PGPASSWORD=urllib.parse.unquote(u.password or ""),
                    PGDATABASE=urllib.parse.unquote(u.path.removeprefix("/")),
                    PGCONNECT_TIMEOUT="5", PGSSLMODE="disable")
    except (TypeError, ValueError):
        raise RuntimeError("Invalid local database configuration") from None


def query(url, sql):
    env = {"PATH": os.environ["PATH"], **connection_env(url)}
    try:
        result = subprocess.run([PSQL, "-X", "-At", "-v", "ON_ERROR_STOP=1", "-c", sql],
                                env=env, capture_output=True, text=True, timeout=15)
    except (OSError, subprocess.SubprocessError):
        raise RuntimeError("Owned database query unavailable; connection details withheld") from None
    if result.returncode:
        raise RuntimeError("Owned database query failed; no connection details emitted")
    return result.stdout.strip()


def allocate(label, package):
    assert package in PACKAGES and re.fullmatch(r"[a-z0-9_-]+", label)
    frozen_sources = source_hashes()
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
    database = "f2_" + label.replace("-", "_")[:25] + "_" + uuid.uuid4().hex[:8]
    assert re.fullmatch(r"[a-z0-9_]+", database)
    assert query(admin, "SELECT count(*) FROM pg_database WHERE datname='" + database + "'") == "0"
    private = Path(tempfile.mkdtemp(prefix="multica-f2-")).resolve()
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
              "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "source_sha256": frozen_sources}
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


def expected_inventory(path, package, hashes, regex):
    value = read_json(path, "Explicit expected test inventory required")
    names = value.get("tests") if isinstance(value, dict) else None
    if (not isinstance(value, dict) or set(value) != {"package", "tests"} or
            value["package"] != package or not isinstance(names, list) or not names or
            any(not isinstance(name, str) or
                re.fullmatch(r"Test[A-Z0-9_][A-Za-z0-9_]*", name) is None or
                name == "TestMain" for name in names) or len(set(names)) != len(names)):
        raise RuntimeError("Expected inventory must bind one package and unique top-level test names")
    names = sorted(names)
    selected_regex = "^(" + "|".join(names) + ")$"
    if regex is not None and regex != selected_regex:
        raise RuntimeError("Test regex must exactly equal the regex derived from the expected inventory")
    declared = set()
    prefix = "server/" + package.removeprefix("./") + "/"
    for name in hashes:
        if name.startswith(prefix) and "/" not in name[len(prefix):] and name.endswith("_test.go"):
            declared.update(re.findall(r"(?m)^func\s+(Test[A-Za-z0-9_]+)\s*\(\s*\w+\s+\*testing\.T\s*\)",
                                       (ROOT / name).read_text()))
    if not set(names).issubset(declared):
        raise RuntimeError("Expected test names must be declared in the package's source inventory")
    try:
        module = re.search(r"(?m)^module\s+(\S+)\s*$", (ROOT / "server/go.mod").read_text())
        if module is None:
            raise ValueError()
        package_id = module.group(1).strip('"') + package[1:]
        inventory_hash = hashlib.sha256(Path(path).read_bytes()).hexdigest()
    except (OSError, ValueError):
        raise RuntimeError("Test package identity or inventory cannot be read") from None
    return names, selected_regex, package_id, inventory_hash


def inspect_test_events(output, expected, package_id):
    results, runs, passes, unexpected = [], {}, {}, set()
    failures, package_passes = set(), 0
    for line in output.splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if not isinstance(event, dict) or event.get("Action") not in {"run", "pass", "fail", "skip"}:
            continue
        name, action = event.get("Test"), event["Action"]
        if event.get("Package") != package_id:
            failures.add("unexpected_package_events")
        if name is None:
            if action == "pass" and event.get("Package") == package_id:
                package_passes += 1
            elif action in {"fail", "skip"}:
                failures.add("package_failed_or_skipped")
            continue
        if not isinstance(name, str) or not name:
            failures.add("invalid_test_event")
            continue
        root = name.split("/", 1)[0]
        if root not in expected:
            unexpected.add(name)
        if action in {"pass", "fail", "skip"}:
            results.append({"test": name, "action": action})
        if action in {"fail", "skip"}:
            failures.add("test_failed_or_skipped")
        if name == root and event.get("Package") == package_id:
            if action == "run":
                runs[root] = runs.get(root, 0) + 1
            elif action == "pass":
                passes[root] = passes.get(root, 0) + 1
    missing = sorted(name for name in expected if runs.get(name) != 1 or passes.get(name) != 1)
    if missing:
        failures.add("expected_test_run_or_pass_missing")
    if unexpected:
        failures.add("unexpected_tests")
    if package_passes != 1:
        failures.add("package_pass_missing_or_duplicated")
    return results, sorted(failures), missing, sorted(unexpected)


def redact_output(output, url):
    parsed = urllib.parse.urlsplit(url)
    secrets = {url, parsed.password or "", urllib.parse.unquote(parsed.password or "")}
    for secret in sorted(filter(None, secrets), key=len, reverse=True):
        for value in {secret, json.dumps(secret)[1:-1]}:
            output = output.replace(value, "[redacted]")
    return output


def run(label, regex, race, expected_tests_file):
    if not isinstance(label, str) or re.fullmatch(r"[a-z0-9_-]+", label) is None:
        raise RuntimeError("Invalid allocation label")
    record_path = EVIDENCE / (label + ".json")
    record = read_json(record_path, "Recorded task allocation required")
    if (not isinstance(record, dict) or record.get("label") != label or
            record.get("state") != "ready" or record.get("package") not in PACKAGES or
            not isinstance(record.get("database"), str) or
            re.fullmatch("f2_" + re.escape(label.replace("-", "_")[:25]) + r"_[0-9a-f]{8}",
                         record["database"]) is None):
        raise RuntimeError("Invalid task allocation identity or state")
    hashes = source_hashes()
    if (record.get("runner_sha256") != hashlib.sha256(Path(__file__).read_bytes()).hexdigest() or
            record.get("source_sha256") != hashes):
        raise RuntimeError("Allocated runner/source identity changed; allocate a fresh run")
    expected, regex, package_id, inventory_hash = expected_inventory(
        expected_tests_file, record["package"], hashes, regex)
    env = read_json(record.get("private_environment"), "Private task environment required")
    if (not isinstance(env, dict) or not env.get("PATH") or not env.get("DATABASE_URL") or
            any(not isinstance(key, str) or not isinstance(value, str) for key, value in env.items()) or
            connection_env(env["DATABASE_URL"])["PGDATABASE"] != record["database"]):
        raise RuntimeError("Private environment does not match the allocated database")
    if query(env["DATABASE_URL"], "SELECT current_database()") != record["database"]:
        raise RuntimeError("Actual database does not match the task allocation")
    argv = ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "go", "-C", "server", "test",
            record["package"], "-count=1", "-json", "-timeout=15m", "-run", regex]
    if race:
        argv.append("-race")
    log_path = EVIDENCE / (label + "-tests.jsonl")
    if log_path.exists():
        raise RuntimeError("Test log already exists; previous evidence must be preserved")
    process_error, test_exit, output = None, None, ""
    # Reserve the log before starting a process, preventing two invocations of
    # one ready allocation from both running its database-backed test package.
    with log_path.open("x") as log:
        record.update(state="running", command=argv, test_source_sha256=hashes,
                      expected_tests=expected, expected_package=package_id,
                      expected_test_inventory_sha256=inventory_hash)
        record_path.write_text(json.dumps(record, indent=2) + "\n")
        try:
            result = subprocess.run(argv, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True)
            test_exit, output = result.returncode, result.stdout or ""
        except (OSError, subprocess.SubprocessError):
            process_error = "test_process_unavailable"
        except KeyboardInterrupt:
            process_error = "test_process_interrupted"
        events, inventory_failures, missing, unexpected = inspect_test_events(output, expected, package_id)
        log.write(redact_output(output, env["DATABASE_URL"]))
    failures = list(inventory_failures)
    if process_error:
        failures.append(process_error)
    if test_exit != 0:
        failures.append("test_exit_not_zero")
    final_hashes = None
    try:
        final_hashes = source_hashes()
        if final_hashes != hashes:
            failures.append("source_identity_changed")
    except RuntimeError:
        failures.append("final_source_identity_unavailable")
    accepted = not failures
    runner_exit = 0 if accepted else (test_exit or (130 if process_error == "test_process_interrupted" else 3))
    record.update(state="complete", exit_code=test_exit, test_exit_code=test_exit,
                  accepted=accepted, acceptance_status="passed" if accepted else "failed",
                  acceptance_exit_code=runner_exit, acceptance_failures=sorted(set(failures)),
                  named_test_results=events, missing_expected_tests=missing,
                  unexpected_tests=unexpected, final_source_sha256=final_hashes,
                  inventory_complete=not inventory_failures,
                  log_sha256=hashlib.sha256(log_path.read_bytes()).hexdigest())
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({"label": label, "exit": test_exit, "accepted": accepted,
                      "acceptance_failures": record["acceptance_failures"],
                      "counts": {s: sum(e["action"] == s for e in events) for s in ["pass", "fail", "skip"]},
                      "failed": [e["test"] for e in events if e["action"] == "fail"], "log": str(log_path)}))
    return runner_exit


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="action", required=True)
    a = sub.add_parser("allocate")
    a.add_argument("label")
    a.add_argument("package", choices=sorted(PACKAGES))
    r = sub.add_parser("run")
    r.add_argument("label")
    r.add_argument("--expected-tests-file", required=True,
                   help='JSON object with exact "package" and top-level "tests" list')
    r.add_argument("--regex", help="Optional exact regex derived from the test inventory")
    r.add_argument("--race", action="store_true")
    args = parser.parse_args()
    try:
        result = (allocate(args.label, args.package) if args.action == "allocate" else
                  run(args.label, args.regex, args.race, args.expected_tests_file))
    except (RuntimeError, OSError, ValueError, KeyError, AssertionError):
        # Connection/configuration exception messages may contain credentials.
        parser.exit(2, "F2 test runner refused the invalid or unavailable allocation; no test acceptance recorded.\n")
    raise SystemExit(result)
