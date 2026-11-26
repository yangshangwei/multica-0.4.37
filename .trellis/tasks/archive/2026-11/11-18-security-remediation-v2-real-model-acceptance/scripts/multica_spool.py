#!/usr/bin/env python3
"""Transparent local CLI transport: untrusted argv, fixed original task identity.

The model-side executable only writes ordinary files. A host-owned broker executes
an exact compiled Multica CLI, without a shell, with the server-claimed mat_ token.
No MCP server or daemon credential is added to a restricted reviewer's tool set.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import threading
import time
import uuid

MAX_REQUEST = 2 << 20
MAX_OUTPUT = 4 << 20
COMMANDS = {
    ("issue", "get"): (1, {"output"}, set()),
    ("issue", "runs"): (1, {"output"}, {"full-id"}),
    ("issue", "usage"): (1, {"output"}, set()),
    ("issue", "comment", "list"): (1, {"output", "limit", "max-bytes", "cursor", "since", "thread", "tail", "recent", "before", "before-id"}, {"roots-only", "compact", "summary", "full"}),
    ("issue", "comment", "add"): (1, {"content", "parent", "output", "request-id", "idempotency-key"}, {"content-stdin"}),
    ("issue", "comment", "receipt"): (2, {"output"}, set()),
    ("issue", "subscriber", "list"): (1, {"output"}, set()),
    ("issue", "subscriber", "add"): (1, {"user-id", "output"}, set()),
    ("issue", "status"): (2, {"output"}, {"no-start"}),
    ("issue", "update"): (1, {"status", "output"}, {"no-start"}),
    ("issue", "create"): (0, {"title", "description", "status", "priority", "project", "output"}, {"description-stdin", "allow-duplicate"}),
    ("squad", "get"): (1, {"output"}, set()),
    ("squad", "list"): (0, {"output"}, set()),
    ("squad", "member", "list"): (1, {"output"}, set()),
    ("squad", "activity"): (2, {"reason", "request-id", "output"}, set()),
}
REVIEW_COMMANDS = {("issue", "get"), ("issue", "comment", "list"), ("issue", "comment", "add")}
HELP_COMMANDS = set(COMMANDS) | {key[:length] for key in COMMANDS for length in range(1, len(key))}


class Refused(ValueError):
    pass


def contained(path: Path, root: Path) -> Path:
    path = path.absolute()
    resolved = path.resolve(strict=True)
    if path != resolved or (resolved != root and root not in resolved.parents):
        raise Refused("path is outside the current role or traverses a symlink")
    return resolved


def parse(argv: list[str]):
    if not isinstance(argv, list) or not argv or len(argv) > 128 or any(not isinstance(x, str) or "\0" in x or len(x) > MAX_REQUEST for x in argv):
        raise Refused("invalid argument vector")
    if argv[-1] == "--help":
        prefix = tuple(argv[:-1])
        if prefix not in HELP_COMMANDS:
            raise Refused("help is limited to an exact declared command")
        return prefix, [], {"help": True}
    prefix = next((key for key in sorted(COMMANDS, key=len, reverse=True) if tuple(argv[:len(key)]) == key), None)
    if prefix is None:
        raise Refused("command is outside this trial's CLI surface")
    expected, values, switches = COMMANDS[prefix]
    positional, flags = [], {}
    rest = iter(argv[len(prefix):])
    for arg in rest:
        if not arg.startswith("-"):
            positional.append(arg)
            continue
        if not arg.startswith("--"):
            raise Refused("short/global flags are not accepted")
        key, separator, value = arg[2:].partition("=")
        if key in flags:
            raise Refused("duplicate flag")
        if key in values:
            if not separator:
                try:
                    value = next(rest)
                except StopIteration:
                    raise Refused("missing flag value") from None
            flags[key] = value
        elif key in switches:
            if separator and value not in {"true", "false"}:
                raise Refused("invalid Boolean flag")
            flags[key] = value != "false"
        else:
            raise Refused("flag is outside this trial's CLI surface")
    if len(positional) != expected:
        raise Refused("unexpected positional arguments")
    if "output" in flags and flags["output"] not in {"json", "table"}:
        raise Refused("unsupported output format")
    return prefix, positional, flags


def validate_request(request, manifest, root):
    if not isinstance(request, dict) or set(request) != {"argv", "stdin_b64", "cwd"}:
        raise Refused("request schema rejected")
    cwd = contained(Path(request["cwd"]), root)
    prefix, positions, flags = parse(request["argv"])
    if flags.get("help"):
        if request["stdin_b64"]:
            raise Refused("help does not accept stdin")
        return cwd, b""
    issue_ids = {manifest["issue_id"], manifest.get("issue_identifier", manifest["issue_id"])}
    if prefix[0] == "issue" and prefix != ("issue", "create") and positions[0] not in issue_ids:
        raise Refused("issue is outside the current claim")
    if prefix in {("squad", "get"), ("squad", "member", "list")} and positions[0] != manifest.get("squad_id"):
        raise Refused("squad is outside the current claim")
    if prefix == ("squad", "activity") and positions[0] not in issue_ids:
        raise Refused("activity is outside the current issue")
    restricted = manifest.get("restricted", False) or manifest.get("role") in {"security-reviewer", "code-reviewer", "migration-reviewer"}
    if restricted and prefix not in REVIEW_COMMANDS:
        raise Refused("restricted reviewer command refused")
    leader = manifest.get("is_leader_task") is True
    if prefix in {("issue", "status"), ("issue", "update"), ("issue", "create"), ("squad", "activity"), ("issue", "subscriber", "add")} and not leader:
        raise Refused("command requires the actual leader task")
    if prefix == ("issue", "subscriber", "add") and flags.get("user-id") not in manifest.get("owner_ids", []):
        raise Refused("subscriber is not a registered owner")
    if prefix == ("issue", "create"):
        if flags.get("status") != "backlog" or not flags.get("title"):
            raise Refused("only unassigned backlog variant creation is allowed")
        if "project" in flags and flags["project"] != manifest.get("project_id"):
            raise Refused("variant project is outside the fixture")
    if prefix == ("squad", "activity") and positions[1] not in {"action", "no_action", "failed"}:
        raise Refused("activity outcome is invalid")
    try:
        body = base64.b64decode(request["stdin_b64"], validate=True)
    except (ValueError, TypeError):
        raise Refused("stdin encoding rejected") from None
    if len(body) > MAX_REQUEST:
        raise Refused("stdin exceeds transport bound")
    if body and not (flags.get("content-stdin") or flags.get("description-stdin")):
        raise Refused("unexpected stdin")
    return cwd, body


def atomic_write(path, content, mode=0o600):
    fd, temporary = tempfile.mkstemp(prefix=".spool-", dir=path.parent)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


class Broker:
    def __init__(self, manifest, workdir, requests, responses, executable, environment, audit):
        self.manifest = manifest
        self.workdir = Path(workdir).resolve(strict=True)
        self.requests, self.responses = Path(requests), Path(responses)
        self.executable = str(Path(executable).resolve(strict=True))
        self.environment = dict(environment)
        self.audit = Path(audit)
        self.stopped = threading.Event()
        self.seen = set()
        self.failure = None
        self.outputs = self.audit.parent / "cli-outputs"
        self.outputs.mkdir(mode=0o700)
        if not self.environment.get("MULTICA_TOKEN", "").startswith("mat_"):
            raise Refused("broker requires the original task credential")
        for key, source in [("MULTICA_TASK_ID", "task_id"), ("MULTICA_AGENT_ID", "agent_id"), ("MULTICA_WORKSPACE_ID", "workspace_id")]:
            if self.environment.get(key) != self.manifest.get(source):
                raise Refused("broker actor identity differs from claim")

    def process(self, path):
        request_id = path.stem
        if not re.fullmatch(r"[0-9a-f]{32}", request_id) or request_id in self.seen:
            return
        self.seen.add(request_id)
        started = time.time()
        receipt = {"request_id": request_id, "started_at": started, "task_id": self.manifest["task_id"]}
        result = {"exit_code": 126, "stdout_b64": "", "stderr_b64": ""}
        try:
            fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            with os.fdopen(fd, "rb") as stream:
                st = os.fstat(stream.fileno())
                if not stat.S_ISREG(st.st_mode) or st.st_size > MAX_REQUEST:
                    raise Refused("request file rejected")
                raw = stream.read(MAX_REQUEST + 1)
            receipt["request_sha256"] = hashlib.sha256(raw).hexdigest()
            request = json.loads(raw)
            if isinstance(request, dict) and isinstance(request.get("argv"), list):
                receipt["attempted_argv"] = request["argv"]
            cwd, stdin = validate_request(request, self.manifest, self.workdir)
            receipt["argv"] = request["argv"]
            receipt["stdin_sha256"] = hashlib.sha256(stdin).hexdigest()
            process = subprocess.run([self.executable, *request["argv"]], input=stdin, cwd=cwd, env=self.environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=90)
            if len(process.stdout) + len(process.stderr) > MAX_OUTPUT:
                raise Refused("CLI output exceeds transport bound")
            result = {"exit_code": process.returncode, "stdout_b64": base64.b64encode(process.stdout).decode(), "stderr_b64": base64.b64encode(process.stderr).decode()}
            receipt.update(exit_code=process.returncode, stdout_sha256=hashlib.sha256(process.stdout).hexdigest(), stderr_sha256=hashlib.sha256(process.stderr).hexdigest())
            token = self.environment["MULTICA_TOKEN"].encode()
            atomic_write(self.outputs / (request_id + ".stdout"), process.stdout.replace(token, b"[REDACTED-TASK-TOKEN]"))
            atomic_write(self.outputs / (request_id + ".stderr"), process.stderr.replace(token, b"[REDACTED-TASK-TOKEN]"))
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            # This error is content-free: never echo argv, token or raw CLI errors.
            result["stderr_b64"] = base64.b64encode(("Local Multica transport refused: " + type(error).__name__ + "\n").encode()).decode()
            receipt.update(refused=True, reason=type(error).__name__)
        receipt["finished_at"] = time.time()
        raw_result = (json.dumps(result) + "\n").encode()
        receipt["response_sha256"] = hashlib.sha256(raw_result).hexdigest()
        with self.audit.open("a") as stream:
            stream.write(json.dumps(receipt) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
        atomic_write(self.responses / (request_id + ".json"), raw_result, 0o444)

    def run(self):
        while not self.stopped.wait(0.025):
            try:
                for path in sorted(self.requests.glob("*.json")):
                    self.process(path)
            except OSError:
                self.failure = "broker evidence write failed"
                self.stopped.set()


def client(argv):
    # The shim reads content-file within the sandbox and sends bytes as stdin.
    # The trusted host broker never opens a model-selected body path.
    cwd = Path.cwd().resolve()
    root = Path(os.environ["SECREM_ROLE_WORKDIR"])
    contained(cwd, root)
    stdin = b""
    converted = list(argv)
    for flag in ["--content-file", "--description-file"]:
        occurrences = [i for i, value in enumerate(converted) if value == flag or value.startswith(flag + "=")]
        if occurrences:
            if len(occurrences) != 1:
                raise Refused("duplicate content path")
            index = occurrences[0]
            key, equal, value = converted[index].partition("=")
            if not equal:
                if index + 1 >= len(converted):
                    raise Refused("content path missing")
                value = converted[index + 1]
            candidate = (cwd / value).absolute()
            # /tmp is the role container tmpfs, never a host-selected path.
            target = contained(candidate, Path("/tmp")) if Path("/tmp") in candidate.parents else contained(candidate, root)
            stdin = target.read_bytes()
            converted[index:index + (1 if equal else 2)] = [flag.replace("-file", "-stdin")]
    if not stdin and ("--content-stdin" in converted or "--description-stdin" in converted):
        stdin = sys.stdin.buffer.read(MAX_REQUEST + 1)
    if len(stdin) > MAX_REQUEST:
        raise Refused("stdin too large")
    request_id = uuid.uuid4().hex
    request = {"argv": converted, "stdin_b64": base64.b64encode(stdin).decode(), "cwd": str(cwd)}
    raw = (json.dumps(request) + "\n").encode()
    if len(raw) > MAX_REQUEST:
        raise Refused("request too large")
    requests = Path(os.environ["SECREM_REQUEST_DIR"])
    responses = Path(os.environ["SECREM_RESPONSE_DIR"])
    contained(requests, root)
    contained(responses, root)
    atomic_write(requests / (request_id + ".json"), raw, 0o644)
    response_path = responses / (request_id + ".json")
    deadline = time.monotonic() + 100
    while time.monotonic() < deadline:
        if response_path.exists():
            with response_path.open("rb") as stream:
                response = json.load(stream)
            sys.stdout.buffer.write(base64.b64decode(response["stdout_b64"], validate=True))
            sys.stderr.buffer.write(base64.b64decode(response["stderr_b64"], validate=True))
            return response["exit_code"]
        time.sleep(0.025)
    raise Refused("local CLI reply timed out")


if __name__ == "__main__":
    try:
        raise SystemExit(client(sys.argv[1:]))
    except (Refused, OSError, ValueError, KeyError):
        print("Local Multica transport unavailable or command refused", file=sys.stderr)
        raise SystemExit(126)
