"""Reconcile retained campaign evidence without executing application code."""

import argparse
import base64
import binascii
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[4]
TASK = ROOT / ".trellis/tasks/11-24-main-post-merge-e2e-acceptance"
CONTEXT = json.loads((TASK / "run-context.json").read_text())
QA = Path(CONTEXT["qa"])
OUTPUT = QA / "coverage-audit"
BASELINE = "runtime/legacy-broad-02"
PASS = "PASS"
FAIL = "FAIL"
NOT_RUN = "NOT_RUN"
BLOCKED = "BLOCKED"


def read(path):
    return json.loads(path.read_text())


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def relative(path):
    return str(path.relative_to(QA))


def flatten(report):
    """Keep Playwright ids and the human-readable file/title/project tuple."""
    rows = []

    def visit(suite, ancestors):
        titles = [*ancestors, suite["title"]] if suite.get("title") else ancestors
        for spec in suite.get("specs", []):
            for test in spec.get("tests", []):
                project = test.get("projectId", test.get("projectName", ""))
                rows.append({
                    "identity": f"{spec['id']}::{project}",
                    "file": spec["file"],
                    "title": " / ".join([*titles, spec["title"]]),
                    "project": project,
                    "expected_status": test["expectedStatus"],
                    "status": test["status"],
                    "results": test.get("results", []),
                    "annotations": test.get("annotations", []),
                })
        for child in suite.get("suites", []):
            visit(child, titles)

    for suite in report.get("suites", []):
        visit(suite, [])
    return rows


def reconcile(collection, report):
    planned, observed = flatten(collection), flatten(report)
    issues = []
    for label, rows in (("collection", planned), ("report", observed)):
        duplicates = [key for key, n in Counter(row["identity"] for row in rows).items() if n != 1]
        if duplicates:
            issues.append({"kind": f"duplicate_{label}_identities", "identities": duplicates})
    expected = {row["identity"]: row for row in planned}
    actual = {row["identity"]: row for row in observed}
    if expected.keys() != actual.keys():
        issues.append({"kind": "identity_set_mismatch", "missing": sorted(expected.keys() - actual.keys()),
                       "unexpected": sorted(actual.keys() - expected.keys())})
    terminals = Counter()
    for row in observed:
        identity = row["identity"]
        original = expected.get(identity)
        if original and any(row[key] != original[key] for key in ("file", "title", "project")):
            issues.append({"kind": "identity_name_mismatch", "identity": identity})
        results = row["results"]
        if len(results) != 1 or results[0].get("retry") != 0:
            issues.append({"kind": "non_first_attempt_terminal", "identity": identity})
        status = results[-1].get("status") if results else "not_run"
        row["terminal_status"] = status
        terminals[status] += 1
        if status not in {"passed", "failed", "timedOut", "interrupted", "skipped"}:
            issues.append({"kind": "invalid_terminal", "identity": identity, "status": status})
        if status == "passed" and (results[-1].get("errors") or row["expected_status"] != "passed"):
            issues.append({"kind": "invalid_pass", "identity": identity})
    if collection.get("errors") or report.get("errors"):
        issues.append({"kind": "global_report_errors", "collection_count": len(collection.get("errors", [])),
                       "report_count": len(report.get("errors", []))})
    if report.get("config", {}).get("workers") != 1:
        issues.append({"kind": "unexpected_worker_count"})
    for project in report.get("config", {}).get("projects", []):
        if project.get("retries") != 0:
            issues.append({"kind": "unexpected_retry_count", "project": project.get("id")})
    return {"planned": len(planned), "observed": len(observed), "terminal_counts": dict(terminals),
            "issues": issues, "inventory_complete": not issues, "cases": observed}


RESULT_FAILURES = {
    "Browser command or complete named inventory did not pass",
    "HTTP-only probe did not pass with a recorded report",
    "Reviewed API probe failed",
}


