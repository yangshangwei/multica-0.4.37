#!/usr/bin/env python3
"""Append one preregistered remainder of the original context experiment.

Usage: run_scenario.py <scenario> <arm> <start_trial> [--check-only]

The original product HEADs and trial IDs stay frozen. The separately hashed
successor framework only hardens E5's existing canary stop rule. Check-only
never starts Go or probes the Provider; a live run requires the coordinator's
independent-review, complete-verification and exclusive-window clearance.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
FRAMEWORK_DIR = Path("internal/daemon/collaborationrun")
SESSION_ONLY_ENV = (
    "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXECPATH",
    "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_CHILD_SESSION",
    "CLAUDE_CODE_SESSION_ATTENDED", "CLAUDE_CODE_MESSAGING_SOCKET",
    "CLAUDE_CODE_MESSAGING_TOKEN", "CLAUDE_PID", "CLAUDE_EFFORT",
    "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR",
)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def fail(message):
    print("REFUSED: " + message, file=sys.stderr)
    raise SystemExit(2)


def preflight(prerun, arm):
    source = prerun["source_arms"][arm]
    server = Path(source["cwd"])
    repo = server.parent
    head = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    if head != source["source_commit"]:
        fail(arm + " product HEAD differs from preregistration")
    paths = set()
    joined = bytearray()
    for name, expected in sorted(prerun["framework_files"].items()):
        relative = Path("server") / FRAMEWORK_DIR / name
        paths.add(relative.as_posix())
        file = repo / relative
        data = file.read_bytes()
        if hashlib.sha256(data).hexdigest() != expected:
            fail(arm + " framework file differs from preregistration: " + name)
        joined.extend(name.encode() + b"\0" + str(len(data)).encode() + b"\0" + data)
    if hashlib.sha256(joined).hexdigest() != prerun["framework_sha256"]:
        fail(arm + " combined framework differs from preregistration")
    actual_names = {file.name for file in (server / FRAMEWORK_DIR).glob("host_context_experiment*.go")}
    if actual_names != set(prerun["framework_files"]):
        fail(arm + " has an unregistered framework file")
    status = subprocess.check_output(["git", "-C", str(repo), "status", "--porcelain", "--untracked-files=all"], text=True)
    for line in status.splitlines():
        if line[3:] not in paths:
            fail(arm + " has an unregistered source change")
    return str(server), head


def require_live_clearance(prerun):
    path = HERE / "execution-clearance.json"
    if not path.is_file():
        fail("coordinator review/verification/exclusive-window clearance is absent")
    clearance = json.loads(path.read_text())
    for key in ("authorization_confirmed", "wholecheck_passed", "exclusive_window_confirmed"):
        if clearance.get(key) is not True:
            fail("coordinator clearance has not confirmed " + key)
    required = {
        "owner_session": prerun["registered_by_session"],
        "authorization_confirmed": True,
        "wholecheck_passed": True,
        "exclusive_window_confirmed": True,
        "blocking_findings_open": 0,
        "remaining_series_call_cap": prerun["remaining_provider_call_cap"],
        "framework_sha256": prerun["framework_sha256"],
        "runner_sha256": sha256(__file__),
        "prerun_sha256": sha256(HERE / "prerun.json"),
    }
    if any(clearance.get(key) != expected for key, expected in required.items()):
        fail("coordinator clearance does not bind the current reviewed execution")


def main():
    args = sys.argv[1:]
    if any(arg.startswith("--") and arg != "--check-only" for arg in args):
        fail("unknown option")
    check_only = "--check-only" in args
    args = [arg for arg in args if arg != "--check-only"]
    if len(args) != 3:
        fail("usage: run_scenario.py <scenario> <arm> <start_trial> [--check-only]")
    scenario, arm, start = args
    try:
        start_trial = int(start)
    except ValueError:
        fail("start_trial must be a preregistered integer")
    prerun = json.loads((HERE / "prerun.json").read_text())
    if prerun["trials"] != 3 or sum(item["call_cap"] for item in prerun["ordered_plan"]) != 14:
        fail("the fixed three-slot series and remaining 14-call cap differ")
    matches = [item for item in prerun["ordered_plan"]
               if (item["scenario"], item["arm"], item["start_trial"]) == (scenario, arm, start_trial)]
    if len(matches) != 1:
        fail("no single preregistered scenario/arm/trial remainder")
    sources = {name: preflight(prerun, name) for name in prerun["source_arms"]}
    if sha256(prerun["executable"]) != prerun["executable_sha256"]:
        fail("the fixed Provider executable differs from preregistration")
    server, head = sources[arm]
    out = HERE / (scenario + "-" + arm)
    if out.exists():
        fail("scenario output already exists; never overwrite or replace evidence")
    if check_only:
        print("PREFLIGHT OK: %s/%s start_trial=%d call_cap=%d source=%s framework=%s"
              % (scenario, arm, start_trial, matches[0]["call_cap"], head, prerun["framework_sha256"]))
        return 0
    require_live_clearance(prerun)
    out.mkdir(mode=0o700)
    explicit_env = {
        "MULTICA_RUN_REAL_AGENT_SMOKE": "1",
        "MULTICA_HOST_SMOKE_PROVIDER": prerun["provider"],
        "MULTICA_HOST_SMOKE_EXECUTABLE": prerun["executable"],
        "MULTICA_CONTEXT_EXPERIMENT_OUT": str(out / "records.jsonl"),
        "MULTICA_CONTEXT_EXPERIMENT_COMMIT": head,
        "MULTICA_CONTEXT_EXPERIMENT_ARM": arm,
        "MULTICA_CONTEXT_EXPERIMENT_SCENARIO": scenario,
        "MULTICA_CONTEXT_EXPERIMENT_TRIALS": "3",
        "MULTICA_CONTEXT_EXPERIMENT_START_TRIAL": str(start_trial),
        "MULTICA_CONTEXT_EXPERIMENT_MODEL": prerun["requested_model"],
        "MULTICA_CONTEXT_EXPERIMENT_THINKING_LEVEL": prerun["thinking"],
        "MULTICA_CONTEXT_EXPERIMENT_SERVICE_TIER": prerun["service_tier"],
    }
    command = [prerun["go_executable"], "test", "-tags=agentintegration", "./" + str(FRAMEWORK_DIR),
               "-run", "^TestHostContextExperiment$", "-count=1", "-v", "-timeout=45m"]
    record = {
        "cwd": server, "command": command, "explicit_environment": explicit_env,
        "removed_session_environment": [key for key in SESSION_ONLY_ENV if key in os.environ],
        "started_at": now(), "state": "running", "launched_by": prerun["registered_by_session"],
        "account_settings": "inherited unchanged; credentials never recorded",
        "preflight": "both frozen product HEADs and exact successor framework match",
        "framework_sha256": prerun["framework_sha256"], "runner_sha256": sha256(__file__),
        "call_cap": matches[0]["call_cap"], "prior_invalid_slots_preserved": True,
    }
    path = out / "command.json"
    with path.open("x") as file:
        json.dump(record, file, indent=2)
    path.chmod(0o600)
    env = {key: value for key, value in os.environ.items() if key not in SESSION_ONLY_ENV}
    for key in list(env):
        if key.startswith("COLLAB_CONTEXT_TEST_") or key == "COLLAB_HOST_TEST_MODE":
            env.pop(key)
    env.update(explicit_env)
    log_path = out / "go-test.log"
    with log_path.open("xb") as log:
        log_path.chmod(0o600)
        code = subprocess.call(command, cwd=server, env=env, stdout=log, stderr=subprocess.STDOUT)
    record.update(finished_at=now(), state="complete", exit_code=code, log_sha256=sha256(log_path))
    path.write_text(json.dumps(record, indent=2) + "\n")
    print("FINISHED %s/%s exit_code=%d" % (scenario, arm, code))
    return code


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, KeyError):
        fail("required preregistered file or data is unavailable")
