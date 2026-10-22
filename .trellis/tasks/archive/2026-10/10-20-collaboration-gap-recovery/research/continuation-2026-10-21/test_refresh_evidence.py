"""Regression tests for task evidence refresh; all inputs and outputs are private."""

import contextlib
import copy
import importlib.util
import io
import json
import pathlib
import shutil
import sys
import tempfile
import unittest
from unittest import mock


SCRIPT = pathlib.Path(__file__).with_name("refresh-evidence.py")
SPEC = importlib.util.spec_from_file_location("refresh_evidence", SCRIPT)
refresh = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(refresh)


class RefreshEvidenceTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="multica-evidence-test-")
        self.addCleanup(temporary.cleanup)
        self.repo = pathlib.Path(temporary.name)
        self.task = self.repo / ".trellis/tasks/10-20-collaboration-gap-recovery"
        self.here = self.task / "research/continuation-2026-10-21"
        self.here.mkdir(parents=True)
        for name, value in {
            "REPO": self.repo,
            "TASK": self.task,
            "HERE": self.here,
            "MATRIX_PATH": self.task / "acceptance-matrix.json",
            "CATALOG_PATH": self.task / "research/s6-evidence-map.json",
        }.items():
            patch = mock.patch.object(refresh, name, value)
            patch.start()
            self.addCleanup(patch.stop)
        criteria = [
            {
                "id": f"AC{number:02}",
                "requirements": "R01",
                "criterion": f"Complete criterion {number}",
                "source": "parent/acceptance.md",
                "source_line": number,
                "primary_owner": "P1",
                "delivery_slices": ["S6"],
                "status": "not_verified_in_this_task",
                "test_identities": [],
                "missing_current_test_evidence": [],
                "remaining_gaps": ["Unreviewed criterion scope remains pending."],
                "assessment": "Partial support; full acceptance remains pending.",
                "evidence": [],
            }
            for number in range(1, 98)
        ]
        criteria[0].update(
            missing_current_test_evidence=[
                "internal/service::TestCanonicalCriterion",
                "internal/service::TestStillMissing",
            ],
            remaining_gaps=[
                "Current completed execution evidence has not yet been collected.",
                "The complete independent scope still needs review.",
            ],
            evidence=[{
                "run_id": "previous-partial",
                "scope": "source_review_only",
                "coverage": "combined_full_criterion_support",
            }],
        )
        criteria[1].update(
            status="passed",
            test_identities=["internal/service::TestCanonicalCriterion"],
            remaining_gaps=[],
            assessment="Independently reviewed before this refresh.",
        )
        self.matrix = {
            "criteria": criteria,
            "status_vocabulary": ["not_verified_in_this_task", "passed", "failed", "skipped", "unsupported"],
            "task_status": "in_progress",
            "full_acceptance_complete": False,
            "audit": {"status_counts": {"passed": 1, "not_verified_in_this_task": 96}},
            "limitations": ["Retain the observed rollout and provider limits."],
        }
        self.write_json(refresh.MATRIX_PATH, self.matrix)
        self.write_json(refresh.CATALOG_PATH, {"evidence": {"previous-partial": {"kind": "source_review_only"}}})
        tests = ["TestCanonicalCriterion"] + [f"TestPrivateInventory{number}" for number in range(1152)]
        self.write_json(self.here / "go-service-partitions.json", {"private": {"tests": tests}})
        files = {
            "go-service-partition-1.jsonl": "internal/service",
            "go-service-partition-2.jsonl": "internal/service",
            "go-service-partition-3.jsonl": "internal/service",
            "go-service-redis.jsonl": "internal/service",
            "go-other-packages.jsonl": "internal/handler",
            "go-remaining-packages.jsonl": "internal/collaboration",
            "go-other-redis.jsonl": "internal/handler",
            "linux-router.jsonl": "cmd/server",
        }
        for name, package in files.items():
            package = "github.com/multica-ai/multica/server/" + package
            events = []
            if name == "go-service-partition-2.jsonl":
                events = [{"Action": "pass", "Package": package, "Test": test, "Elapsed": 0.01} for test in tests]
            events.append({"Action": "pass", "Package": package, "Elapsed": 1})
            (self.here / name).write_text("".join(json.dumps(event) + "\n" for event in events))
        report_root = self.repo / ".omx/reports/cwc-gap-recovery-20261021"
        reports = [
            report_root / "continuation-mentions-web/attempt-6.json",
            report_root / "continuation-mentions-desktop/attempt-2.json",
            report_root / "continuation-controls-web-v5.json",
            report_root / "continuation-controls-desktop-v2.json",
            self.here / "playwright-admin-web.json",
            self.here / "playwright-admin-desktop.json",
            self.here / "playwright-prerequisites.json",
        ]
        for path in reports:
            self.write_json(path, {
                "stats": {"expected": 1, "unexpected": 0, "skipped": 0, "flaky": 0},
                "suites": [{"specs": [{
                    "file": "private.spec.ts",
                    "title": "private fixture pass",
                    "line": 1,
                    "tests": [{"results": [{"status": "passed", "retry": 0, "duration": 1}]}],
                }]}],
            })
        self.sentinels = {}
        for name in ["go-execution-summary.json", "ledger-proposal.json", "acceptance-matrix.next.json", "s6-evidence-map.next.json"]:
            path = self.here / name
            path.write_text("Existing coordinator-owned artifact: " + name + "\n")
            self.sentinels[path] = path.read_bytes()
        self.authoritative = {path: path.read_bytes() for path in [refresh.MATRIX_PATH, refresh.CATALOG_PATH]}

    @staticmethod
    def write_json(path, value):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value) + "\n")

    def invoke(self, *arguments):
        stdout, stderr = io.StringIO(), io.StringIO()
        with mock.patch.object(sys, "argv", [str(SCRIPT), *arguments]), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            refresh.main()
        result = json.loads(stdout.getvalue())
        if "output_directory" in result:
            self.addCleanup(shutil.rmtree, result["output_directory"])
        return result

    def test_default_refresh_preserves_acceptance_and_existing_files(self):
        output_directory = self.repo / "fresh-output"
        output_directory.mkdir()
        with mock.patch("tempfile.mkdtemp", return_value=str(output_directory)):
            result = self.invoke()
        for path, content in {**self.authoritative, **self.sentinels}.items():
            self.assertEqual(path.read_bytes(), content, str(path))
        matrix = json.loads((pathlib.Path(result["output_directory"]) / "acceptance-matrix.next.json").read_text())
        self.assertEqual(result["promoted_ids"], [])
        self.assertEqual(matrix["task_status"], "in_progress")
        self.assertIs(matrix["full_acceptance_complete"], False)
        self.assertEqual(matrix["limitations"], self.matrix["limitations"])
        for before, after in zip(self.matrix["criteria"], matrix["criteria"]):
            for field in ["status", "remaining_gaps", "assessment", "criterion", "primary_owner"]:
                self.assertEqual(after[field], before[field], f"{before['id']} {field}")

    def test_resolved_canonical_identity_is_retained_and_pending_links_are_partial(self):
        result = self.invoke()
        output = pathlib.Path(result.get("output_directory", self.here))
        matrix = json.loads((output / "acceptance-matrix.next.json").read_text())
        row = matrix["criteria"][0]
        self.assertIn("internal/service::TestCanonicalCriterion", row["test_identities"])
        self.assertEqual(row["missing_current_test_evidence"], ["internal/service::TestStillMissing"])
        self.assertEqual(row["status"], "not_verified_in_this_task")
        self.assertEqual(row["remaining_gaps"], self.matrix["criteria"][0]["remaining_gaps"])
        self.assertTrue(all(item["coverage"] == "partial_support" for item in row["evidence"]))
        self.assertEqual(result["promoted_ids"], [])

    def test_all_missing_executions_resolved_does_not_promote_or_clear_clause_gaps(self):
        matrix = copy.deepcopy(self.matrix)
        matrix["criteria"][0]["missing_current_test_evidence"] = ["internal/service::TestCanonicalCriterion"]
        matrix["criteria"][0]["remaining_gaps"] = ["Current completed execution evidence has not yet been collected."]
        self.write_json(refresh.MATRIX_PATH, matrix)
        result = self.invoke()
        output = pathlib.Path(result.get("output_directory", self.here))
        row = json.loads((output / "acceptance-matrix.next.json").read_text())["criteria"][0]
        self.assertEqual(row["missing_current_test_evidence"], [])
        self.assertEqual(row["status"], "not_verified_in_this_task")
        self.assertEqual(row["remaining_gaps"], matrix["criteria"][0]["remaining_gaps"])

    def test_apply_and_promote_are_rejected_without_touching_private_authoritative_files(self):
        for arguments in [("--apply",), ("--promote", "AC01")]:
            with self.subTest(arguments=arguments):
                with self.assertRaises(SystemExit) as raised:
                    self.invoke(*arguments)
                self.assertEqual(raised.exception.code, 2)
        for path, content in self.authoritative.items():
            self.assertEqual(path.read_bytes(), content)

    def test_existing_output_directory_is_never_overwritten(self):
        with self.assertRaises(FileExistsError):
            self.invoke("--output-dir", str(self.here))
        for path, content in self.sentinels.items():
            self.assertEqual(path.read_bytes(), content)

    def test_failed_artifact_refuses_evidence_output(self):
        path = self.here / "go-service-partition-2.jsonl"
        with path.open("a") as file:
            file.write(json.dumps({"Action": "fail", "Package": "github.com/multica-ai/multica/server/internal/service", "Test": "TestCanonicalCriterion"}) + "\n")
        output = self.repo / "failed-output"
        with self.assertRaises(AssertionError):
            self.invoke("--output-dir", str(output))
        self.assertFalse(output.exists())

    def test_recorded_artifact_hash_drift_is_not_reblessed(self):
        self.write_json(refresh.CATALOG_PATH, {"evidence": {"continuation-service-2": {
            "artifacts": [{"path": str(self.here / "go-service-partition-2.jsonl"), "sha256": "0" * 64}],
        }}})
        output = self.repo / "drift-output"
        with self.assertRaisesRegex(ValueError, "Recorded artifact changed"):
            self.invoke("--output-dir", str(output))
        self.assertFalse(output.exists())

    def test_catalog_preserves_reviewed_provenance_and_limits(self):
        self.write_json(refresh.CATALOG_PATH, {"evidence": {"continuation-service-2": {
            "environment": {"schema_migration": 999, "reviewed_scope": "synthetic-private"},
            "source_manifest": "retained/source-manifest.json",
            "report": "retained/review.md",
            "artifacts": [{"path": "retained/source-manifest.json", "sha256": "1" * 64}],
            "limits": ["A retained boundary must survive refresh."],
        }}})
        output = self.repo / "provenance-output"
        self.invoke("--output-dir", str(output))
        entry = json.loads((output / "s6-evidence-map.next.json").read_text())["evidence"]["continuation-service-2"]
        self.assertEqual(entry["source_manifest"], "retained/source-manifest.json")
        self.assertEqual(entry["report"], "retained/review.md")
        self.assertEqual(entry["environment"]["schema_migration"], 999)
        self.assertEqual(entry["environment"]["reviewed_scope"], "synthetic-private")
        self.assertIn("A retained boundary must survive refresh.", entry["limits"])
        self.assertIn({"path": "retained/source-manifest.json", "sha256": "1" * 64}, entry["artifacts"])

    def test_duplicate_criterion_ids_refuse_output(self):
        matrix = copy.deepcopy(self.matrix)
        matrix["criteria"][-1]["id"] = matrix["criteria"][0]["id"]
        self.write_json(refresh.MATRIX_PATH, matrix)
        output = self.repo / "duplicate-output"
        with self.assertRaises(AssertionError):
            self.invoke("--output-dir", str(output))
        self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