def inspect_lane(directory):
    """Validate recorded historical evidence; do not infer live resource state."""
    completion_path = directory / "completion.json"
    result = {"lane": relative(directory), "complete": completion_path.is_file(),
              "integrity_issues": [], "acceptance_failures": [], "cases": []}
    if not completion_path.is_file():
        result["status"] = NOT_RUN
        result["reason"] = "No completion receipt; execution/preparation may be in progress"
        if all((directory / name).is_file() for name in ("collection.json", "playwright.json")):
            raw = reconcile(read(directory / "collection.json"), read(directory / "playwright.json"))
            result["unaccepted_raw_inventory"] = raw
            result["unaccepted_raw_sha256"] = {name: sha(directory / name) for name in ("collection.json", "playwright.json")}
            result["status"] = "INTERRUPTED" if (directory / "interruption-recovery-before.json").is_file() else "INCOMPLETE"
            result["reason"] = "Raw terminals retained without admission: original command exit and completion are unproved"
        return result
    completion = read(completion_path)
    result.update({"completion_sha256": sha(completion_path), "exit": completion.get("exit"),
                   "browser_exit": completion.get("browser_exit"), "probe_only": completion.get("probe_only", False),
                   "completed_at": completion.get("completed_at"), "evidence": {}})
    for name, checksum in completion.get("evidence_sha256", {}).items():
        path = directory / name
        valid = Path(name).name == name and path.is_file() and not path.is_symlink() and sha(path) == checksum
        result["evidence"][name] = {"sha256": checksum, "verified": valid}
        if not valid:
            result["integrity_issues"].append(f"Recorded evidence hash mismatch: {name}")
    required = {"run-intent.json", "admission.json", "run.json", "source-end.json", "cleanup.json"}
    required |= {"probe-result.json", "probe.receipt.json"} if result["probe_only"] else {
        "collection.json", "playwright.json", "named-inventory.json"}
    missing = sorted(required - completion.get("evidence_sha256", {}).keys())
    if missing:
        result["integrity_issues"].append("Missing required bound evidence: " + ", ".join(missing))
    if (directory / "run.json").is_file():
        run = read(directory / "run.json")
        result.update({"mode": run.get("mode"), "scope": run.get("scope"), "profile": run.get("profile"),
                       "command": run.get("command"), "candidate": run.get("candidate")})
        if run.get("candidate") != CONTEXT["candidate"]:
            result["integrity_issues"].append("Run candidate differs")
        source = run.get("source", {})
        if source.get("candidate") != CONTEXT["candidate"] or source.get("source_fingerprint") != CONTEXT["source_fingerprint"]:
            result["integrity_issues"].append("Run source fingerprint differs")
    if (directory / "source-end.json").is_file():
        source = read(directory / "source-end.json")
        if source.get("candidate") != CONTEXT["candidate"] or source.get("source_fingerprint") != CONTEXT["source_fingerprint"]:
            result["integrity_issues"].append("Final source fingerprint differs")
    if (directory / "cleanup.json").is_file():
        cleanup = read(directory / "cleanup.json")
        result["recorded_ports_closed"] = cleanup.get("ports_closed")
        if not cleanup.get("ports_closed") or any(value is not True for value in cleanup.get("ports_closed", {}).values()):
            result["integrity_issues"].append("Owned listener closure is not fully recorded")
        gates = [cleanup.get(name) for name in ("original_gate", "final_gate")]
        if all(isinstance(gate, dict) for gate in gates):
            if any(gates[0].get(key) != gates[1].get(key) for key in ("enabled", "creation_enabled", "host_execution_enabled")):
                result["integrity_issues"].append("Original gate booleans differ after cleanup")
            if gates[1].get("epoch", 0) < gates[0].get("epoch", 0):
                result["integrity_issues"].append("Gate epoch moved backwards")
        for issue in cleanup.get("failures", []):
            result["acceptance_failures" if issue in RESULT_FAILURES else "integrity_issues"].append(issue)
    for issue in completion.get("source_and_resource_failures", []):
        target = result["acceptance_failures" if issue in RESULT_FAILURES else "integrity_issues"]
        if issue not in target:
            target.append(issue)
    if not result["probe_only"] and all((directory / name).is_file() for name in ("collection.json", "playwright.json")):
        checked = reconcile(read(directory / "collection.json"), read(directory / "playwright.json"))
        result.update(checked)
        if checked["issues"]:
            result["integrity_issues"].append("Raw collection/report reconciliation failed")
        if (directory / "named-inventory.json").is_file():
            named = read(directory / "named-inventory.json")
            # The original recorder also sets inventory_complete=false for an
            # ordinary test assertion failure. Preserve that field, but compare
            # cardinality/results independently rather than inventing corruption.
            result["stored_inventory_complete"] = named.get("inventory_complete")
            result["stored_inventory_problems"] = named.get("problems", [])
            comparable = ("planned", "observed", "terminal_counts")
            if any(named.get(key) != checked[key] for key in comparable):
                result["integrity_issues"].append("Stored named summary differs from independent raw parse")
            by_id = {row["identity"]: row for row in named.get("cases", [])}
            for row in checked["cases"]:
                previous = by_id.get(row["identity"])
                if previous is None or any(row[key] != previous.get(key) for key in ("file", "title", "results", "status")):
                    result["integrity_issues"].append("Stored named case differs: " + row["identity"])
    result["status"] = PASS if result.get("exit") == 0 and not result["integrity_issues"] and not result["acceptance_failures"] else FAIL
    return result


def category(filename):
    """Return the planned supplemental lane and the actual prerequisite family."""
    if filename in {"collaboration-performance.spec.ts"}:
        return "L10", "100-account performance", "Fresh external manifest, exact 100-user topology and prepare → propagation → writeback"
    if filename in {"collaboration-host-installed.spec.ts", "collaboration-installed-main.spec.ts", "collaboration-installed-clients.spec.ts"}:
        return "L7", "packaged/installed clients", "Exact current package or pinned legacy OCI + relay; full Main requires independent native admission"
    if filename == "auth-device.spec.ts":
        return "L4", "device", "Dedicated device-enabled API with real device login"
    if filename.startswith(("admin-", "platform-admin", "collaboration-administration-")):
        return "L3", "administration", "Independent password API, synthetic administrator and per-spec initialization/enrollment/storage"
    if filename == "inbox-snooze.spec.ts":
        return "L2", "password inbox snooze", "Dedicated exact inbox allocation, password API and matching native renderer; a generic flag is insufficient"
    if filename.startswith(("issue-assist-flow", "quick-create-")):
        return "L1", "legacy supplemental", "Deterministic local assist/recommendation provider as required by this identity"
    if filename.startswith(("collaboration-host-", "collaboration-memory-runtime", "collaboration-business-scenarios", "collaboration-multi-account", "collaboration-timeout", "collaboration-order-export", "collaboration-runtime-output", "collaboration-recovery-runtime", "collaboration-knowledge-deployed", "project-knowledge-automation", "collaboration-stage-")):
        return "L6", "controlled host/stage/knowledge", "Fresh managed password allocation, exact native fixture binaries and case-specific signed stage/knowledge inputs"
    if filename.startswith(("collaboration-control-", "collaboration-cross-client", "collaboration-lane-close", "collaboration-lifecycle-controls", "collaboration-leave-", "collaboration-live-revoke", "collaboration-terminal-reopen", "collaboration-offline-result", "issue-completion-authorization")):
        return "L5", "namespace/control/fixture", "Explicit namespace-only or M1 allocation; Linux-named fixtures require matching built executables and independent admission"
    if filename == "collaboration-detail-journey.spec.ts":
        return "L1", "manual detail", "Current listener/build provenance and explicit detail permit, with execution closed"
    return "L2", "password/manual/recovery", "Compatible password API, current production Web/renderer, real actors and exact family permit"


