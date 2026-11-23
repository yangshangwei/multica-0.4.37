#!/usr/bin/env python3
"""Capture actual production-daemon stdin; no real provider/account is used."""
import datetime
import json
import os
from pathlib import Path
import sys
import uuid

if "--version" in sys.argv:
    print("2.1.316 (Claude Code)")
    sys.exit(0)

capture = Path(os.environ["P6_PROVIDER_CAPTURE"])
if not capture.is_absolute() or not capture.parent.is_dir():
    raise SystemExit("Explicit owned capture path is required")
prompt = sys.stdin.readline()
session_id = str(uuid.uuid4())
if "--resume" in sys.argv:
    session_id = sys.argv[sys.argv.index("--resume") + 1]
record = {"at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
          "task_id": os.environ.get("MULTICA_TASK_ID"), "prompt": prompt,
          "argv": sys.argv[1:], "session_id": session_id, "pid": os.getpid()}
with capture.open("a") as output:
    output.write(json.dumps(record) + "\n")
print(json.dumps({"type": "system", "subtype": "init", "session_id": session_id}), flush=True)
print(json.dumps({"type": "assistant", "message": {"role": "assistant", "content": [
    {"type": "text", "text": "Controlled task execution captured its knowledge context."}]}}), flush=True)
print(json.dumps({"type": "result", "subtype": "success", "result": "Controlled task complete.",
                  "is_error": False, "session_id": session_id, "total_cost_usd": 0,
                  "usage": {"input_tokens": 10, "output_tokens": 5}}), flush=True)
