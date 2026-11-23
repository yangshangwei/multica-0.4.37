#!/usr/bin/env python3
"""Read-only independent review of the RK1 continuation candidate."""
from __future__ import annotations
import datetime as dt, hashlib, json, os, pathlib, subprocess

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parents[4]
REG = HERE / "real-execution-registration.json"
REG_SHA = HERE / "real-execution-registration.sha256"
MANIFEST = HERE / "current-source-manifest.json"
APPROVAL = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261111/approval-registration.json"
AUTH = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261111/real-execution-authorization.json"
RESULT = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/results/RK1-H3a-t2.json"
INVOCATION = pathlib.Path("/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/rk1/RK1-H3a/invocation-20261105T125045Z.json")
REVIEW = REPO / ".trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/h3a-entry/independent-review.json"
EXPECTED = {
    "approval": "7d743b5e1defa5fe8875c79a84d95c9982e9b0bfb650258a936a9214ed74f53c",
    "authorization": "1f56add2daca80c4f17575550979ce783f7a1ea860ef63e78e584e80e258f580",
    "result": "9a4b94050ad696c46512b0833d27398ac0c6ef48770f6085d1db1283f9d628ce",
    "invocation": "909ac2da3054ab19ce4a75290ce9c5b06778740835efa414f6f206d863ddd3ed",
    "review": "e5807dddbc40ff880b431fb5ba002457c407fb2d77b75ee76d3b3841d30b991b",
}

def sha(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()

def framework_sha() -> str:
    h = hashlib.sha256()
    for path in sorted((REPO / "server/cmd/server").glob("rk1_*_test.go")):
        data = path.read_bytes()
        h.update(path.name.encode() + b"\0" + str(len(data)).encode() + b"\0" + data)
    return h.hexdigest()

def check(name: str, condition: bool, detail: object) -> dict[str, object]:
    return {"name": name, "passed": bool(condition), "detail": detail}

def main() -> int:
    reg = json.loads(REG.read_text())
    manifest = json.loads(MANIFEST.read_text())
    checks = []
    for name, path, expected in [("approval", APPROVAL, EXPECTED["approval"]), ("authorization", AUTH, EXPECTED["authorization"]), ("result", RESULT, EXPECTED["result"]), ("invocation", INVOCATION, EXPECTED["invocation"]), ("review", REVIEW, EXPECTED["review"])]:
        actual = sha(path) if path.is_file() else None
        checks.append(check(f"historical_{name}_hash", actual == expected, {"expected": expected, "actual": actual}))
    current = reg["current"]
    checks.extend([
        check("registration_sidecar", REG_SHA.read_text().strip() == sha(REG), {"registration": sha(REG), "sidecar": REG_SHA.read_text().strip()}),
        check("source_manifest_hash", sha(MANIFEST) == current["source_manifest_sha256"], {"actual": sha(MANIFEST), "registered": current["source_manifest_sha256"]}),
        check("source_head_and_id", manifest.get("head") == current["head"] and manifest.get("source_id") == current["source_id"], {"manifest_head": manifest.get("head"), "registered_head": current["head"], "manifest_source_id": manifest.get("source_id"), "registered_source_id": current["source_id"]}),
        check("dirty_source_is_blocked", bool(manifest.get("dirty_product_paths")) and current["source_binding_ready"] is False, {"dirty_product_path_count": len(manifest.get("dirty_product_paths", [])), "ready": current["source_binding_ready"]}),
        check("framework_hash", framework_sha() == current["framework_sha256"], {"actual": framework_sha(), "registered": current["framework_sha256"]}),
        check("task_launcher_hash", sha(pathlib.Path(current["launcher"])) == current["launcher_sha256"], {"path": current["launcher"], "sha256": sha(pathlib.Path(current["launcher"]))}),
        check("provider_hash", sha(pathlib.Path(current["provider"]["executable"])) == current["provider"]["executable_sha256"], {"version": current["provider"]["version"], "sha256": current["provider"]["executable_sha256"]}),
        check("gate_all_closed", all(reg["execution_gate"].get(key) is False for key in ("runnable", "allow_build", "allow_check", "allow_run")), reg["execution_gate"]),
        check("historical_debit_preserved", reg["historical"]["consumed"] == 12 and reg["historical"]["remaining"] == 13, reg["historical"]),
        check("output_separate", pathlib.Path(reg["output"]).resolve() != RESULT.parent.resolve() and not str(pathlib.Path(reg["output"]).resolve()).startswith(str(RESULT.parent.resolve()) + os.sep), {"output": reg["output"], "historical_results": str(RESULT.parent)}),
        check("no_real_calls_in_verification", json.loads((HERE / "verification/implementation-verification.json").read_text())["real_execution"]["provider_calls"] == 0, "provider_calls=0"),
    ])
    ps = subprocess.run(["ps", "-axo", "pid=,command="], capture_output=True, text=True, check=False).stdout.splitlines()
    target = [line.strip() for line in ps if any(token in line for token in ("rk1-provider", "run_continuation.py", "/.local/bin/claude")) and "independent-review.py" not in line]
    checks.append(check("no_target_provider_process", not target, {"matches": target}))
    result = {"schema_version": 1, "kind": "runtime_h3a_continuation_independent_review", "reviewed_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "scope": "read-only current binding, historical evidence and default-closed continuation entry", "checks": checks, "passed": all(item["passed"] for item in checks), "real_execution": {"provider_calls": 0, "model_calls": 0, "permit_used": False, "execution_window_claimed": False, "database_mutated": False}, "disposition": "BLOCKED_PREPARATION_ONLY"}
    out = HERE / "independent-review.json"
    out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"passed": result["passed"], "checks": len(checks), "output": str(out), "sha256": sha(out)}, ensure_ascii=False))
    return 0 if result["passed"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