def inspect_performance(directory):
    completion_path = directory / "completion.json"
    base = {"lane": relative(directory), "complete": completion_path.is_file(), "mode": "password",
            "scope": "100-account API/WebSocket/controlled Host sampler; no product browser UI", "cases": [],
            "integrity_issues": [], "acceptance_failures": []}
    if not completion_path.is_file():
        return [{**base, "status": NOT_RUN, "reason": "Performance preparation or sampling still has no complete closure"}]
    completion = read(completion_path)
    base.update({"completion_sha256": sha(completion_path), "completed_at": completion.get("completed_at"),
                 "exit": completion.get("exit")})
    for name, checksum in completion.get("evidence_sha256", {}).items():
        path = directory / name
        if (not path.resolve().is_relative_to(directory.resolve()) or not path.is_file()
                or path.is_symlink() or sha(path) != checksum):
            base["integrity_issues"].append("Performance evidence hash mismatch: " + name)
    required = {"inputs.json", "run-start.json", "allocation.json", "cleanup.json"}
    if not required.issubset(completion.get("evidence_sha256", {})):
        base["integrity_issues"].append("Performance source/allocation/cleanup evidence missing")
    else:
        inputs, cleanup = read(directory / "inputs.json"), read(directory / "cleanup.json")
        if inputs.get("candidate") != CONTEXT["candidate"]:
            base["integrity_issues"].append("Performance candidate differs")
        for key in ("children", "owned_processes", "gate", "resources", "bindings"):
            if cleanup.get("results", {}).get(key, {}).get("status") != "pass":
                base["integrity_issues"].append("Performance cleanup not proven: " + key)
        source = cleanup.get("results", {}).get("bindings", {}).get("source", {})
        if source.get("source_fingerprint") != CONTEXT["source_fingerprint"]:
            base["integrity_issues"].append("Performance final source differs")
    for sample in completion.get("public_sample_files", []):
        path = directory / "samples" / sample["name"]
        if Path(sample["name"]).name != sample["name"] or not path.is_file() or sha(path) != sample["sha256"]:
            base["integrity_issues"].append("Performance sample hash mismatch: " + sample["name"])
    phases = []
    for phase in ("prepare", "propagation", "writeback"):
        target = directory / phase
        row = {**base, "lane": relative(target), "integrity_issues": list(base["integrity_issues"]), "cases": []}
        receipts = [item for item in completion.get("phases", []) if item.get("phase") == phase]
        if not receipts or not all((target / name).is_file() for name in ("collection.json", "playwright.json")):
            row.update(status=NOT_RUN, reason="This canonical phase was not fully executed")
        else:
            row["browser_exit"] = receipts[0].get("exit")
            row.update(reconcile(read(target / "collection.json"), read(target / "playwright.json")))
            if row["planned"] != 1 or any(case["file"] != "collaboration-performance.spec.ts" or f" / {phase}:" not in case["title"] for case in row["cases"]):
                row["integrity_issues"].append("Performance phase identity differs")
            if row["issues"]:
                row["integrity_issues"].append("Performance raw identity ledger incomplete")
            row["status"] = PASS if receipts[0].get("accepted") and not row["integrity_issues"] else FAIL
            # The phase references the run-wide closure, not a nonexistent child completion.
            row["completion_file"] = relative(completion_path)
        phases.append(row)
    return phases


def inspect_knowledge(directory):
    completion_path = directory / "completion.json"
    row = {"lane": relative(directory), "complete": completion_path.is_file(), "mode": "password-managed-knowledge",
           "scope": "Canonical Web/Desktop renderer and production daemon with deterministic local knowledge/model fixtures",
           "cases": [], "integrity_issues": [], "acceptance_failures": []}
    if not completion_path.is_file():
        return {**row, "status": NOT_RUN, "reason": "Knowledge execution has no final completion receipt"}
    completion = read(completion_path)
    row.update({"completion_sha256": sha(completion_path), "completed_at": completion.get("completed_at"),
                "exit": completion.get("exit"), "browser_exit": completion.get("browser_exit")})
    for name, checksum in completion.get("evidence_sha256", {}).items():
        path = directory / name
        if not path.resolve().is_relative_to(directory.resolve()) or not path.is_file() or path.is_symlink() or sha(path) != checksum:
            row["integrity_issues"].append("Knowledge evidence hash mismatch: " + name)
    required = {"inputs.json", "allocation.json", "run.json", "source-end.json", "cleanup.json", "private-evidence-bindings.json",
                "collection.json", "playwright.json", "named-inventory.json"}
    if not required.issubset(completion.get("evidence_sha256", {})):
        row["integrity_issues"].append("Knowledge required source/fixture/cleanup evidence incomplete")
    if (directory / "source-end.json").is_file():
        source = read(directory / "source-end.json")
        if source.get("candidate") != CONTEXT["candidate"] or source.get("source_fingerprint") != CONTEXT["source_fingerprint"]:
            row["integrity_issues"].append("Knowledge final source differs")
    if (directory / "cleanup.json").is_file():
        cleanup = read(directory / "cleanup.json")
        for key in ("children", "owned_processes", "gate", "resources", "bindings"):
            if cleanup.get("results", {}).get(key, {}).get("status") != "pass":
                row["integrity_issues"].append("Knowledge cleanup not proven: " + key)
    if all((directory / name).is_file() for name in ("collection.json", "playwright.json")):
        row.update(reconcile(read(directory / "collection.json"), read(directory / "playwright.json")))
        if row["planned"] != 2 or any(case["file"] != "project-knowledge-automation.spec.ts" for case in row["cases"]):
            row["integrity_issues"].append("Knowledge canonical two-case identity differs")
        if row["issues"]:
            row["integrity_issues"].append("Knowledge raw identity ledger incomplete")
    row["status"] = PASS if completion.get("exit") == 0 and not row["integrity_issues"] else FAIL
    return row


