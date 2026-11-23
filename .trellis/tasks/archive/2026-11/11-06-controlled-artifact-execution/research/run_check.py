#!/usr/bin/env python3
"""Run a recorded, task-owned verification without exporting private credentials."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

parser = argparse.ArgumentParser()
parser.add_argument("--environment", choices=["pure", "baseline", "migrations", "handler", "router", "service", "handler_final", "handler_race_final", "router_final", "router_race_final"], required=True)
parser.add_argument("--label", required=True)
parser.add_argument("command", nargs=argparse.REMAINDER)
args = parser.parse_args()
if not args.label or any(c not in "abcdefghijklmnopqrstuvwxyz0123456789-_" for c in args.label):
    parser.error("label must be a new lowercase run ID")
command = args.command[1:] if args.command[:1] == ["--"] else args.command
if not command or command[0] not in ("go", "bash", "make"):
    parser.error("expected a Go, make or guarded shell verification")
research = Path(__file__).resolve().parent
allocation = json.loads((research / "execution-allocation-20261110.json").read_text())
work = Path(allocation["worktree"])
private = json.loads(Path(allocation["private_environment"]).read_text())
state = Path(allocation["state_root"])
env = private["env"] if args.environment == "pure" else json.loads((state / (args.environment + "-env.json")).read_text())
if args.environment != "pure":
    expected = allocation["databases"][args.environment]["name"]
    if env.get("TASK_DATABASE_NAME") != expected:
        raise SystemExit("database allocation mismatch")
    found = subprocess.check_output(["docker", "exec", allocation["container"], "psql", "-U", "artifact_test", "-d", expected, "-Atqc", "SELECT current_database()"], text=True).strip()
    if found != expected:
        raise SystemExit("live database identity mismatch")
output = research / (args.label + ".json")
log = state / "logs" / (args.label + ".log")
if output.exists() or log.exists():
    raise SystemExit("run ID already used; retain the original result")
started = time.monotonic()
with log.open("xb") as handle:
    result = subprocess.run(["bash", str(work / "scripts/go-test-with-agent-cli-guard.sh"), *command], cwd=work, env=env, stdin=subprocess.DEVNULL, stdout=handle, stderr=subprocess.STDOUT)
counts = {"pass": 0, "fail": 0, "skip": 0}
names = {key: [] for key in counts}
for line in log.read_text(errors="replace").splitlines():
    try:
        event = json.loads(line)
    except (ValueError, TypeError):
        continue
    if isinstance(event, dict) and event.get("Test") and event.get("Action") in counts:
        action = event["Action"]
        counts[action] += 1
        names[action].append(event.get("Package", "") + ":" + event["Test"])
record = {"command": command, "environment": args.environment, "database": env.get("TASK_DATABASE_NAME"), "exit_code": result.returncode, "seconds": round(time.monotonic() - started, 3), "finished_at": datetime.datetime.now().astimezone().isoformat(), "log": str(log), "log_sha256": hashlib.sha256(log.read_bytes()).hexdigest(), "named_test_counts": counts, "test_names": names, "agent_cli_guard": True}
output.write_text(json.dumps(record, indent=2) + "\n")
print(json.dumps({key: record[key] for key in ["command", "environment", "exit_code", "seconds", "named_test_counts"]}))
print("record:", output)
raise SystemExit(result.returncode)
