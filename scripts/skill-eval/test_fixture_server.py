"""Black-box stdlib tests; no installed agent or model account is used."""
import hashlib
import json
import os
from pathlib import Path
import selectors
import subprocess
import sys
import tempfile
import unittest

SERVER = Path(__file__).with_name("fixture_server.py")


class FixtureServerTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="skill-eval-test-")
        base = Path(self.directory.name)
        self.root = base / "workspace"
        self.root.mkdir()
        self.evidence = base / "evidence"
        self.evidence.mkdir()
        self.outside = base / "outside.txt"
        self.outside.write_text("outside-private-canary")
        checks = base / "checks.json"
        checks.write_text(json.dumps({"checks": {}, "frozen_execution_inputs": {}, "sandbox_profile": "unused"}))
        self.process = subprocess.Popen(
            [sys.executable, str(SERVER), "--root", str(self.root), "--evidence", str(self.evidence), "--checks", str(checks)],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, bufsize=1, env={"PATH": os.defpath},
        )
        self.selector = selectors.DefaultSelector()
        self.selector.register(self.process.stdout, selectors.EVENT_READ)
        self.number = 0
        self.rpc("initialize", {"protocolVersion": "2024-11-05"})

    def tearDown(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=3)
        self.selector.close()
        stderr = self.process.stderr.read()
        self.process.stdout.close()
        self.process.stderr.close()
        self.directory.cleanup()
        self.assertEqual(stderr, "")

    def rpc(self, method, params):
        self.number += 1
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "id": self.number, "method": method, "params": params}) + "\n")
        self.process.stdin.flush()
        self.assertTrue(self.selector.select(timeout=4), "fixture MCP did not respond")
        line = self.process.stdout.readline()
        self.assertTrue(line, "fixture MCP exited before responding")
        result = json.loads(line)
        self.assertEqual(result["id"], self.number)
        self.assertNotIn("error", result)
        return result["result"]

    def call(self, tool, arguments, *, error=False):
        response = self.rpc("tools/call", {"name": tool, "arguments": arguments})
        self.assertEqual(response["isError"], error, response)
        return json.loads(response["content"][0]["text"])

    def file(self, relative, content):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def search(self, query, paths=None, **limits):
        return self.call("search", {"query": query, "paths": paths or ["."], **limits})

    def test_existing_read_write_and_command_boundary(self):
        self.file("docs/current.md", "before\n")
        self.call("write_file", {"path": "docs/current.md", "content": "after\n"})
        self.assertEqual(self.call("read_file", {"path": "docs/current.md"})["text"], "after\n")
        self.call("read_file", {"path": str(self.outside)}, error=True)
        self.call("write_file", {"path": str(self.outside), "content": "no"}, error=True)
        self.call("run_check", {"name": "arbitrary-command"}, error=True)
        self.assertEqual(self.outside.read_text(), "outside-private-canary")

    def test_search_is_advertised_as_read_only_literal(self):
        tools = self.rpc("tools/list", {})["tools"]
        search = next((tool for tool in tools if tool["name"] == "search"), None)
        self.assertIsNotNone(search, "the literal search tool is missing")
        self.assertTrue(search["annotations"]["readOnlyHint"])
        self.assertIn("literal", search["description"].lower())

    def test_case_aliases_cannot_modify_the_original_skill_tree(self):
        original = self.file(".claude/skills/example/SKILL.md", "immutable skill\n")
        expected = hashlib.sha256(original.read_bytes()).hexdigest()
        for alias in [".CLAUDE", ".ClAuDe"]:
            with self.subTest(alias=alias):
                response = self.rpc("tools/call", {"name": "write_file", "arguments": {"path": alias + "/skills/example/SKILL.md", "content": "overwrite"}})
                self.assertEqual(hashlib.sha256(original.read_bytes()).hexdigest(), expected, "reserved directory aliases must preserve the original skill bytes")
                self.assertTrue(response["isError"], "case aliases of .claude must be rejected on every filesystem")

    def test_literal_matching_returns_actual_path_line_text_and_hash(self):
        path = self.file("docs/notes.md", "axb0\nA.B[0]\na.b[0] is literal\n")
        result = self.search("a.b[0]")
        self.assertEqual(len(result["matches"]), 1)
        match = result["matches"][0]
        self.assertEqual((match["path"], match["line"], match["text"]), ("docs/notes.md", 3, "a.b[0] is literal"))
        self.assertEqual(match["sha256"], hashlib.sha256(path.read_bytes()).hexdigest())
        self.assertEqual(result["scope"]["paths"], ["."])
        self.assertEqual(result["match_mode"], "literal")
        self.assertTrue(result["case_sensitive"])
        self.assertTrue(result["complete"])
        self.assertFalse(result["truncated"])

    def test_absence_is_complete_only_for_the_declared_scope(self):
        self.file("docs/current.md", "new flag\n")
        self.file("history.md", "old flag\n")
        result = self.search("old flag", ["docs"])
        self.assertEqual(result["matches"], [])
        self.assertEqual(result["scope"]["paths"], ["docs"])
        self.assertEqual([f["path"] for f in result["files"]], ["docs/current.md"])
        self.assertTrue(result["complete"])

    def test_search_after_edit_preserves_legitimate_history_and_test_mentions(self):
        self.file("docs/current.md", "Run --timeout 30.\n")
        history = self.file("docs/history.md", "Version 1 used --timeout.\n")
        tests = self.file("tests/flags.txt", "Reject --timeout with unknown-flag.\n")
        before = self.search("--timeout")
        self.assertEqual(len(before["matches"]), 3)
        self.call("write_file", {"path": "docs/current.md", "content": "Run --wait-seconds 45.\n"})
        after = self.search("--timeout")
        self.assertEqual({m["path"] for m in after["matches"]}, {"docs/history.md", "tests/flags.txt"})
        current = next(f for f in after["files"] if f["path"] == "docs/current.md")
        self.assertEqual(current["sha256"], hashlib.sha256((self.root / current["path"]).read_bytes()).hexdigest())
        self.assertTrue(after["complete"])
        self.assertEqual(history.read_text(), "Version 1 used --timeout.\n")
        self.assertEqual(tests.read_text(), "Reject --timeout with unknown-flag.\n")

    def test_overlapping_scopes_do_not_duplicate_matches(self):
        self.file("docs/a.md", "target\n")
        result = self.search("target", ["docs", "docs/a.md"])
        self.assertEqual(len(result["matches"]), 1)
        self.assertEqual(result["files_scanned"], 1)

    def test_case_or_hardlink_aliases_count_one_file_identity(self):
        original = self.file("docs/a.md", "target\n")
        alias = self.root / "DOCS/a.md"
        if not alias.exists():
            alias.parent.mkdir()
            os.link(original, alias)
        self.assertEqual((original.stat().st_dev, original.stat().st_ino), (alias.stat().st_dev, alias.stat().st_ino))
        result = self.search("target", ["docs", "DOCS"])
        self.assertEqual(len(result["matches"]), 1)
        self.assertEqual(result["files_scanned"], 1)
        self.assertEqual(result["matched_lines"], 1)
        self.assertEqual(result["bytes_scanned"], len(original.read_bytes()))
        limited = self.search("target", ["docs", "DOCS"], max_files=1)
        self.assertTrue(limited["complete"], "a second spelling of the same file must not exhaust the file limit")

    def test_distinct_case_sensitive_files_are_not_merged(self):
        self.file("docs/a.md", "target lower\n")
        alias = self.root / "DOCS/a.md"
        if alias.exists():
            self.skipTest("this filesystem is case-insensitive; the real case-alias path is covered separately")
        self.file("DOCS/a.md", "target upper\n")
        result = self.search("target", ["docs", "DOCS"])
        self.assertEqual({match["text"] for match in result["matches"]}, {"target lower", "target upper"})
        self.assertEqual(result["files_scanned"], 2)

    def test_result_limit_is_explicit_without_inventing_complete_coverage(self):
        self.file("docs/a.md", "target one\ntarget two\n")
        result = self.search("target", max_results=1)
        self.assertEqual(len(result["matches"]), 1)
        self.assertTrue(result["truncated"])
        self.assertFalse(result["complete"])
        self.assertIn("max_results", result["limit_reasons"])
        self.assertEqual(result["limits"]["max_results"], 1)
        self.assertEqual(result["matched_lines"], 2)

    def test_exact_result_limit_does_not_claim_an_unseen_extra_match(self):
        self.file("docs/a.md", "target\nother\n")
        result = self.search("target", max_results=1)
        self.assertEqual(len(result["matches"]), 1)
        self.assertTrue(result["complete"])
        self.assertFalse(result["truncated"])

    def test_file_limit_is_reported(self):
        self.file("a.md", "target\n")
        self.file("b.md", "target\n")
        result = self.search("target", max_files=1)
        self.assertEqual(result["files_scanned"], 1)
        self.assertFalse(result["complete"])
        self.assertTrue(result["truncated"])
        self.assertIn("max_files", result["limit_reasons"])

    def test_byte_limit_cannot_turn_an_unread_file_into_a_clean_search(self):
        self.file("a.md", "target\n")
        result = self.search("target", max_bytes=1)
        self.assertEqual(result["matches"], [])
        self.assertFalse(result["complete"])
        self.assertTrue(result["truncated"])
        self.assertIn("max_bytes", result["limit_reasons"])
        self.assertEqual(result["skipped_files"][0]["path"], "a.md")

    def test_invalid_paths_traversals_and_symlink_escapes_are_rejected(self):
        self.file("docs/a.md", "target\n")
        (self.root / "linked.txt").symlink_to(self.outside)
        (self.root / "linked-dir").symlink_to(self.outside.parent, target_is_directory=True)
        for path in [str(self.outside), "../outside.txt", "docs/../docs/a.md", "linked.txt", "linked-dir", "."]:
            with self.subTest(path=path):
                result = self.call("search", {"query": "canary", "paths": [path]}, error=True)
                self.assertNotIn("outside-private-canary", json.dumps(result))

    def test_invalid_search_arguments_are_rejected(self):
        self.file("a.md", "target\n")
        arguments = [
            {"query": ""}, {"query": "a\nb"}, {"query": 2},
            {"query": "target", "paths": []}, {"query": "target", "paths": "a.md"},
            {"query": "target", "max_results": 0}, {"query": "target", "max_files": True},
            {"query": "target", "max_bytes": 1000001}, {"query": "target", "paths": ["missing"]},
        ]
        for argument in arguments:
            with self.subTest(argument=argument):
                self.call("search", argument, error=True)

    def test_non_utf8_file_is_an_explicit_gap(self):
        (self.root / "binary.bin").write_bytes(b"\xff\xfe")
        result = self.search("target")
        self.assertFalse(result["complete"])
        self.assertEqual(result["skipped_files"][0]["reason"], "non_utf8")


if __name__ == "__main__":
    unittest.main()
