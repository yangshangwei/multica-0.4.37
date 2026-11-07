from collections import Counter
from datetime import datetime, timezone
import difflib
import hashlib
import json
import os
from pathlib import Path
import subprocess
import zipfile

ROOT = Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103")
OUT = Path(__file__).resolve().parent
DATA = OUT.parent / "triage-focus-dynamic"
HEAD = "d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f"
REASON = "Same upload failure; retain the customer's additional context"


def read(path):
    return json.loads(Path(path).read_text())


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


report = read(DATA / "diagnostic-report.json")
registration = read(DATA / "control-source-registration.json")
natural_registration = read(DATA / "natural-source-registration.json")
preparation = read(DATA / "preparation.json")
private = Path(preparation["private"])
runtime = Path(preparation["diagnostic_checkout"])
original_path = ROOT / "e2e/fixtures/triage-duplicate-picker.ts"
original = original_path.read_text()
assert sha(original_path) == natural_registration["original_fixture_sha256"]
assert subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip() == HEAD
assert original == subprocess.check_output(
    ["git", "show", HEAD + ":e2e/fixtures/triage-duplicate-picker.ts"], cwd=ROOT, text=True
)
original_assertions = [line.strip() for line in original.splitlines() if "expect(" in line]
assert original_assertions == natural_registration["original_assertion_lines"]
assert len(original_assertions) == 52
original_fill = '  await dialog.getByRole("textbox", { name: copy.reason, exact: true }).fill(reason);\n'
new_focus = "  await expect(input).toBeFocused();\n"
new_reason = '  await expect(dialog.getByRole("textbox", { name: copy.reason, exact: true })).toHaveValue(reason);\n'
proposed = original.replace(original_fill, new_focus + original_fill + new_reason, 1)
assert proposed != original and proposed.replace(new_focus + original_fill + new_reason, original_fill, 1) == original
assert len([line for line in proposed.splitlines() if "expect(" in line]) == 54
(OUT / "proposed-minimum.diff").write_text("".join(difflib.unified_diff(
    original.splitlines(keepends=True), proposed.splitlines(keepends=True),
    fromfile="a/e2e/fixtures/triage-duplicate-picker.ts",
    tofile="b/e2e/fixtures/triage-duplicate-picker.ts",
)))

source_verification = []
for control in registration["controls"]:
    fixture = Path(control["fixture"])
    config = Path(control["config"])
    spec = Path(control["spec"])
    assert sha(fixture) == control["fixture_sha256"] and sha(config) == control["config_sha256"]
    code = fixture.read_text()
    assert all(count <= Counter(line.strip() for line in code.splitlines())[line]
               for line, count in Counter(original_assertions).items())
    reconstructed = code
    for line in [
        'import { installInitialFocusControl, retainControlCheckpoint, saveControlCheckpoint } from "./focus-control";\n',
        'import { installFocusObserver, saveFocusObserver } from "./focus-observer";\n',
        "  await installFocusObserver(page, locale);\n",
        '  await installInitialFocusControl(page, "' + control["mode"] + '");\n',
        "  await retainControlCheckpoint(page);\n",
    ]:
        assert line in reconstructed
        reconstructed = reconstructed.replace(line, "", 1)
    reconstructed = reconstructed.replace(
        'import type { TestApiClient } from "' + str(runtime / "e2e/fixtures") + '";',
        'import type { TestApiClient } from "../fixtures";',
    ).replace(
        'import { p1EnableTriage } from "' + str(runtime / "e2e/fixtures/project-p1") + '";',
        'import { p1EnableTriage } from "./project-p1";',
    ).replace("  try {\n  const { api, workspace }", "  const { api, workspace }", 1)
    suffix = """  } finally {
    await saveControlCheckpoint(page, info);
    await saveFocusObserver(page, info, locale);
  }
}
"""
    assert reconstructed.endswith(suffix)
    reconstructed = reconstructed[:-len(suffix)] + "}\n"
    expected = proposed if control["mode"] == "await-initial" else original
    assert reconstructed == expected
    assert "retries: 0" in config.read_text() and "timeout: 60000" in config.read_text()
    assert "test.setTimeout(120_000)" in spec.read_text() and "force:" not in code
    for helper in ["focus-control.ts", "focus-observer.ts"]:
        assert (fixture.parent / helper).read_bytes() == (DATA / helper).read_bytes()
    source_verification.append({
        "mode": control["mode"], "fixture": str(fixture), "fixture_sha256": sha(fixture),
        "config_sha256": sha(config), "spec_sha256": sha(spec),
        "original_assertions_preserved": 52, "private_fixture_reconstructs_exact_original_or_two_assertion_delta": True,
    })
