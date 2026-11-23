import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
from urllib.parse import urlsplit


EXPECTED_ROOT = Path("/Volumes/artisan/code/2026/mc-collaboration-final4-20261102")
ADAPTERS = Path("/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v6/check-bin")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def parse_generated_environment(source):
    values = {}
    for number, line in enumerate(source.splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, separator, raw = line.partition("=")
        require(separator and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key), "Invalid task environment assignment")
        require("$(" not in raw and "`" not in raw, "Task environment command substitution is forbidden")
        try:
            tokens = shlex.split(raw, posix=True)
        except ValueError:
            raise RuntimeError("Invalid task environment value at line " + str(number)) from None
        require(len(tokens) <= 1, "Task environment value requires generated quoting")
        value = tokens[0] if tokens else ""
        # Shell quoting does not protect a value from Make's later expansion.
        require("$" not in value and all(char.isprintable() for char in value),
                "Task environment normalized values cannot contain dollar signs or control/newline characters")
        values[key] = value
    return values


def renderer_url(values):
    raw = values.get("COLLABORATION_ELECTRON_RENDERER_URL", "")
    require(raw == values.get("CHANGELOG_ELECTRON_RENDERER_URL") and raw, "Task renderer URLs must agree")
    try:
        url = urlsplit(raw)
        valid = (url.scheme in ("http", "https") and url.hostname in ("127.0.0.1", "localhost", "::1")
                 and url.username is None and url.password is None and url.port != 0)
    except ValueError:
        valid = False
    require(valid, "Task renderer URL requires credential-free loopback HTTP(S)")
    return raw


