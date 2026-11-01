#!/usr/bin/env python3
"""Launch exactly one registered RK1 runtime scenario against the real Claude CLI.

Usage: run_rk1.py <scenario> <start_trial> [--check-only]

Refuses unless framework-registration.json says runnable, the scenario has a
runtime budget in runtime-addendum.json, every earlier scenario is complete,
no trial on record is incomplete or blocking_review, no started trial lacks a
public record, start_trial is the next unattempted trial, the remaining cap
covers every launch this call can start (consumed launches are the larger of
the public count and the wrapper's private launch records), this directory is
clean in git except results/, the product roots are clean and their
source_id, the framework SHA-256 and this file's SHA-256 all equal the
registration. Raw evidence stays under the gitignored .omx/private/rk1/;
only each trial's public.json is copied into results/.
"""
import datetime
import glob
import hashlib
import json
import os
import subprocess
import sys
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, *[".."] * 5))
SERVER = os.path.join(REPO, "server")
RESULTS = os.path.join(HERE, "results")
PRIVATE = os.path.join(REPO, ".omx", "private", "rk1")
SESSION_ONLY_ENV = [
    "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXECPATH", "CLAUDE_CODE_SESSION_ID",
    "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_SESSION_ATTENDED", "CLAUDE_CODE_MESSAGING_SOCKET",
    "CLAUDE_CODE_MESSAGING_TOKEN", "CLAUDE_PID", "CLAUDE_EFFORT", "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR",
    "MULTICA_KNOWLEDGE_CONTEXT_ENABLED", "ELECTRON_RUN_AS_NODE",
]
# Operator Claude configuration the real child loads (production host mode keeps
# the operator's HOME). Only SHA-256 values are recorded, never content.
OPERATOR_FILES = ["settings.json", "settings.local.json", "CLAUDE.md"]
OPERATOR_DIRS = ["hooks", "skills", "plugins", "agents", "commands"]
MANAGED_ROOTS = {"darwin": "/Library/Application Support/ClaudeCode", "linux": "/etc/claude-code"}
MINUTES_PER_LAUNCH = 7  # 5 min Provider timeout + 1 min Runner slack + fixture/seed runs


def fail(msg):
    print("REFUSED: " + msg, file=sys.stderr)
    sys.exit(2)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write_json(path, value, exclusive=False):
    with open(path, "x" if exclusive else "w") as f:
        json.dump(value, f, indent=1)
        f.flush()
        os.fsync(f.fileno())


def tree_sha256(root):
    """SHA-256 over sorted relative path, kind and content hash (or link target)."""
    lines = []
    for current, dirs, files in os.walk(root):
        for name in dirs:
            path = os.path.join(current, name)
            if os.path.islink(path):
                lines.append("%s\0link\0%s\n" % (os.path.relpath(path, root), os.readlink(path)))
        for name in files:
            path = os.path.join(current, name)
            rel = os.path.relpath(path, root)
            if os.path.islink(path):
                lines.append("%s\0link\0%s\n" % (rel, os.readlink(path)))
                continue
            try:
                lines.append("%s\0file\0%s\n" % (rel, sha256_file(path)))
            except OSError:
                lines.append("%s\0unreadable\0\n" % rel)
    h = hashlib.sha256()
    for line in sorted(lines):
        h.update(line.encode("utf-8", "surrogateescape"))
    return h.hexdigest()


def framework_sha256():
    # Same bytes as rk1FrameworkHash in server/cmd/server/rk1_entry_test.go.
    h = hashlib.sha256()
    for path in sorted(glob.glob(os.path.join(SERVER, "cmd", "server", "rk1_*_test.go"))):
        with open(path, "rb") as f:
            content = f.read()
        h.update(b"%s\x00%d\x00" % (os.path.basename(path).encode(), len(content)))
        h.update(content)
    return h.hexdigest()


def source_binding():
    out = os.path.join(PRIVATE, "source-manifest-check.json")
    os.makedirs(PRIVATE, exist_ok=True)
    try:
        subprocess.run([sys.executable, os.path.join(HERE, "source_manifest.py"), REPO, out], check=True, capture_output=True)
    except subprocess.CalledProcessError:
        fail("source manifest could not be generated for %s" % REPO)
    with open(out) as f:
        return json.load(f)