assert sha(registration["server_input_module"]) == registration["server_input_sha256"]
assert sha(registration["hook_path"]) == registration["hook_sha256"]
assert sha(runtime / "node_modules/playwright-core/lib/server/input.js") == sha(ROOT / "node_modules/playwright-core/lib/server/input.js")
assert sha(DATA / "playwright-insertText-control.cjs") == sha(registration["hook_path"])
chunk_name = "52665-8b5d10f3a8d46d14.js"
original_chunk = ROOT / "apps/web/.next/static/chunks" / chunk_name
runtime_chunk = runtime / "apps/web/.next/static/chunks" / chunk_name
assert sha(original_chunk) == sha(runtime_chunk)
compiled_line = runtime_chunk.read_text().splitlines()[11]
assert "let u=()=>e?.focus({preventScroll:n});l?u():r=requestAnimationFrame(u)" in compiled_line
assert "(0,S.p)(e,{preventScroll:e===ew})" in compiled_line
chunk_evidence = {
    "original_path": str(original_chunk), "runtime_path": str(runtime_chunk), "sha256": sha(runtime_chunk),
    "callback_source": "()=>e?.focus({preventScroll:n})",
    "actual_callback_stack_columns": {"enqueueFocus": [19649, 19682], "FloatingFocusManager": 30794},
    "stack_maps_to_actual_unchanged_production_chunk": True,
}
hook = Path(registration["hook_path"]).read_text()
assert "const original = Keyboard.prototype.insertText" in hook
assert "return original.call(this, progress, text)" in hook
assert "await this._raw.sendText(progress, text)" in Path(registration["server_input_module"]).read_text()
assert ".value =" not in (DATA / "focus-control.ts").read_text()

artifacts = read(DATA / "diagnostic-artifact-manifest.json")
for artifact in artifacts:
    path = Path(artifact["path"])
    assert sha(path) == artifact["sha256"] and path.stat().st_size == artifact["bytes"]
assert sha(DATA / "diagnostic-artifact-manifest.json") == report["artifact_manifest_sha256"]
cases = {}
traces = []


def leaves(node):
    result = []
    for spec in node.get("specs", []):
        for test in spec.get("tests", []):
            result.append({
                "title": spec["title"], "file": spec.get("file"), "status": test["status"],
                "expected": test.get("expectedStatus"),
                "actual": [{"status": run["status"], "retry": run["retry"]} for run in test.get("results", [])],
            })
    for child in node.get("suites", []):
        result.extend(leaves(child))
    return result


