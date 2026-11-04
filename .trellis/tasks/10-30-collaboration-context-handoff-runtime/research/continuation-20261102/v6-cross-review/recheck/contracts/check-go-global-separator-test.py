import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent
WRAPPER = ROOT / "check-bin/go"


class GlobalSeparatorTests(unittest.TestCase):
    def setUp(self):
        ROOT.joinpath(".tmp").mkdir(mode=0o700, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix="global-separator-", dir=ROOT / ".tmp")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.record = self.root / "fake-calls.jsonl"
        self.go = self.root / "fake-go"
        self.go.write_text('''#!/usr/bin/env python3
import json, os, sys
phase = 'list' if 'list' in sys.argv[1:] else 'forward'
with open(os.environ['SEPARATOR_FAKE_RECORD'], 'a') as record:
    record.write(json.dumps({'phase': phase, 'argv': sys.argv[1:], 'pool_max': os.environ.get('DATABASE_MAX_CONNS')}) + '\\n')
if phase == 'list': print('example/first')
''')
        self.go.chmod(0o700)
        self.evidence = self.root / "evidence"
        self.evidence.mkdir()
        self.env = {**os.environ, "MULTICA_TASK_REAL_GO": str(self.go), "SEPARATOR_FAKE_RECORD": str(self.record),
                    "MULTICA_TASK_PACKAGE_DB_MODE": "", "MULTICA_TASK_PACKAGE_DB_EVIDENCE": str(self.evidence),
                    "DATABASE_URL": "", "DATABASE_MAX_CONNS": "10"}

    def invoke(self, argv, **changes):
        return subprocess.run([str(WRAPPER), *argv], env={**self.env, **changes}, capture_output=True, text=True)

    def calls(self):
        return [json.loads(line) for line in self.record.read_text().splitlines()] if self.record.exists() else []

    def test_global_separator_test_requires_isolation_context(self):
        result = self.invoke(["--", "test", "./fixture"])
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(self.calls(), [])

    def test_leading_C_then_global_separator_test_requires_isolation_context(self):
        result = self.invoke(["-C", str(self.root), "--", "test", "./fixture"])
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(self.calls(), [])

    def test_global_separator_real_compile_flag_forwards_exactly(self):
        argv = ["--", "test", "-c", "./fixture"]
        result = self.invoke(argv)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["argv"] for call in self.calls()], [argv])

    def test_global_separator_non_test_version_keeps_raw_forwarding(self):
        argv = ["--", "version"]
        result = self.invoke(argv)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["argv"] for call in self.calls()], [argv])
        self.assertEqual(self.calls()[0]["pool_max"], "10")

    def test_test_local_separator_c_is_not_compile_only(self):
        result = self.invoke(["test", "--", "-c"])
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(self.calls(), [])

    def test_global_separator_recursive_test_resolves_its_recorded_package(self):
        database = "ccheck_20261102010101000000_abcdef123456"
        (self.evidence / (database + ".json")).write_text(json.dumps({
            "database": database, "state": "running", "template": "multica_check_fixture_go", "package": "example/first"
        }))
        argv = ["--", "test", "./fixture"]
        result = self.invoke(argv, MULTICA_TASK_PACKAGE_DB_MODE="0", DATABASE_URL="postgres://127.0.0.1/" + database)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["phase"] for call in self.calls()], ["list", "forward"])
        self.assertEqual(self.calls()[0]["argv"], ["list", "./fixture"])
        self.assertEqual(self.calls()[1]["argv"], argv)
        self.assertIsNone(self.calls()[1]["pool_max"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
