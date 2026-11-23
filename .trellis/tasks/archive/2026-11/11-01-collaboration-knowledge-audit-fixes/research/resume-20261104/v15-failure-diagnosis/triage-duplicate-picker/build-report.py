from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import subprocess

ROOT = Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103")
OUT = Path(__file__).resolve().parent
ART = ROOT / "test-results/triage-duplicate-picker-We-a2833-firms-the-chosen-target-en--chromium"
LOG = ROOT / ".omx/private/final-check-v15/make-check-v15.log"
HEAD = "d41573e3a4eb3bd6c36feed3f0d542a0e066ee0f"


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


raw = LOG.read_bytes()
text = raw.decode(errors="replace")
start = text.index("  1) [chromium] › e2e/triage-duplicate-picker.spec.ts")
end = text.index("[304/441]", start)
(OUT / "failure-log-excerpt.txt").write_text(text[start:end])
context = (ART / "error-context.md").read_text()
(OUT / "failure-dialog-dom.txt").write_text(context[context.rfind("  - dialog"):])

paths = [
    "e2e/fixtures/triage-duplicate-picker.ts",
    "packages/views/triage/triage-action-dialog.tsx",
    "packages/views/triage/triage-duplicate-picker.tsx",
    "packages/ui/components/ui/dialog.tsx",
    "packages/ui/components/ui/textarea.tsx",
    "packages/views/triage/triage-fields.tsx",
    "node_modules/playwright-core/lib/server/dom.js",
    "node_modules/playwright-core/lib/generated/injectedScriptSource.js",
    "packages/ui/node_modules/@base-ui/react/dialog/popup/DialogPopup.js",
    "packages/ui/node_modules/@base-ui/react/floating-ui-react/components/FloatingFocusManager.js",
    "packages/ui/node_modules/@base-ui/react/floating-ui-react/utils/enqueueFocus.js",
]
sources = [
    {"path": str(ROOT / path), "realpath": str((ROOT / path).resolve()), "sha256": sha(ROOT / path)}
    for path in paths
]
tracked = [path for path in paths if "/node_modules/" not in path and not path.startswith("node_modules/")]
delta = subprocess.check_output(
    ["git", "diff", "d0b132fff0c477eb2d744933e13db730240231aa", HEAD, "--", *tracked],
    cwd=ROOT,
    text=True,
)
assert not delta
prior = ROOT / ".omx/private/final-check-v14/makecheck-playwright-results-v9.json"
prior_cases = []


def walk(node):
    for spec in node.get("specs", []):
        if "duplicate picker browses" in spec["title"]:
            for test in spec.get("tests", []):
                prior_cases.append({
                    "file": spec.get("file"),
                    "title": spec["title"],
                    "status": test["status"],
                    "actual": [result["status"] for result in test.get("results", [])],
                })
    for child in node.get("suites", []):
        walk(child)


