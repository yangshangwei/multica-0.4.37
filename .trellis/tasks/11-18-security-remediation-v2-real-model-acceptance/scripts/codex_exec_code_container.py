#!/usr/bin/env python3
"""Run official Codex exec with a registered code-role filesystem policy.

The stdin prompt and generated AGENTS.md remain byte-identical. Stdout is the
native CLI JSONL stream, not an app-server or SDK protocol translation.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re
import select
import signal
import subprocess
import sys
import threading
import time
import uuid

import codex_container as common
import codex_exec_container as scratch_transport
import multica_spool
import runtime_guard
import contained_git_role_handoff

SCRIPTS = Path(__file__).resolve().parent
LABEL = "multica.task=secrem-v2-cli-20261120"
MODE = "official_cli_exec_code"
MAX_PROMPT_BYTES = 8 << 20
MAX_OUTPUT_BYTES = 64 << 20
SCRATCH_GENERATED = {"AGENTS.md", ".agent_context", ".multica", ".agents", ".codex", ".git",
                     ".aws", ".azure", ".config", ".kube", ".ssh", ".gnupg"}


def direct_args(args):
    if args == ["exec", "--json", "--skip-git-repo-check", "-"]:
        return args, None
    if len(args) == 6 and args[:2] == ["exec", "resume"] and args[2:4] == ["--json", "--skip-git-repo-check"] and args[-1] == "-":
        session = args[4]
        if not re.fullmatch(r"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}", session):
            raise ValueError("only an exact observed session UUID can resume")
        return args, session
    raise ValueError("only the fixed official exec CLI entry is accepted")


def native_terminal(raw):
    """Extract only native terminal facts; no inferred or fabricated success."""
    events = []
    invalid = 0
    for line in raw.splitlines():
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except (ValueError, UnicodeDecodeError):
            invalid += 1
            continue
        if not isinstance(value, dict):
            invalid += 1
            continue
        if value.get("type") == "item.completed" and not isinstance(value.get("item"), dict):
            invalid += 1
            continue
        events.append(value)
    completed = [v for v in events if v.get("type") == "turn.completed"]
    failed = [v for v in events if v.get("type") in {"turn.failed", "error"}]
    threads = [v.get("thread_id") for v in events if v.get("type") == "thread.started"]
    messages = [v.get("item", {}).get("text", "") for v in events if v.get("type") == "item.completed" and v.get("item", {}).get("type") == "agent_message"]
    thread_valid = len(threads) == 1 and isinstance(threads[0], str) and bool(re.fullmatch(r"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}", threads[0]))
    return {"valid": invalid == 0 and len(completed) == 1 and not failed and thread_valid,
            "invalid_lines": invalid, "completed_turns": len(completed), "failed_events": len(failed),
            "thread_ids": threads, "usage": completed[0].get("usage") if len(completed) == 1 else None,
            "last_message": messages[-1] if messages else None}


def regular_read(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as stream:
        import stat
        if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
            raise ValueError("expected regular CLI evidence file")
        body = stream.read(MAX_OUTPUT_BYTES + 1)
        if len(body) > MAX_OUTPUT_BYTES:
            raise ValueError("regular CLI evidence file exceeds bound")
        return body


def git_read(repo, *args):
    result = contained_git_role_handoff.inspect(repo, list(args))
    if result.returncode:
        raise ValueError("registered Git source cannot be read")
    return result.stdout


def retained_head(cwd):
    """Read model-owned Git identity without executing host Git/config hooks."""
    gitdir = cwd / ".git"
    if gitdir.resolve() != gitdir:
        raise ValueError("post-run Git directory alias")
    head = regular_read(gitdir / "HEAD").decode().strip()
    if head.startswith("ref: "):
        ref = head[5:]
        if not re.fullmatch(r"refs/heads/[A-Za-z0-9._/-]+", ref) or ".." in Path(ref).parts:
            raise ValueError("post-run Git reference rejected")
        path = gitdir / ref
        if path.resolve() != path:
            raise ValueError("post-run Git reference alias")
        head = regular_read(path).decode().strip()
    if not re.fullmatch(r"[a-f0-9]{40}", head):
        raise ValueError("post-run full Git commit required")
    return head


def code_policy(cwd, manifest):
    mode = manifest.get("role_mode")
    if mode not in {"implementer", "verifier", "verifier_recheck", "qa", "reviewer", "analysis"} or manifest.get("no_repository") is not False:
        raise ValueError("explicit code-role mode required")
    if manifest.get("repo_root") != str(cwd):
        raise ValueError("claimed repository differs from actual work directory")
    expected_role = {"implementer":"implementer", "verifier":"vulnerability-verifier", "verifier_recheck":"vulnerability-verifier", "qa":"qa-engineer", "analysis":"product-analyst"}
    role = manifest.get("role")
    if mode == "reviewer" and role not in {"security-reviewer", "code-reviewer", "migration-reviewer"}:
        raise ValueError("review-only policy requires an actual reviewer")
    if mode != "reviewer" and role != expected_role[mode]:
        raise ValueError("code-role policy differs from actual agent role")
    if manifest.get("is_leader_task") is True:
        raise ValueError("leader cannot receive a writable code policy")
    gitdir = cwd / ".git"
    if gitdir.is_symlink() or not gitdir.is_dir() or (gitdir / "commondir").exists() or (gitdir / "objects/info/alternates").exists():
        raise ValueError("role needs its own full Git clone")
    if git_read(cwd,"rev-parse","--show-toplevel").decode().strip() != "/repo":
        raise ValueError("code policy requires the exact repository root")
    baseline = manifest.get("baseline_commit", "")
    if not re.fullmatch(r"[a-f0-9]{40}", baseline) or git_read(cwd,"rev-parse","HEAD").decode().strip() != baseline:
        raise ValueError("code baseline differs from actual HEAD")
    for raw in git_read(cwd,"ls-tree","-r","-z","HEAD").split(b"\0"):
        if raw and raw.split(b" ",1)[0] not in {b"100644",b"100755"}:
            raise ValueError("code fixture contains a filesystem edge")
    relative = manifest.get("protected_test_dir")
    if relative != "tests":
        raise ValueError("registered test subtree must be tests")
    tests = cwd / relative
    if tests.is_symlink() or not tests.is_dir():
        raise ValueError("test subtree missing or aliased")
    protected = manifest.get("protected_test_sha256")
    if not isinstance(protected, dict):
        raise ValueError("existing test hashes required")
    for relative, expected in protected.items():
        rel = Path(relative)
        if rel.is_absolute() or ".." in rel.parts or not rel.parts or rel.parts[0] != "tests" or not re.fullmatch(r"[a-f0-9]{64}", expected):
            raise ValueError("protected test path rejected")
        target = cwd / rel
        if target.resolve() != target or not target.is_file() or common.sha(target) != expected:
            raise ValueError("protected test differs")
    return mode, tests, protected


def instruction_assets(cwd):
    assets = []
    for relative in ["AGENTS.md", ".agent_context", ".agents", ".codex", ".aws", ".azure", ".config", ".kube", ".ssh", ".gnupg"]:
        target = cwd / relative
        if target.exists():
            if target.is_symlink():
                raise ValueError("instruction asset alias")
            assets.append(target)
    return assets


def leader_prepared_manifest(cwd, manifest):
    if manifest.get("is_leader_task") is not True or manifest.get("leader_ignores_project_resources") is not True or manifest.get("prepared_repo_count") != 0 or manifest.get("no_repository") is not True:
        raise ValueError("leader scratch requires actual coordinator preparation proof")
    ignored = manifest.get("ignored_project_paths")
    if not isinstance(ignored, list) or (manifest.get("project_resource_count", 0) and not ignored):
        raise ValueError("ignored project paths must be explicitly recorded")
    for value in ignored:
        if not isinstance(value, str):
            raise ValueError("ignored project path is not a string")
        source = Path(value)
        if not source.is_absolute() or source.resolve(strict=True) != source or not source.is_dir():
            raise ValueError("ignored project path must be canonical")
        if cwd == source or source in cwd.parents or cwd in source.parents:
            raise ValueError("leader scratch overlaps a project directory")
    return {**manifest, "repo_count": 0, "project_resource_count": 0}


def post_filesystem_evidence(cwd, manifest, agents_sha256):
    evidence = {"agents_unchanged": hashlib.sha256(regular_read(cwd / "AGENTS.md")).hexdigest() == agents_sha256}
    if manifest["role_mode"] == "leader_scratch":
        # The claim can describe protected project tests while production has
        # intentionally prepared no repository for this coordinator. Preserve
        # that provenance, but never claim to have checked an unmounted tree.
        evidence.update(protected_tests_status="not_mounted", protected_tests_unchanged=None)
        return evidence
    evidence["protected_tests_unchanged"] = all((cwd / relative).resolve() == cwd / relative and hashlib.sha256(regular_read(cwd / relative)).hexdigest() == expected for relative,expected in manifest.get("protected_test_sha256", {}).items())
    evidence["protected_tests_status"] = "checked"
    evidence["head_after"] = retained_head(cwd)
    return evidence


def prepare(config, manifest, cwd, original_home, run):
    if manifest.get("role_mode") == "leader_scratch":
        # The original claim counts remain in metadata. The production preparer
        # intentionally mounts no project repository for the leader, so the
        # shared scratch validator sees this separately attested prepared set.
        prepared = leader_prepared_manifest(cwd, manifest)
        return scratch_transport.prepare(config, prepared, cwd, original_home, run)
    roots = [Path(config["evidence_root"]), Path(config["allowed_workspaces_root"])]
    sandbox_home = run / "codex-home"
    sandbox_home.mkdir(mode=0o777)
    sandbox_home.chmod(0o777)
    common.copy_regular_tree(original_home / "skills", sandbox_home / "skills")
    destination = common.task_session_destination(original_home, manifest, roots)
    common.copy_regular_tree(destination, sandbox_home / "sessions")
    for name in ["sessions", "log", "logs"]:
        (sandbox_home / name).mkdir(mode=0o777, exist_ok=True)
        (sandbox_home / name).chmod(0o777)
    role_mode, tests, protected = code_policy(cwd, manifest)
    if type(manifest.get("read_only_code")) is not bool or manifest["read_only_code"] != (role_mode != "implementer"):
        raise ValueError("role code-readonly declaration differs from policy")
    for reserved in [".git", ".agents", ".codex", ".aws", ".azure", ".config/gcloud", ".kube", ".ssh", ".gnupg"]:
        path = cwd / reserved
        if not path.exists():
            path.mkdir(mode=0o755, parents=True)
    protected_assets = instruction_assets(cwd)
    multica = cwd / ".multica"
    multica.mkdir(mode=0o755, exist_ok=True)
    comments = multica / "comments"
    comments.mkdir(mode=0o755, exist_ok=True)
    for item in multica.iterdir():
        if item.is_symlink():
            raise ValueError("Multica scratch alias")
        if item.name != "comments":
            protected_assets.append(item)
    requests, responses = cwd / (".secrem-requests-" + run.name), cwd / (".secrem-responses-" + run.name)
    for directory in [requests, responses]:
        directory.mkdir(mode=0o777)
        directory.chmod(0o777)
    proxy_token = Path(config["model_proxy_token_file"]).read_text().strip()
    if not proxy_token or "\n" in proxy_token:
        raise ValueError("proxy credential rejected")
    config_path = run / "config.toml"
    budget = common.write_codex_config(config_path, config, cwd, original_home, proxy_token, requests, responses)
    if role_mode in {"implementer", "verifier"}:
        text = config_path.read_text().replace("[sandbox_workspace_write]\n", "[sandbox_workspace_write]\nwritable_roots = " + json.dumps([str(cwd / ".git")]) + "\n")
        config_path.chmod(0o600); config_path.write_text(text); config_path.chmod(0o444)
    (sandbox_home / "config.toml").touch(mode=0o644)
    requirements = SCRIPTS.parent / "fixtures/native-requirements.toml"
    shim = run / "multica"
    shim.write_text('#!/bin/sh\nexec python3 /opt/secrem/multica_spool.py "$@"\n')
    shim.chmod(0o555)
    broker_home, broker_config = run / "broker-home", run / "broker-config"
    broker_home.mkdir(mode=0o700)
    broker_config.mkdir(mode=0o700)
    broker_env = {"PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "HOME": str(broker_home), "MULTICA_TASK_CONFIG_ROOT": str(broker_config)}
    for key in ["MULTICA_TOKEN", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID", "MULTICA_AGENT_NAME", "MULTICA_TASK_ID", "MULTICA_SERVER_URL"]:
        broker_env[key] = os.environ[key]
    broker = multica_spool.Broker(manifest, cwd, requests, responses, config["cli_path"], broker_env, run / "cli-audit.jsonl")
    mounts = [(cwd, cwd, role_mode != "implementer"), (comments, comments, False), (requests, requests, False), (responses, responses, True),
              (sandbox_home, original_home, False), (config_path, original_home / "config.toml", True),
              (requirements, Path("/etc/codex/requirements.toml"), True),
              (shim, Path("/usr/local/bin/multica"), True),
              (SCRIPTS / "multica_spool.py", Path("/opt/secrem/multica_spool.py"), True)]
    mounts += [(asset, asset, True) for asset in protected_assets]
    if role_mode == "implementer":
        mounts.append((tests, tests, True))
    elif role_mode == "verifier":
        mounts += [(tests, tests, False), (cwd / ".git", cwd / ".git", False)]
        mounts += [(cwd / relative, cwd / relative, True) for relative in protected]
    if (sandbox_home / "skills").exists():
        mounts.append((sandbox_home / "skills", original_home / "skills", True))
    return sandbox_home, destination, broker, mounts, budget


def main():
    config = common.checked_config(os.environ["SECREM_TRANSPORT_CONFIG"])
    if config.get("transport_mode") != MODE:
        raise ValueError("official CLI mode must be explicit")
    image = config.get("image", common.DEFAULT_IMAGE)
    if sys.argv[1:] == ["--version"]:
        return subprocess.run(["docker", "run", "--rm", "--pull=never", "--network=none", "--label", LABEL, "--entrypoint", "codex", image, "--version"]).returncode
    args, resume = direct_args(sys.argv[1:])
    registration = common.registration_from_driver(config)
    if registration.get("transport_mode") != MODE:
        raise ValueError("registration must identify official CLI execution")
    if registration.get("model") != config["model"]:
        raise ValueError("model differs from registration")
    if registration["mode"] == "real":
        if config.get("fake_endpoint_script"):
            raise ValueError("real execution refuses a synthetic endpoint launcher")
        network = json.loads(subprocess.check_output(["docker", "network", "inspect", config["network"]], text=True))[0]
        if network.get("Internal") is not True:
            raise ValueError("real model network must deny direct egress")
        policy = runtime_guard.private_path(registration["transport_policy_file"])
        if common.sha(policy) != registration["transport_policy_sha256"]:
            raise ValueError("transport policy changed")
        if json.loads(policy.read_text()) != {k:v for k,v in config.items() if k not in {"registration_file", "registration_sha256"}}:
            raise ValueError("transport policy differs from admission")
    elif config["network"] != "none":
        raise ValueError("fake CLI proof requires network none")
    evidence = Path(config["evidence_root"])
    roots = [evidence, Path(config["allowed_workspaces_root"])]
    cwd = common.rooted(Path.cwd(), roots)
    home = common.rooted(os.environ["CODEX_HOME"], roots)
    task_id = os.environ["MULTICA_TASK_ID"]
    if not re.fullmatch(r"[a-f0-9-]{36}", task_id):
        raise ValueError("task identity rejected")
    manifest_path = Path(config["claim_manifest_dir"]) / (task_id + ".json")
    manifest = json.loads(manifest_path.read_text())
    for field,key in [("task_id","MULTICA_TASK_ID"),("agent_id","MULTICA_AGENT_ID"),("workspace_id","MULTICA_WORKSPACE_ID")]:
        if manifest.get(field) != os.environ.get(key):
            raise ValueError("claim identity differs")
    if not os.environ.get("MULTICA_TOKEN", "").startswith("mat_"):
        raise ValueError("original claimed task token required")
    if resume and resume != manifest.get("resume_session_id"):
        raise ValueError("resume session is not bound to this claim")
    agents = regular_read(cwd / "AGENTS.md")
    prompt = sys.stdin.buffer.read(MAX_PROMPT_BYTES + 1)
    if not prompt or len(prompt) > MAX_PROMPT_BYTES:
        raise ValueError("complete bounded prompt required")
    prompt.decode("utf-8")
    run = evidence / "cli-transport" / (task_id + "-" + uuid.uuid4().hex[:10])
    run.mkdir(parents=True, mode=0o700)
    prompt_path = run / "prompt.txt"
    prompt_path.write_bytes(prompt)
    prompt_path.chmod(0o600)
    sandbox_home, destination, broker, mounts, budget = prepare(config, manifest, cwd, home, run)
    if resume:
        files = list((sandbox_home / "sessions").rglob("*" + resume + "*"))
        if not any(p.is_file() and not p.is_symlink() for p in files):
            raise ValueError("the exact observed resume session is unavailable")
    result_file = home / "last-message.txt"
    native_args = [*args[:-1], "--output-last-message", str(result_file), "-"]
    container = "secrem-v2-cli-role-" + uuid.uuid4().hex[:12]
    command = ["docker", "run", "-i", "--rm", "--name", container, "--label", LABEL, "--pull=never", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=unconfined", f"--user={os.getuid()}:{os.getgid()}", "--pids-limit=128", "--memory=1536m", "--network", config["network"], "--tmpfs=/tmp:rw,nosuid,size=128m", f"--tmpfs=/home/node:rw,nosuid,size=32m,uid={os.getuid()},gid={os.getgid()}", "--workdir", str(cwd), "-e", "HOME=/home/node", "-e", "CODEX_HOME=" + str(home), "-e", "PATH=/usr/local/bin:/usr/bin:/bin"]
    if config.get("fake_endpoint_script"):
        endpoint = runtime_guard.private_path(config["fake_endpoint_script"])
        if common.sha(endpoint) != config.get("fake_endpoint_sha256"):
            raise ValueError("synthetic endpoint source differs")
        mounts.append((endpoint, Path("/opt/secrem/fake_endpoint.py"), True))
        command += ["-e", "SECREM_PROBE_ISSUE=" + manifest["issue_id"], "-e", "SECREM_PROBE_ROLE=" + manifest["role_mode"]]
    for source,target,readonly in mounts:
        command += ["--mount", f"type=bind,src={source},dst={target}" + (",readonly" if readonly else "")]
    if config.get("fake_endpoint_script"):
        command += ["--entrypoint", "python3", image, "/opt/secrem/fake_endpoint.py", *native_args]
    else:
        command += ["--entrypoint", "codex", image, *native_args]
    sequence, timeout = runtime_guard.reserve_start(registration)
    metadata = {"execution_mode": MODE, "run_id":run.name,"container":container,"sequence":sequence,"started_at":time.time(),
                "prompt_sha256":hashlib.sha256(prompt).hexdigest(),"prompt_bytes":len(prompt),"agents_sha256":hashlib.sha256(agents).hexdigest(),
                "agents_bytes":len(agents),"project_doc_max_bytes":budget,"native_argv":["codex",*native_args],"resume_session_id":resume,
                "wrapper_sha256":common.sha(__file__),"helper_sha256":common.sha(common.__file__),"spool_sha256":common.sha(multica_spool.__file__),
                "requirements_sha256":common.sha(SCRIPTS.parent / "fixtures/native-requirements.toml"),"image":image,"config_sha256":common.sha(run / "config.toml"),
                "role_mode":manifest.get("role_mode"),"baseline_commit":manifest.get("baseline_commit"),"protected_test_sha256":manifest.get("protected_test_sha256",{}),
                "claim_repo_count":manifest.get("repo_count"),"claim_project_resource_count":manifest.get("project_resource_count"),"prepared_repo_count":manifest.get("prepared_repo_count"),"leader_ignores_project_resources":manifest.get("leader_ignores_project_resources",False),"ignored_project_paths":manifest.get("ignored_project_paths",[])}
    runtime_guard.atomic_save(run / "metadata.json", metadata)
    broker_thread = threading.Thread(target=broker.run, daemon=True)
    broker_thread.start()
    process = None
    outcome, exit_code = "spawn_failed",125
    stop = threading.Event()
    previous = {}
    for sig in [signal.SIGINT,signal.SIGTERM]:
        previous[sig] = signal.signal(sig, lambda signum,frame:stop.set())
    try:
        with (run / "stderr.log").open("wb") as stderr, (run / "stdout.jsonl").open("wb") as stdout:
            process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=stderr)
            runtime_guard.mark_start(registration,sequence,process.pid)
            def send_prompt():
                try:
                    process.stdin.write(prompt)
                    process.stdin.close()
                except (BrokenPipeError,OSError):
                    stop.set()
            threading.Thread(target=send_prompt, daemon=True).start()
            deadline = time.monotonic()+timeout
            output_bytes = 0
            while process.poll() is None:
                if broker.failure:
                    outcome,exit_code="broker_failed",125
                    break
                if stop.is_set() or time.monotonic()>=deadline:
                    outcome,exit_code=("interrupted",130) if stop.is_set() else ("timed_out",124)
                    break
                ready,_,_=select.select([process.stdout],[],[],0.1)
                if ready:
                    data=os.read(process.stdout.fileno(),65536)
                    if data:
                        output_bytes += len(data)
                        if output_bytes > MAX_OUTPUT_BYTES:
                            outcome,exit_code="output_limit",125
                            break
                        stdout.write(data);stdout.flush()
                        sys.stdout.buffer.write(data);sys.stdout.buffer.flush()
            else:
                exit_code=process.returncode
                outcome="completed" if exit_code==0 else "failed"
            if process.poll() is not None:
                data=process.stdout.read()
                if data:
                    if output_bytes + len(data) > MAX_OUTPUT_BYTES:
                        outcome,exit_code="output_limit",125
                    else:
                        stdout.write(data);stdout.flush();sys.stdout.buffer.write(data);sys.stdout.buffer.flush()
            stdout.flush();os.fsync(stdout.fileno());stderr.flush();os.fsync(stderr.fileno())
    finally:
        stop.set()
        subprocess.run(["docker","rm","-f",container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=20)
        if process is not None:
            process.wait(timeout=10)
        broker.stopped.set();broker_thread.join(timeout=95)
        if broker.failure or broker_thread.is_alive():
            outcome,exit_code="broker_failed",125
        for sig,old in previous.items():
            signal.signal(sig,old)
        try:
            terminal=native_terminal(regular_read(run / "stdout.jsonl"))
            last=regular_read(sandbox_home / "last-message.txt") if (sandbox_home / "last-message.txt").exists() else None
            if last is not None:
                (run / "last-message.txt").write_bytes(last)
                (run / "last-message.txt").chmod(0o600)
            if outcome=="completed" and (not terminal["valid"] or last is None or terminal["last_message"]!=last.decode("utf-8").rstrip("\n")):
                outcome,exit_code="incomplete_terminal",125
            metadata["terminal"]=terminal
            metadata["last_message_sha256"]=hashlib.sha256(last).hexdigest() if last is not None else None
            outcome,exit_code,refused=common.retain_sessions(sandbox_home / "sessions",destination,outcome,exit_code)
            metadata["session_retention_refused"]=refused
        except (OSError,ValueError):
            outcome,exit_code="evidence_failed",125
        try:
            metadata.update(post_filesystem_evidence(cwd, manifest, metadata["agents_sha256"]))
            if manifest["role_mode"] in {"verifier_recheck", "qa", "reviewer", "analysis"} and metadata["head_after"] != manifest["baseline_commit"]:
                outcome,exit_code="read_only_head_changed",125
            if not metadata["agents_unchanged"] or metadata["protected_tests_unchanged"] is False:
                outcome,exit_code="instruction_changed",125
        except (OSError,ValueError):
            outcome,exit_code="evidence_failed",125
        metadata.update(finished_at=time.time(),outcome=outcome,exit_code=exit_code)
        runtime_guard.atomic_save(run / "metadata.json",metadata)
        runtime_guard.finish_start(registration,sequence,outcome,exit_code)
    return exit_code


if __name__=="__main__":
    try:
        raise SystemExit(main())
    except (OSError,ValueError,KeyError,runtime_guard.Refused) as error:
        print("Official Codex CLI transport refused: "+type(error).__name__,file=sys.stderr)
        raise SystemExit(125)
