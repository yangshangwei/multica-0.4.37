"""Offline adversarial checks for result counting; no application execution."""

import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import audit_coverage as audit
from audit_coverage import latest_execution_status, reconcile, status_for


class EvidenceAccounting(unittest.TestCase):
    def setUp(self):
        case = {"id": "one", "file": "one.spec.ts", "title": "case", "tests": [{
            "projectId": "chromium", "expectedStatus": "passed", "status": "expected", "annotations": [],
            "results": [{"status": "passed", "retry": 0, "errors": []}],
        }]}
        self.report = {"suites": [{"title": "one.spec.ts", "specs": [case]}], "errors": [],
                       "config": {"workers": 1, "projects": [{"id": "chromium", "retries": 0}]}}
        self.collection = copy.deepcopy(self.report)
        self.collection["suites"][0]["specs"][0]["tests"][0]["results"] = []

    def test_same_identity_is_counted_once(self):
        self.assertTrue(reconcile(self.collection, self.report)["inventory_complete"])

    def test_missing_identity_refuses_complete(self):
        self.report["suites"] = []
        self.assertFalse(reconcile(self.collection, self.report)["inventory_complete"])

    def test_duplicate_identity_refuses_complete(self):
        self.report["suites"][0]["specs"].append(copy.deepcopy(self.report["suites"][0]["specs"][0]))
        self.assertFalse(reconcile(self.collection, self.report)["inventory_complete"])

    def test_name_drift_under_same_id_refuses_complete(self):
        self.report["suites"][0]["specs"][0]["title"] = "different case"
        self.assertFalse(reconcile(self.collection, self.report)["inventory_complete"])

    def test_failed_test_can_have_complete_identity_ledger(self):
        self.report["suites"][0]["specs"][0]["tests"][0]["results"][0].update(status="failed", errors=[{"message": "assertion"}])
        checked = reconcile(self.collection, self.report)
        self.assertTrue(checked["inventory_complete"])
        self.assertEqual(checked["terminal_counts"], {"failed": 1})

    def test_retry_is_not_first_attempt(self):
        self.report["suites"][0]["specs"][0]["tests"][0]["results"][0]["retry"] = 1
        self.assertFalse(reconcile(self.collection, self.report)["inventory_complete"])

    def test_latest_pass_retains_original_failure(self):
        history = [{"status": "FAIL", "completed_at": "2026-11-24"}, {"status": "PASS", "completed_at": "2026-11-25"}]
        self.assertEqual(status_for(history), "FAIL")
        self.assertEqual(latest_execution_status(history), "PASS")

    def test_skip_never_becomes_pass(self):
        self.assertEqual(status_for([{"status": "SKIP"}]), "NOT_RUN")

    def test_raw_pass_without_completion_is_never_admitted(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            lane = root / "business"
            lane.mkdir()
            (lane / "collection.json").write_text(json.dumps(self.collection))
            (lane / "playwright.json").write_text(json.dumps(self.report))
            (lane / "interruption-recovery-before.json").write_text('{}')
            with patch.object(audit, "QA", root):
                checked = audit.inspect_lane(lane)
            self.assertEqual(checked["status"], "INTERRUPTED")
            self.assertEqual(checked["cases"], [])
            self.assertEqual(checked["unaccepted_raw_inventory"]["terminal_counts"], {"passed": 1})
            self.assertFalse(checked["complete"])


class LinuxClosure(unittest.TestCase):
    def test_failed_journey_can_have_valid_cleanup_but_never_pass(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            outer = root / "linux-main-acceptance/linux-main-04"
            nested = root / "runtime/linux-main-04"
            registry = root / "private/linux-main-acceptance/linux-main-04/resources.private.json"
            files = {outer / "cleanup.json": {"failures": [], "results": {key: {"status": "pass"} for key in ("owned_processes", "containers", "bindings")}},
                     outer / "docker-commands.json": {}, registry: {}, nested / "completion.json": {"exit": 1},
                     nested / "cleanup.json": {}, nested / "named-inventory.json": {}, nested / "source-end.json": {}}
            for file, value in files.items():
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text(json.dumps(value))
            completion = {"exit": 1, "status": "fail", "runtime_exit": 1, "runtime_report": str(nested), "cleanup_failures": [],
                          "evidence_problems": ["nested runtime did not complete successfully"],
                          "evidence_sha256": {str(file): audit.sha(file) for file in files}}
            (outer / "completion.json").write_text(json.dumps(completion))
            lane = {"lane": "runtime/linux-main-04", "exit": 1, "status": "FAIL", "integrity_issues": [], "inventory_complete": True}
            with patch.object(audit, "QA", root), patch.dict(audit.CONTEXT, {"secure": str(root / "private")}):
                self.assertEqual(audit.linux_main_closure(lane)["status"], "PASS")
                case = {"file": "collaboration-installed-main.spec.ts", "terminal_status": "passed"}
                self.assertEqual(audit.case_observation(lane, case)["status"], "BLOCKED")
                case["terminal_status"] = "failed"
                self.assertEqual(audit.case_observation(lane, case)["status"], "FAIL")
                registry.write_text('{"changed":true}')
                self.assertEqual(audit.linux_main_closure(lane)["status"], "FAIL")
                registry.unlink()
                self.assertEqual(audit.linux_main_closure(lane)["status"], "FAIL")


class KnowledgeEvidence(unittest.TestCase):
    def setUp(self):
        self.run = {"database": "task-db", "api": "http://localhost:1234", "health": {"pid": 123, "commit": "candidate"}}
        self.evidence = {
            "expected_knowledge_enabled": True,
            "approved": {"candidate_id": "candidate", "version": "1", "source_issue_id": "issue", "content_sha256": "hash"},
            "actual_input": {"section_present": True, "no_session_persistence": True},
            "knowledge_events": {"knowledge_claim": 1, "knowledge_preparation": 1, "knowledge_authorization": 1},
            "environment": {"database": "task-db", "api": "http://localhost:1234", "api_health": self.run["health"], "auth_mode": "password"},
            "run": {"id": "run", "progress_state": "settled", "proof_state": "host_exit_observed", "business_outcome": "success", "launch_count": "1"},
            "daemon": {"exit_code": 0, "errors": [], "stderr": "", "ledger": [{
                "run_id": "run", "phase": "proof_acknowledged", "namespace_present": False, "sealed": True,
                "proof_state": "host_exit_observed", "launches": [{"attempt_seq": "1", "kind": "provider", "phase": "attempt_reaped",
                    "process_observed": True, "terminal_accepted": True, "authorized": True, "terminal_kind": "managed_launch_ended"}],
            }]},
        }
        self.evidence["actual_input"]["contributions"] = [self.evidence["approved"].copy()]

    def test_enabled_requires_exact_approved_disclosure(self):
        self.assertEqual(audit.knowledge_witness_issues(self.evidence, self.run), [])
        self.evidence["actual_input"]["contributions"][0]["candidate_id"] = "other"
        self.assertTrue(audit.knowledge_witness_issues(self.evidence, self.run))

    def test_disabled_requires_no_disclosure_or_events(self):
        self.evidence["expected_knowledge_enabled"] = False
        self.evidence["actual_input"] = {"section_present": False, "contributions": []}
        self.evidence["knowledge_events"] = dict.fromkeys(self.evidence["knowledge_events"], 0)
        self.assertEqual(audit.knowledge_witness_issues(self.evidence, self.run), [])
        self.evidence["knowledge_events"]["knowledge_claim"] = 1
        self.assertTrue(audit.knowledge_witness_issues(self.evidence, self.run))

    def test_wrong_database_refuses_witness(self):
        self.evidence["environment"]["database"] = "foreign"
        self.assertTrue(audit.knowledge_witness_issues(self.evidence, self.run))

    def test_unreaped_process_refuses_witness(self):
        self.evidence["daemon"]["ledger"][0]["launches"][0]["phase"] = "running"
        self.assertTrue(audit.knowledge_witness_issues(self.evidence, self.run))

    def test_missing_or_non_boolean_expectation_refuses_witness(self):
        for value in (None, "false", 1):
            self.evidence["expected_knowledge_enabled"] = value
            self.assertTrue(audit.knowledge_witness_issues(self.evidence, self.run))

    def test_both_variants_require_independent_admitted_evidence(self):
        on = {"status": "PASS", "enabled": True}
        off = {"status": "PASS", "enabled": False}
        self.assertFalse(audit.both_knowledge_variants([on, on]))
        self.assertFalse(audit.both_knowledge_variants([on, {**off, "status": "BLOCKED"}]))
        self.assertTrue(audit.both_knowledge_variants([on, off]))

    def test_missing_corrupt_or_nonobject_attachment_never_closes_gap(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "runtime").mkdir()
            (root / "runtime/binary-builds.json").write_text('{"commands": []}')
            lane_dir = root / "knowledge"
            lane_dir.mkdir()
            (lane_dir / "run.json").write_text(json.dumps(self.run))
            (lane_dir / "playwright.json").write_text('{}')
            for body in (None, "not base64", "W10="):
                attachments = [] if body is None else [{"name": "deployed-knowledge-gate-evidence", "contentType": "application/json", "body": body}]
                case = {"identity": "knowledge::chromium", "file": "collaboration-knowledge-deployed.spec.ts",
                        "terminal_status": "passed", "results": [{"attachments": attachments}]}
                lane = {"lane": "knowledge", "cases": [case], "integrity_issues": [], "inventory_complete": True,
                        "status": "PASS", "completion_sha256": "fixture"}
                with patch.object(audit, "QA", root):
                    rows = audit.knowledge_variants([lane])
                self.assertEqual(rows[0]["status"], "BLOCKED")
                self.assertFalse(audit.both_knowledge_variants(rows))


if __name__ == "__main__":
    unittest.main()