def native_closure(lane):
    directory = QA / "native-acceptance" / Path(lane["lane"]).name
    if not (directory / "run-start.json").exists():
        return None
    issues = []
    if not (directory / "completion.json").is_file():
        return {"status": NOT_RUN, "reason": "Native protocol/process closure has no completion receipt"}
    completion = read(directory / "completion.json")
    for name, checksum in completion.get("evidence_sha256", {}).items():
        path = directory / name
        if Path(name).name != name or not path.is_file() or path.is_symlink() or sha(path) != checksum:
            issues.append("Native evidence hash mismatch: " + name)
    required = {"run-start.json", "cleanup.json", "protocol-original.json", "protocol-restoration.json"}
    if not required.issubset(completion.get("evidence_sha256", {})):
        issues.append("Required native closure evidence is missing")
    elif not issues:
        original, restored = read(directory / "protocol-original.json"), read(directory / "protocol-restoration.json")
        if restored.get("status") != "pass" or any(original.get(key) != restored.get("final", {}).get(key)
                                                    for key in ("scheme", "default_application_url", "handlers", "registrations")):
            issues.append("Original protocol URL/registration set was not restored exactly")
        cleanup = read(directory / "cleanup.json")
        if cleanup.get("failures") or any(cleanup.get("results", {}).get(key, {}).get("status") != "pass"
                                          for key in ("owned_processes", "protocol", "bindings")):
            issues.append("Native processes/protocol/source closure did not pass")
    return {"status": PASS if not issues else FAIL, "journey_exit": completion.get("exit"),
            "completion": relative(directory / "completion.json"), "completion_sha256": sha(directory / "completion.json"),
            "issues": issues, "scope": completion.get("scope")}


def adapter_closure(lane, case):
    label, filename = Path(lane["lane"]).name, case["file"]
    if filename == "collaboration-installed-main.spec.ts" and label.startswith("linux-main-"):
        return linux_main_closure(lane)
    directory = None
    required = ()
    if filename == "collaboration-installed-clients.spec.ts" and label.startswith("installed-cli-"):
        directory = QA / "installed-cli-acceptance" / label
        required = ("resource-cleanup.json", "process-cleanup.json")
    elif filename in {"collaboration-stage-execution.spec.ts", "issue-completion-authorization.spec.ts"} and label.startswith(("stage-", "m1-")):
        directory = QA / "stage-acceptance" / label
        required = ("cleanup.json",)
    elif filename == "admin-resource-publishing.spec.ts" and label.startswith("resource-publishing-"):
        directory = QA / "runtime" / (label + "-adapter")
        required = ("owned-process-cleanup.json",)
    elif filename in {"collaboration-cross-client-execution.spec.ts", "collaboration-lane-close-controls.spec.ts", "collaboration-lifecycle-controls.spec.ts", "collaboration-offline-result-notifications.spec.ts"}:
        directory = QA / "runtime" / (label + "-adapter")
        required = ("container-cleanup.json", "process-cleanup.json")
    if directory is None:
        return None
    issues = []
    if not (directory / "completion.json").is_file():
        return {"status": NOT_RUN, "reason": "Adapter final closure not recorded", "directory": relative(directory)}
    completion = read(directory / "completion.json")
    for name, checksum in completion.get("evidence_sha256", {}).items():
        path = directory / name
        if not path.resolve().is_relative_to(directory.resolve()) or not path.is_file() or sha(path) != checksum:
            issues.append("Adapter evidence hash mismatch: " + name)
    if completion.get("cleanup_failures") or completion.get("bindings_unchanged") is False:
        issues.append("Adapter source or cleanup failed")
    for name in required:
        path = directory / name
        if not path.is_file():
            issues.append("Adapter cleanup absent: " + name)
            continue
        cleanup = read(path)
        if "results" in cleanup:
            if cleanup.get("failures") or any(item.get("status") != "pass" for item in cleanup["results"].values()):
                issues.append("Adapter nested cleanup not complete: " + name)
        elif cleanup.get("status") != "pass" or cleanup.get("remaining") or cleanup.get("fallback_termination") or cleanup.get("fallback"):
            issues.append("Adapter cleanup not complete: " + name)
    if filename == "collaboration-installed-clients.spec.ts" and completion.get("status") != "pass":
        issues.append("Exact installed CLI resource-count acceptance did not pass")
    return {"status": PASS if not issues else FAIL, "issues": issues, "directory": relative(directory),
            "completion_sha256": sha(directory / "completion.json"), "recorded_suite_status": completion.get("status")}


def linux_main_closure(lane):
    label = Path(lane["lane"]).name
    directory, nested = QA / "linux-main-acceptance" / label, QA / lane["lane"]
    registry = Path(CONTEXT["secure"]) / "linux-main-acceptance" / label / "resources.private.json"
    if not (directory / "completion.json").is_file():
        return {"status": NOT_RUN, "directory": relative(directory), "reason": "Linux outer closure missing"}
    completion = read(directory / "completion.json")
    required = {directory / "cleanup.json", directory / "docker-commands.json", registry,
                nested / "completion.json", nested / "cleanup.json", nested / "named-inventory.json", nested / "source-end.json"}
    expected = completion.get("evidence_sha256", {})
    issues = []
    if set(expected) != {str(path) for path in required}:
        issues.append("Linux final evidence set differs")
    for path in required:
        if not path.is_file() or path.is_symlink() or sha(path) != expected.get(str(path)):
            issues.append("Linux final evidence hash mismatch: " + path.name)
    if not issues:
        cleanup = read(directory / "cleanup.json")
        if cleanup.get("failures") or any(cleanup.get("results", {}).get(key, {}).get("status") != "pass"
                                          for key in ("owned_processes", "containers", "bindings")):
            issues.append("Linux owned resource or final binding closure failed")
        if read(directory / "docker-commands.json") != read(registry):
            issues.append("Linux registry differs from captured Docker commands")
        if completion.get("runtime_report") != str(nested) or completion.get("runtime_exit") != lane.get("exit"):
            issues.append("Linux nested runtime binding differs")
    if completion.get("cleanup_failures"):
        issues.append("Linux cleanup failures retained")
    # A named business failure does not erase a verified cleanup, nor become PASS.
    unexpected = [item for item in completion.get("evidence_problems", [])
                  if item != "nested runtime did not complete successfully"]
    issues.extend(unexpected)
    return {"status": PASS if not issues else FAIL, "issues": issues, "directory": relative(directory),
            "completion_sha256": sha(directory / "completion.json"), "recorded_suite_status": completion.get("status"),
            "journey_exit": completion.get("exit"), "scope": "Resource/hash closure only; actual Main boundary requires raw boot/timeline evidence"}