for mode, expected_exit, count, actual_status in [
    ("natural", 0, 2, "passed"), ("redirect", 1, 1, "failed"), ("await-initial", 0, 1, "passed")
]:
    result_path = DATA / (mode + "-result.json")
    result = read(result_path)
    raw_report = read(result["report_path"])
    terminals = leaves(raw_report)
    assert result["exit_code"] == expected_exit and result["retries"] == 0
    assert sha(result["log_path"]) == result["log_sha256"]
    assert sha(result["report_path"]) == result["report_sha256"]
    assert len(terminals) == count and all(t["actual"] == [{"status": actual_status, "retry": 0}] for t in terminals)
    assert raw_report["stats"]["skipped"] == raw_report["stats"]["flaky"] == 0
    cases[mode] = {
        "result": str(result_path), "result_sha256": sha(result_path), "exit_code": expected_exit,
        "report_sha256": sha(result["report_path"]), "log_sha256": sha(result["log_path"]),
        "terminals": terminals, "retry_count": 0,
    }
    for path in sorted((private / (mode + "-artifacts")).rglob("trace.zip")):
        with zipfile.ZipFile(path) as archive:
            stream = [json.loads(line) for line in archive.read("0-trace.trace").decode().splitlines()]
        before = [event for event in stream if event["type"] == "before"]
        after = {event["callId"]: event for event in stream if event["type"] == "after"}
        target_label = "重复目标任务" if "zh-Hans" in path.parent.name else "Duplicate target task"
        reason_label = "原因" if target_label == "重复目标任务" else "Reason"
        reason = "同一上传故障，保留客户的补充说明" if reason_label == "原因" else REASON
        fill = next(e for e in before if e.get("method") == "fill" and e.get("params", {}).get("value") == reason)
        assert "textbox" in fill["params"]["selector"] and reason_label in fill["params"]["selector"]
        assert not after[fill["callId"]].get("error")
        empty = next(e for e in before if e.get("method") == "expect"
                     and e.get("params", {}).get("expression") == "to.have.value"
                     and target_label in e["params"].get("selector", "")
                     and e["params"].get("expectedText", [{}])[0].get("string") == "")
        row = {
            "mode": mode, "path": str(path), "sha256": sha(path), "bytes": path.stat().st_size,
            "fill_call": {"callId": fill["callId"], "start": fill["startTime"], "end": after[fill["callId"]]["endTime"],
                          "selector": fill["params"]["selector"], "value": reason},
            "original_empty_assertion_call": {"callId": empty["callId"], "start": empty["startTime"],
                                             "end": after[empty["callId"]]["endTime"]},
        }
        if mode == "await-initial":
            focus = next(e for e in before if e.get("method") == "expect"
                         and e["params"].get("expression") == "to.be.focused"
                         and target_label in e["params"].get("selector", "") and e["startTime"] < fill["startTime"])
            immediate_reason = next(e for e in before if e.get("method") == "expect"
                                    and e["params"].get("expression") == "to.have.value"
                                    and reason_label in e["params"].get("selector", "")
                                    and e["startTime"] > fill["startTime"] and e["startTime"] < empty["startTime"])
            assert after[focus["callId"]]["endTime"] < fill["startTime"] and not after[focus["callId"]].get("error")
            assert not after[immediate_reason["callId"]].get("error")
            row["new_initial_focus_assertion"] = {"start": focus["startTime"], "end": after[focus["callId"]]["endTime"]}
            row["new_immediate_reason_assertion"] = {"start": immediate_reason["startTime"], "end": after[immediate_reason["callId"]]["endTime"]}
        traces.append(row)
