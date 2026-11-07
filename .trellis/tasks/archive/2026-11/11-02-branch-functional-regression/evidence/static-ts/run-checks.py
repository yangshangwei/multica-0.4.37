from __future__ import annotations

import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile


REPO = Path(__file__).resolve().parents[5]
OUT = Path(__file__).resolve().parent
TASK = OUT.parent.parent
EXPECTED = "7af9ab9905c98c77bb091223ebe687c639475bb4"
RUNS: list[dict] = []


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def head() -> str:
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=REPO, text=True).strip()


def clean_env() -> dict[str, str]:
    keep = {"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "TERM", "TMPDIR", "TMP", "TEMP", "__CF_USER_TEXT_ENCODING"}
    env = {k: v for k, v in os.environ.items() if k in keep or k.startswith("LC_")}
    env["NO_COLOR"] = "1"
    env["CI"] = "1"
    env["TURBO_TELEMETRY_DISABLED"] = "1"
    return env


def record(name: str, command: list[str], *, cwd: Path = REPO, note: str = "") -> dict:
    if head() != EXPECTED:
        raise RuntimeError("Candidate changed before " + name)
    run_dir = OUT / name
    run_dir.mkdir(parents=True, exist_ok=False)
    entry = {"id": name, "command": command, "cwd": str(cwd), "candidate_before": head(), "started_at": now(), "note": note, "state": "running"}
    (run_dir / "run.json").write_text(json.dumps(entry, indent=2) + "\n")
    print("START", name, flush=True)
    with (run_dir / "stdout.log").open("w") as stdout, (run_dir / "stderr.log").open("w") as stderr:
        result = subprocess.run(command, cwd=cwd, env=clean_env(), stdout=stdout, stderr=stderr)
    entry.update({"exit_code": result.returncode, "finished_at": now(), "candidate_after": head(), "state": "complete"})
    for stream in ["stdout", "stderr"]:
        path = run_dir / (stream + ".log")
        entry[stream + "_sha256"] = hashlib.sha256(path.read_bytes()).hexdigest()
    (run_dir / "run.json").write_text(json.dumps(entry, indent=2) + "\n")
    RUNS.append(entry)
    (OUT / "runs.json").write_text(json.dumps(RUNS, indent=2) + "\n")
    print("DONE", name, "exit", result.returncode, flush=True)
    return entry


def source_snapshot() -> Path:
    snapshot = OUT / "source-probe-snapshot"
    snapshot.mkdir(exist_ok=False)
    source_exts = {".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".json", ".sh"}
    command = ["git", "archive", "--format=tar", EXPECTED, "apps", "packages", "scripts", "e2e", "package.json"]
    process = subprocess.Popen(command, cwd=REPO, stdout=subprocess.PIPE)
    assert process.stdout is not None
    extracted = []
    with tarfile.open(fileobj=process.stdout, mode="r|") as archive:
        for member in archive:
            if member.isfile() and Path(member.name).suffix in source_exts:
                archive.extract(member, path=snapshot, filter="data")
                extracted.append(member.name)
    process.stdout.close()
    if process.wait() != 0:
        raise RuntimeError("Cannot archive pinned source")
    (snapshot / "node_modules").symlink_to(REPO / "node_modules", target_is_directory=True)
    (OUT / "source-probe-snapshot.json").write_text(json.dumps({"candidate": EXPECTED, "command": command, "files": extracted, "scope": "Tracked source files from candidate; UI liveness parser input extensions preserved; temporary probes write only here", "node_modules": "symlink to existing installed repository dependencies"}, indent=2) + "\n")
    return snapshot


OUT.mkdir(parents=True, exist_ok=True)
assert head() == EXPECTED
record("01-static", ["pnpm", "exec", "turbo", "run", "lint", "typecheck", "--filter=!@multica/mobile", "--concurrency=1", "--force"])
record("02-ui-exports", ["pnpm", "check:ui-exports"])
ts = record("03-typescript", ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "pnpm", "exec", "turbo", "run", "test", "--filter=!@multica/mobile", "--concurrency=1", "--force", "--env-mode=loose", "--", "--maxWorkers=2"], note="Minimal allowlisted process environment has no database/provider credentials. Loose Turbo forwarding retains only that environment plus the outer Agent CLI guard marker so any ambient command is recorded and fails the guard.")
if ts["exit_code"] != 0:
    log = (OUT / "03-typescript" / "stdout.log").read_text()
    for package in ["@multica/core", "@multica/views", "@multica/web", "@multica/desktop", "@multica/docs"]:
        if not re.search(re.escape(package) + r":test:.*Test Files\s+", log):
            slug = package.split("/")[1]
            record("03-supplement-" + slug, ["bash", "scripts/go-test-with-agent-cli-guard.sh", "--", "pnpm", "--filter", package, "exec", "vitest", "run", "--maxWorkers=2"], note="Supplement for a package with no completed Vitest file summary after Turbo stopped. Original failure remains retained.")
record("04-test-go-script", ["bash", "scripts/test-go.test.sh"])
record("05-dev-env-script", ["bash", "scripts/dev-env.test.sh"], note="Inspected fixture uses temporary registry, fake psql and fake service launchers; only test-owned sleep processes are started.")
record("06-check-script", ["bash", "scripts/check.test.sh"], note="Inspected fixture replaces psql/Go/services; no real database or service startup.")
snapshot = source_snapshot()
record("07-ui-exports-script", ["bash", "scripts/check-ui-wildcard-exports.test.sh"], cwd=snapshot, note="Source-mutating probes run on pinned tracked-source snapshot entirely within this lane's evidence directory, never in the shared checkout.")
record("08-native-checker-script", ["node", "--test", "scripts/check-collaboration-native-tests.test.mjs"])
record("09-seed-script", ["pnpm", "test:seed"], note="SEED_TEST_DATABASE_URL is absent in the allowlisted environment; database integration remains explicitly skipped.")
status = subprocess.check_output(["git", "status", "--short"], cwd=REPO, text=True)
(OUT / "product-status-after.txt").write_text(status)
print("COMPLETE", len(RUNS), "commands; nonzero", sum(r["exit_code"] != 0 for r in RUNS), flush=True)