def case_observation(lane, case):
    terminal = case["terminal_status"]
    valid = not lane["integrity_issues"] and lane.get("inventory_complete")
    if terminal in {"failed", "timedOut", "interrupted"}:
        status = FAIL
    elif terminal == "passed" and valid:
        status = PASS
    elif terminal == "skipped":
        status = "SKIP"
    else:
        status = BLOCKED
    closure = native_closure(lane) if case["file"] == "collaboration-host-installed.spec.ts" else None
    if status == PASS and closure is not None and (closure["status"] != PASS or closure.get("journey_exit") != 0):
        status = BLOCKED
    adapter = adapter_closure(lane, case)
    if status == PASS and adapter is not None and adapter["status"] != PASS:
        status = BLOCKED
    if status == PASS and adapter is not None and adapter.get("journey_exit", 0) != 0:
        status = BLOCKED
    diagnosis = {"category": "UNTRIAGED", "product_failure_proven": False} if status == FAIL else None
    known = {
        "runtime/inbox-snooze-01": {"category": "TEST_ALLOCATION_PREREQUISITE", "product_failure_proven": False,
                                   "reason": "Generic opt-in omitted required password inbox allocation; failed before actor/business setup"},
        "runtime-supplement/inbox-snooze-02": {"category": "TEST_LOCATOR_DRIFT", "product_failure_proven": False,
                                              "reason": "Fixture clicks workspace-name button, which now selects current workspace; actual dropdown is Switch workspace; snooze/wakeup assertions not reached",
                                              "evidence": "runtime-supplement/inbox-snooze-02/failure-classification.json"},
        "runtime/installed-main-02": {"category": "TEST_LOCATOR_DRIFT", "product_failure_proven": False,
                                     "reason": "Canonical controller waits for obsolete Collaboration heading; later full Main controls unverified"},
        "runtime/resource-publishing-01": {"category": "TEST_LOCATOR_DRIFT", "product_failure_proven": False,
                                           "reason": "Native consumer waits for old Skills navigation link; canonical end-to-end market journey not completed"},
        "runtime/installed-cli-02": {"category": "TEST_EXECUTABLE_PREREQUISITE", "product_failure_proven": False,
                                    "reason": "Docker/runc refuses /fixture/relay execution before first ordinary positive old-CLI write; copied-volume file mode/ownership cause unobserved",
                                    "evidence": "installed-cli-acceptance/installed-cli-02/failure-classification.json"},
        "runtime-supplement/manual-detail-01": {"category": "TEST_LOCATOR_DRIFT", "product_failure_proven": False,
                                              "reason": "assertJourneyScopeRestoration still expects Workspaces and Collaboration navigation buttons; current unified sidebar uses Workspaces group label; full restoration journey unverified"},
        "runtime-supplement/pay-creation-01": {"category": "TEST_LOCATOR_DRIFT", "product_failure_proven": False,
                                             "reason": "PAY creation openSession waits for old Collaboration heading while account participating view renders Participating; creation UI assertions unexecuted"},
    }
    if status == FAIL and lane["lane"] in known:
        diagnosis = known[lane["lane"]]
    classification_path = QA / lane["lane"] / "failure-classification.json"
    if status == FAIL and classification_path.is_file():
        classification = read(classification_path)
        if classification.get("candidate") == CONTEXT["candidate"]:
            diagnosis = {"category": classification.get("category", classification.get("failure_class", "UNTRIAGED")),
                         "product_failure_proven": classification.get("product_failure_proven", False),
                         "reason": classification.get("classification", classification.get("reason", classification.get("observed"))),
                         "evidence": relative(classification_path), "evidence_sha256": sha(classification_path)}
    return {"lane": lane["lane"], "status": status, "raw_terminal": terminal, "failure_classification": diagnosis,
            "lane_status": lane["status"], "mode": lane.get("mode"), "scope": lane.get("scope"),
            "evidence": lane["lane"] + "/playwright.json", "completion": lane.get("completion_file", lane["lane"] + "/completion.json"),
            "completion_sha256": lane.get("completion_sha256"), "integrity_issues": lane["integrity_issues"],
            "completed_at": lane.get("completed_at"), "native_closure": closure, "adapter_closure": adapter}


def status_for(observations):
    # A later pass cannot silently erase an original failed attempt.
    statuses = {row["status"] for row in observations}
    if FAIL in statuses:
        return FAIL
    if PASS in statuses:
        return PASS
    if BLOCKED in statuses:
        return BLOCKED
    return NOT_RUN


def latest_execution_status(observations):
    executed = [row for row in observations if row["status"] != "SKIP"]
    return max(executed, key=lambda row: row.get("completed_at") or "")["status"] if executed else NOT_RUN


def combine_status(statuses):
    return next((value for value in (FAIL, BLOCKED, NOT_RUN, PASS) if value in statuses), NOT_RUN)


def evidence_rows(paths):
    return [{"file": path, "exists": (QA / path).is_file(),
             "sha256": sha(QA / path) if (QA / path).is_file() else None} for path in paths]


