#!/usr/bin/env python3
"""Record the task-owned mobile verification lane without changing app code."""

import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[5]
EVIDENCE = Path(__file__).resolve().parent
EXPECTED_SHA = "7af9ab9905c98c77bb091223ebe687c639475bb4"


def utc_now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    if sha() != EXPECTED_SHA:
        raise RuntimeError("Candidate SHA changed before mobile verification")
    env = os.environ.copy()
    env.update({
        "CI": "1",
        "EXPO_OFFLINE": "1",
        "EXPO_NO_TELEMETRY": "1",
        "EXPO_NO_DOTENV": "1",
    })
    commands = [
        ("typecheck", ["pnpm", "--filter", "@multica/mobile", "typecheck"]),
        ("lint", ["pnpm", "--filter", "@multica/mobile", "lint"]),
        ("unit", ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "pnpm", "--filter", "@multica/mobile", "exec", "vitest", "run", "--maxWorkers=1", "--reporter=json", "--outputFile=" + str(EVIDENCE / "vitest-results.json")]),
        ("ios-wrapper", ["bash", "apps/mobile/scripts/ios-run.test.sh"]),
        ("simulator-inventory", ["xcrun", "simctl", "list", "devices", "available", "--json"]),
    ]
    results = []
    for name, command in commands:
        if sha() != EXPECTED_SHA:
            raise RuntimeError("Candidate SHA changed during mobile verification")
        stdout = EVIDENCE / (name + ".stdout.txt")
        stderr = EVIDENCE / (name + ".stderr.txt")
        record = {
            "name": name,
            "candidate_sha": sha(),
            "command": command,
            "cwd": str(ROOT),
            "started_at": utc_now(),
            "state": "running",
            "environment_overrides": {k: env[k] for k in ["CI", "EXPO_OFFLINE", "EXPO_NO_TELEMETRY", "EXPO_NO_DOTENV"]},
        }
        record_path = EVIDENCE / (name + ".json")
        record_path.write_text(json.dumps(record, indent=2) + "\n")
        with stdout.open("w") as out, stderr.open("w") as err:
            result = subprocess.run(command, cwd=ROOT, env=env, stdout=out, stderr=err)
        record.update({
            "state": "complete",
            "finished_at": utc_now(),
            "exit_code": result.returncode,
            "candidate_sha_after": sha(),
            "stdout": stdout.name,
            "stderr": stderr.name,
            "stdout_sha256": digest(stdout),
            "stderr_sha256": digest(stderr),
        })
        record_path.write_text(json.dumps(record, indent=2) + "\n")
        results.append(record)
        (EVIDENCE / "commands.json").write_text(json.dumps(results, indent=2) + "\n")
        print(json.dumps({"name": name, "exit_code": result.returncode, "finished_at": record["finished_at"]}), flush=True)
    print("Mobile command lane finished", flush=True)


if __name__ == "__main__":
    main()
