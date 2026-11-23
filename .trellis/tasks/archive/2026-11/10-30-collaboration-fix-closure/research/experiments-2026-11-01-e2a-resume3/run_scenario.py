#!/usr/bin/env python3
"""Run exactly one registered scenario of the original context experiment.

Usage: run_scenario.py <scenario> <arm> <start_trial> [--check-only]

The scenario/arm/start_trial must match an entry in prerun.json. The script
refuses to run if either frozen HEAD or any framework hash differs, if a
product file is dirty, or if the scenario directory already exists. It writes
command.json before launch and adds finished_at/exit_code afterwards. The
framework writes records.jsonl itself; this script never edits it.
"""
import datetime
import hashlib
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
FRAMEWORK_DIR = "internal/daemon/collaborationrun"
# Session-only markers of the launching Claude Code shell. The child CLI must
# not see them; model/effort settings still come from ~/.claude/settings.json.
SESSION_ONLY_ENV = [
    "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXECPATH",
    "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_CHILD_SESSION",
    "CLAUDE_CODE_SESSION_ATTENDED", "CLAUDE_CODE_MESSAGING_SOCKET",
    "CLAUDE_CODE_MESSAGING_TOKEN", "CLAUDE_PID", "CLAUDE_EFFORT",
    "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR",
]


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha256(path):
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def fail(msg):
    print("REFUSED: " + msg, file=sys.stderr)
    sys.exit(2)


def preflight(prerun, arm):
    src = prerun["source_arms"][arm]
    server = src["cwd"]
    repo = os.path.dirname(server)
    head = subprocess.check_output(["git", "-C", repo, "rev-parse", "HEAD"], text=True).strip()
    if head != src["source_commit"]:
        fail("%s HEAD %s != registered %s" % (arm, head, src["source_commit"]))
    framework_paths = set()
    for name, want in prerun["framework_files"].items():
        rel = "server/%s/%s" % (FRAMEWORK_DIR, name)
        framework_paths.add(rel)
        got = sha256(os.path.join(repo, rel))
        if got != want:
            fail("%s %s sha256 %s != registered %s" % (arm, name, got, want))
    status = subprocess.check_output(["git", "-C", repo, "status", "--porcelain"], text=True)
    for line in status.splitlines():
        path = line[3:]
        if path not in framework_paths:
            fail("%s has unregistered change: %s" % (arm, line))
    return server, head


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    check_only = "--check-only" in sys.argv[1:]
    if len(args) != 3:
        fail("usage: run_scenario.py <scenario> <arm> <start_trial> [--check-only]")
    scenario, arm, start_trial = args[0], args[1], int(args[2])
    with open(os.path.join(HERE, "prerun.json")) as f:
        prerun = json.load(f)
    matches = [p for p in prerun["ordered_plan"]
               if p["scenario"] == scenario and p["arm"] == arm and p["start_trial"] == start_trial]
    if len(matches) != 1:
        fail("no single registered plan entry for %s/%s/%d" % (scenario, arm, start_trial))
    server, head = preflight(prerun, arm)
    out_dir = os.path.join(HERE, "%s-%s" % (scenario, arm))
    if os.path.exists(out_dir):
        fail("%s already exists; never overwrite evidence" % out_dir)
    explicit_env = {
        "MULTICA_RUN_REAL_AGENT_SMOKE": "1",
        "MULTICA_HOST_SMOKE_PROVIDER": prerun["provider"],
        "MULTICA_HOST_SMOKE_EXECUTABLE": prerun["executable"],
        "MULTICA_CONTEXT_EXPERIMENT_OUT": os.path.join(out_dir, "records.jsonl"),
        "MULTICA_CONTEXT_EXPERIMENT_COMMIT": head,
        "MULTICA_CONTEXT_EXPERIMENT_ARM": arm,
        "MULTICA_CONTEXT_EXPERIMENT_SCENARIO": scenario,
        "MULTICA_CONTEXT_EXPERIMENT_TRIALS": str(prerun["trials"]),
        "MULTICA_CONTEXT_EXPERIMENT_START_TRIAL": str(start_trial),
        "MULTICA_CONTEXT_EXPERIMENT_MODEL": prerun["requested_model"],
        "MULTICA_CONTEXT_EXPERIMENT_THINKING_LEVEL": prerun["thinking"],
        "MULTICA_CONTEXT_EXPERIMENT_SERVICE_TIER": prerun["service_tier"],
    }
    command = ["go", "test", "-tags=agentintegration", "./" + FRAMEWORK_DIR,
               "-run", "^TestHostContextExperiment$", "-count=1", "-v", "-timeout=45m"]
    if check_only:
        print("PREFLIGHT OK: %s/%s start_trial=%d call_cap=%d cwd=%s head=%s"
              % (scenario, arm, start_trial, matches[0]["call_cap"], server, head))
        return
    os.makedirs(out_dir)
    record = {
        "cwd": server,
        "command": command,
        "explicit_environment": explicit_env,
        "removed_session_environment": [k for k in SESSION_ONLY_ENV if k in os.environ],
        "started_at": now(),
        "account_settings": "inherited unchanged from ~/.claude/settings.json; no credentials recorded",
        "preflight": "frozen HEAD and three registered framework hashes match; no product source drift",
        "launched_by": prerun["registered_by_session"],
    }
    record_path = os.path.join(out_dir, "command.json")
    with open(record_path, "w") as f:
        json.dump(record, f, indent=2)
    env = {k: v for k, v in os.environ.items() if k not in SESSION_ONLY_ENV}
    env.update(explicit_env)
    with open(os.path.join(out_dir, "go-test.log"), "wb") as log:
        code = subprocess.call(command, cwd=server, env=env, stdout=log, stderr=subprocess.STDOUT)
    record["finished_at"] = now()
    record["exit_code"] = code
    with open(record_path, "w") as f:
        json.dump(record, f, indent=2)
    print("FINISHED %s/%s exit_code=%d" % (scenario, arm, code))
    sys.exit(code)


if __name__ == "__main__":
    main()