def security_evidence(lanes):
    checked = []
    for lane in lanes:
        if not Path(lane["lane"]).name.startswith("security-boundary-") or not lane["complete"]:
            continue
        directory = QA / lane["lane"]
        if not all((directory / name).is_file() for name in ("probe-result.json", "security-boundary-receipt.json", "security-boundary-playwright.json")):
            continue
        report = read(directory / "probe-result.json")
        receipt = read(directory / "security-boundary-receipt.json")
        issues = []
        for field, name in (("raw_report_sha256", "security-boundary-playwright.json"), ("probe_report_sha256", "probe-result.json"), ("stderr_sha256", "security-boundary-playwright.stderr")):
            if receipt.get(field) != sha(directory / name):
                issues.append("Nested security receipt mismatch: " + name)
        rows = flatten(read(directory / "security-boundary-playwright.json"))
        complete = (len(rows) == 1 and len(rows[0]["results"]) == 1 and rows[0]["results"][0].get("retry") == 0)
        accepted = (lane["status"] == PASS and not issues and complete and receipt.get("accepted") is True
                    and report.get("status") == "pass" and report.get("playwright_request_routing") is False
                    and len(report.get("cases", [])) == 11 and all(row.get("status") == PASS for row in report["cases"]))
        checked.append({"lane": lane["lane"], "status": PASS if accepted else FAIL, "completed_at": lane.get("completed_at"),
                        "witness_counts": dict(Counter(row["status"] for row in report.get("cases", []))),
                        "no_request_routing": report.get("playwright_request_routing") is False,
                        "integrity_issues": issues, "probe_result": lane["lane"] + "/probe-result.json",
                        "probe_result_sha256": sha(directory / "probe-result.json"),
                        "limitation": "Original security01 apparent browser-CORS denial PASS is inadmissible: Playwright synthesized its preflight" if Path(lane["lane"]).name.endswith("-01") else "Actual cookie/profile, Chromium CORS and browser WS refusal plus separate RFC6455 status; not a product UI journey"})
    return checked


def knowledge_witness_issues(evidence, run):
    """Check actual Host disclosure/events and termination, not a run label."""
    issues = []
    enabled = evidence.get("expected_knowledge_enabled")
    if type(enabled) is not bool:
        return ["Knowledge expectation is not a literal boolean"]
    observed = evidence.get("actual_input", {})
    approved = evidence.get("approved", {})
    expected = [{key: approved.get(key) for key in ("candidate_id", "version", "source_issue_id", "content_sha256")}] if enabled else []
    if (observed.get("section_present") is not enabled or observed.get("contributions") != expected
            or (enabled and observed.get("no_session_persistence") is not True)):
        issues.append("Actual Host knowledge input differs from the approved configuration")
    counts = {key: int(enabled) for key in ("knowledge_claim", "knowledge_preparation", "knowledge_authorization")}
    if evidence.get("knowledge_events") != counts:
        issues.append("Exact knowledge event counts differ")
    env = evidence.get("environment", {})
    if (env.get("database") != run.get("database") or env.get("api") != run.get("api")
            or env.get("api_health") != run.get("health") or env.get("auth_mode") != "password"):
        issues.append("Knowledge witness API/database identity differs")
    settled = evidence.get("run", {})
    if any(settled.get(key) != value for key, value in {"progress_state": "settled", "proof_state": "host_exit_observed",
                                                     "business_outcome": "success", "launch_count": "1"}.items()):
        issues.append("Single settled Host execution is unproved")
    daemon = evidence.get("daemon", {})
    ledger = daemon.get("ledger", [])
    if daemon.get("exit_code") != 0 or daemon.get("errors") != [] or daemon.get("stderr") != "" or len(ledger) != 1:
        issues.append("Daemon exit or unique termination ledger is unproved")
    else:
        item = ledger[0]
        expected_ledger = {"run_id": settled.get("id"), "phase": "proof_acknowledged", "namespace_present": False,
                           "sealed": True, "proof_state": "host_exit_observed"}
        launches = item.get("launches", [])
        if (any(item.get(key) != value for key, value in expected_ledger.items()) or len(launches) != 1
                or any(launches[0].get(key) != value for key, value in {"attempt_seq": "1", "kind": "provider",
                       "phase": "attempt_reaped", "process_observed": True, "terminal_accepted": True,
                       "authorized": True, "terminal_kind": "managed_launch_ended"}.items())):
            issues.append("Host namespace/provider termination is unproved")
    return issues


def knowledge_variants(lanes):
    checked = []
    builds = {row["name"]: row for row in read(QA / "runtime/binary-builds.json")["commands"]}
    binaries = {"COLLABORATION_HOST_DAEMON": "daemon-test", "COLLABORATION_HOST_PROVIDER": "host-provider",
                "COLLABORATION_HOST_CLI": "multica"}
    for lane in lanes:
        for case in lane["cases"]:
            if case["file"] != "collaboration-knowledge-deployed.spec.ts" or case_observation(lane, case)["status"] != PASS:
                continue
            attachments = [item for item in case["results"][0].get("attachments", [])
                           if item.get("name") == "deployed-knowledge-gate-evidence" and item.get("contentType") == "application/json"]
            issues, evidence, checksum = [], {}, None
            try:
                if len(attachments) != 1:
                    raise ValueError("Expected one embedded deployed-knowledge witness")
                raw = base64.b64decode(attachments[0]["body"], validate=True)
                checksum = hashlib.sha256(raw).hexdigest()
                evidence = json.loads(raw)
                if not isinstance(evidence, dict):
                    raise ValueError("Knowledge witness must be an object")
                issues.extend(knowledge_witness_issues(evidence, read(QA / lane["lane"] / "run.json")))
                expected_sources = {name: sha(ROOT / name) for name in (
                    "e2e/collaboration-knowledge-deployed.spec.ts", "e2e/fixtures/collaboration-host-provider.go",
                    "e2e/fixtures/collaboration-host-runtime.ts")}
                if evidence.get("source_sha256") != expected_sources:
                    issues.append("Knowledge witness source hashes differ")
                for key, name in binaries.items():
                    build = builds[name]
                    if build.get("exit") != 0 or evidence.get("binary_sha256", {}).get(key) != build["sha256"] or sha(Path(build["path"])) != build["sha256"]:
                        issues.append("Knowledge binary binding differs: " + name)
            except (KeyError, ValueError, binascii.Error, OSError) as error:
                issues.append("Knowledge embedded witness cannot be verified: " + str(error))
                if not isinstance(evidence, dict):
                    evidence = {}
            checked.append({"lane": lane["lane"], "identity": case["identity"], "status": PASS if not issues and lane["status"] == PASS else BLOCKED,
                            "enabled": evidence.get("expected_knowledge_enabled"), "integrity_issues": issues,
                            "attachment_sha256": checksum, "completion_sha256": lane["completion_sha256"],
                            "playwright_sha256": sha(QA / lane["lane"] / "playwright.json"),
                            "knowledge_events": evidence.get("knowledge_events"), "run": evidence.get("run"),
                            "scope": "Actual deterministic Host input; paired configuration proof, one original identity"})
    return checked


