#!/usr/bin/env python3
"""Record exact isolated Go verification invocations and their evidence."""

import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
from urllib.parse import urlparse


out = Path(__file__).resolve().parent
root = out.parents[4]
database = "multica_knowledge_c2_20261101_7d95f46f"
assert urlparse(os.environ["DATABASE_URL"]).path.removeprefix("/") == database
label, *command = sys.argv[1:]
assert command and command[0] == "go"
log = out / f"{label}.jsonl"
record = {
    "command": command,
    "cwd": str(root / "server"),
    "environment_file": "/tmp/multica_knowledge_c2_20261101_7d95f46f.env",
    "database": database,
    "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
}
with log.open("wb") as stream:
    result = subprocess.run(command, cwd=record["cwd"], stdout=stream, stderr=subprocess.STDOUT)
record["finished_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
record["exit_code"] = result.returncode
record["log"] = log.name
record["log_sha256"] = hashlib.sha256(log.read_bytes()).hexdigest()
events = [json.loads(line) for line in log.read_text().splitlines() if line.startswith("{")]
for action in ("run", "pass", "fail", "skip"):
    record[action] = [event["Test"] for event in events if event.get("Action") == action and "Test" in event]
record["top_level_passes"] = len([name for name in record["pass"] if "/" not in name])
record["leaf_passes"] = len([name for name in record["pass"] if not any(other.startswith(name + "/") for other in record["pass"])])
(out / f"{label}.command.json").write_text(json.dumps(record, indent=2) + "\n")
print(json.dumps({key: record[key] for key in ("command", "started_at", "finished_at", "exit_code", "top_level_passes", "leaf_passes", "fail", "skip")}))
sys.exit(result.returncode)
