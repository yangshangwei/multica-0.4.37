"""Connection-free launcher contracts using only owned fake Git/Make processes."""
import hashlib
import json
import os
from pathlib import Path
import runpy
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).with_name("run-make-check.py")
MODULE = runpy.run_path(str(SOURCE), run_name="launcher_contract_module")


class EnvironmentParserTests(unittest.TestCase):
    def test_quoted_values_are_literal_and_keep_spaces_unicode_and_substitution(self):
        text = "export FIRST='a b # 中文'\nSECOND=\"$(printf unused) `printf unused`\"\nEMPTY=''\n"
        self.assertEqual(MODULE["parse_environment"](text),
                         {"FIRST": "a b # 中文", "SECOND": "$(printf unused) `printf unused`", "EMPTY": ""})

    def test_unsupported_shell_statements_fail_without_evaluation(self):
        for text in ("unset KEY", "OK=literal\nprintf unused", "export 1BAD=x", "BROKEN='missing"):
            with self.assertRaises(RuntimeError):
                MODULE["parse_environment"](text)

    def test_canonical_copy_remains_literal_when_downstream_shell_loads_it(self):
        with tempfile.TemporaryDirectory(prefix="multica-literal-env-") as temp:
            path = Path(temp) / "canonical.env"
            value = "$(printf expanded) `printf expanded` 中文\nsecond line"
            MODULE["write_canonical_environment"](path, {"LITERAL": value})
            command = ["/bin/bash", "-c", 'set -a; . "$1"; printf "%s" "$LITERAL"', "owned-fixture", str(path)]
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, value)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(FileExistsError):
                MODULE["write_canonical_environment"](path, {"LITERAL": "overwrite"})


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="multica-launcher-contract-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.out = self.root / ".omx/private/final-candidate"
        self.out.mkdir(parents=True)
        self.launcher = self.out / "run-make-check.py"
        shutil.copy2(SOURCE, self.launcher)
        self.adapters = self.root / "adapters"
        self.adapters.mkdir()
        for name in ("go", "pnpm"):
            (self.adapters / name).write_text("reviewed-owned-fake-adapter-" + name)
        self.home = self.out / "test-home"
        self.home.mkdir(mode=0o700)
        self.build_temp = self.root / "build-temp"
        self.build_temp.mkdir(mode=0o700)
        self.allocation = self.out / "build-temp-allocation.json"
        self.allocation.write_text(json.dumps({"directory": str(self.build_temp), "owner_uid": os.geteuid(), "mode": "0700"}))
        self.source = self.out / "check-source-input.env"
        self.literal = "$(printf should_not_expand) `printf should_not_expand` 中文"
        self.fake_bin = self.root / "bin"
        self.fake_bin.mkdir()
        values = {"SAFE_LITERAL": self.literal, "PATH": str(self.fake_bin) + ":" + os.environ["PATH"],
                  "DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1",
                  "COLLABORATION_ELECTRON_RENDERER_URL": "http://127.0.0.1:19844",
                  "CHANGELOG_ELECTRON_RENDERER_URL": "http://127.0.0.1:19844",
                  "MULTICA_LLM_API_KEY": "", "MULTICA_LLM_BASE_URL": ""}
        self.source.write_text("\n".join(f"{key}={shlex.quote(value)}" for key, value in values.items()) + "\n")
        self.source.chmod(0o600)
        self.sha = "fixture-candidate-sha"
        git = self.fake_bin / "git"
        git.write_text('''#!/usr/bin/env python3
import os, sys
args = sys.argv[1:]
if args == ['rev-parse', 'HEAD']: print(os.environ['CHECK_FAKE_SHA'])
elif args == ['status', '--porcelain']:
 if os.environ.get('CHECK_FAKE_DIRTY') == '1': print(' M fixture-product-file')
else: sys.exit(1)
''')
        make = self.fake_bin / "make"
        make.write_text('''#!/usr/bin/env python3
import json, os, subprocess, sys
path = sys.argv[-1].split('=', 1)[1]
command = ['/bin/bash', '-c', r'set -a; . "$1"; printf "%s\\0%s\\0%s\\0%s" "$SAFE_LITERAL" "$DATABASE_MAX_CONNS" "$DATABASE_MIN_CONNS" "$HOME"', 'owned-downstream-loader', path]
result = subprocess.run(command, capture_output=True)
if result.returncode: sys.exit(result.returncode)
literal, maximum, minimum, home = result.stdout.decode().split('\\0')
with open(os.environ['CHECK_FAKE_MAKE_RECORD'], 'w') as sink:
 json.dump({'arguments': sys.argv[1:], 'literal': literal, 'max': maximum, 'min': minimum, 'home': home}, sink)
print('owned fake make executed; no repository tests or services were started')
sys.exit(int(os.environ.get('CHECK_FAKE_MAKE_EXIT', '0')))
''')
        git.chmod(0o700)
        make.chmod(0o700)
        self.called = self.root / "fake-make-call.json"
        self.env = {**os.environ, "PATH": str(self.fake_bin) + ":" + os.environ["PATH"], "CHECK_FAKE_SHA": self.sha,
                    "CHECK_FAKE_MAKE_RECORD": str(self.called), "PYTHONDONTWRITEBYTECODE": "1"}
        self.env.pop("MULTICA_LLM_API_KEY", None)
        self.env.pop("MULTICA_LLM_BASE_URL", None)
        self.config = {"candidate_root": str(self.root), "candidate_sha": self.sha,
                       "adapters_root": str(self.adapters), "adapter_sha256": {name: self.hash(self.adapters / name) for name in ("go", "pnpm")},
                       "launcher_sha256": self.hash(self.launcher), "source_env": str(self.source), "source_env_sha256": self.hash(self.source),
                       "build_temp_allocation": str(self.allocation), "renderer_url": "http://127.0.0.1:19844",
                       "task_home": str(self.home), "go_cache": str(self.root / "cache"), "go_mod_cache": str(self.root / "modules"),
                       "go_path": str(self.root / "go-path"), "playwright_browsers_path": str(self.root / "browsers"),
                       "real_go": str(self.adapters / "go"), "real_pnpm": str(self.adapters / "pnpm"), "psql": "unused-fake-path"}
        (self.out / "launcher-config.json").write_text(json.dumps(self.config))
        self.clearance = {"verdict": "pass", "blocking_findings_open": 0,
                          "adapter_sha256": self.config["adapter_sha256"], "launcher_sha256": self.config["launcher_sha256"]}
        (self.out / "independent-review-clearance.json").write_text(json.dumps(self.clearance))

    @staticmethod
    def hash(path):
        return hashlib.sha256(path.read_bytes()).hexdigest()

    def launch(self, **env):
        return subprocess.run([sys.executable, str(self.launcher)], env={**self.env, **env}, capture_output=True, text=True)

    def assert_refused(self, result):
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.called.exists())

    def test_fake_launch_preserves_literal_downstream_input_and_api_budget(self):
        before = self.source.read_bytes()
        result = self.launch()
        self.assertEqual(result.returncode, 0, result.stderr)
        call = json.loads(self.called.read_text())
        self.assertEqual(call["literal"], self.literal)
        self.assertEqual((call["max"], call["min"], call["home"]), ("10", "1", str(self.home.resolve())))
        self.assertIn("check-source.canonical.env", call["arguments"][-1])
        self.assertEqual(self.source.read_bytes(), before)
        record = json.loads((self.out / "make-check.json").read_text())
        self.assertEqual(record["exit_code"], 0)
        self.assertEqual(record["adapter_sha256"], self.config["adapter_sha256"])
        self.assertEqual(record["source_env_sha256"], self.config["source_env_sha256"])
        self.assertFalse(record["real_provider_allowed"])

    def test_actual_changed_adapter_is_refused_before_make(self):
        (self.adapters / "go").write_text("changed after review")
        self.assert_refused(self.launch())

    def test_actual_changed_launcher_is_refused_before_make(self):
        with self.launcher.open("a") as sink:
            sink.write("\n# changed after review\n")
        self.assert_refused(self.launch())

    def test_actual_changed_source_is_refused_before_make(self):
        with self.source.open("a") as sink:
            sink.write("ADDED=literal\n")
        self.assert_refused(self.launch())

    def test_missing_clearance_is_refused_before_make(self):
        (self.out / "independent-review-clearance.json").unlink()
        self.assert_refused(self.launch())

    def test_dirty_candidate_and_nonprivate_home_are_refused_before_make(self):
        self.assert_refused(self.launch(CHECK_FAKE_DIRTY="1"))
        self.home.chmod(0o755)
        self.assert_refused(self.launch())

    def test_original_check_record_is_not_overwritten(self):
        path = self.out / "make-check.json"
        path.write_text("original evidence")
        self.assert_refused(self.launch())
        self.assertEqual(path.read_text(), "original evidence")

    def test_fake_make_failure_retains_real_exit_code(self):
        result = self.launch(CHECK_FAKE_MAKE_EXIT="29")
        self.assertEqual(result.returncode, 29, result.stderr)
        self.assertEqual(json.loads((self.out / "make-check.json").read_text())["exit_code"], 29)


if __name__ == "__main__":
    unittest.main(verbosity=2)