def trials_on_record(budget):
    """Accept only original, contiguous real-trial records in this series.

    A copied fake record or a duplicate under another filename must not advance
    the cursor or free quota. These checks precede all executable/account work.
    """
    records = []
    for path in sorted(glob.glob(os.path.join(RESULTS, "RK1-*.json"))):
        label = os.path.basename(path)[:-5]
        try:
            with open(path) as f:
                record = json.load(f)
        except (OSError, ValueError):
            fail("invalid trial record: " + label)
        scenario, trial = record.get("scenario"), record.get("trial")
        if (scenario not in budget or type(trial) is not int
                or not 1 <= trial <= budget[scenario]["trials"]
                or record.get("label") != label or label != "%s-t%d" % (scenario, trial)
                or record.get("mode") != "real" or not isinstance(record.get("runs"), list)):
            fail("invalid trial record: " + label)
        for run in record["runs"]:
            if (not isinstance(run, dict) or type(run.get("physical_launches")) is not int
                    or run["physical_launches"] < 0 or type(run.get("measured")) is not bool):
                fail("invalid launch count: " + label)
        records.append(record)
    for scenario in budget:
        trials = sorted(r["trial"] for r in records if r["scenario"] == scenario)
        if trials != list(range(1, len(trials) + 1)):
            fail("non-contiguous trials: " + scenario)
    return records


def real_launches(record):
    return sum(r["physical_launches"] for r in record.get("runs", []) if r.get("measured"))


def private_launches():
    """Real launches per trial label from the wrapper's own launch records.

    A trial that died before its public record was complete still consumed
    these. An unreadable record counts as a real launch.
    """
    counts = {}
    for path in glob.glob(os.path.join(PRIVATE, "*", "RK1-*-t*", "provider", "*", "launch-*", "launch.json")):
        label = path.split(os.sep)[-5]
        try:
            with open(path) as f:
                real = json.load(f).get("mode") == "real"
        except (OSError, ValueError):
            real = True
        counts[label] = counts.get(label, 0) + int(real)
    return counts


def launch_accounting(records):
    public = {r["label"]: real_launches(r) for r in records}
    private = private_launches()
    counted = {label: max(public.get(label, 0), private.get(label, 0))
               for label in sorted(set(public) | set(private))}
    return {"consumed": sum(counted.values()), "counted_per_trial": counted,
            "public_per_trial": public, "private_per_trial": private}


def check_prior_invocations():
    # A compile/launcher failure can happen before a trial directory exists.
    # Its started invocation is still an incomplete result, never a free retry.
    for path in sorted(glob.glob(os.path.join(PRIVATE, "*", "invocation-*.json"))):
        try:
            with open(path) as f:
                record = json.load(f)
        except (OSError, ValueError):
            fail("stop rule: unreadable invocation on record")
        if record.get("exit_code") != 0 or record.get("stop_state", {}).get("triggered") is True:
            fail("stop rule: incomplete or failed invocation on record: " + os.path.basename(path))


def inputs_dirty():
    """Paths under this directory that differ from HEAD, results/ excepted."""
    rel = os.path.relpath(HERE, REPO)
    try:
        status = subprocess.run(["git", "-C", REPO, "status", "--porcelain=v1", "--untracked-files=all", "--", rel],
                                check=True, capture_output=True, text=True).stdout
    except (OSError, subprocess.CalledProcessError):
        fail("git status failed for %s" % rel)
    results = os.path.relpath(RESULTS, REPO) + "/"
    return [line for line in status.splitlines()
            if not all(p.strip('"').startswith(results) for p in line[3:].split(" -> "))]


def operator_facts():
    claude = os.path.join(os.path.expanduser("~"), ".claude")
    facts = {}
    for name in OPERATOR_FILES:
        path = os.path.join(claude, name)
        facts["~/.claude/" + name] = sha256_file(path) if os.path.isfile(path) else None
    for name in OPERATOR_DIRS:
        path = os.path.join(claude, name)
        facts["~/.claude/" + name + "/"] = tree_sha256(path) if os.path.isdir(path) else None
    managed = MANAGED_ROOTS.get(sys.platform)
    if managed:
        facts["managed:" + managed + "/"] = tree_sha256(managed) if os.path.isdir(managed) else None
    settings = {}
    try:
        with open(os.path.join(claude, "settings.json")) as f:
            settings = json.load(f)
    except (OSError, ValueError):
        pass
    # Whether Claude's 30,000-char tool output preview was changed; the value
    # itself is configuration content and is not recorded.
    facts["output_cap_overridden"] = bool(settings.get("bashOutputMaxChars") or os.environ.get("BASH_MAX_OUTPUT_LENGTH")
                                          or (settings.get("env") or {}).get("BASH_MAX_OUTPUT_LENGTH"))
    return facts


