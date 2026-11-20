"""Baseline contracts; the independent verifier adds the missing tenant case."""

import unittest

from workspace_reader import IssueRow, read_issue


class BaselineTests(unittest.TestCase):
    def test_same_workspace_read(self):
        audit = []
        rows = {"issue-one": IssueRow("workspace-one", "Synthetic task one")}
        self.assertEqual(
            read_issue(rows, "issue-one", "workspace-one", audit),
            {"id": "issue-one", "title": "Synthetic task one"},
        )
        self.assertEqual(audit, ["issue-one"])

    def test_missing_issue_has_no_audit(self):
        audit = []
        self.assertIsNone(read_issue({}, "missing", "workspace-one", audit))
        self.assertEqual(audit, [])
