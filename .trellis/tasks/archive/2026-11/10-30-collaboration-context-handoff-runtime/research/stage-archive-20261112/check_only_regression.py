"""Exercise launcher modes without a Provider, database, or execution permit.

Run with an explicit launcher path, including the preserved pre-fix copy.
All admission and process operations are mocks; no registered fixture is read.
"""
import importlib.util
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


launcher_path = Path(sys.argv.pop(1)).resolve()
spec = importlib.util.spec_from_file_location("continuation_under_test", launcher_path)
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)


class LauncherModeTests(unittest.TestCase):
    def invoke(self, *, check_only, real_gate, problems=()):
        with tempfile.TemporaryDirectory(prefix="runtime-launcher-mock-") as temp:
            root = Path(temp)
            output = root / "uncreated-output"
            binding = {
                "output": str(output),
                "current": {"operator_config_sha256": "fixture-config"},
                "execution_gate": dict.fromkeys(
                    ("runnable", "allow_build", "allow_check", "allow_run"), True
                ),
            }
            arguments = ["RK1-H3a", "3"]
            if check_only:
                arguments.append("--check-only")
            with (
                patch.object(launcher, "HERE", root),
                patch.object(launcher, "load_binding", return_value=(binding, None)),
                patch.object(launcher, "validate", return_value=list(problems)),
                patch.object(launcher, "sha", return_value="fixture-sha"),
                patch.object(launcher, "write_json") as writes,
                patch.object(
                    launcher.subprocess, "run", return_value=SimpleNamespace(returncode=0)
                ) as process,
                patch.dict(os.environ, {"MULTICA_RUN_REAL_AGENT_SMOKE": real_gate}),
            ):
                result = launcher.main(arguments)
                return result, process.call_count, writes.call_count, output.exists()

    def test_check_only_never_launches_with_fully_open_gates(self):
        self.assertEqual(
            self.invoke(check_only=True, real_gate="1"),
            (0, 0, 1, False),
        )

    def test_check_only_does_not_require_real_process_opt_in(self):
        self.assertEqual(
            self.invoke(check_only=True, real_gate=""),
            (0, 0, 1, False),
        )

    def test_failed_preflight_remains_refused_without_process(self):
        self.assertEqual(
            self.invoke(check_only=True, real_gate="1", problems=["fixture refusal"]),
            (2, 0, 1, False),
        )

    def test_run_mode_still_requires_real_process_opt_in(self):
        self.assertEqual(
            self.invoke(check_only=False, real_gate=""),
            (2, 0, 1, False),
        )

    def test_run_mode_keeps_existing_dispatch_after_admission(self):
        self.assertEqual(
            self.invoke(check_only=False, real_gate="1"),
            (0, 1, 3, True),
        )


if __name__ == "__main__":
    unittest.main()
