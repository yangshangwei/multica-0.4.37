#!/usr/bin/env python3
"""Explicit AC43 test transport: one managed wrapper, one real codex exec child.

The adapter session handle is bridge-local. The actual Codex thread ID is
retained separately; this wrapper never claims to be the Codex app-server.
Unsupported protocol operations/events fail closed. No credentials are printed.
"""
import hashlib
import json
import os
import re
from pathlib import Path
import signal
import subprocess
import sys
import time
import uuid


class BridgeError(Exception):
    pass


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def emit(value):
    sys.stdout.write(json.dumps(value, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def response(request_id, result):
    emit({"id": request_id, "result": result})


def refuse(request_id, code, message):
    emit({"id": request_id, "error": {"code": code, "message": message}})


class Bridge:
    def __init__(self, executable, evidence):
        self.executable = executable
        self.evidence = evidence
        self.handle = None
        self.cwd = None
        self.model = None
        self.child = None
        self.used = False
        self.terminated = False
        self.initialized = False

    def signal(self, number, _frame):
        self.terminated = True
        if self.child is not None and self.child.poll() is None:
            self.child.send_signal(number)

    def notification(self, method, turn_id, **params):
        emit({"method": method, "params": {"threadId": self.handle,
              "turnId": turn_id, **params}})

    def setup(self, params):
        allowed = {"model", "modelProvider", "profile", "cwd", "approvalPolicy",
                   "sandbox", "config", "baseInstructions", "developerInstructions",
                   "compactPrompt", "includeApplyPatchTool", "experimentalRawEvents",
                   "persistExtendedHistory"}
        if set(params) - allowed:
            raise BridgeError("unsupported thread configuration")
        for key, value in params.items():
            if key not in {"model", "cwd", "experimentalRawEvents", "persistExtendedHistory"} and value is not None:
                raise BridgeError("unsupported thread configuration override")
        if params.get("experimentalRawEvents") not in (None, False):
            raise BridgeError("raw events are unsupported")
        cwd, model = params.get("cwd"), params.get("model")
        if not isinstance(cwd, str) or not Path(cwd).is_absolute() or not Path(cwd).is_dir():
            raise BridgeError("absolute existing cwd required")
        if not isinstance(model, str) or not model or any(c in model for c in "\r\n\0"):
            raise BridgeError("explicit model required")
        if not os.environ.get("CODEX_HOME") or not Path(os.environ["CODEX_HOME"]).is_absolute():
            raise BridgeError("task CODEX_HOME required")
        self.cwd, self.model = cwd, model
        self.handle = "ac43-exec-bridge-" + str(uuid.uuid4())
        return {"thread": {"id": self.handle}, "transport": "codex_exec_test_bridge",
                "logical_handle": True}

    def turn(self, request_id, params):
        if set(params) - {"threadId", "input"} or params.get("threadId") != self.handle or self.used:
            raise BridgeError("unsupported turn or session")
        parts = params.get("input")
        if not isinstance(parts, list) or len(parts) != 1 or not isinstance(parts[0], dict):
            raise BridgeError("one text input required")
        item = parts[0]
        if item.get("type") != "text" or not isinstance(item.get("text"), str) or set(item) - {"type", "text", "text_elements"} or item.get("text_elements") not in (None, []):
            raise BridgeError("unsupported input content")
        prompt = item["text"]
        self.used = True
        turn = "bridge-turn-" + str(uuid.uuid4())
        folder = self.evidence / str(uuid.uuid4())
        folder.mkdir(mode=0o700)
        argv = [self.executable, "exec", "--json", "--skip-git-repo-check",
                "--sandbox", "read-only", "--model", self.model, "-"]
        facts = {"transport": "codex_exec_test_bridge", "logical_handle": True,
                 "adapter_session_handle": self.handle, "provider_actual_thread_id": None,
                 "managed_wrapper_pid": os.getpid(), "process_group": os.getpgrp(),
                 "argv": argv, "cwd": self.cwd, "codex_home": os.environ["CODEX_HOME"],
                 "prompt_sha256": hashlib.sha256(prompt.encode()).hexdigest(),
                 "bridge_sha256": sha(__file__), "real_binary_sha256": sha(self.executable),
                 "started_at_unix": time.time(), "status": "started", "physical_exec_children": 0, "spawn_attempted": True, "sandbox": "read-only", "pre_turn_config_warnings": 0}
        def save():
            target = folder / "launch.json"
            target.write_text(json.dumps(facts, indent=2) + "\n")
            target.chmod(0o600)
        save()
        started = False
        terminal = None
        answer = None
        actual = None
        failure = None
        exit_code = None
        with (folder / "stderr.private").open("wb") as stderr, (folder / "stdout.private").open("wb") as raw:
            try:
                # Inherit the supervisor's process group: Stop kills the wrapper
                # and this child together; no detached session or background work.
                self.child = subprocess.Popen(argv, cwd=self.cwd, stdin=subprocess.PIPE,
                    stdout=subprocess.PIPE, stderr=stderr, start_new_session=False)
                facts["physical_exec_children"] = 1
                facts["child_pid"] = self.child.pid
                facts["child_process_group"] = os.getpgid(self.child.pid)
                save()
                response(request_id, {"turn": {"id": turn}, "transport": "codex_exec_test_bridge"})
                self.child.stdin.write(prompt.encode())
                self.child.stdin.close()
                for line in self.child.stdout:
                    raw.write(line)
                    raw.flush()
                    try:
                        event = json.loads(line)
                    except (ValueError, UnicodeError):
                        raise BridgeError("invalid exec JSON")
                    kind = event.get("type")
                    if kind == "thread.started":
                        identity = event.get("thread_id")
                        if not isinstance(identity, str) or not identity or actual is not None:
                            raise BridgeError("invalid or duplicate exec thread identity")
                        uuid.UUID(identity)
                        actual = identity
                        facts["provider_actual_thread_id"] = actual
                        save()
                    elif kind == "turn.started":
                        if not actual or started:
                            raise BridgeError("invalid exec turn boundary")
                        started = True
                        self.notification("turn/started", turn, turn={"id": turn})
                    elif kind in ("item.started", "item.updated", "item.completed"):
                        value = event.get("item", {})
                        item_type = value.get("type")
                        # Codex 0.165 emits this known inert config warning twice
                        # before turn.started. Preserve it privately and count it;
                        # no other error/item is reclassified as a warning.
                        warning = value.get("message", "")
                        if (not started and terminal is None and kind == "item.completed"
                            and item_type == "error" and isinstance(warning, str)
                            and re.fullmatch(r"Codex is ignoring 1 unrecognized configuration setting\. Check for typos or deprecated settings\.\n  user [^\r\n]+: `disable_response_storage` is ignored\.", warning)):
                            facts["pre_turn_config_warnings"] += 1
                            continue
                        if not started or terminal is not None:
                            raise BridgeError("item outside active turn")
                        if item_type == "reasoning":
                            # Reasoning is preserved privately; it is not an answer.
                            continue
                        if item_type != "agent_message" or kind != "item.completed":
                            raise BridgeError("unsupported exec item; recording is incomplete")
                        if not isinstance(value.get("text"), str) or answer is not None:
                            raise BridgeError("invalid or ambiguous final answer")
                        answer = value["text"]
                        self.notification("item/completed", turn,
                            item={"id": value.get("id", "exec-answer"), "type": "agentMessage",
                                  "phase": "final_answer", "text": answer})
                    elif kind == "turn.completed":
                        usage = event.get("usage")
                        if not started or terminal is not None or not isinstance(usage, dict):
                            raise BridgeError("invalid exec completion")
                        for key in ("input_tokens", "output_tokens", "cached_input_tokens"):
                            if not isinstance(usage.get(key), int) or isinstance(usage[key], bool) or usage[key] < 0:
                                raise BridgeError("invalid exec usage")
                        terminal = usage
                    elif kind in ("error", "turn.failed"):
                        raise BridgeError("exec reported failure; details retained privately")
                    else:
                        raise BridgeError("unsupported exec event; recording is incomplete")
                self.child.stdout.close()
                exit_code = self.child.wait()
                if exit_code != 0 or not actual or terminal is None or answer is None or self.terminated:
                    raise BridgeError("exec did not complete successfully")
            except (BridgeError, OSError, ValueError) as error:
                failure = str(error) if isinstance(error, BridgeError) else "exec bridge IO or identity failure"
                if self.child is not None:
                    if self.child.poll() is None:
                        self.child.kill()
                    exit_code = self.child.wait()
            finally:
                facts.update(finished_at_unix=time.time(), exit_code=exit_code,
                             actual_exec_usage=terminal, status="failed" if failure else "completed",
                             failure=failure, stdout_drained=failure is None)
                save()
        if failure:
            self.notification("turn/completed", turn, turn={"id": turn, "status": "failed",
                "error": {"message": failure}})
        else:
            self.notification("turn/completed", turn, turn={"id": turn, "status": "completed", "usage": terminal})

    def serve(self):
        for line in sys.stdin:
            request_id = None
            try:
                request = json.loads(line)
                request_id, method = request.get("id"), request.get("method")
                params = request.get("params") or {}
                if method == "initialized" and request_id is None:
                    continue
                if method == "initialize" and not self.initialized:
                    self.initialized = True
                    response(request_id, {"userAgent": "ac43-codex-exec-test-bridge"})
                elif not self.initialized:
                    raise BridgeError("initialize required")
                elif method == "thread/start" and self.handle is None:
                    response(request_id, self.setup(params))
                elif method == "turn/start" and self.handle is not None:
                    self.turn(request_id, params)
                else:
                    refuse(request_id, -32601, "unsupported AC43 exec bridge operation")
            except (BridgeError, ValueError, TypeError) as error:
                refuse(request_id, -32602, str(error) if isinstance(error, BridgeError) else "invalid bridge request")


def main():
    os.umask(0o077)
    executable = os.environ.get("TEST_CODEX_EXEC_PATH", "")
    if not Path(executable).is_absolute() or not Path(executable).is_file() or not os.access(executable, os.X_OK):
        return 64
    if sys.argv[1:] == ["--version"]:
        os.execv(executable, [executable, "--version"])
    if sys.argv[1:] != ["app-server", "--listen", "stdio://"]:
        return 64
    evidence = Path(os.environ.get("TEST_CODEX_EXEC_EVIDENCE", ""))
    if not evidence.is_absolute() or not evidence.is_dir() or evidence.stat().st_mode & 0o077:
        return 64
    bridge = Bridge(executable, evidence)
    signal.signal(signal.SIGTERM, bridge.signal)
    signal.signal(signal.SIGINT, bridge.signal)
    bridge.serve()
    return 0


if __name__ == "__main__":
    sys.exit(main())