def main():
    launcher = Path(__file__).resolve()
    root = launcher.parents[3]
    require(root == EXPECTED_ROOT, "Unexpected candidate checkout")
    out = root / ".omx/private/final-candidate"
    record_path = out / "make-check-v6.json"
    log_path = out / "make-check-v6.log"
    safe_source = out / "check-source-v6.safe.env"
    require(not record_path.exists() and not log_path.exists() and not safe_source.exists(), "Never overwrite previous check evidence")
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    require(not subprocess.check_output(["git", "status", "--porcelain"], cwd=root, text=True).strip(), "Final candidate must start clean")
    clearance_path = out / "independent-review-clearance-v6.json"
    require(clearance_path.is_file(), "Pending independent v6 and current RKI6 result review")
    clearance = json.loads(clearance_path.read_text())
    require(clearance.get("verdict") in ("pass", "pass-with-notes") and clearance.get("blocking_findings_open") == 0,
            "Independent review has open blocking findings")
    source = out / "check-source.env"
    source_bytes = source.read_bytes()
    adapter_hashes = {name: sha256((ADAPTERS / name).read_bytes()) for name in ("go", "pnpm")}
    launcher_hash = sha256(launcher.read_bytes())
    source_hash = sha256(source_bytes)
    require(clearance.get("candidate_sha") == head, "Candidate differs from independent review")
    require(clearance.get("adapter_sha256") == adapter_hashes, "Actual adapter differs from independent review")
    require(clearance.get("launcher_sha256") == launcher_hash, "Actual launcher differs from independent review")
    require(clearance.get("source_env_sha256") == source_hash, "Actual source configuration differs from independent review")

    # Parse the reviewed source as data. The unchanged make scripts receive a
    # canonical, shell-quoted copy, never the original unparsed dotenv bytes.
    values = parse_generated_environment(source_bytes.decode())
    renderer = renderer_url(values)
    task_home = out / "full-check-home"
    values.update(HOME=str(task_home), GOCACHE="/Volumes/artisan/dev-cache/go-build",
                  GOMODCACHE="/Users/artisan/go/pkg/mod", GOPATH="/Users/artisan/go")
    env = {**os.environ, **values}
    for key in list(env):
        if key.startswith(("MULTICA_TASK_", "GIT_CONFIG_")) or key in (
            "MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID",
            "MULTICA_AGENT_NAME", "MULTICA_RUN_REAL_AGENT_SMOKE", "ELECTRON_RUN_AS_NODE",
        ):
            env.pop(key, None)
            values.pop(key, None)
    require(not env.get("MULTICA_LLM_API_KEY") and not env.get("MULTICA_LLM_BASE_URL"), "External model configuration is forbidden")
    values.update(DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1")
    env.update(
        PATH=str(ADAPTERS) + ":/opt/homebrew/opt/libpq/bin:" + env["PATH"],
        MULTICA_TASK_REAL_GO="/opt/homebrew/bin/go", MULTICA_TASK_REAL_PNPM="/opt/homebrew/bin/pnpm",
        MULTICA_TASK_PSQL="/opt/homebrew/opt/libpq/bin/psql", MULTICA_TASK_PACKAGE_DB_MODE="1",
        MULTICA_TASK_PACKAGE_DB_EVIDENCE=str(out / "go-packages-v6"), MULTICA_TASK_CANDIDATE_ROOT=str(root),
        MULTICA_TASK_RENDERER_URL=renderer, MULTICA_DEV_HOME=str(out / "dev-registry"),
        MULTICA_DEV_TMPDIR="/private/tmp/multica-final-check4-yxg6alb4", MULTICA_DEV_PROFILES_HOME=str(out / "profiles"),
        MULTICA_DEV_WORKSPACES_PARENT=str(out / "workspaces"), NEXT_TELEMETRY_DISABLED="1",
        GOPROXY="file:///Users/artisan/go/pkg/mod/cache/download", GOSUMDB="off", GOTOOLCHAIN="local",
        DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1", ENV_FILE=str(safe_source),
    )
    allocation = json.loads((out / "build-temp-allocation.json").read_text())
    task_temp = Path(env["MULTICA_DEV_TMPDIR"])
    require(str(task_temp) == allocation["directory"] and str(task_temp).startswith("/private/tmp/multica-final-check4-"),
            "Build temporary directory differs from allocation")
    require(task_temp.is_dir() and task_temp.stat().st_uid == os.geteuid() and task_temp.stat().st_mode & 0o777 == 0o700,
            "Build temporary directory must be task-owned mode0700")
    require(subprocess.run(["git", "-C", str(task_temp), "rev-parse", "--show-toplevel"], env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE).returncode != 0,
            "Build temporary directory must be outside every Git worktree")
    require(not task_home.is_symlink(), "Full-check home must not redirect outside task output")
    task_home.mkdir(mode=0o700, exist_ok=True)
    require(task_home.stat().st_uid == os.geteuid() and task_home.stat().st_mode & 0o777 == 0o700,
            "Full-check home must be task-owned mode0700")

    safe_bytes = ("".join(key + "=" + shlex.quote(value) + "\n" for key, value in values.items())).encode()
    descriptor = os.open(safe_source, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as target:
        target.write(safe_bytes)
    command = ["make", "check", "ENV_FILE=" + str(safe_source)]
    record = {
        "candidate_sha": head, "candidate_root": str(root), "command": ["make", "check", "ENV_FILE=<canonical private task env>"],
        "adapter_sha256": adapter_hashes, "launcher_sha256": launcher_hash, "source_env_sha256": source_hash,
        "effective_env_sha256": sha256(safe_bytes), "environment_loading": "data parsing plus canonical shell-quoted task copy",
        "environment_value_contract": "normalized values contain no dollar signs or control/newline characters",
        "database_policy": "a separate newly migrated task clone per Go package; never business default; package databases and failures retained",
        "api_pool": {"max": 10, "min": 1}, "Go_test_pool_overrides": "removed only in actual test adapter branch",
        "home": str(task_home), "Go_cache_paths": {key: env[key] for key in ("GOCACHE", "GOMODCACHE", "GOPATH")},
        "renderer": renderer, "knowledge_default": False, "real_provider_allowed": False,
        "build_temp_directory": str(task_temp), "build_temp_git_root_absent": True,
        "state": "running", "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    }
    with log_path.open("x") as sink:
        process = subprocess.Popen(command, cwd=root, env=env, stdout=sink, stderr=subprocess.STDOUT)
        record["process_id"] = process.pid
        record_path.write_text(json.dumps(record, indent=2) + "\n")
        code = process.wait()
    record.update(state="complete", exit_code=code, finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  log_sha256=sha256(log_path.read_bytes()))
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps(record))
    return code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, ValueError, OSError) as error:
        message = str(error) if isinstance(error, RuntimeError) else "Required task file or data is unavailable"
        print("REFUSED: " + message)
        raise SystemExit(2)
