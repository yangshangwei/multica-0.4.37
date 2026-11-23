"""Unchanged pnpm adapter contracts; no pnpm, browser or Agent CLI executes."""
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent
WRAPPER = ROOT / "check-bin/pnpm"
OBSERVATIONS = []


class Forwarded(Exception):
    pass


class PnpmReportingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="pnpm-", dir=ROOT / ".tmp")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        scripts = self.root / "scripts"
        scripts.mkdir()
        self.guard = scripts / "go-test-with-agent-cli-guard.sh"
        self.guard.write_text("# test marker; never executed\n")
        self.env = {"MULTICA_TASK_REAL_PNPM": "/fixture/fake-pnpm", "MULTICA_TASK_CANDIDATE_ROOT": str(self.root),
                    "MULTICA_TASK_RENDERER_URL": "http://127.0.0.1:19841", "NEXT_PUBLIC_API_URL": "http://127.0.0.1:18081",
                    "DATABASE_URL": "postgres://127.0.0.1/multica_check_fixture_api",
                    "MULTICA_TASK_PACKAGE_DB_EVIDENCE": str(self.root / "packages"),
                    "PLAYWRIGHT_BASE_URL": "http://127.0.0.1:19081"}

    def invoke(self, argv, **changes):
        calls = []
        output = io.StringIO()
        def recorder(executable, arguments, environment):
            calls.append({"executable": executable, "argv": arguments, "environment": dict(environment)})
            raise Forwarded()
        with mock.patch.dict(os.environ, {**self.env, **changes}, clear=True), \
             mock.patch.object(sys, "argv", [str(WRAPPER), *argv]), \
             mock.patch.object(os, "execve", recorder), mock.patch.object(os, "execvpe", recorder), \
             mock.patch.object(os, "execv", side_effect=AssertionError("Unexpected execv")), \
             contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            terminal = "returned"
            try:
                runpy.run_path(str(WRAPPER), run_name="__main__")
            except Forwarded:
                terminal = "forwarded"
            except (SystemExit, AssertionError) as error:
                terminal = type(error).__name__
        record = {"test": self.id(), "argv": argv, "terminal": terminal, "calls": calls,
                  "actual_pnpm_or_guard_calls": 0, "actual_network_calls": 0}
        OBSERVATIONS.append(record)
        return record

    def test_selection_workers_retries_timeout_and_default_reporter_are_preserved(self):
        argv = ["exec", "playwright", "test", "e2e/one.spec.ts", "e2e/two.spec.ts", "--grep", "named case",
                "--workers=3", "--retries=0", "--timeout=32000", "--project=chromium", "--output=fixture-artifacts"]
        result = self.invoke(argv)
        self.assertEqual(result["terminal"], "forwarded")
        call = result["calls"][0]
        self.assertEqual(call["argv"], ["bash", str(self.guard), "--", "/fixture/fake-pnpm", *argv, "--reporter=line,json"])
        env = call["environment"]
        self.assertEqual(env["PLAYWRIGHT_JSON_OUTPUT_NAME"], str(self.root / "makecheck-playwright-results.json"))
        self.assertEqual(env["COLLABORATION_E2E_API_URL"], self.env["NEXT_PUBLIC_API_URL"])
        self.assertEqual(env["CHANGELOG_E2E_API_URL"], self.env["NEXT_PUBLIC_API_URL"])
        self.assertEqual(env["COLLABORATION_ELECTRON_SOURCE_ROOT"], str(self.root))
        self.assertEqual(env["COLLABORATION_ELECTRON_RENDERER_URL"], self.env["MULTICA_TASK_RENDERER_URL"])
        self.assertEqual(env["CHANGELOG_ELECTRON_RENDERER_URL"], self.env["MULTICA_TASK_RENDERER_URL"])
        evidence = json.loads((self.root / "makecheck-playwright-environment.json").read_text())
        self.assertEqual(evidence["command"], ["pnpm", *argv, "--reporter=line,json"])
        self.assertFalse(evidence["provider_accounts"])

    def test_both_explicit_reporter_forms_remain_unchanged(self):
        for reporter in (["--reporter=dot"], ["--reporter", "line,json"]):
            with self.subTest(reporter=reporter):
                argv = ["exec", "playwright", "test", "e2e/one.spec.ts", "--workers=1", *reporter]
                self.assertEqual(self.invoke(argv)["calls"][0]["argv"][4:], argv)

    def test_non_playwright_commands_forward_without_guard_or_added_flags(self):
        for argv in (["lint"], ["--filter", "@multica/views", "test"], ["exec", "turbo", "typecheck"]):
            with self.subTest(argv=argv):
                self.assertEqual(self.invoke(argv)["calls"][0]["argv"], ["/fixture/fake-pnpm", *argv])
                self.assertFalse((self.root / "makecheck-playwright-environment.json").exists())

    def test_default_database_or_remote_api_refuses_before_forwarding(self):
        for changes in ({"DATABASE_URL": "postgres://127.0.0.1/multica"}, {"NEXT_PUBLIC_API_URL": "https://example.invalid"}):
            with self.subTest(changes=changes):
                result = self.invoke(["exec", "playwright", "test"], **changes)
                self.assertEqual(result["terminal"], "SystemExit")
                self.assertEqual(result["calls"], [])

    def test_missing_repository_guard_refuses_before_pnpm_execution(self):
        self.guard.unlink()
        result = self.invoke(["exec", "playwright", "test"])
        self.assertEqual(result["terminal"], "AssertionError")
        self.assertEqual(result["calls"], [])


if __name__ == "__main__":
    result = unittest.main(verbosity=2, exit=False).result
    report = {"wrapper": str(WRAPPER), "wrapper_sha256": hashlib.sha256(WRAPPER.read_bytes()).hexdigest(),
              "tests_run": result.testsRun, "failure_count": len(result.failures), "error_count": len(result.errors),
              "observations": OBSERVATIONS, "method": "Exact pnpm source with all execve/execvpe boundaries mocked"}
    output = os.environ.get("CHECK_PNPM_EVIDENCE")
    if output:
        Path(output).write_text(json.dumps(report, indent=2) + "\n")
    raise SystemExit(0 if result.wasSuccessful() else 1)