def both_knowledge_variants(variants):
    return {row["enabled"] for row in variants if row["status"] == PASS and type(row["enabled"]) is bool} == {True, False}


def build_matrix(baseline_rows, skips, security, variants):
    config_path = OUTPUT / "coverage-map.json"
    mapping = read(config_path)
    domains, lanes = [], []
    for definition in mapping["domains"]:
        rows = [row for row in baseline_rows if re.search(definition["files"], row["file"])]
        statuses = [row["latest_execution_status"] for row in rows]
        gaps = list(definition["gaps"])
        accepted_security = [row for row in security if row["status"] == PASS]
        if accepted_security:
            gaps = [gap for gap in gaps if gap.get("resolver") != "security_boundary_probe"]
        if both_knowledge_variants(variants):
            gaps = [gap for gap in gaps if gap.get("resolver") != "knowledge_deployed_both"]
        statuses.extend(gap["status"] for gap in gaps)
        if definition.get("known_failure"):
            statuses.append(FAIL)
        extra = evidence_rows(definition["extra"])
        if any(not row["exists"] for row in extra):
            statuses.append(BLOCKED)
        elif not statuses and extra:
            statuses.append(PASS)
        domains.append({**definition, "gaps": gaps, "status": combine_status(statuses),
                        "status_basis": "Latest complete execution per named identity plus unresolved known defects and explicit gaps",
                        "retained_history_status": combine_status([row["status"] for row in rows] + ([FAIL] if definition.get("known_failure") else [])),
                        "named_identity_statuses": dict(Counter(row["status"] for row in rows)),
                        "latest_named_identity_statuses": dict(Counter(row["latest_execution_status"] for row in rows)),
                        "identities": [row["identity"] for row in rows], "extra_evidence": extra,
                        "completed_browser_lanes": sorted({item["lane"] for row in rows for item in row["observations"]
                                                           if item["status"] in {PASS, FAIL}}),
                        "supplemental_security": accepted_security if definition["id"] in {"T02", "T15"} else [],
                        "knowledge_variants": variants if definition["id"] == "T14" else [],
                        "retained_failure_classifications": [item["failure_classification"] for row in rows for item in row["observations"] if item.get("failure_classification")],
                        "acceptance_scope": "Mapped canonical cases and explicit retained extra evidence; counts overlap other domains"})
    for definition in mapping["lanes"]:
        rows = [row for row in skips if row["planned_supplement_lane"] == definition["id"]]
        status = definition.get("status", combine_status([row["latest_execution_status"] for row in rows]))
        if definition.get("variant_requirement") and not both_knowledge_variants(variants):
            status = combine_status([status, NOT_RUN])
        lanes.append({**definition, "status": status, "original_skip_statuses": dict(Counter(row["status"] for row in rows)),
                      "variant_evidence": variants if definition.get("variant_requirement") else [],
                      "latest_skip_statuses": dict(Counter(row["latest_execution_status"] for row in rows)),
                      "retained_history_status": combine_status([row["status"] for row in rows]),
                      "identities": [row["identity"] for row in rows], "evidence": evidence_rows(definition["evidence"]),
                      "completed_lanes": sorted({item["lane"] for row in rows for item in row["observations"]
                                                 if item["lane"] != BASELINE and item["status"] in {PASS, FAIL}})})
    return {"mapping_sha256": sha(config_path), "scope": mapping["scope"], "domains": domains, "lanes": lanes}


