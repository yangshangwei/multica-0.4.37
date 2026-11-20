#!/usr/bin/env python3
"""Synthetic app-server exercising real CLI/API identity, never a model result."""
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

if os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE"):
    raise SystemExit("Fake provider refuses real opt-in")
if "--version" in sys.argv:
    print("codex-cli 0.164.0")
    raise SystemExit(0)
if "app-server" not in sys.argv:
    raise SystemExit(2)
config = json.loads(Path(os.environ["SECREM_HARNESS_CONFIG"]).read_text())
if config["mode"] != "fake":
    raise SystemExit(3)
task_id = os.environ["MULTICA_TASK_ID"]
manifest = json.loads((Path(config["claim_manifest_dir"]) / (task_id + ".json")).read_text())
thread = str(uuid.uuid4())

def emit(value):
    print(json.dumps(value), flush=True)

def command(argv):
    result = subprocess.run([config["cli_path"]] + argv, capture_output=True, text=True, env=os.environ.copy())
    if result.returncode:
        sys.stderr.write(result.stderr)
        raise SystemExit(7)
    return result.stdout

for line in sys.stdin:
    request = json.loads(line)
    method = request.get("method")
    if method == "initialize":
        emit({"id": request["id"], "result": {"userAgent": "secrem-scripted-provider"}})
    elif method in {"thread/start", "thread/resume"}:
        emit({"id": request["id"], "result": {"thread": {"id": thread, "ephemeral": False, "path": None}, "model": config["model"]}})
    elif method == "turn/start":
        turn = str(uuid.uuid4())
        emit({"id": request["id"], "result": {"turn": {"id": turn, "status": "inProgress"}}})
        emit({"method": "turn/started", "params": {"threadId": thread, "turn": {"id": turn, "status": "inProgress"}}})
        issue = manifest["issue_id"]
        owner = manifest["owner_ids"][0]
        command(["issue", "get", issue, "--output", "json"])
        command(["issue", "comment", "add", issue, "--content", "分诊：hold/unknown。合成生产演练命中停止条件，停止所有本地修复和成员派发，交 incident 与人工。[@安全负责人](mention://member/" + owner + ") 请接手人工处置；尚未确认风险消除。", "--idempotency-key", str(uuid.uuid4()), "--output", "json"])
        command(["squad", "activity", issue, "action", "--reason", "Synthetic production incident handed to the named human owner"])
        command(["issue", "update", issue, "--status", "in_review"])
        emit({"method": "item/completed", "params": {"threadId": thread, "turnId": turn, "item": {"id": str(uuid.uuid4()), "type": "agentMessage", "text": "Synthetic handoff recorded; this is scripted plumbing evidence."}}})
        emit({"method": "turn/completed", "params": {"threadId": thread, "turn": {"id": turn, "status": "completed"}}})
    elif "id" in request:
        emit({"id": request["id"], "result": {}})
