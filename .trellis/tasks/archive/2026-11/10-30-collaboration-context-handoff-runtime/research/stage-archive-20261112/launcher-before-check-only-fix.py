#!/usr/bin/env python3
"""Task-local launcher for the RK1 historic-stop continuation.

The launcher validates the immutable historic exception and a separate current
binding before invoking the existing Go RK1 scenario collector. The checked-in
registration is deliberately closed; a check-only run therefore performs no
Provider, CLI, permit, database mutation, or window claim.
"""
from __future__ import annotations

import datetime as dt
import hashlib
import json
import os
import pathlib
import runpy
import subprocess
import sys
import tempfile
from typing import Any

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parents[4]
REGISTRATION = HERE / "real-execution-registration.json"
REGISTRATION_SHA = HERE / "real-execution-registration.sha256"
APPROVAL = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261111/approval-registration.json"
AUTHORIZATION = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261111/real-execution-authorization.json"
HISTORICAL_RESULT = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/results/RK1-H3a-t2.json"
HISTORICAL_INVOCATION = pathlib.Path("/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/rk1/RK1-H3a/invocation-20261105T125045Z.json")
HISTORICAL_REVIEW = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/h3a-entry/independent-review.json"
ORDER = [("RK1-H3a", 3, 1), ("RK1-H3b", 1, 3), ("RK1-H5", 1, 3), ("RK1-H6", 1, 6)]
HISTORICAL = {
    "result": "9a4b94050ad696c46512b0833d27398ac0c6ef48770f6085d1db1283f9d628ce",
    "invocation": "909ac2da3054ab19ce4a75290ce9c5b06778740835efa414f6f206d863ddd3ed",
    "review": "e5807dddbc40ff880b431fb5ba002457c407fb2d77b75ee76d3b3841d30b991b",
    "approval": "7d743b5e1defa5fe8875c79a84d95c9982e9b0bfb650258a936a9214ed74f53c",
    "authorization": "1f56add2daca80c4f17575550979ce783f7a1ea860ef63e78e584e80e258f580",
}