def audit(label):
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", label):
        raise ValueError("Output label must be a simple new directory name")
    destination = OUTPUT / label
    if destination.exists():
        raise ValueError("Existing audit snapshots are immutable; choose a new label")
    lanes = []
    for parent in ("runtime", "runtime-supplement"):
        for directory in sorted((QA / parent).iterdir()):
            if directory.is_dir() and (directory / "run-intent.json").is_file():
                lanes.append(inspect_lane(directory))
    performance = QA / "performance"
    if performance.is_dir():
        for directory in sorted(performance.iterdir()):
            if directory.is_dir() and (directory / "inputs.json").is_file():
                lanes.extend(inspect_performance(directory))
    knowledge = QA / "knowledge-acceptance"
    if knowledge.is_dir():
        for directory in sorted(knowledge.iterdir()):
            if directory.is_dir() and (directory / "run-start.json").is_file():
                lanes.append(inspect_knowledge(directory))
    baseline = next(row for row in lanes if row["lane"] == BASELINE)
    if baseline["integrity_issues"] or baseline.get("planned") != 538 or baseline.get("terminal_counts") != {"passed": 400, "skipped": 138}:
        raise RuntimeError("Default baseline does not independently reconcile to 538 / 400 / 138")
    observations = {}
    for lane in lanes:
        for case in lane["cases"]:
            observations.setdefault(case["identity"], []).append(case_observation(lane, case))
    dispositions, baseline_rows = [], []
    for case in baseline["cases"]:
        items = observations[case["identity"]]
        status = status_for(items)
        lane, group, prerequisite = category(case["file"])
        row = {key: case[key] for key in ("identity", "file", "title", "project")}
        row.update({"initial_status": case["terminal_status"], "status": status,
                    "status_basis": "Any retained failed attempt keeps the conservative history status FAIL",
                    "latest_execution_status": latest_execution_status(items),
                    "retained_failed_attempts": [item["lane"] for item in items if item["status"] == FAIL],
                    "planned_supplement_lane": lane, "family": group, "prerequisite": prerequisite,
                    "original_skip_reasons": sorted({a.get("description", "") for a in case["annotations"] if a.get("type") == "skip"}),
                    "observations": items,
                    "proof_boundary": "Only the exact named case and recorded execution mode; no inferred installer, commercial-model or native-mobile acceptance"})
        row["unaccepted_attempts"] = [{"lane": item["lane"], "status": item["status"], "raw_terminal": raw["terminal_status"],
                                       "reason": item["reason"], "raw_sha256": item["unaccepted_raw_sha256"]}
                                      for item in lanes for raw in item.get("unaccepted_raw_inventory", {}).get("cases", [])
                                      if raw["identity"] == case["identity"]]
        if status == NOT_RUN:
            row["disposition_reason"] = "No complete first-attempt execution of this exact identity has been admitted; original SKIP is retained"
        elif status == FAIL:
            row["disposition_reason"] = "At least one actual failed attempt remains retained; later successes do not erase it"
        elif status == BLOCKED:
            row["disposition_reason"] = "Observed evidence is not admissible because required identity/resource/inventory verification is incomplete"
        else:
            row["disposition_reason"] = "An exact first-attempt named PASS has matching retained source, inventory and cleanup evidence"
        baseline_rows.append(row)
        if case["terminal_status"] == "skipped":
            dispositions.append(row)
    counts = dict(Counter(row["status"] for row in dispositions))
    security = security_evidence(lanes)
    variants = knowledge_variants(lanes)
    matrix = build_matrix(baseline_rows, dispositions, security, variants)
    captured = datetime.now(timezone.utc).isoformat()
    summary = {"candidate": CONTEXT["candidate"], "source_fingerprint": CONTEXT["source_fingerprint"],
               "captured_at": captured, "audit_script_sha256": sha(Path(__file__)),
               "default": {key: baseline[key] for key in ("planned", "observed", "terminal_counts", "inventory_complete")},
               "original_skips": len(dispositions), "skip_dispositions": counts,
               "skip_latest_execution_statuses": dict(Counter(row["latest_execution_status"] for row in dispositions)),
               "original_identity_dispositions": dict(Counter(row["status"] for row in baseline_rows)),
               "lane_count": len(lanes), "incomplete_lanes": [row["lane"] for row in lanes if not row["complete"]],
               "record_integrity_issues": [{"lane": row["lane"], "issues": row["integrity_issues"]} for row in lanes if row["integrity_issues"]],
               "scope": "Historical raw-evidence audit; not final live process/database shutdown and not a new product test execution",
               "source_files_changed": False,
               "release_green": False}
    destination.mkdir(mode=0o700)
    for name, data in (("summary.json", summary), ("original-skip-dispositions.json", dispositions),
                       ("original-identity-dispositions.json", baseline_rows), ("lane-audit.json", lanes),
                       ("coverage-matrix.json", matrix), ("security-probe-audit.json", security)):
        with (destination / name).open("x") as stream:
            json.dump(data, stream, ensure_ascii=False, indent=2)
            stream.write("\n")
    (destination / "knowledge-variants-audit.json").write_text(json.dumps(variants, indent=2) + "\n")
    grouped = {}
    for row in dispositions:
        grouped.setdefault(row["file"], []).append(row)
    lines = ["# Default skip disposition", "", f"Captured: {captured}. Candidate: `{CONTEXT['candidate']}`.", "",
             "The original 138 SKIP terminals are retained. This table records subsequent same-identity evidence; it does not rewrite the default run.", "",
             "| File | Original skips | Current dispositions | Evidence lanes |", "| --- | ---: | --- | --- |"]
    for filename, rows in grouped.items():
        counts_text = ", ".join(f"{key} {value}" for key, value in sorted(Counter(row["status"] for row in rows).items()))
        evidence = sorted({observation["lane"] for row in rows for observation in row["observations"] if observation["lane"] != BASELINE})
        lines.append(f"| `{filename}` | {len(rows)} | {counts_text} | " + (", ".join(f"`{lane}`" for lane in evidence) or "No completed supplement") + " |")
    (destination / "skip-dispositions.md").write_text("\n".join(lines) + "\n")
    matrix_lines = ["# Current plan coverage", "", f"Captured: {captured}. Source remains `{CONTEXT['candidate']}`.", "",
                    "Statuses describe the mapped evidence and known gaps, not exhaustive behavior beyond those tests. Domain counts overlap.", "",
                    "| ID | Domain | Status | Named case dispositions | Remaining boundary |", "| --- | --- | --- | --- | --- |"]
    for row in matrix["domains"]:
        count_text = ", ".join(f"{key}:{value}" for key, value in sorted(row["latest_named_identity_statuses"].items())) or "See independent non-browser receipts"
        boundary = row.get("known_failure") or "; ".join(gap["scope"] for gap in row["gaps"]) or row["boundary"]
        matrix_lines.append(f"| {row['id']} | {row['name']} | {row['status']} | {count_text} | {boundary} |")
    matrix_lines.extend(["", "| Lane | Status | Scope |", "| --- | --- | --- |"])
    matrix_lines.extend(f"| {row['id']} | {row['status']} | {row['boundary']} |" for row in matrix["lanes"])
    (destination / "coverage-matrix.md").write_text("\n".join(matrix_lines) + "\n")
    print(json.dumps({"output": str(destination), "default": summary["default"], "skip_dispositions": summary["skip_dispositions"],
                      "latest_execution_statuses": summary["skip_latest_execution_statuses"],
                      "incomplete_lanes": summary["incomplete_lanes"], "retained_record_issues": len(summary["record_integrity_issues"])}, ensure_ascii=False))
    return summary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--label", required=True)
    args = parser.parse_args()
    try:
        audit(args.label)
    except (KeyError, ValueError, RuntimeError, OSError) as error:
        print(f"Coverage audit refused: {type(error).__name__}: {error}", file=sys.stderr)
        sys.exit(2)
