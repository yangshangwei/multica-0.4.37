"""Import and opt-in guards must work without finding a real agent CLI."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

RUNNER = Path(__file__).with_name("runner.py")


def load_runner():
    spec = importlib.util.spec_from_file_location("skill_eval_runner_test", RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class RunnerTests(unittest.TestCase):
    def test_import_has_no_agent_lookup_or_process_execution(self):
        with mock.patch("shutil.which", side_effect=AssertionError("agent lookup during import")), mock.patch("subprocess.Popen", side_effect=AssertionError("process during import")):
            load_runner()

    def test_missing_opt_in_stops_before_agent_lookup(self):
        module = load_runner()
        arguments = ["--cases", "unused", "--skills", "unused", "--inventory", "unused", "--output", "unused", "--phase", "baseline"]
        with mock.patch.dict(os.environ, {}, clear=True), mock.patch("shutil.which", side_effect=AssertionError("agent lookup before opt-in")), contextlib.redirect_stderr(io.StringIO()) as stderr:
            with self.assertRaises(SystemExit) as error:
                module.main(arguments)
        self.assertEqual(error.exception.code, 2)
        self.assertIn("MULTICA_RUN_REAL_AGENT_SMOKE=1", stderr.getvalue())

    def test_complete_skill_tree_is_copied_including_references(self):
        module = load_runner()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source" / "example"
            (source / "references").mkdir(parents=True)
            (source / "SKILL.md").write_text("---\nname: example\ndescription: Example\nuser-invocable: false\n---\nRead references/facts.md.\n")
            (source / "references/facts.md").write_text("Source-backed facts.\n")
            destination = root / "snapshot"
            manifest = module.copy_skills(source.parent, destination)
            self.assertEqual({item["path"] for item in manifest}, {"example/SKILL.md", "example/references/facts.md"})
            for item in manifest:
                self.assertEqual((source.parent / item["path"]).read_bytes(), (destination / item["path"]).read_bytes())

    def test_skill_snapshot_rejects_symlink_escape(self):
        module = load_runner()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "skills" / "example"
            source.mkdir(parents=True)
            (source / "SKILL.md").write_text("example")
            external = root / "outside.txt"
            external.write_text("private")
            (source / "reference.md").symlink_to(external)
            with self.assertRaises(ValueError):
                module.copy_skills(source.parent, root / "snapshot")

    def test_fixture_paths_cannot_escape_or_overwrite_skill_delivery(self):
        module = load_runner()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "workspace"
            root.mkdir()
            for path in ["../outside.txt", "/absolute.txt", ".claude/skills/override/SKILL.md", ".CLAUDE/skills/override/SKILL.md", ".ClAuDe/skills/override/SKILL.md"]:
                with self.subTest(path=path), self.assertRaises(ValueError):
                    module.write_fixture(root, {path: "no"})

    def test_provider_failure_stops_queued_account_calls(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            marker = root / "calls.txt"
            executable = bin_dir / "claude"
            executable.write_text(
                "#!" + sys.executable + "\nimport json, sys\nfrom pathlib import Path\n"
                "if '--version' in sys.argv:\n print('fixture-only-cli'); sys.exit(0)\n"
                "with Path(" + repr(str(marker)) + ").open('a') as stream: stream.write('call\\n')\n"
                "print(json.dumps({'type':'system','subtype':'init','model':'fixture-only','plugins':[],'tools':[]}))\n"
                "print(json.dumps({'type':'result','subtype':'success','is_error':True,'result':'API Error: 402 fixture budget exhausted','total_cost_usd':0}))\n"
                "sys.exit(1)\n"
            )
            executable.chmod(0o755)
            skill = root / "skills/example/SKILL.md"
            skill.parent.mkdir(parents=True)
            skill.write_text("---\nname: example\ndescription: Test\n---\nFixture only.\n")
            cases = root / "cases.json"
            cases.write_text(json.dumps([{"id": name, "prompt": "fixture only", "files": {"task.md": "fixture"}} for name in ["one", "two", "three"]]))
            inventory = root / "inventory.json"
            inventory.write_text('{"skills":[],"plugins":[]}')
            output = root / "output"
            result = subprocess.run(
                [sys.executable, str(RUNNER), "--cases", str(cases), "--skills", str(root / "skills"), "--inventory", str(inventory), "--output", str(output), "--phase", "stop-test", "--concurrency", "1", "--timeout", "30"],
                env={"PATH": str(bin_dir) + os.pathsep + os.defpath, "MULTICA_RUN_REAL_AGENT_SMOKE": "1"},
                capture_output=True, text=True, timeout=15,
            )
            self.assertEqual(result.returncode, 1, "an unsuccessful real-run phase must have a failing process status")
            self.assertEqual(marker.read_text().splitlines(), ["call"], "queued cases must not continue after a native provider failure")
            results = json.loads((output / "stop-test-results.json").read_text())
            self.assertEqual(sum(item.get("native_status") == "not_run" for item in results), 2)


if __name__ == "__main__":
    unittest.main()
