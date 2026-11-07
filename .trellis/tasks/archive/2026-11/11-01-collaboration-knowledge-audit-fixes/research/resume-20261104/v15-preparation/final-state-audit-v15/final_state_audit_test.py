# Execution coordinator: 01a1a3c9-2459-7de1-8244-55640c5f6143; template only, no execution authorization.
"""Credential-free final-state regressions; no repository commands or services."""
import hashlib
import json
import os
from pathlib import Path
import sys
import types
import unittest
from unittest.mock import patch


HERE = Path(__file__).resolve().parent
REGISTERED = Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/final-check-v15")
NEXT_PATH = "apps/web/next-env.d.ts"
NEXT_BEFORE = b'/// <reference types="next" />\nimport "./.next/dev/types/routes.d.ts";\n// generated declaration\n'
NEXT_AFTER = NEXT_BEFORE.replace(b'"./.next/dev/types/routes.d.ts"', b'"./.next/types/routes.d.ts"')


def load(path):
    module = types.ModuleType(path.stem)
    module.__file__ = str(path)
    exec(compile(path.read_bytes(), str(path), "exec"), module.__dict__)
    return module


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class FinalStateTests(unittest.TestCase):
    def setUp(self):
        self.auditor_path = HERE / "final_state_audit.py"
        with patch.dict(os.environ, {"CHECK_LAUNCHER_SOURCE": str(REGISTERED / "run-make-check-v15-provenance.py")}):
            fixture_module = load(REGISTERED / "fake-preflight-v15-provenance.py")
        self.fixture = fixture_module.LauncherEntryTests(methodName="runTest")
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.root, self.out = self.fixture.root, self.fixture.out
        self.next_file = self.root / NEXT_PATH
        self.next_file.parent.mkdir(parents=True, exist_ok=True)
        self.next_file.write_bytes(NEXT_BEFORE)
        self.fixture.source_paths = sorted(self.fixture.source_paths + [NEXT_PATH])
        binding = json.loads(self.fixture.binding_file.read_text())
        rows = [{"path": name, "sha256": digest(self.root / name), "mode": (self.root / name).stat().st_mode & 0o777}
                for name in self.fixture.source_paths]
        binding["source"]["tracked_files"] = rows
        binding["source"]["tracked_fingerprint"] = hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        product = binding["source"]["product_source"]
        manifest_path = Path(product["manifest_path"])
        manifest = json.loads(manifest_path.read_text())
        manifest["sha256"] = {row["path"]: row["sha256"] for row in rows}
        manifest["file_count"] = len(rows)
        manifest["source_id"] = hashlib.sha256("".join(name + "\0" + value + "\n" for name, value in manifest["sha256"].items()).encode()).hexdigest()
        manifest_path.write_text(json.dumps(manifest))
        product.update(source_id=manifest["source_id"], file_count=len(rows), manifest_sha256=digest(manifest_path))
        self.fixture.binding_file.write_text(json.dumps(binding))
        self.fixture.write_clearance()
        self.review = {**json.loads(self.fixture.clearance.read_text()), "candidate_root": str(self.root)}
        self.safe_env = self.out / "check-source-v15.safe.env"
        self.safe_env.write_bytes(self.fixture.source_env.read_bytes())
        log = self.out / "make-check-v15.log"
        log.write_text("Synthetic completion; no repository command executed.\n")
        self.run = {
            "state": "complete", "exit_code": 0, "candidate_root": str(self.root),
            "candidate_sha": self.review["candidate_sha"], "adapter_sha256": self.review["adapter_sha256"],
            "launcher_sha256": self.review["launcher_sha256"], "source_env_sha256": self.review["source_env_sha256"],
            "owned_bindings_sha256": self.review["bindings_sha256"],
            "resource_ownership_sha256": self.review["resource_ownership_sha256"],
            "effective_env_sha256": digest(self.safe_env), "log_sha256": digest(log),
            "canonical_product_source_id": product["source_id"],
            "dev_environment_source_id": binding["source"]["checkout_source_id"],
            "canonical_product_file_count": len(rows), "tracked_snapshot_fingerprint": binding["source"]["tracked_fingerprint"],
            "cli_sha256": binding["cli"]["sha256"],
        }
        self.run_file = self.out / "make-check-v15.json"
        self.run_file.write_text(json.dumps(self.run))

    def audit(self):
        if not self.auditor_path.is_file():
            self.fail("Final-state audit is not implemented")
        auditor = load(self.auditor_path)
        fixture = self.fixture

        def metadata(command, **kwargs):
            if command[:3] == ["git", "rev-parse", "HEAD"]:
                return fixture.head + "\n"
            if command[:3] == ["git", "status", "--porcelain"]:
                return ""
            if command[:3] == ["git", "ls-files", "-z"]:
                return "\0".join(fixture.source_paths).encode() + b"\0"
            if command[:2] == ["git", "show"]:
                self.assertEqual(command[2], self.review["candidate_sha"] + ":" + NEXT_PATH)
                return NEXT_BEFORE
            if command[0] == str(fixture.tool_paths["node"]):
                return json.dumps(fixture.mock_graph)
            raise AssertionError("Unexpected subprocess: " + repr(command))

        with patch("subprocess.check_output", side_effect=metadata), \
                patch("subprocess.run", side_effect=AssertionError("No command execution allowed")), \
                patch("subprocess.Popen", side_effect=AssertionError("No process execution allowed")):
            return auditor.audit(self.root, self.review)

    def test_clean_final_state_passes_identity_audit(self):
        result = self.audit()
        self.assertEqual(result["status"], "pass")
        self.assertFalse(result["next_env"]["generated_rewrite"])
        self.assertEqual(result["canonical_raw_source_id"], self.run["canonical_product_source_id"])

    def test_exact_generated_import_is_recorded_without_writing_source(self):
        self.next_file.write_bytes(NEXT_AFTER)
        result = self.audit()
        self.assertEqual(result["status"], "pass")
        self.assertTrue(result["next_env"]["generated_rewrite"])
        self.assertEqual(result["next_env"]["raw_sha256"], digest(self.next_file))
        self.assertEqual(result["next_env"]["normalized_sha256"], hashlib.sha256(NEXT_BEFORE).hexdigest())
        self.assertNotEqual(result["canonical_raw_source_id"], result["canonical_normalized_source_id"])
        self.assertEqual(self.next_file.read_bytes(), NEXT_AFTER)

    def test_unrelated_tracked_mutation_is_rejected_even_if_git_status_is_clean(self):
        source = self.root / "packages/core/auth/index.ts"
        source.write_text("// changed after Make\n")
        with self.assertRaisesRegex(RuntimeError, "Tracked source changed"):
            self.audit()

    def test_foreign_workspace_link_is_rejected(self):
        foreign = Path(self.fixture.temp.name) / "foreign-core"
        foreign.mkdir()
        self.fixture.workspace_link.unlink()
        self.fixture.workspace_link.symlink_to(foreign, target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, "workspace dependency escapes"):
            self.audit()

    def test_changed_desktop_artifact_is_rejected(self):
        (self.root / "apps/desktop/out/renderer/index.html").write_text("changed renderer")
        with self.assertRaisesRegex(RuntimeError, "desktop artifact changed"):
            self.audit()

    def test_generated_import_plus_extra_declaration_is_rejected(self):
        self.next_file.write_bytes(NEXT_AFTER + b"declare const unrelated: string;\n")
        with self.assertRaisesRegex(RuntimeError, "Next declaration"):
            self.audit()

    def test_next_declaration_symlink_is_rejected(self):
        target = Path(self.fixture.temp.name) / "foreign-next-env"
        target.write_bytes(NEXT_AFTER)
        self.next_file.unlink()
        self.next_file.symlink_to(target)
        with self.assertRaisesRegex(RuntimeError, "Next declaration"):
            self.audit()

    def test_pinned_tool_mutation_is_rejected(self):
        self.fixture.tool_paths["psql"].write_text("changed tool")
        with self.assertRaisesRegex(RuntimeError, "Pinned executable changed"):
            self.audit()

    def test_canonical_execution_environment_mutation_is_rejected(self):
        self.safe_env.write_text("CHANGED=1\n")
        with self.assertRaisesRegex(RuntimeError, "execution environment"):
            self.audit()

    def test_incomplete_run_is_rejected(self):
        self.run["state"] = "running"
        self.run_file.write_text(json.dumps(self.run))
        with self.assertRaisesRegex(RuntimeError, "complete run"):
            self.audit()

    def test_changed_development_environment_source_identity_is_rejected(self):
        self.run["dev_environment_source_id"] = "changed-after-Make"
        self.run_file.write_text(json.dumps(self.run))
        with self.assertRaisesRegex(RuntimeError, "Complete run identity differs"):
            self.audit()

    def test_failed_whole_command_remains_failed_after_identity_pass(self):
        self.run["exit_code"] = 2
        self.run_file.write_text(json.dumps(self.run))
        result = self.audit()
        self.assertEqual(result["status"], "pass")
        self.assertEqual(result["wholecheck_exit_code"], 2)
        self.assertFalse(result["wholecheck_passed"])

    def test_generated_import_with_changed_file_mode_is_rejected(self):
        self.next_file.write_bytes(NEXT_AFTER)
        self.next_file.chmod((self.next_file.stat().st_mode & 0o777) ^ 0o100)
        with self.assertRaisesRegex(RuntimeError, "Tracked source changed"):
            self.audit()

    def test_completed_run_log_mutation_is_rejected(self):
        (self.out / "make-check-v15.log").write_text("changed after Make\n")
        with self.assertRaisesRegex(RuntimeError, "Complete run log changed"):
            self.audit()

    def test_missing_review_retains_a_failure_report(self):
        auditor = load(self.auditor_path)
        reports = Path(self.fixture.temp.name) / "audit-reports"
        reports.mkdir()
        output = reports / "missing-review.json"
        with patch.object(auditor, "HERE", reports), patch.object(auditor, "REVIEW", reports / "absent-review.json"), \
                patch.object(sys, "argv", [str(self.auditor_path), "--output", str(output)]), \
                patch.object(auditor, "audit", side_effect=AssertionError("Missing review must prevent candidate audit")):
            try:
                code = auditor.main()
            except FileNotFoundError:
                self.fail("Missing review must retain an explicit JSON failure report")
        self.assertEqual(code, 2)
        self.assertEqual(json.loads(output.read_text())["status"], "fail")
        self.assertEqual(output.stat().st_mode & 0o777, 0o600)

    def test_existing_report_is_never_overwritten(self):
        auditor = load(self.auditor_path)
        reports = Path(self.fixture.temp.name) / "audit-reports"
        reports.mkdir()
        output = reports / "retained.json"
        output.write_text("previous evidence")
        with patch.object(auditor, "HERE", reports), \
                patch.object(sys, "argv", [str(self.auditor_path), "--output", str(output)]), \
                patch.object(auditor, "audit", side_effect=AssertionError("Existing evidence must prevent candidate audit")):
            with self.assertRaises(FileExistsError):
                auditor.main()
        self.assertEqual(output.read_text(), "previous evidence")


if __name__ == "__main__":
    unittest.main(verbosity=2)
