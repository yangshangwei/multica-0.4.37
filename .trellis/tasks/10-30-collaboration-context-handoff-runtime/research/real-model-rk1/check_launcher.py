#!/usr/bin/env python3
"""Exercise run_rk1.py refusals with --check-only against scratch copies.

Cases with git=True build a scratch repository (copied RK1 Go files under
server/, stub apps/ and packages/, the launcher inputs committed) and register
its real framework hash, launcher hash and source_id, so the checks past the
input-cleanliness gate and the PREFLIGHT line itself are reached.
"""
import glob
import hashlib
import json
import os
import re
import runpy
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

SRC = str(Path(__file__).resolve().parent)
WT = str(Path(__file__).resolve().parents[5])
INPUTS = ["run_rk1.py", "runtime-addendum.json", "framework-registration.json", "source_manifest.py"]
GIT = ["git", "-c", "user.name=rk1-check", "-c", "user.email=rk1-check@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"]


def sha256(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


def framework(root):
    h = hashlib.sha256()
    for path in sorted(glob.glob(os.path.join(root, "server", "cmd", "server", "rk1_*_test.go"))):
        content = open(path, "rb").read()
        h.update(b"%s\x00%d\x00" % (os.path.basename(path).encode(), len(content)))
        h.update(content)
    return h.hexdigest()


def case(name, records, args, want, reg_patch=None, private=None, git=False, mutate=None, stub=None, expected_exit=None):
    root = tempfile.mkdtemp(prefix="rk1-launch-")
    # Mirror the 5-level layout so REPO resolves to the scratch root.
    here = os.path.join(root, "a", "b", "c", "d", "e")
    os.makedirs(os.path.join(here, "results"))
    for f in INPUTS:
        shutil.copy(os.path.join(SRC, f), here)
    reg_path = os.path.join(here, "framework-registration.json")
    reg = json.load(open(reg_path))
    reg["runnable"] = True
    provider = os.path.join(root, "test-provider")
    with open(provider, "w") as f:
        f.write("test-owned Provider executable fingerprint; never executed\n")
    reg["provider_executable"] = provider
    reg["provider_executable_sha256"] = sha256(provider)
    operator_home = os.path.join(root, "operator-home")
    os.makedirs(operator_home)
    original_home = os.environ["HOME"]
    try:
        os.environ["HOME"] = operator_home
        reg["operator_configuration"] = runpy.run_path(os.path.join(here, "run_rk1.py"))["operator_facts"]()
    finally:
        os.environ["HOME"] = original_home
    if git:
        os.makedirs(os.path.join(root, "server", "cmd", "server"))
        for path in glob.glob(WT + "/server/cmd/server/rk1_*_test.go"):
            shutil.copy(path, os.path.join(root, "server", "cmd", "server"))
        for product in ["apps", "packages"]:
            os.makedirs(os.path.join(root, product))
            open(os.path.join(root, product, "README"), "w").write(product + "\n")
        subprocess.run(GIT + ["-C", root, "init", "-q"], check=True)
        subprocess.run(GIT + ["-C", root, "add", "-A"], check=True)
        subprocess.run(GIT + ["-C", root, "commit", "-qm", "product"], check=True)
        manifest = os.path.join(root, "manifest.json")
        subprocess.run([sys.executable, os.path.join(here, "source_manifest.py"), root, manifest], check=True, capture_output=True)
        reg.update({"framework_sha256": framework(root), "launcher_sha256": sha256(os.path.join(here, "run_rk1.py")),
                    "source_binding": {"source_id": json.load(open(manifest))["source_id"]}})
        os.unlink(manifest)
    reg.update(reg_patch or {})
    json.dump(reg, open(reg_path, "w"))
    if git:
        subprocess.run(GIT + ["-C", root, "add", "-A"], check=True)
        subprocess.run(GIT + ["-C", root, "commit", "-qm", "inputs"], check=True)
    if mutate:
        mutate(here)
    for label, r in records.items():
        json.dump(r, open(os.path.join(here, "results", label + ".json"), "w"))
    for rel, content in (private or {}).items():
        path = os.path.join(root, ".omx", "private", "rk1", rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        if content is not None:
            open(path, "w").write(content)
    env = {**os.environ, "HOME": operator_home, "DATABASE_URL": "postgres://x@127.0.0.1/multica_rk1_check"}
    options = ["--check-only"]
    if stub:
        bindir = os.path.join(root, "fake-bin")
        os.makedirs(bindir)
        executable = os.path.join(bindir, "go")
        with open(executable, "w") as f:
            f.write("#!%s\n" % sys.executable + STUB_GO.replace("STUB_MODE", repr(stub)))
        os.chmod(executable, 0o700)
        env["PATH"] = bindir + os.pathsep + env["PATH"]
        options = []
    p = subprocess.run([sys.executable, os.path.join(here, "run_rk1.py"), *args, *options], capture_output=True, text=True, env=env)
    text = (p.stdout + p.stderr).strip()
    out = text.splitlines()[-1] if text else ""
    if expected_exit is None:
        expected_exit = 0 if want.startswith("PREFLIGHT") else 2
    ok = want in out and p.returncode == expected_exit
    if stub:
        invocation = json.load(open(glob.glob(os.path.join(root, ".omx", "private", "rk1", "*", "invocation-*.json"))[0]))
        # The stub records two launches, with the public summary reporting
        # only one. Quota uses the larger actual private count.
        ok = ok and invocation["launch_budget"]["after"]["consumed"] == 2
        ok = ok and invocation["launch_budget"]["remaining"] == 23
        ok = ok and invocation["stop_state"]["triggered"] == (stub != "complete")
        if stub == "missing":
            public = json.load(open(os.path.join(here, "results", "RK1-E0a-t1.json")))
            ok = ok and public["status"] == "incomplete" and public["launches_unknown"] is True
        if stub != "complete":
            again = subprocess.run([sys.executable, os.path.join(here, "run_rk1.py"), *args, "--check-only"], capture_output=True, text=True, env=env)
            ok = ok and again.returncode == 2 and "stop rule: incomplete or failed invocation" in again.stderr
    print(("OK  " if ok else "BAD ") + name + ": " + out)
    shutil.rmtree(root)
    return ok


def rec(scenario, trial, status="finished", launches=1, verdict="pass"):
    return {"label": "%s-t%d" % (scenario, trial), "scenario": scenario, "trial": trial, "mode": "real", "status": status, "verdict": verdict,
            "runs": [{"measured": True, "physical_launches": launches}]}


STUB_GO = '''import json, os, pathlib, sys
mode = STUB_MODE
scenario = os.environ["MULTICA_RK1_SCENARIO"]
trial = int(os.environ["MULTICA_RK1_START_TRIAL"])
label = "%s-t%d" % (scenario, trial)
out = pathlib.Path(os.environ["MULTICA_RK1_OUT"]) / label
for n in (1, 2):
    launch = out / "provider" / "run" / ("launch-%d" % n)
    launch.mkdir(parents=True)
    (launch / "launch.json").write_text(json.dumps({"mode": "real"}))
if mode != "missing":
    (out / "public.json").write_text(json.dumps({"series": "RK1", "label": label, "scenario": scenario, "trial": trial, "mode": "real",
        "status": "finished", "verdict": "blocking_review" if mode == "blocking" else "pass",
        "runs": [{"measured": True, "physical_launches": 1}]}))
sys.exit(7 if mode == "missing" else 0)
'''


def done(*scenarios):
    out = {"RK1-E0a-t1": rec("RK1-E0a", 1)}
    for s in scenarios:
        out.update({"%s-t%d" % (s, i): rec(s, i) for i in (1, 2, 3)})
    return out


def go_table_matches_addendum():
    """The Go scenario table (trials, launches per trial) against the addendum."""
    text = open(WT + "/server/cmd/server/rk1_scenarios_recall_test.go").read()
    table = {m[0]: (int(m[1]), int(m[2])) for m in re.findall(r'"(RK1-[A-Za-z0-9]+)":\s*\{id: "[^"]+", trials: (\d+), launches: (\d+)', text)}
    addendum = {s["id"]: (s["trials"], s["launches"]) for s in json.load(open(os.path.join(SRC, "runtime-addendum.json")))["runtime_scenarios"]}
    want = {k: (t, t * l) for k, (t, l) in table.items()}
    ok = want == addendum and len(table) == 8
    print(("OK  " if ok else "BAD ") + "go_table_matches_addendum: go=%s addendum=%s" % (sorted(want.items()), sorted(addendum.items())))
    return ok


def write_mutation(name, text):
    return lambda here: open(os.path.join(here, name), "a").write(text)


# 23 real launch records + 1 unreadable (counts as real) + 1 fake (does not).
crowded = {"RK1-E0a/RK1-E0a-t1/provider/run-%d/launch-%d/launch.json" % (r, i): '{"mode": "real"}' for r in (1, 2, 3) for i in range(1, 9)}
crowded["RK1-E0a/RK1-E0a-t1/provider/run-3/launch-8/launch.json"] = "not json"
crowded["RK1-E0a/RK1-E0a-t1/provider/run-4/launch-1/launch.json"] = '{"mode": "fake"}'
crowded["RK1-E0a/RK1-E0a-t1/provider/run-3/launch-7/launch.json"] = '{"mode": "fake"}'

results = [
    go_table_matches_addendum(),
    case("not_runnable", {}, ["RK1-E0a", "1"], "is not runnable", reg_patch={"runnable": False}),
    case("unbudgeted_K1", {}, ["RK1-K1", "1"], "has no runtime budget"),
    case("H2b_waits_for_H2", done("RK1-H1"), ["RK1-H2b", "1"], "order: RK1-H2 has 0 of 3"),
    case("order", {}, ["RK1-H1", "1"], "order: RK1-E0a has 0 of 1"),
    case("incomplete_on_record", {"RK1-E0a-t1": rec("RK1-E0a", 1, "incomplete")}, ["RK1-H1", "1"], "stop rule: incomplete or blocking_review"),
    case("blocking_review_stops", {"RK1-E0a-t1": rec("RK1-E0a", 1), "RK1-H1-t1": rec("RK1-H1", 1, verdict="blocking_review")}, ["RK1-H1", "2"],
         "stop rule: incomplete or blocking_review trial on record: RK1-H1-t1"),
    case("unrecorded_trial_stops", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "trial evidence without a public record: RK1-H1-t1",
         private={"RK1-H1/RK1-H1-t1/public.json": None}),
    case("cursor_skip", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "2"], "next unattempted trial (1)"),
    case("cursor_replay", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-E0a", "1"], "next unattempted trial (2)"),
    case("cap_public", {"RK1-E0a-t1": rec("RK1-E0a", 1, launches=23)}, ["RK1-H1", "1"], "cap: 23 consumed + 3 needed > 25"),
    case("cap_private_launch_records", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "cap: 23 consumed + 3 needed > 25", private=crowded),
    case("fake_record_refused", {"RK1-E0a-t1": {**rec("RK1-E0a", 1), "mode": "fake"}}, ["RK1-H1", "1"], "invalid trial record: RK1-E0a-t1", git=True),
    case("mismatched_filename_refused", {"RK1-E0a-t9": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "invalid trial record: RK1-E0a-t9", git=True),
    case("missing_trial_refused", {"RK1-E0a-t1": rec("RK1-E0a", 1), "RK1-H1-t2": rec("RK1-H1", 2)}, ["RK1-H1", "2"], "non-contiguous trials: RK1-H1", git=True),
    case("negative_launch_count_refused", {"RK1-E0a-t1": rec("RK1-E0a", 1, launches=-10)}, ["RK1-H1", "1"], "invalid launch count: RK1-E0a-t1", git=True),
    case("unfinished_invocation_refused", {}, ["RK1-E0a", "1"], "incomplete or failed invocation on record",
         private={"RK1-E0a/invocation-started.json": '{"started_at":"now"}'}),
    case("failed_invocation_refused", {}, ["RK1-E0a", "1"], "incomplete or failed invocation on record",
         private={"RK1-E0a/invocation-failed.json": '{"exit_code":7}'}),
    case("unreadable_invocation_refused", {}, ["RK1-E0a", "1"], "unreadable invocation on record",
         private={"RK1-E0a/invocation-broken.json": 'not json'}),
    case("not_a_git_checkout", done("RK1-H1", "RK1-H2", "RK1-H2b", "RK1-H3a", "RK1-H3b", "RK1-H5"), ["RK1-H6", "1"], "git status failed"),
    case("inputs_modified", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "launcher inputs differ from HEAD", git=True,
         mutate=write_mutation("runtime-addendum.json", "\n")),
    case("inputs_untracked", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "launcher inputs differ from HEAD", git=True,
         mutate=write_mutation("extra-scenarios.json", "{}\n")),
    case("source_id_mismatch", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "is not the registered binding", git=True,
         reg_patch={"source_binding": {"source_id": "0" * 64}}),
    case("provider_fingerprint_mismatch", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "Provider executable SHA-256 differs", git=True,
         reg_patch={"provider_executable_sha256": "0" * 64}),
    case("operator_fingerprint_mismatch", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "operator configuration fingerprints differ", git=True,
         reg_patch={"operator_configuration": {}}),
    case("framework_mismatch", {"RK1-E0a-t1": rec("RK1-E0a", 1)}, ["RK1-H1", "1"], "framework or launcher SHA-256 differs", git=True,
         reg_patch={"framework_sha256": "0" * 64}),
    # results/ is untracked in these repos, so a clean-except-results tree passes.
    case("preflight_H1_resume", {"RK1-E0a-t1": rec("RK1-E0a", 1), "RK1-H1-t1": rec("RK1-H1", 1)}, ["RK1-H1", "2"],
         "PREFLIGHT OK: RK1-H1 start=2 trials=2 launches=2 consumed=2 cap=25 timeout=19m", git=True),
    case("preflight_full_set_reaches_H6", done("RK1-H1", "RK1-H2", "RK1-H2b", "RK1-H3a", "RK1-H3b", "RK1-H5"), ["RK1-H6", "1"],
         "PREFLIGHT OK: RK1-H6 start=1 trials=3 launches=6 consumed=19 cap=25 timeout=47m", git=True),
    case("execution_accounting", {}, ["RK1-E0a", "1"], "exit_code=0 consumed=2 remaining=23 stopped=False", git=True, stub="complete", expected_exit=0),
    case("execution_missing_public_stops", {}, ["RK1-E0a", "1"], "exit_code=7 consumed=2 remaining=23 stopped=True", git=True, stub="missing", expected_exit=7),
    case("execution_canary_stops", {}, ["RK1-E0a", "1"], "exit_code=1 consumed=2 remaining=23 stopped=True", git=True, stub="blocking", expected_exit=1),
]
sys.exit(0 if all(results) else 1)