def main():
    # Invocation logs and Provider captures are private even with a permissive
    # operator umask. Public records are copied deliberately into results/.
    os.umask(0o077)
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    check_only = "--check-only" in sys.argv[1:]
    if len(args) != 2:
        fail("usage: run_rk1.py <scenario> <start_trial> [--check-only]")
    scenario, start = args[0], int(args[1])
    with open(os.path.join(HERE, "runtime-addendum.json")) as f:
        addendum = json.load(f)
    with open(os.path.join(HERE, "framework-registration.json")) as f:
        reg = json.load(f)
    if reg.get("runnable") is not True:
        fail("framework-registration.json is not runnable: " + "; ".join(reg.get("not_runnable_reasons", [])))
    budget = {s["id"]: s for s in addendum["runtime_scenarios"]}
    if scenario not in budget:
        fail("%s has no runtime budget in runtime-addendum.json" % scenario)
    check_prior_invocations()
    records = trials_on_record(budget)
    broken = [r["label"] for r in records if r.get("status") != "finished" or r.get("verdict") == "blocking_review"]
    if broken:
        fail("stop rule: incomplete or blocking_review trial on record: %s" % ", ".join(broken))
    recorded = {r["label"] for r in records}
    unrecorded = sorted({os.path.basename(d) for d in glob.glob(os.path.join(PRIVATE, "*", "RK1-*-t*")) if os.path.isdir(d)} - recorded)
    if unrecorded:
        fail("stop rule: trial evidence without a public record: %s" % ", ".join(unrecorded))
    for earlier in addendum["order"][: addendum["order"].index(scenario)]:
        done = sum(1 for r in records if r["scenario"] == earlier)
        if done != budget[earlier]["trials"]:
            fail("order: %s has %d of %d trials on record" % (earlier, done, budget[earlier]["trials"]))
    done = sum(1 for r in records if r["scenario"] == scenario)
    if start != done + 1 or start > budget[scenario]["trials"]:
        fail("start_trial must be the next unattempted trial (%d)" % (done + 1))
    before = launch_accounting(records)
    consumed = before["consumed"]
    trials, launches = budget[scenario]["trials"], budget[scenario]["launches"]
    if launches % trials:
        fail("%s launches %d are not a whole number per trial" % (scenario, launches))
    need = (trials - start + 1) * (launches // trials)
    if consumed + need > addendum["runtime_call_cap"]:
        fail("cap: %d consumed + %d needed > %d" % (consumed, need, addendum["runtime_call_cap"]))
    dirty = inputs_dirty()
    if dirty:
        fail("launcher inputs differ from HEAD: %s" % "; ".join(dirty))
    binding = source_binding()
    if binding["dirty_product_paths"]:
        fail("product roots are dirty")
    if binding["source_id"] != reg["source_binding"]["source_id"]:
        fail("source_id %s is not the registered binding" % binding["source_id"])
    if framework_sha256() != reg["framework_sha256"] or sha256_file(os.path.abspath(__file__)) != reg["launcher_sha256"]:
        fail("framework or launcher SHA-256 differs from the registration")
    try:
        provider_sha256 = sha256_file(reg["provider_executable"])
    except (OSError, KeyError):
        fail("registered Provider executable cannot be fingerprinted")
    if provider_sha256 != reg.get("provider_executable_sha256"):
        fail("Provider executable SHA-256 differs from the registration")
    operator = operator_facts()
    if operator != reg.get("operator_configuration"):
        fail("operator configuration fingerprints differ from the registration")
    database = os.environ.get("DATABASE_URL", "")
    if not urllib.parse.urlparse(database).path.lstrip("/").startswith("multica_rk1_"):
        fail("DATABASE_URL must name an owned, freshly migrated multica_rk1_* database")
    out = os.path.join(PRIVATE, scenario)
    explicit = {
        "MULTICA_RUN_REAL_AGENT_SMOKE": "1", "MULTICA_RK1_SCENARIO": scenario, "MULTICA_RK1_START_TRIAL": str(start),
        "MULTICA_RK1_EXECUTABLE": reg["provider_executable"], "MULTICA_RK1_MODEL": reg["requested_model"], "MULTICA_RK1_OUT": out,
        "MULTICA_RK1_FRAMEWORK_SHA256": reg["framework_sha256"], "MULTICA_RK1_SOURCE_ID": reg["source_binding"]["source_id"],
        # TestRK1RealProvider checks these against its frozen scenario table.
        "MULTICA_RK1_TRIALS": str(trials), "MULTICA_RK1_LAUNCHES": str(launches),
    }
    timeout = MINUTES_PER_LAUNCH * need + 5
    command = ["go", "test", "-tags=agentintegration", "./cmd/server", "-run", "^TestRK1RealProvider$", "-count=1", "-v", "-timeout=%dm" % timeout]
    print("PREFLIGHT OK: %s start=%d trials=%d launches=%d consumed=%d cap=%d timeout=%dm head=%s" % (
        scenario, start, trials - start + 1, need, consumed, addendum["runtime_call_cap"], timeout, binding["head"]))
    if check_only:
        return
    os.makedirs(out, exist_ok=True)
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    record = {"scenario": scenario, "start_trial": start, "command": command, "cwd": SERVER, "head": binding["head"],
              "source_id": binding["source_id"], "framework_sha256": reg["framework_sha256"], "launcher_sha256": reg["launcher_sha256"],
              "explicit_environment": {k: v for k, v in explicit.items()}, "removed_session_environment": [k for k in SESSION_ONLY_ENV if k in os.environ],
              "provider_executable_target": os.path.realpath(reg["provider_executable"]), "provider_executable_sha256": provider_sha256,
              "operator_claude_files": operator,
              "launch_budget": {"cap": addendum["runtime_call_cap"], "reserved": need, "before": before},
              "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    record_path = os.path.join(out, "invocation-%s.json" % stamp)
    write_json(record_path, record, exclusive=True)
    env = {k: v for k, v in os.environ.items() if k not in SESSION_ONLY_ENV}
    env.update(explicit)
    with open(os.path.join(out, "go-test-%s.log" % stamp), "xb") as log:
        code = subprocess.call(command, cwd=SERVER, env=env, stdout=log, stderr=subprocess.STDOUT)
    os.makedirs(RESULTS, exist_ok=True)
    for trial_dir in sorted(glob.glob(os.path.join(out, scenario + "-t*"))):
        label = os.path.basename(trial_dir)
        target = os.path.join(RESULTS, label + ".json")
        if os.path.exists(target):
            continue
        public = os.path.join(trial_dir, "public.json")
        if os.path.isfile(public):
            with open(public, "rb") as src, open(target, "xb") as dst:
                dst.write(src.read())
                dst.flush()
                os.fsync(dst.fileno())
            continue
        # A started trial without its record fails closed: the stop rule holds
        # every later launch until a person reviews the private directory.
        write_json(target, {"series": "RK1", "label": label, "scenario": scenario, "trial": int(label.rsplit("-t", 1)[1]), "mode": "real", "status": "incomplete",
                            "verdict": "harness_fault:public_record_missing", "runs": [], "launches_unknown": True}, exclusive=True)
    after_records = trials_on_record(budget)
    after = launch_accounting(after_records)
    stopped = [r["label"] for r in after_records if r.get("status") != "finished" or r.get("verdict") == "blocking_review"]
    record["finished_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    record["executor_exit_code"] = code
    record["exit_code"] = code if code != 0 or not stopped else 1
    record["launch_budget"].update({"after": after, "remaining": max(0, addendum["runtime_call_cap"] - after["consumed"])})
    record["stop_state"] = {"triggered": bool(stopped) or code != 0, "incomplete_or_blocking_trials": stopped}
    write_json(record_path, record)
    print("FINISHED %s exit_code=%d consumed=%d remaining=%d stopped=%s; public records in %s" % (
        scenario, record["exit_code"], after["consumed"], record["launch_budget"]["remaining"], bool(stopped), RESULTS))
    sys.exit(record["exit_code"])


if __name__ == "__main__":
    main()
