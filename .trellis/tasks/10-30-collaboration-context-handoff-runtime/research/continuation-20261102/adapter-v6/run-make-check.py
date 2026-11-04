#!/usr/bin/env python3
"""Coordinator-installed full-check launcher; configuration contains no secrets."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def file_hash(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def parse_environment(text):
    """Read the task's literal shell-quoted assignments without evaluation."""
    lexer = shlex.shlex(text, posix=True)
    lexer.whitespace_split = True
    lexer.commenters = "#"
    values = {}
    try:
        for token in lexer:
            if token == "export":
                continue
            key, separator, value = token.partition("=")
            require(separator and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key),
                    "Task environment contains unsupported assignment syntax")
            require("\0" not in value, "Task environment contains an invalid value")
            values[key] = value
    except ValueError:
        raise RuntimeError("Task environment contains invalid quoting") from None
    return values


def write_canonical_environment(path, values):
    # The repository reads ENV_FILE as shell assignments. Quote every literal
    # value so a later loader cannot execute substitutions from the source.
    content = "".join(f"{key}={shlex.quote(value)}\n" for key, value in values.items())
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as sink:
        sink.write(content)


def validate_pins(config, clearance, source, adapters, launcher):
    require(clearance.get("verdict") in ("pass", "pass-with-notes") and
            clearance.get("blocking_findings_open") == 0,
            "Independent review clearance is missing or blocking")
    hashes = {name: file_hash(adapters / name) for name in ("go", "pnpm")}
    require(hashes == config["adapter_sha256"] == clearance.get("adapter_sha256"),
            "Actual adapters do not match the independently reviewed hashes")
    launcher_hash = file_hash(launcher)
    require(launcher_hash == config["launcher_sha256"] == clearance.get("launcher_sha256"),
            "Actual launcher does not match the independently reviewed hash")
    require(file_hash(source) == config["source_env_sha256"],
            "Task source environment changed after registration")
    return hashes, launcher_hash