walk(json.loads(prior.read_text()))
assert len(prior_cases) == 4 and all(case["actual"] == ["passed"] for case in prior_cases)
proposal = {
    "authorized_now": False,
    "required_window": "After sole V15 exits, final audit/inventory complete and root releases the resource window; root allocates new owned diagnostic output/services/database.",
    "source": "Preserve frozen V15 and its failed run; use a separately registered diagnostic overlay for event recording only.",
    "selection": ["Web en original failing flow", "Web zh-Hans control", "Native en/zh-Hans retain their original flows"],
    "execution": "workers1/retries0; trace on; retain original actions/assertions and actual named terminals/hashes.",
    "instrumentation": [
        "Before dialog opens, install scoped capture-phase focusin/focusout/beforeinput/input/change listeners. Capture monotonic time, event target tag/role/aria-label, activeElement identity and both synthetic field values; never record account credentials.",
        "Mark before opening, after click, immediately before reason.fill, immediately after fill, after target click and assertion failure. Preserve original action order and collect trace/DOM snapshots for the exact fill locator and input-event destination.",
        "Confirm/refute textarea focus/select then automatic target-combobox focus then beforeinput/input carrying reason on combobox while Reason remains empty.",
    ],
    "conditional_control": "Only after the original event sequence is confirmed: await target combobox toBeFocused before reason.fill and immediately verify the reason value, retaining every existing subsequent action and assertion.",
    "forbidden_shortcuts": [
        "No direct DOM value assignment, late clearing/re-filling, sleeps, forced clicks, changed business values/counts or disabled global dialog autofocus.",
        "Keep original failure and successful controlled replay as separate evidence; do not merge them into the original failed full run.",
    ],
    "assertions_to_preserve": [
        "Initial target empty; exactly30 first-page options; all formal candidates after pagination; excluded intake states absent.",
        "Done/cancelled labels; task identifiers and workspace-scoped link lookup; Tab/Escape/ArrowDown/Enter focus behavior.",
        "Failed-next-page busy state/retry without losing first-page options; no decision before explicit submit.",
        "Reason retained after selection; exact duplicate target decision and persisted history reason; zero page errors.",
    ],
}
record = {
    "schema_version": 1,
    "recorded_at": datetime.now(timezone.utc).isoformat(),
    "reviewer": "/root/fixture_final_review",
    "reviewer_thread": os.environ.get("CODEX_THREAD_ID"),
    "active_task": str(OUT.parents[3]),
    "scope": "Read-only diagnosis of original in-progress V15 English duplicate picker failure; no source edit or test rerun.",
    "candidate_sha": HEAD,
    "candidate_head_observed": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
    "candidate_status_observed": subprocess.check_output(["git", "status", "--porcelain=v1"], cwd=ROOT, text=True),
    "confirmed_observation": {
        "confidence": "high",
        "stage": "After reason.fill and target click, before first pagination or duplicate submission.",
        "failure_line": "e2e/fixtures/triage-duplicate-picker.ts:111",
        "expected_target_value": "",
        "actual_target_value": "Same upload failure; retain the customer's additional context",
        "reason_textarea_value": "empty in actual DOM and screenshot",
        "target_combobox_active": True,
        "target_results": "No matching tasks.",
    },
    "root_cause": {
        "confirmed": False,
        "confidence": "medium for actual failed event sequence; high that installed code supports this race",
        "leading_hypothesis": "The fixture fills Reason immediately after opening while Base UI initial first-control focus is queued; scheduled focus may intervene between Playwright textarea focus/select and keyboard.insertText, directing text to target.",
        "supporting_code": [
            "TriageActionDialog:138-141 and duplicate/Reason controls keep reason/setReason and target/setTarget independent; duplicate Reason autoFocus is false. Distinct accessible roles/labels match the live input and textarea.",
            "Fixture:100-111 opens dialog then fills Reason without awaiting intended initial target focus or immediately confirming Reason value.",
            "Installed BaseUI1.3.0 DialogPopup.js:72-81 defaults non-touch initialFocus=true; FloatingFocusManager.js:389-421 queues focus in a microtask and uses previously captured focus; enqueueFocus.js queues actual focus in requestAnimationFrame.",
            "Installed Playwright1.58.2 dom.js:499-524 calls injected.fill/selectText and then keyboard.insertText separately. Injected textarea selectText focuses the textarea and returns needsinput, leaving a scheduling gap.",
        ],
        "alternative_assessment": [
            {"hypothesis": "Mistaken locator/duplicate label", "assessment": "No support in actual source or DOM: distinct textbox/combobox roles and explicit aria-labels identify the intended separate fields."},
            {"hypothesis": "Shared product reason/target setter", "assessment": "Contradicted by source: separate state/value props and event setters, no reason-to-target assignment."},
            {"hypothesis": "Independent product state reset or coupling", "assessment": "Not positively demonstrated; an actual input/action timeline is needed to fully exclude it."},
        ],
        "causal_limit": "No trace.zip or focus/input timeline exists. The screenshot is post-fill/post-target-click and cannot prove focus ownership at insertText. Similar historical focus failures do not establish this one.",
    },
    "minimum_owning_boundary": {
        "diagnostic_file": "e2e/fixtures/triage-duplicate-picker.ts",
        "implementation_authorized_now": False,
        "conditional_fix_after_event_confirmation": "Await intended duplicate-target combobox initial focus before Reason.fill, and immediately assert Reason value. Preserve existing target click/empty assertion and the complete original flow.",
        "production_changes_currently_justified": False,
    },
    "evidence": {
        "original_artifacts": [
            {"path": str(path), "sha256": sha(path), "bytes": path.stat().st_size}
            for path in sorted(ART.rglob("*")) if path.is_file()
        ],
        "trace_files_found": [],
        "screenshot_inspected": True,
        "failure_dialog_dom": {"path": str(OUT / "failure-dialog-dom.txt"), "sha256": sha(OUT / "failure-dialog-dom.txt")},
        "failure_log_excerpt": {"path": str(OUT / "failure-log-excerpt.txt"), "sha256": sha(OUT / "failure-log-excerpt.txt")},
        "mutable_full_log_observation": {
            "path": str(LOG), "prefix_bytes_observed": len(raw), "observed_prefix_sha256": hashlib.sha256(raw).hexdigest(),
            "note": "The full log still appends. This identifies a captured prefix, not its future final hash.",
        },
        "sources": sources,
        "prior_v14": {"path": str(prior), "sha256": sha(prior), "all_four_passed": True, "cases": prior_cases, "relevant_tracked_paths_unchanged_between_v14_and_v15": True},
        "current_zh_following_pass": "Reported by root dispatch; diagnosis does not replace eventual whole JSON terminals.",
        "trace_configuration": "playwright.config.ts has no trace option; p1Failure only attaches errors/text/screenshot.",
    },
    "bounded_after_window_diagnostic": proposal,
    "operations": {
        "write_scope": str(OUT), "source_edits": 0, "product_browser_tests": 0, "DB_service_mutations": 0,
        "process_mutations": 0, "commits": 0, "Provider_calls": 0, "knowledge_budget_allocated": 0,
        "knowledge_calls": 0, "production_knowledge_default": False,
    },
}
(OUT / "report.json").write_text(json.dumps(record, ensure_ascii=False, indent=2) + "\n")
(OUT / "report.md").write_text("""# V15 triage duplicate picker diagnosis

The actual English failure is confirmed: Reason text appears in the duplicate-target combobox while the Reason textarea is empty. The initial empty-target assertion fails at fixture line111. Saved screenshot and DOM agree; the failure precedes pagination or duplicate submission.

The strongest hypothesis is a dialog initial-focus race, with medium causal confidence. Installed Base UI1.3.0 queues first-control focus through a microtask and requestAnimationFrame. Installed Playwright1.58.2 textarea.fill selects/focuses the textarea, then later sends keyboard.insertText. The fixture immediately fills another field after opening. Product reason and target states/setters are separate; labels/roles identify distinct controls.

This run contains no trace or focus/input timeline. Final combobox focus is observed after input.click. The actual focus-stealing event is not yet proven. No production fix or immediate test change is recommended during the shared run. Allfour relevant picker cases passed in v14, with relevant fixture/modal/picker/UI sources unchanged.

After the sole full window releases, retain original actions/assertions and enable trace plus scoped focusin/focusout/beforeinput/input/change and action-stage recording. Confirm whether textarea focus/select is followed by default combobox focus before insertText. If confirmed, the smallest fix belongs to the shared E2E fixture: await intended initial combobox focus before filling Reason and immediately verify Reason value. Keep every original empty-target, count/list, keyboard/focus, target identity, durable decision and history assertion.

No tests, DB/services/processes, source changes, commits or models were run by this diagnosis. Knowledge0/0/defaultfalse remain unchanged. report.json binds actual source/library/artifact hashes, the immutable failure excerpt and full diagnostic/control proposal.
""")
print(json.dumps({
    "report": str(OUT / "report.json"), "sha256": sha(OUT / "report.json"),
    "observed_misdirected_input_confirmed": True, "actual_root_cause_confirmed": False,
    "causal_confidence": "medium", "source_edits": 0, "test_runs": 0,
}))