assert len(traces) == 4
observations = {}
for mode in ["redirect", "await-initial"]:
    descriptor = report["observations"][mode]
    path = Path(descriptor["path"])
    assert sha(path) == descriptor["sha256"]
    observation = read(path)["early"]
    control, focus = observation["control"], observation["focus"]
    assert control["captured"] == control["released"] == 1 and control["pending"] == 0
    assert control["callbacks"][0]["source"] == chunk_evidence["callback_source"]
    assert chunk_name in control["callbacks"][0]["stack"] and focus["dropped"] == 0
    events = focus["events"]
    beforeinput = next(e for e in events if e["type"] == "beforeinput" and e.get("data") == REASON)
    input_event = next(e for e in events if e["type"] == "input" and e.get("data") == REASON)
    expected_tag = "INPUT" if mode == "redirect" else "TEXTAREA"
    assert beforeinput["targetElement"]["tag"] == input_event["targetElement"]["tag"] == expected_tag
    assert beforeinput["isTrusted"] is input_event["isTrusted"] is True
    if mode == "redirect":
        before_release, after_release = control["snapshots"][-2:]
        assert before_release["active"]["tag"] == "TEXTAREA" and after_release["active"]["tag"] == "INPUT"
        assert before_release["target"] == before_release["reason"] == after_release["target"] == after_release["reason"] == ""
        assert after_release["time"] < beforeinput["time"] < input_event["time"]
        assert input_event["target"] == REASON and input_event["reason"] == ""
    else:
        executed = next(s for s in control["snapshots"] if s["phase"] == "actual-focus-native-frame-after")
        before_insert = control["snapshots"][-1]
        reason_focus = next(e for e in events if e["type"] == "focusin" and e["targetElement"]["tag"] == "TEXTAREA")
        assert executed["active"]["tag"] == "INPUT" and executed["time"] < reason_focus["time"] < before_insert["time"]
        assert before_insert["active"]["tag"] == "TEXTAREA" and input_event["reason"] == REASON and input_event["target"] == ""
    observations[mode] = {"path": str(path), "sha256": sha(path), "early_control": control, "early_events": events}