def main():
    launcher = Path(__file__).resolve()
    root = launcher.parents[3]
    out = root / ".omx/private/final-candidate"
    config = json.loads((out / "launcher-config.json").read_text())
    require(root == Path(config["candidate_root"]).resolve(), "Unexpected candidate root")
    record_path = out / "make-check.json"
    require(not record_path.exists(), "Never overwrite previous check evidence")
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    require(head == config["candidate_sha"], "Candidate HEAD changed after registration")
    require(not subprocess.check_output(["git", "status", "--porcelain"], cwd=root, text=True).strip(),
            "Final candidate must start clean")
    clearance_path = out / "independent-review-clearance.json"
    require(clearance_path.is_file(), "Independent adapter and current RKI6 result review is required")
    clearance = json.loads(clearance_path.read_text())
    source = Path(config["source_env"]).resolve()
    require(source.is_relative_to(out) and source.is_file(), "Unexpected task source environment path")
    require(source.stat().st_uid == os.geteuid() and source.stat().st_mode & 0o777 == 0o600,
            "Task source environment must be private and owned")
    adapters = Path(config["adapters_root"]).resolve()
    adapter_hashes, launcher_hash = validate_pins(config, clearance, source, adapters, launcher)
    source_values = parse_environment(source.read_text())
    require(not any(source_values.get(key) for key in
                    ("MULTICA_LLM_API_KEY", "MULTICA_LLM_BASE_URL", "MULTICA_RUN_REAL_AGENT_SMOKE")),
            "Full-check source must not enable real Provider execution")
    denied_keys = {"MULTICA_TOKEN", "MULTICA_SERVER_URL", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID",
                   "MULTICA_AGENT_NAME", "MULTICA_RUN_REAL_AGENT_SMOKE", "ELECTRON_RUN_AS_NODE"}
    source_values = {key: value for key, value in source_values.items()
                     if not key.startswith(("MULTICA_TASK_", "GIT_CONFIG_")) and key not in denied_keys}
    task_home = Path(config["task_home"]).resolve()
    require(task_home.is_relative_to(out) and task_home.is_dir() and
            task_home.stat().st_uid == os.geteuid() and task_home.stat().st_mode & 0o777 == 0o700,
            "Task HOME must be a private owned directory within the candidate evidence scope")
    source_values.update(HOME=str(task_home), GOCACHE=config["go_cache"], GOMODCACHE=config["go_mod_cache"],
                         GOPATH=config["go_path"], PLAYWRIGHT_BROWSERS_PATH=config["playwright_browsers_path"])
    # These belong to the task API. The v6 Go adapter removes them only from
    # executable test children, preserving each test's own env/default behavior.
    source_values.update(DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1")
    env = os.environ.copy()
    env.update(source_values)
    for key in list(env):
        if key.startswith(("MULTICA_TASK_", "GIT_CONFIG_")) or key in denied_keys:
            env.pop(key, None)
    require(not env.get("MULTICA_LLM_API_KEY") and not env.get("MULTICA_LLM_BASE_URL"),
            "Full check must not inherit real Provider configuration")
    allocation_path = Path(config["build_temp_allocation"]).resolve()
    require(allocation_path.is_relative_to(out), "Unexpected build-temp allocation record")
    allocation = json.loads(allocation_path.read_text())
    task_temp = Path(allocation["directory"]).resolve()
    require(task_temp.is_dir() and task_temp.stat().st_uid == os.geteuid() and
            task_temp.stat().st_mode & 0o777 == 0o700,
            "Build temporary directory must be private and owned")
    require(allocation.get("owner_uid") == os.geteuid() and allocation.get("mode") == "0700",
            "Build-temp ownership does not match its allocation record")
    renderer = config["renderer_url"]
    require(source_values.get("COLLABORATION_ELECTRON_RENDERER_URL") == renderer and
            source_values.get("CHANGELOG_ELECTRON_RENDERER_URL") == renderer,
            "Native renderer configuration differs from the registered source")
    env.update(PATH=str(adapters) + ":/opt/homebrew/opt/libpq/bin:" + env["PATH"],
               MULTICA_TASK_REAL_GO=config["real_go"], MULTICA_TASK_REAL_PNPM=config["real_pnpm"],
               MULTICA_TASK_PSQL=config["psql"], MULTICA_TASK_PACKAGE_DB_MODE="1",
               MULTICA_TASK_PACKAGE_DB_EVIDENCE=str(out / "go-packages"),
               MULTICA_TASK_CANDIDATE_ROOT=str(root), MULTICA_TASK_RENDERER_URL=renderer,
               MULTICA_DEV_HOME=str(out / "dev-registry"), MULTICA_DEV_TMPDIR=str(task_temp),
               MULTICA_DEV_PROFILES_HOME=str(out / "profiles"),
               MULTICA_DEV_WORKSPACES_PARENT=str(out / "workspaces"), NEXT_TELEMETRY_DISABLED="1",
               GOPROXY="file:///Users/artisan/go/pkg/mod/cache/download", GOSUMDB="off", GOTOOLCHAIN="local")
    require(subprocess.run(["git", "-C", str(task_temp), "rev-parse", "--show-toplevel"], env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE).returncode != 0,
            "Build TMPDIR must be outside every Git worktree")
    canonical = out / "check-source.canonical.env"
    write_canonical_environment(canonical, source_values)
    env["ENV_FILE"] = str(canonical)
    command = ["make", "check", "ENV_FILE=" + str(canonical)]
    record = {"candidate_sha": head, "candidate_root": str(root),
              "command": ["make", "check", "ENV_FILE=<private canonical task env>"],
              "adapter_sha256": adapter_hashes, "launcher_sha256": launcher_hash,
              "source_env_sha256": file_hash(source), "canonical_source_env_sha256": file_hash(canonical),
              "database_policy": "separate newly migrated task clone per Go package; retained package failures",
              "runtime_API_pool": {"max": 10, "min": 1}, "Go_test_API_pool_env": "removed at v6 executable-test boundary",
              "renderer": renderer, "knowledge_default": False, "real_provider_allowed": False,
              "task_HOME": str(task_home), "Go_cache_paths": {key: env[key] for key in ("GOCACHE", "GOMODCACHE", "GOPATH")},
              "Playwright_browser_path": env["PLAYWRIGHT_BROWSERS_PATH"],
              "build_temp_directory": str(task_temp), "build_temp_git_root_absent": True,
              "state": "starting", "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    descriptor = os.open(record_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as sink:
        sink.write(json.dumps(record, indent=2) + "\n")
    log = out / "make-check.log"
    descriptor = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as sink:
        process = subprocess.Popen(command, cwd=root, env=env, stdout=sink, stderr=subprocess.STDOUT)
        record.update(state="running", process_id=process.pid)
        record_path.write_text(json.dumps(record, indent=2) + "\n")
        code = process.wait()
    record.update(state="complete", exit_code=code,
                  finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(), log_sha256=file_hash(log))
    record_path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps(record))
    return code


if __name__ == "__main__":
    raise SystemExit(main())
