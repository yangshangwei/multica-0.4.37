import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import shlex
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch


CANDIDATE = "64a9994ec549e8dff73ab14456dc9a9f4b63f7c7"
REAL_ROOT = "/Volumes/artisan/code/2026/mc-collaboration-final4-20261102"
REAL_ADAPTER_BASE = "/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101"
SOURCE = Path(os.environ.get("CHECK_LAUNCHER_SOURCE", REAL_ROOT + "/.omx/private/final-candidate/run-make-check-v6.py"))


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class LauncherEntryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="multica-launcher-contract-")
        self.addCleanup(self.temp.cleanup)
        self.root = (Path(self.temp.name) / "candidate").resolve()
        self.out = self.root / ".omx/private/final-candidate"
        self.out.mkdir(parents=True)
        self.adapters = (Path(self.temp.name) / "check-bin").resolve()
        self.adapters.mkdir()
        self.build_temp = (Path(self.temp.name) / "build-temp").resolve()
        self.build_temp.mkdir(mode=0o700)
        version = "adapter-v6" if SOURCE.name.endswith("-v6.py") else "adapter-v5"
        for name in ("go", "pnpm"):
            shutil.copy2(Path(REAL_ADAPTER_BASE) / version / "check-bin" / name, self.adapters / name)
        # Only filesystem locations change. All execution and refusal logic is
        # the original entrypoint, and no actual make/database process is run.
        source = SOURCE.read_text().replace(REAL_ROOT, str(self.root))
        for item in ("adapter-v5", "adapter-v6"):
            source = source.replace(REAL_ADAPTER_BASE + "/" + item + "/check-bin", str(self.adapters))
        source = source.replace("/private/tmp/multica-final-check4-yxg6alb4", str(self.build_temp))
        source = source.replace("/private/tmp/multica-final-check4-", str(self.build_temp.parent) + "/build-")
        self.launcher = self.out / SOURCE.name
        self.launcher.write_text(source)
        self.renderer = "http://127.0.0.1:19845/" if version == "adapter-v6" else "http://127.0.0.1:19844"
        self.source_env = self.out / "check-source.env"
        self.source_env.write_text("\n".join([
            "PATH=/usr/bin:/bin", "DATABASE_URL=postgres://fixture@127.0.0.1/task_seed",
            "MULTICA_LLM_API_KEY=", "MULTICA_LLM_BASE_URL=",
            "COLLABORATION_ELECTRON_RENDERER_URL=" + self.renderer,
            "CHANGELOG_ELECTRON_RENDERER_URL=" + self.renderer,
            "DATABASE_MAX_CONNS=10", "DATABASE_MIN_CONNS=1", "REDIS_TEST_URL=redis://127.0.0.1:16422/0",
        ]) + "\n")
        (self.out / "build-temp-allocation.json").write_text(json.dumps({"directory": str(self.build_temp)}))
        self.clearance = self.out / ("independent-review-clearance-v6.json" if version == "adapter-v6" else "independent-review-clearance.json")
        self.write_clearance()
        self.head = CANDIDATE
        self.launches = []
        self.bash_source_calls = 0

    def write_clearance(self, **overrides):
        clearance = {
            "verdict": "pass-with-notes", "blocking_findings_open": 0, "candidate_sha": CANDIDATE,
            "adapter_sha256": {name: digest(self.adapters / name) for name in ("go", "pnpm")},
            "launcher_sha256": digest(self.launcher), "source_env_sha256": digest(self.source_env),
        }
        clearance.update(overrides)
        self.clearance.write_text(json.dumps(clearance))

    def run_entry(self):
        def read_only_output(command, **kwargs):
            if command[:3] == ["git", "rev-parse", "HEAD"]:
                return self.head + "\n"
            if command[:3] == ["git", "status", "--porcelain"]:
                return ""
            # Spy on the old unsafe shell-loading path without executing it.
            # The fixed launcher reads the same source bytes as data directly.
            if command[:2] == ["bash", "-c"]:
                self.bash_source_calls += 1
                values = {**os.environ, "PATH": "/usr/bin:/bin", "MULTICA_LLM_API_KEY": "", "MULTICA_LLM_BASE_URL": "",
                          "COLLABORATION_ELECTRON_RENDERER_URL": self.renderer,
                          "CHANGELOG_ELECTRON_RENDERER_URL": self.renderer}
                return b"\0".join((key + "=" + value).encode() for key, value in values.items())
            raise AssertionError("Unexpected command before fake make launch")

        def read_only_run(command, **kwargs):
            if command[:2] == ["git", "-C"] and command[-2:] == ["rev-parse", "--show-toplevel"]:
                return subprocess.CompletedProcess(command, 1, b"", b"")
            raise AssertionError("Unexpected subprocess.run")

        launches = self.launches

        class FakeMake:
            pid = 987654

            def __init__(self, command, **kwargs):
                launches.append({"command": command, "env": kwargs["env"]})
                kwargs["stdout"].write("Synthetic make-check completion; no command was executed.\n")

            def wait(self):
                return 0

        code = 0
        text = io.StringIO()
        with contextlib.redirect_stdout(text), contextlib.redirect_stderr(text), \
                patch("subprocess.check_output", side_effect=read_only_output), \
                patch("subprocess.run", side_effect=read_only_run), patch("subprocess.Popen", FakeMake):
            try:
                runpy.run_path(str(self.launcher), run_name="__main__")
            except SystemExit as error:
                code = error.code
            except (AssertionError, RuntimeError, ValueError) as error:
                code = 2
                text.write(type(error).__name__ + ": " + str(error))
        return code, text.getvalue()

    def assert_refused_before_make(self):
        code, output = self.run_entry()
        self.assertEqual(code, 2, output)
        self.assertEqual(self.launches, [], "Identity/configuration refusal must precede make/database startup")

    def test_tampered_go_adapter_is_refused_before_make(self):
        with (self.adapters / "go").open("a") as file:
            file.write("\n# changed after independent review\n")
        self.assert_refused_before_make()

    def test_tampered_pnpm_adapter_is_refused_before_make(self):
        with (self.adapters / "pnpm").open("a") as file:
            file.write("\n# changed after independent review\n")
        self.assert_refused_before_make()

    def test_tampered_launcher_is_refused_before_make(self):
        with self.launcher.open("a") as file:
            file.write("\n# changed after independent review\n")
        self.assert_refused_before_make()

    def test_tampered_source_config_is_refused_before_make(self):
        with self.source_env.open("a") as file:
            file.write("EXTRA_FIXTURE_SETTING=1\n")
        self.assert_refused_before_make()

    def test_candidate_mismatch_is_refused_before_make(self):
        self.head = "f" * 40
        self.assert_refused_before_make()

    def test_command_substitution_is_not_executed(self):
        marker = Path(self.temp.name) / "payload-was-executed"
        with self.source_env.open("a") as file:
            file.write("PAYLOAD=$(printf touched > " + shlex.quote(str(marker)) + ")\n")
        self.write_clearance()
        self.assert_refused_before_make()
        self.assertFalse(marker.exists(), "Dotenv is data; command substitution must never execute")
        self.assertEqual(self.bash_source_calls, 0)

    def test_backtick_substitution_is_not_executed(self):
        marker = Path(self.temp.name) / "backtick-was-executed"
        with self.source_env.open("a") as file:
            file.write("PAYLOAD=`printf touched > " + shlex.quote(str(marker)) + "`\n")
        self.write_clearance()
        self.assert_refused_before_make()
        self.assertFalse(marker.exists(), "Dotenv backticks must never execute")
        self.assertEqual(self.bash_source_calls, 0)

    def test_blocking_review_cannot_launch(self):
        self.write_clearance(blocking_findings_open=1)
        self.assert_refused_before_make()

    def test_missing_review_cannot_launch(self):
        self.clearance.unlink()
        self.assert_refused_before_make()

    def test_matching_review_preserves_api_pool_and_dynamic_renderer(self):
        code, output = self.run_entry()
        self.assertEqual(code, 0, output)
        self.assertEqual(len(self.launches), 1)
        env = self.launches[0]["env"]
        self.assertEqual(env["DATABASE_MAX_CONNS"], "10")
        self.assertEqual(env["DATABASE_MIN_CONNS"], "1")
        self.assertEqual(env["MULTICA_TASK_RENDERER_URL"], self.renderer)
        self.assertEqual(env["COLLABORATION_ELECTRON_RENDERER_URL"], self.renderer)
        self.assertEqual(env["CHANGELOG_ELECTRON_RENDERER_URL"], self.renderer)
        self.assertEqual(env["REDIS_TEST_URL"], "redis://127.0.0.1:16422/0")

    def test_generated_quoted_escaped_empty_and_duplicate_values_are_data(self):
        with self.source_env.open("a") as file:
            file.write("FIXTURE='old value'\nFIXTURE=new\\ value\nEMPTY=\nQUOTED=\"plain value\"\n")
        self.write_clearance()
        code, output = self.run_entry()
        self.assertEqual(code, 0, output)
        self.assertEqual(self.launches[0]["env"]["FIXTURE"], "new value")
        self.assertEqual(self.launches[0]["env"]["EMPTY"], "")
        self.assertEqual(self.launches[0]["env"]["QUOTED"], "plain value")

    def test_non_loopback_renderer_is_refused_before_make(self):
        self.source_env.write_text(self.source_env.read_text().replace(self.renderer, "http://example.invalid:19845/"))
        self.write_clearance()
        self.assert_refused_before_make()

    def test_credentialed_renderer_is_refused_before_make(self):
        self.source_env.write_text(self.source_env.read_text().replace(self.renderer, "http://fixture:fixture@127.0.0.1:19845/"))
        self.write_clearance()
        self.assert_refused_before_make()


if __name__ == "__main__":
    unittest.main(verbosity=2)
