"""Pure launcher admission tests. No subprocess, database or network is used."""

import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location(
    "f2_test_runner", Path(__file__).with_name("f2-test-runner.py")
)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class AdmissionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="f2-runner-unit-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.evidence = self.root / "evidence"
        self.evidence.mkdir()
        self.package = "./internal/handler"
        self.package_id = "example.test/server/internal/handler"
        self.source = self.root / "server/internal/handler/example_test.go"
        self.source.parent.mkdir(parents=True)
        self.source.write_text("package handler\nfunc TestAllowed(t *testing.T) {}\n")
        (self.root / "server/go.mod").write_text("module example.test/server\n")
        self.manifest = self.root / "owned.json"
        self.manifest.write_text(json.dumps({"files": [
            "server/go.mod", "server/internal/handler/example_test.go"
        ]}))
        self.permit = self.root / "permit.json"
        self.permit.write_text(json.dumps({"package": self.package,
                                           "tests": ["TestAllowed"]}))
        self.database = "f2_admission_abc12345"
        self.url = "postgres://tester:unit-secret@localhost/" + self.database
        self.env_path = self.root / "environment.json"
        self.env_path.write_text(json.dumps({"DATABASE_URL": self.url, "PATH": "/fake"}))
        self.record = self.evidence / "admission.json"
        for name, value in {"ROOT": self.root, "EVIDENCE": self.evidence,
                            "MANIFEST": self.manifest}.items():
            patched = patch.object(runner, name, value, create=True)
            patched.start()
            self.addCleanup(patched.stop)
        self.write_ready_record()
        # A mistakenly unmocked process call is a test failure, never execution.
        self.process = patch.object(runner.subprocess, "run",
                                    side_effect=AssertionError("unexpected process"))
        self.process_mock = self.process.start()
        self.addCleanup(self.process.stop)
        self.query = patch.object(runner, "query", return_value=self.database)
        self.query_mock = self.query.start()
        self.addCleanup(self.query.stop)

    def write_ready_record(self, **changes):
        value = {"label": "admission", "package": self.package,
                 "database": self.database, "state": "ready",
                 "private_environment": str(self.env_path),
                 "runner_sha256": hashlib.sha256(Path(runner.__file__).read_bytes()).hexdigest(),
                 "source_sha256": runner.source_hashes()}
        value.update(changes)
        self.record.write_text(json.dumps(value))

    def event(self, action, test=None, **extra):
        value = {"Action": action, "Package": self.package_id, **extra}
        if test is not None:
            value["Test"] = test
        return value

    def passing_events(self):
        return [self.event("run", "TestAllowed"),
                self.event("pass", "TestAllowed"), self.event("pass")]

    def execute(self, events=None, exit_code=0, mutate=None, regex=None):
        if events is None:
            events = self.passing_events()

        def fake_process(argv, **kwargs):
            self.assertIn("scripts/go-test-with-agent-cli-guard.sh", argv)
            self.assertEqual(argv[argv.index("-run") + 1], "^(TestAllowed)$")
            self.assertEqual(kwargs["env"]["DATABASE_URL"], self.url)
            if mutate:
                mutate()
            return subprocess.CompletedProcess(
                argv, exit_code,
                stdout="\n".join(json.dumps(event) for event in events) + "\n",
            )

        self.process_mock.side_effect = fake_process
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            result = runner.run("admission", regex, False, self.permit)
        return result, json.loads(self.record.read_text()), output.getvalue()

    def test_missing_manifest_file_is_refused(self):
        self.source.unlink()
        with self.assertRaisesRegex(RuntimeError, "source"):
            runner.source_hashes()

    def test_manifest_paths_must_be_nonempty_unique_and_contained(self):
        for paths in [[], ["server/go.mod", "server/go.mod"], ["../escape"],
                      [str(self.source)], ["server/./go.mod"], ["server//go.mod"],
                      ["server\\go.mod"], [123]]:
            with self.subTest(paths=paths):
                self.manifest.write_text(json.dumps({"files": paths}))
                with self.assertRaises(RuntimeError):
                    runner.source_hashes()
        outside = self.root.parent / (self.root.name + "-outside")
        outside.write_text("outside")
        self.addCleanup(outside.unlink)
        (self.root / "escape").symlink_to(outside)
        self.manifest.write_text(json.dumps({"files": ["escape"]}))
        with self.assertRaises(RuntimeError):
            runner.source_hashes()

    def test_exact_named_run_is_accepted_and_logged(self):
        code, record, _ = self.execute()
        self.assertEqual(code, 0)
        self.assertTrue(record["accepted"])
        self.assertEqual(record["test_exit_code"], 0)
        self.assertEqual(record["expected_tests"], ["TestAllowed"])
        self.assertTrue(record["inventory_complete"])
        self.assertEqual(record["test_source_sha256"], record["final_source_sha256"])

    def test_regex_or_expected_source_name_mismatch_never_queries_database(self):
        for regex, names in [("^Test", ["TestAllowed"]),
                             (None, ["TestMissing"])]:
            with self.subTest(regex=regex, names=names):
                self.permit.write_text(json.dumps({"package": self.package, "tests": names}))
                with self.assertRaises(RuntimeError):
                    runner.run("admission", regex, False, self.permit)
        self.query_mock.assert_not_called()
        self.process_mock.assert_not_called()

    def test_wrong_record_package_or_database_never_executes(self):
        for changes in [{"package": "./..."}, {"label": "different"},
                        {"database": "shared_database"}]:
            with self.subTest(changes=changes):
                self.write_ready_record(**changes)
                with self.assertRaises(RuntimeError):
                    runner.run("admission", None, False, self.permit)
        self.write_ready_record()
        self.env_path.write_text(json.dumps({"DATABASE_URL": self.url + "_wrong"}))
        with self.assertRaises(RuntimeError):
            runner.run("admission", None, False, self.permit)
        self.query_mock.assert_not_called()
        self.process_mock.assert_not_called()

    def test_actual_database_mismatch_refuses_before_test_process(self):
        self.query_mock.return_value = "unowned"
        with self.assertRaises(RuntimeError):
            runner.run("admission", None, False, self.permit)
        self.process_mock.assert_not_called()

    def test_skipped_missing_unexpected_and_wrong_package_events_fail_acceptance(self):
        cases = [[], [self.event("pass")],
                 [self.event("run", "TestAllowed"), self.event("skip", "TestAllowed"),
                  self.event("pass")],
                 self.passing_events() + [self.event("run", "TestOther"),
                                          self.event("pass", "TestOther")],
                 self.passing_events() + [self.event("skip", "TestAllowed/subcase")],
                 [self.event("run", "TestAllowed"), self.event("pass", "TestAllowed")],
                 [self.event("run", "TestAllowed", Package="example.test/wrong"),
                  self.event("pass", "TestAllowed", Package="example.test/wrong"),
                  self.event("pass", Package="example.test/wrong")]]
        for events in cases:
            with self.subTest(events=events):
                self.write_ready_record()
                log = self.evidence / "admission-tests.jsonl"
                if log.exists():
                    log.unlink()
                code, record, _ = self.execute(events)
                self.assertNotEqual(code, 0)
                self.assertFalse(record["accepted"])
                self.assertEqual(record["test_exit_code"], 0)

    def test_original_test_exit_is_preserved(self):
        code, record, _ = self.execute([self.event("run", "TestAllowed"),
                                         self.event("fail", "TestAllowed"),
                                         self.event("fail")], exit_code=7)
        self.assertEqual(code, 7)
        self.assertEqual(record["test_exit_code"], 7)
        self.assertEqual(record["exit_code"], 7)
        self.assertFalse(record["accepted"])

    def test_source_drift_keeps_passing_test_exit_but_refuses_acceptance(self):
        code, record, _ = self.execute(mutate=lambda: self.source.write_text("changed"))
        self.assertNotEqual(code, 0)
        self.assertEqual(record["test_exit_code"], 0)
        self.assertFalse(record["accepted"])
        self.assertIn("source_identity_changed", record["acceptance_failures"])

    def test_missing_source_after_run_preserves_failure_evidence(self):
        code, record, _ = self.execute(mutate=self.source.unlink)
        self.assertNotEqual(code, 0)
        self.assertEqual(record["test_exit_code"], 0)
        self.assertIsNone(record["final_source_sha256"])
        self.assertTrue((self.evidence / "admission-tests.jsonl").is_file())

    def test_changed_source_since_allocation_is_not_admitted(self):
        self.source.write_text(self.source.read_text() + "// edited\n")
        with self.assertRaises(RuntimeError):
            runner.run("admission", None, False, self.permit)
        self.query_mock.assert_not_called()
        self.process_mock.assert_not_called()

    def test_connection_secrets_are_not_recorded_or_printed(self):
        events = self.passing_events()
        events.insert(1, self.event("output", "TestAllowed",
                                   Output="connection failed " + self.url + " unit-secret"))
        code, record, printed = self.execute(events)
        self.assertEqual(code, 0)
        public = json.dumps(record) + printed + (self.evidence / "admission-tests.jsonl").read_text()
        self.assertNotIn(self.url, public)
        self.assertNotIn("unit-secret", public)

    def test_existing_log_is_not_overwritten_or_reexecuted(self):
        log = self.evidence / "admission-tests.jsonl"
        log.write_text("retained failure")
        with self.assertRaises(RuntimeError):
            runner.run("admission", None, False, self.permit)
        self.assertEqual(log.read_text(), "retained failure")
        self.assertEqual(json.loads(self.record.read_text())["state"], "ready")
        self.process_mock.assert_not_called()

    def test_launch_error_retains_failed_record_without_exception_secrets(self):
        self.process_mock.side_effect = OSError("failed " + self.url)
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = runner.run("admission", None, False, self.permit)
        record = json.loads(self.record.read_text())
        self.assertNotEqual(code, 0)
        self.assertIsNone(record["test_exit_code"])
        self.assertFalse(record["accepted"])
        self.assertIn("test_process_unavailable", record["acceptance_failures"])
        self.assertNotIn(self.url, self.record.read_text() + output.getvalue())
        self.assertTrue((self.evidence / "admission-tests.jsonl").is_file())


if __name__ == "__main__":
    unittest.main()
