#!/usr/bin/env python3
"""Private, hash-bound launch budget. No real launch without both explicit gates."""

import argparse
from contextlib import contextmanager
import fcntl
import hashlib
import json
import math
import os
from pathlib import Path
import signal
import stat
import subprocess
import tempfile
import time


class Refused(RuntimeError):
    pass


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def private_path(value, exists=True):
    path = Path(value)
    if not path.is_absolute() or path.resolve() != path:
        raise Refused("Path must be absolute and must not traverse symlinks")
    if exists and (not path.is_file() or stat.S_IMODE(path.stat().st_mode) & 0o077):
        raise Refused("Private input must be an owner-only regular file")
    if stat.S_IMODE(path.parent.stat().st_mode) & 0o077:
        raise Refused("Private directory must be owner-only")
    return path


def load_registration(path, expected_hash):
    path = private_path(path)
    if len(expected_hash) != 64 or digest(path) != expected_hash:
        raise Refused("Registration hash mismatch")
    data = json.loads(path.read_text())
    if data.get("schema_version") != 1 or data.get("mode") not in {"fake", "real"}:
        raise Refused("Registration schema/mode rejected")
    if data.get("mode") == "real":
        if data.get("enabled") is not True or os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE") != "1":
            raise Refused("Real entry disabled")
        if not all(data.get(k) for k in ("model", "account_alias", "integration_commit", "isolation_evidence_sha256")):
            raise Refused("Real execution bindings incomplete")
        if not data.get("source_hashes"):
            raise Refused("Real source bindings required")
        # The isolated tool broker and actual daemon admission are not wired yet.
        # A boolean or an unverified hash must never enable the preparation build.
        raise Refused("Preparation build cannot admit real execution")
    elif os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE"):
        raise Refused("Fake entry refuses real-agent opt-in")
    for key in ("max_starts", "max_model_requests"):
        if type(data.get(key)) is not int or data[key] <= 0:
            raise Refused("A positive finite count budget is required")
    for key in ("max_seconds", "per_start_seconds", "expires_at"):
        if type(data.get(key)) not in (int, float) or not math.isfinite(data[key]) or data[key] <= 0:
            raise Refused("A positive finite time budget is required")
    if data["per_start_seconds"] > data["max_seconds"] or time.time() >= data["expires_at"]:
        raise Refused("Registration expired or time budgets inconsistent")
    private_path(data["ledger"], exists=False)
    if not isinstance(data.get("argv"), list) or not data["argv"] or not all(isinstance(v, str) for v in data["argv"]):
        raise Refused("Exact argument vector is required")
    executable = Path(data["argv"][0])
    if not executable.is_absolute() or not executable.is_file() or digest(executable) != data.get("executable_sha256"):
        raise Refused("Executable binding differs")
    if not Path(data["cwd"]).is_absolute() or Path(data["cwd"]).resolve() != Path(data["cwd"]) or not Path(data["cwd"]).is_dir():
        raise Refused("Working directory binding rejected")
    for filename, expected in data.get("source_hashes", {}).items():
        if not Path(filename).is_absolute() or digest(filename) != expected:
            raise Refused("Source binding differs")
    if not isinstance(data.get("environment"), dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in data["environment"].items()):
        raise Refused("Explicit environment required")
    data["registration_sha256"] = expected_hash
    return data


def atomic_save(path, value):
    path = Path(path)
    fd, temporary = tempfile.mkstemp(prefix=".ledger-", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            json.dump(value, stream, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


@contextmanager
def ledger(registration):
    path = private_path(registration["ledger"], exists=False)
    lockpath = str(path) + ".lock"
    fd = os.open(lockpath, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        if path.exists():
            private_path(str(path))
            data = json.loads(path.read_text())
            if data.get("registration_sha256") != registration["registration_sha256"]:
                raise Refused("Ledger belongs to another registration")
        else:
            data = {"registration_sha256": registration["registration_sha256"], "starts": [], "model_requests": [], "first_reserved_at": None, "stopped": False}
        yield data
        atomic_save(path, data)
    finally:
        fcntl.flock(fd, fcntl.LOCK_UN)
        os.close(fd)


def remaining(registration, data):
    deadline = registration["expires_at"]
    if data["first_reserved_at"] is not None:
        deadline = min(deadline, data["first_reserved_at"] + registration["max_seconds"])
    return deadline - time.time()


def reserve_start(registration):
    with ledger(registration) as data:
        if data["stopped"] or remaining(registration, data) <= 0 or len(data["starts"]) >= registration["max_starts"]:
            raise Refused("Invocation/time budget exhausted or stopped")
        if any(row.get("finished_at") is None for row in data["starts"]):
            raise Refused("An earlier reservation has no terminal record")
        now = time.time()
        if data["first_reserved_at"] is None:
            data["first_reserved_at"] = now
        sequence = len(data["starts"]) + 1
        data["starts"].append({"sequence": sequence, "reserved_at": now, "started_at": None, "finished_at": None})
        timeout = min(remaining(registration, data), registration["per_start_seconds"])
    return sequence, timeout


def reserve_model_request(registration):
    with ledger(registration) as data:
        if data["stopped"] or remaining(registration, data) <= 0 or len(data["model_requests"]) >= registration["max_model_requests"]:
            raise Refused("Model-request/time budget exhausted")
        if not data["starts"] or data["starts"][-1]["started_at"] is None or data["starts"][-1]["finished_at"] is not None:
            raise Refused("No active registered process")
        process_remaining = data["starts"][-1]["started_at"] + registration["per_start_seconds"] - time.time()
        if process_remaining <= 0:
            raise Refused("Active process time budget exhausted")
        data["model_requests"].append({"sequence": len(data["model_requests"]) + 1, "reserved_at": time.time(), "start_sequence": data["starts"][-1]["sequence"]})
        return min(remaining(registration, data), process_remaining)


def execute(registration):
    sequence, timeout = reserve_start(registration)
    process = None
    outcome, exit_code = "spawn_failed", 125
    old_handlers = {}

    def interrupt(signum, frame):
        raise InterruptedError("Registered process interrupted")

    try:
        for sig in (signal.SIGINT, signal.SIGTERM):
            old_handlers[sig] = signal.signal(sig, interrupt)
        process = subprocess.Popen(registration["argv"], cwd=registration["cwd"], env=registration["environment"], start_new_session=True)
        with ledger(registration) as data:
            data["starts"][sequence - 1].update(started_at=time.time(), pid=process.pid)
        exit_code = process.wait(timeout=timeout)
        outcome = "completed" if exit_code == 0 else "failed"
    except subprocess.TimeoutExpired:
        outcome, exit_code = "timed_out", 124
    except InterruptedError:
        outcome, exit_code = "interrupted", 130
    finally:
        if process is not None:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=5)
        for sig, handler in old_handlers.items():
            signal.signal(sig, handler)
        with ledger(registration) as data:
            data["starts"][sequence - 1].update(finished_at=time.time(), outcome=outcome, exit_code=exit_code)
            if outcome != "completed":
                data["stopped"] = True
    return exit_code


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--registration", required=True)
    parser.add_argument("--sha256", required=True)
    arguments = parser.parse_args()
    try:
        raise SystemExit(execute(load_registration(arguments.registration, arguments.sha256)))
    except (Refused, OSError, ValueError):
        raise SystemExit("Registration or execution refused; inspect private evidence")