natural_en = report["observations"]["focus-observation-en"]
natural_zh = report["observations"]["focus-observation-zh-Hans"]
assert sha(natural_en["path"]) == natural_en["sha256"] and sha(natural_zh["path"]) == natural_zh["sha256"]
en = read(natural_en["path"])
assert en["dropped"] == 0 and read(natural_zh["path"]) == {"unavailable": True}
initial_focus = next(e for e in en["events"] if e["type"] == "focusin" and e["targetElement"]["tag"] == "INPUT")
reason_focus = next(e for e in en["events"] if e["type"] == "focusin" and e["targetElement"]["tag"] == "TEXTAREA")
reason_input = next(e for e in en["events"] if e["type"] == "input" and e.get("data") == REASON)
assert initial_focus["time"] < reason_focus["time"] < reason_input["time"] and reason_input["target"] == "" and reason_input["reason"] == REASON
result = {
    "schema_version": 1, "reviewed_at": datetime.now(timezone.utc).isoformat(),
    "reviewer": "/root/fixture_final_review", "reviewer_thread": os.environ.get("CODEX_THREAD_ID"),
    "active_task": str(OUT.parents[3]), "candidate_sha": HEAD,
    "verdict": "pass-for-two-assertion-fixture-protocol", "blocking_findings_open": 0,
    "mechanism": {"confirmed": True, "confidence": "high", "scope": "Reachable real Base UI/Playwright scheduling mechanism, demonstrated under explicit control.",
                  "historical_v15_attribution_confirmed": False, "historical_confidence": "medium",
                  "exact_production_chunk": chunk_evidence, "pinned_server_input_sha256": registration["server_input_sha256"],
                  "hook_sha256": registration["hook_sha256"], "trusted_event_chains_verified": True},
    "source_verification": source_verification, "original_fixture_sha256": sha(original_path),
    "proposed_fixture_sha256": hashlib.sha256(proposed.encode()).hexdigest(),
    "allowed_tracked_boundary": ["e2e/fixtures/triage-duplicate-picker.ts"],
    "proposed_diff": {"path": str(OUT / "proposed-minimum.diff"), "sha256": sha(OUT / "proposed-minimum.diff"),
                      "original_assertions_preserved": 52, "additional_assertions": 2, "other_source_changes": 0},
    "fixture_protocol_assessment": "Awaiting the actual initial target focus establishes dialog readiness before an unrelated field fill. Immediate Reason value verification catches incorrect input destination. Neither clears values nor changes product semantics, locators, business values, original assertions, retries or timeouts.",
    "cases": cases, "traces": traces, "controlled_observations": observations,
    "diagnostic_report": {"path": str(DATA / "diagnostic-report.json"), "sha256": sha(DATA / "diagnostic-report.json")},
    "artifact_manifest": {"path": str(DATA / "diagnostic-artifact-manifest.json"), "sha256": sha(DATA / "diagnostic-artifact-manifest.json"), "files_verified": len(artifacts)},
    "limits": [
        "Natural EN/ZH passed once each with passive telemetry, not an uninstrumented original timing reproduction; observer installation/listeners/RAF collection and trace can perturb scheduling.",
        "Natural ZH has a complete trace and original assertions passing, but its final focus collector is unavailable; no natural ZH focus ownership claim.",
        "Red holds a real callback until the server insertion boundary; green releases at its native frame and awaits initial focus. These are deliberate scheduling variants, not the same forced schedule with one variable toggled. Green proves the intended readiness protocol; fresh uncontrolled EN/ZH is required for the tracked delta.",
        "The original V15 has no focus timeline. The confirmed mechanism and identical symptom do not upgrade historical attribution to proven.",
    ],
    "root_next": "Root may apply exactly the generated two-assertion fixture delta, run fresh original EN/ZH without scheduling hook/observer on the dedicated diagnostic window, preserve all old evidence, independently review exact applied bytes and freeze the new candidate. Complete V16 admission/shared acceptance remain separate.",
    "operations": {"write_scope": str(OUT), "source_changes": 0, "DB_browser_test_service_mutations": 0,
                   "commits": 0, "models": 0, "knowledge_budget_allocated": 0, "knowledge_calls": 0,
                   "production_knowledge_default": False, "execution_clearance_granted": False},
}
(OUT / "review.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
(OUT / "report.md").write_text("""# Independent triage focus mechanism and fixture review

Verdict: pass-for-two-assertion-fixture-protocol, zero blockers. Root may apply only the two additional assertions in the generated proposed-minimum.diff to the shared triage duplicate picker fixture, then run fresh original EN/ZH without control hooks before freezing a candidate.

The reachable mechanism is confirmed with high confidence. The negative trace resolves fill to the correct Reason textarea. Its real queued focus callback comes from the unchanged production Next chunk's Base UI enqueueFocus/FloatingFocusManager code. The server module remains byte-identical to pinned Playwright. Trusted events show textarea focus, callback focus to target, then native beforeinput/input with reason in target and Reason empty. The original empty-target assertion fails with exit1.

The positive control records the actual initial callback completing at its native frame before Reason focus; pending0 is verified at real server insertText. Trusted input lands in Reason with target empty. Its entire applicable original API/pagination/keyboard/history flow passes with exit0, no retry. Private fixtures reconstruct byte-for-byte to the original or only the proposed two-assertion delta after removing diagnostic imports/wrappers. All52 original assertion lines remain, plus2 in the positive condition.

The change establishes correct fixture readiness and immediately verifies input destination. It does not clear/repair values, rewrite product behavior, alter locators, relax counts/list/value/history assertions or change timeouts/retries.

Natural EN/ZH both passed once. EN retained correct focus/input events; ZH retained a complete trace but its final collector is unavailable. Observation/trace overhead may perturb timing. Red and green deliberately use different callback-release schedules; this is not an identical-schedule isolated TDD reproduction. Historical V15 attribution stays medium confidence because its actual focus timing was not saved.

All four traces and34 artifacts, result/log/source hashes and exact event chains are bound in review.json. Reviewer made no source, browser, DB, service, model or commit changes and grants no complete V16/shared acceptance or execution clearance. Knowledge0/0/defaultfalse remains unchanged.
""")
print(json.dumps({"verdict": result["verdict"], "blockers": 0, "report": str(OUT / "review.json"),
                  "sha256": sha(OUT / "review.json"), "mechanism_confidence": "high",
                  "historical_attribution": "medium", "tracked_source_changed": False}))