def sha(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def string_sha(value: str) -> str:
    return hashlib.sha256(value.encode()).hexdigest()


def write_json(path: pathlib.Path, value: Any, exclusive: bool = False) -> None:
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    flags = "x" if exclusive else "w"
    with path.open(flags) as f:
        json.dump(value, f, ensure_ascii=False, indent=2)
        f.write("\n")
        f.flush()
        os.fsync(f.fileno())


def resolve(value: str) -> pathlib.Path:
    p = pathlib.Path(value)
    return p if p.is_absolute() else (REPO / p).resolve()


def framework_sha() -> str:
    h = hashlib.sha256()
    for p in sorted((REPO / "server/cmd/server").glob("rk1_*_test.go")):
        data = p.read_bytes()
        h.update(p.name.encode() + b"\0" + str(len(data)).encode() + b"\0" + data)
    return h.hexdigest()


def fail(message: str) -> int:
    print("REFUSED: " + message, file=sys.stderr)
    return 2


def load_binding() -> tuple[dict[str, Any] | None, str | None]:
    if not REGISTRATION.is_file() or not REGISTRATION_SHA.is_file():
        return None, "continuation registration is missing"
    expected = REGISTRATION_SHA.read_text().strip()
    actual = sha(REGISTRATION)
    if expected != actual:
        return None, "continuation registration hash changed"
    try:
        binding = json.loads(REGISTRATION.read_text())
    except json.JSONDecodeError:
        return None, "continuation registration is invalid JSON"
    if binding.get("schema_version") != 1 or binding.get("kind") != "runtime_h3a_continuation_execution_registration":
        return None, "continuation registration kind/version is invalid"
    return binding, None


def validate(binding: dict[str, Any], scenario: str, trial: int) -> list[str]:
    problems: list[str] = []
    for name, path, expected in [
        ("approval", APPROVAL, HISTORICAL["approval"]),
        ("authorization", AUTHORIZATION, HISTORICAL["authorization"]),
        ("result", HISTORICAL_RESULT, HISTORICAL["result"]),
        ("invocation", HISTORICAL_INVOCATION, HISTORICAL["invocation"]),
        ("review", HISTORICAL_REVIEW, HISTORICAL["review"]),
    ]:
        if not path.is_file() or sha(path) != expected:
            problems.append(f"{name} historical evidence changed")
    historical = binding.get("historical", {})
    if historical.get("consumed") != 12 or historical.get("remaining") != 13:
        problems.append("historical 12 debit or 13 remaining slots is missing")
    current = binding.get("current", {})
    if binding.get("execution_gate", {}).get("runnable") is not True or not all(binding.get("execution_gate", {}).get(k) is True for k in ("allow_build", "allow_check", "allow_run")):
        problems.append("current execution gate is closed")
    for key in ("source_binding_ready", "framework_binding_ready", "cli_ready", "configuration_ready", "database_ready", "window_ready", "permit_ready", "gateway_ready"):
        if current.get(key) is not True:
            problems.append(f"current {key} is not proven")
    manifest = resolve(current.get("source_manifest", ""))
    if not manifest.is_file():
        problems.append("current source manifest is missing")
    else:
        if sha(manifest) != current.get("source_manifest_sha256"):
            problems.append("current source manifest hash changed")
        try:
            source = json.loads(manifest.read_text())
            if source.get("source_id") != current.get("source_id") or source.get("head") != current.get("head") or source.get("dirty_product_paths"):
                problems.append("current source is dirty or drifted")
        except json.JSONDecodeError:
            problems.append("current source manifest is invalid")
    launcher = resolve(current.get("launcher", ""))
    if not launcher.is_file() or sha(launcher) != current.get("launcher_sha256"):
        problems.append("current task-local launcher bytes changed")
    executable = pathlib.Path(current.get("provider", {}).get("executable", ""))
    if not executable.is_absolute() or not executable.is_file() or sha(executable) != current.get("provider", {}).get("executable_sha256"):
        problems.append("current Provider executable binding changed")
    if framework_sha() != current.get("framework_sha256"):
        problems.append("current RK1 framework bytes changed")
    try:
        operator_facts = runpy.run_path(str(REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py"))["operator_facts"]()
        operator_sha = hashlib.sha256(json.dumps(operator_facts, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    except Exception:
        operator_sha = ""
    if operator_sha != current.get("operator_config_sha256"):
        problems.append("current operator configuration changed")
    output = resolve(binding.get("output", ""))
    ledger = resolve(binding.get("ledger", ""))
    historical_root = HISTORICAL_RESULT.parent.resolve()
    if not output.is_absolute() or output.resolve() == historical_root or str(output.resolve()).startswith(str(historical_root) + os.sep):
        problems.append("continuation output aliases historical results")
    if ledger.resolve() == output.resolve() or str(ledger.resolve()).startswith(str(historical_root) + os.sep):
        problems.append("continuation ledger aliases historical evidence/output")
    database_name = current.get("database", {}).get("name", "")
    database_url = os.environ.get("DATABASE_URL", "")
    if database_url:
        database_path = database_url.split("/", 3)[-1].split("?", 1)[0]
        if database_path != database_name:
            problems.append("DATABASE_URL is not the registered task database")
    if not any((scenario == s and trial == t) for s, t, _ in ORDER):
        problems.append("scenario/trial is outside the authorized continuation order")
    return problems


def main(argv: list[str]) -> int:
    if len(argv) not in (2, 3):
        return fail("usage: run_continuation.py <scenario> <trial> [--check-only]")
    scenario = argv[0]
    try:
        trial = int(argv[1])
    except ValueError:
        return fail("trial must be an integer")
    check_only = len(argv) == 3 and argv[2] == "--check-only"
    if len(argv) == 3 and not check_only:
        return fail("unknown option")
    binding, error = load_binding()
    if error:
        return fail(error)
    assert binding is not None
    problems = validate(binding, scenario, trial)
    record = {
        "schema_version": 1,
        "kind": "runtime_h3a_continuation_preflight",
        "recorded_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
        "scenario": scenario,
        "trial": trial,
        "check_only": check_only,
        "provider_cli_invoked": False,
        "model_calls": 0,
        "permit_used": False,
        "execution_window_claimed": False,
        "historical": HISTORICAL,
        "budget": {"cap": 25, "prior_actual_starts": 12, "remaining": 13},
        "gate": binding.get("execution_gate", {}),
        "problems": problems,
    }
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    write_json(HERE / "preflight" / f"{stamp}-{scenario}-t{trial}.json", record, exclusive=True)
    if problems:
        return fail("; ".join(problems))
    if os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE") != "1":
        return fail("explicit real Provider gate is required")
    output = resolve(binding["output"])
    output.mkdir(mode=0o700, parents=True, exist_ok=True)
    invocation = output / f"invocation-{stamp}.json"
    command = [
        "go", "-C", "server", "test", "-tags=agentintegration", "./cmd/server", "-run", "^TestRK1RuntimeContinuationReal$", "-count=1", "-v", "-timeout=10m",
    ]
    launch_record = {
        "schema_version": 1,
        "kind": "runtime_h3a_continuation_invocation",
        "started_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
        "command": command,
        "scenario": scenario,
        "trial": trial,
        "registration": str(REGISTRATION),
        "registration_sha256": sha(REGISTRATION),
        "budget_before": {"cap": 25, "consumed": 12, "remaining": 13},
    }
    write_json(invocation, launch_record, exclusive=True)
    env = dict(os.environ)
    env.update({
        "RK1_RUNTIME_CONTINUATION_REGISTRATION": str(REGISTRATION),
        "RK1_RUNTIME_CONTINUATION_REGISTRATION_SHA256": sha(REGISTRATION),
        "RK1_RUNTIME_CONTINUATION_SCENARIO": scenario,
        "RK1_RUNTIME_CONTINUATION_TRIAL": str(trial),
        "RK1_RUNTIME_CONTINUATION_OPERATOR_CONFIG_SHA256": binding["current"]["operator_config_sha256"],
    })
    proc = subprocess.run(command, cwd=REPO, env=env)
    launch_record.update({"finished_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "exit_code": proc.returncode})
    write_json(invocation, launch_record)
    return proc.returncode


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
