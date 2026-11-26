#!/usr/bin/env python3
"""Codex app-server executable wrapper for the isolated security acceptance task.

This leaves the native JSON-RPC byte stream and generated AGENTS.md unchanged.
The shell's Multica executable is a file-spool transport to the actual host CLI.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re
import select
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import threading
import time
import uuid

import multica_spool
import runtime_guard

SCRIPTS = Path(__file__).resolve().parent
DEFAULT_IMAGE = "sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f"
LABEL = "multica.task=secrem-v2-real-20261120"


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def rooted(path, roots, existing=True):
    path = Path(path).absolute()
    if path.resolve(strict=existing) != path:
        raise ValueError("unresolved or symlinked execution path")
    if not any(path == root or root in path.parents for root in roots):
        raise ValueError("execution path is outside this trial")
    return path


def checked_config(path):
    path = Path(path).resolve(strict=True)
    if not path.is_file() or stat.S_IMODE(path.stat().st_mode) & 0o077:
        raise ValueError("transport configuration is not private")
    config = json.loads(path.read_text())
    if config.get("schema_version") != 1:
        raise ValueError("transport schema rejected")
    for key in ["evidence_root", "allowed_workspaces_root", "claim_manifest_dir", "cli_path"]:
        value = Path(config[key])
        if not value.is_absolute() or value.resolve(strict=True) != value:
            raise ValueError("transport path rejected")
    if sha(config["cli_path"]) != config["cli_sha256"]:
        raise ValueError("compiled CLI differs")
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", config.get("image", DEFAULT_IMAGE)):
        raise ValueError("image must be immutable")
    return config


def copy_regular_tree(source, destination):
    """Copy only regular descendants; no provider-controlled symlink is followed."""
    source, destination = Path(source), Path(destination)
    if not source.exists():
        if source.is_symlink():
            raise ValueError("dangling context symlink")
        return
    if source.is_symlink() or not source.is_dir():
        raise ValueError("context source must be a real directory")
    if destination.is_symlink():
        raise ValueError("context destination symlink")
    destination.mkdir(parents=True, exist_ok=True)
    for item in source.iterdir():
        info = item.lstat()
        target = destination / item.name
        if stat.S_ISLNK(info.st_mode) or target.is_symlink():
            raise ValueError("provider context symlink refused")
        if stat.S_ISDIR(info.st_mode):
            copy_regular_tree(item, target)
        elif stat.S_ISREG(info.st_mode):
            source_fd = os.open(item, os.O_RDONLY | os.O_NOFOLLOW)
            try:
                if not stat.S_ISREG(os.fstat(source_fd).st_mode):
                    raise ValueError("context file changed kind")
                flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW
                destination_fd = os.open(target, flags, 0o600)
                try:
                    with os.fdopen(os.dup(source_fd), "rb") as inp, os.fdopen(os.dup(destination_fd), "wb") as out:
                        shutil.copyfileobj(inp, out)
                finally:
                    os.close(destination_fd)
            finally:
                os.close(source_fd)
        else:
            raise ValueError("unsupported context file kind")


def retain_sessions(source, destination, outcome, exit_code):
    try:
        copy_regular_tree(source, destination)
    except (OSError, ValueError):
        return "evidence_failed", 125, True
    return outcome, exit_code, False


def task_session_destination(original_home, manifest, roots):
    candidate = original_home / "sessions"
    if candidate.is_symlink():
        resolved = rooted(candidate.resolve(strict=True), roots)
        # The production preparer uses an issue-specific store. Resolve only
        # this single known preparer link, never links inside provider output.
        if manifest["issue_id"] not in str(resolved):
            raise ValueError("session store is not bound to the current issue")
        return resolved
    if candidate.exists():
        return rooted(candidate, roots)
    rooted(candidate.parent, roots)
    return candidate


def write_codex_config(path, config, cwd, home, proxy_token, requests, responses):
    keys = {
        "model": config["model"],
        "model_provider": "secrem_proxy",
        "model_reasoning_effort": config.get("model_reasoning_effort", "max"),
        "approval_policy": "never",
        "sandbox_mode": "workspace-write",
        "project_doc_max_bytes": max(int(config.get("project_doc_max_bytes", 131072)), len((cwd / "AGENTS.md").read_bytes()) + 32768),
        "check_for_update_on_startup": False,
        "cli_auth_credentials_store": "file",
    }
    lines = []
    for key, value in keys.items():
        lines.append(f"{key} = {json.dumps(value)}")
    lines += [
        "", "[model_providers.secrem_proxy]", 'name = "Security acceptance gateway"',
        "base_url = " + json.dumps(config["model_gateway_url"]), 'wire_api = "responses"',
        "requires_openai_auth = false", "experimental_bearer_token = " + json.dumps(proxy_token),
        "", "[sandbox_workspace_write]", "network_access = false",
        "", "[shell_environment_policy]", 'inherit = "none"',
        "[shell_environment_policy.set]", 'PATH = "/usr/local/bin:/usr/bin:/bin"',
        'HOME = "/home/node"', "CODEX_HOME = " + json.dumps(str(home)),
        "SECREM_ROLE_WORKDIR = " + json.dumps(str(cwd)),
        "SECREM_REQUEST_DIR = " + json.dumps(str(requests)),
        "SECREM_RESPONSE_DIR = " + json.dumps(str(responses)),
    ]
    # The spool does not use these for authorization. They keep ordinary CLI
    # context introspection usable without exposing provider/account credentials.
    for key in ["MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID", "MULTICA_AGENT_NAME", "MULTICA_TASK_ID"]:
        if value := os.environ.get(key):
            lines.append(key + " = " + json.dumps(value))
    path.write_text("\n".join(lines) + "\n")
    path.chmod(0o444)
    return keys["project_doc_max_bytes"]


def registration_from_driver(config):
    # Product buildEnv deliberately removes inherited MULTICA_* keys. The
    # tagged test entry already checks its real gate; retain that decision in a
    # test-private marker which is never copied into the role container.
    if os.environ.get("SECREM_REAL_EXECUTION_AUTHORIZED") == "1":
        os.environ["MULTICA_RUN_REAL_AGENT_SMOKE"] = "1"
    return runtime_guard.load_registration(config["registration_file"], config["registration_sha256"])


def main():
    config = checked_config(os.environ["SECREM_TRANSPORT_CONFIG"])
    args = sys.argv[1:]
    image = config.get("image", DEFAULT_IMAGE)
    if args == ["--version"]:
        # Local metadata probe only; no account, gateway, or real ledger access.
        return subprocess.run(["docker", "run", "--rm", "--pull=never", "--network=none", "--label", LABEL, "--entrypoint", "codex", image, "--version"]).returncode
    if args != ["app-server", "--listen", "stdio://"]:
        raise ValueError("only the fixed native app-server entry is accepted")
    evidence = Path(config["evidence_root"])
    roots = [evidence, Path(config["allowed_workspaces_root"])]
    cwd = rooted(Path.cwd(), roots)
    original_home = rooted(os.environ["CODEX_HOME"], roots)
    task_id = os.environ["MULTICA_TASK_ID"]
    if not re.fullmatch(r"[a-f0-9-]{36}", task_id):
        raise ValueError("task ID rejected")
    manifest_path = Path(config["claim_manifest_dir"]) / (task_id + ".json")
    manifest = json.loads(manifest_path.read_text())
    manifest.setdefault("task_id", manifest.get("id"))
    for field, envkey in [("task_id", "MULTICA_TASK_ID"), ("agent_id", "MULTICA_AGENT_ID"), ("workspace_id", "MULTICA_WORKSPACE_ID")]:
        if manifest.get(field) != os.environ.get(envkey):
            raise ValueError("manifest differs from actual daemon identity")
    if not os.environ.get("MULTICA_TOKEN", "").startswith("mat_"):
        raise ValueError("only original server-claimed task credentials are allowed")
    if not (cwd / "AGENTS.md").is_file():
        raise ValueError("complete generated AGENTS is required")
    run_id = task_id + "-" + uuid.uuid4().hex[:10]
    run = evidence / "transport" / run_id
    run.mkdir(parents=True, mode=0o700)
    sandbox_home = run / "codex-home"
    sandbox_home.mkdir(mode=0o777)
    sandbox_home.chmod(0o777)
    skills = original_home / "skills"
    if skills.is_symlink():
        raise ValueError("skill root must not be a symlink")
    copy_regular_tree(skills, sandbox_home / "skills")
    session_destination = task_session_destination(original_home, manifest, roots)
    copy_regular_tree(session_destination, sandbox_home / "sessions")
    for name in ["sessions", "log", "logs"]:
        (sandbox_home / name).mkdir(mode=0o777, exist_ok=True)
        (sandbox_home / name).chmod(0o777)
    # The process-private home has no link to host account storage.
    proxy_token = Path(config["model_proxy_token_file"]).read_text().strip()
    if not proxy_token or "\n" in proxy_token:
        raise ValueError("proxy credential rejected")
    config_path = run / "config.toml"
    if type(manifest.get("read_only_code")) is not bool:
        raise ValueError("explicit role filesystem mode required")
    if (manifest.get("is_leader_task") or manifest.get("role") in {"security-reviewer", "code-reviewer", "migration-reviewer", "qa-engineer"}) and not manifest["read_only_code"]:
        raise ValueError("coordinator and review code must be read-only")
    for reserved in [".git", ".agents", ".codex", ".aws", ".azure", ".config/gcloud", ".kube", ".ssh", ".gnupg"]:
        reserved_path = cwd / reserved
        if not reserved_path.exists():
            reserved_path.mkdir(mode=0o755, parents=True)
    requests, responses = cwd / (".secrem-requests-" + run_id), cwd / (".secrem-responses-" + run_id)
    budget = write_codex_config(config_path, config, cwd, original_home, proxy_token, requests, responses)
    (sandbox_home / "config.toml").touch(mode=0o644)
    requirements = SCRIPTS.parent / "fixtures/native-requirements.toml"
    shim = run / "multica"
    shim.write_text('#!/bin/sh\nexec python3 /opt/secrem/multica_spool.py "$@"\n')
    shim.chmod(0o555)
    for folder in [requests, responses]:
        if folder.exists():
            raise ValueError("spool directory already exists")
        folder.mkdir(mode=0o777)
        folder.chmod(0o777)
    # The host broker's account-free home/config is never mounted in the role.
    broker_home = run / "broker-home"
    broker_home.mkdir(mode=0o700)
    broker_config = run / "broker-cli-config"
    broker_config.mkdir(mode=0o700)
    broker_env = {"PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "HOME": str(broker_home), "MULTICA_TASK_CONFIG_ROOT": str(broker_config)}
    for key in ["MULTICA_TOKEN", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID", "MULTICA_AGENT_NAME", "MULTICA_TASK_ID", "MULTICA_SERVER_URL"]:
        broker_env[key] = os.environ[key]
    broker = multica_spool.Broker(manifest, cwd, requests, responses, config["cli_path"], broker_env, run / "cli-audit.jsonl")
    broker_thread = threading.Thread(target=broker.run, daemon=True)
    container = "secrem-v2-role-" + uuid.uuid4().hex[:12]
    mounts = [
        (str(cwd), str(cwd), bool(manifest.get("read_only_code", False))),
        (str(requests), str(requests), False), (str(responses), str(responses), True),
        (str(sandbox_home), str(original_home), False),
        (str(config_path), str(original_home / "config.toml"), True),
        (str(requirements), "/etc/codex/requirements.toml", True),
        (str(shim), "/usr/local/bin/multica", True),
        (str(SCRIPTS / "multica_spool.py"), "/opt/secrem/multica_spool.py", True),
    ]
    command = ["docker", "run", "-i", "--rm", "--name", container, "--label", LABEL, "--pull=never", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=unconfined", f"--user={os.getuid()}:{os.getgid()}", "--pids-limit=128", "--memory=1536m", "--network", config["network"], "--tmpfs=/tmp:rw,nosuid,size=128m", f"--tmpfs=/home/node:rw,nosuid,size=32m,uid={os.getuid()},gid={os.getgid()}", "--workdir", str(cwd), "-e", "HOME=/home/node", "-e", "CODEX_HOME=" + str(original_home), "-e", "PATH=/usr/local/bin:/usr/bin:/bin"]
    for source, target, readonly in mounts:
        command += ["--mount", f"type=bind,src={source},dst={target}" + (",readonly" if readonly else "")]
    command += ["--entrypoint", "codex", image, *args]
    runtime_guard.private_path(config["registration_file"])
    registration = registration_from_driver(config)
    if registration["mode"] == "real":
        network = json.loads(subprocess.check_output(["docker", "network", "inspect", config["network"]], text=True))[0]
        if network.get("Internal") is not True:
            raise ValueError("real model network must deny direct external egress")
    elif config["network"] != "none":
        raise ValueError("fake native probes require network none")
    if registration["mode"] == "real":
        policy_path = runtime_guard.private_path(registration["transport_policy_file"])
        if sha(policy_path) != registration["transport_policy_sha256"]:
            raise ValueError("transport policy changed")
        projection = {key: value for key, value in config.items() if key not in {"registration_file", "registration_sha256"}}
        if json.loads(policy_path.read_text()) != projection:
            raise ValueError("transport configuration differs from reviewed policy")
    if registration.get("model") != config["model"]:
        raise ValueError("model differs from launch registration")
    sequence, timeout = runtime_guard.reserve_start(registration)
    metadata = {"schema_version": 1, "run_id": run_id, "container": container, "manifest_sha256": sha(manifest_path), "agents_sha256": sha(cwd / "AGENTS.md"), "agents_bytes": (cwd / "AGENTS.md").stat().st_size, "project_doc_max_bytes": budget, "image": image, "requirements_sha256": sha(requirements), "spool_sha256": sha(SCRIPTS / "multica_spool.py"), "wrapper_sha256": sha(__file__), "config_sha256": sha(config_path), "sequence": sequence, "mounted_sources": [source for source, _, _ in mounts], "read_only_code": bool(manifest.get("read_only_code", False)), "started_at": time.time()}
    (run / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    broker_thread.start()
    process = None
    exit_code, outcome = 125, "spawn_failed"
    stop = threading.Event()
    def cancelled(signum, frame):
        stop.set()
    for sig in [signal.SIGINT, signal.SIGTERM]:
        signal.signal(sig, cancelled)
    try:
        with (run / "stderr.log").open("wb") as errlog, (run / "rpc-input.jsonl").open("ab") as inputlog, (run / "rpc-output.jsonl").open("ab") as outputlog:
            process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errlog)
            # stdin is held until the durable start is visible to the gateway.
            runtime_guard.mark_start(registration, sequence, process.pid)
            def forward_input():
                while not stop.is_set():
                    try:
                        ready, _, _ = select.select([sys.stdin.buffer], [], [], 0.1)
                        if not ready:
                            continue
                        data = os.read(sys.stdin.fileno(), 65536)
                        if not data:
                            process.stdin.close()
                            return
                        inputlog.write(data); inputlog.flush()
                        process.stdin.write(data); process.stdin.flush()
                    except (BrokenPipeError, OSError, ValueError):
                        return
            threading.Thread(target=forward_input, daemon=True).start()
            deadline = time.monotonic() + timeout
            while process.poll() is None:
                if broker.failure:
                    outcome, exit_code = "broker_failed", 125
                    break
                if stop.is_set() or time.monotonic() >= deadline:
                    outcome, exit_code = ("interrupted", 130) if stop.is_set() else ("timed_out", 124)
                    break
                ready, _, _ = select.select([process.stdout], [], [], 0.1)
                if ready:
                    data = os.read(process.stdout.fileno(), 65536)
                    if data:
                        outputlog.write(data); outputlog.flush()
                        sys.stdout.buffer.write(data); sys.stdout.buffer.flush()
            else:
                exit_code = process.returncode
                outcome = "completed" if exit_code == 0 else "failed"
            if process.poll() is not None:
                data = process.stdout.read()
                if data:
                    outputlog.write(data); outputlog.flush(); sys.stdout.buffer.write(data); sys.stdout.buffer.flush()
    finally:
        stop.set()
        # Remove only the exact container generated by this wrapper.
        subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20)
        if process is not None:
            process.wait(timeout=10)
        broker.stopped.set()
        broker_thread.join(timeout=95)
        if broker_thread.is_alive() or broker.failure:
            outcome, exit_code = "broker_failed", 125
        # Container has exited: there is no model process racing this walk.
        outcome, exit_code, refused = retain_sessions(sandbox_home / "sessions", session_destination, outcome, exit_code)
        metadata["session_retention_refused"] = refused
        metadata.update(finished_at=time.time(), outcome=outcome, exit_code=exit_code, agents_unchanged=sha(cwd / "AGENTS.md") == metadata["agents_sha256"])
        (run / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
        runtime_guard.finish_start(registration, sequence, outcome, exit_code)

    return exit_code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, KeyError, runtime_guard.Refused) as error:
        print("Isolated Codex transport refused: " + type(error).__name__, file=sys.stderr)
        raise SystemExit(125)
