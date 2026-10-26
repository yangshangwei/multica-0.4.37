"""Ordinary in-memory rows exercise workspace isolation without a server."""

import unittest

from workspace_reader import IssueRow, read_issue


class AuthorizationContractTests(unittest.TestCase):
    def setUp(self) -> None:
        self.rows = {
            "issue-one": IssueRow("workspace-one", "Synthetic task one"),
            "issue-two": IssueRow("workspace-two", "Synthetic task two"),
        }
        self.audit: list[str] = []

    def test_current_workspace_can_read_its_row(self) -> None:
        self.assertEqual(
            read_issue(self.rows, "issue-one", "workspace-one", self.audit),
            {"id": "issue-one", "title": "Synthetic task one"},
        )
        self.assertEqual(self.audit, ["issue-one"])

    def test_other_workspace_is_denied_before_read_audit(self) -> None:
        result = read_issue(self.rows, "issue-two", "workspace-one", self.audit)
        self.assertEqual((result, self.audit), (None, []))

    def test_unknown_issue_has_no_read_audit(self) -> None:
        result = read_issue(self.rows, "missing", "workspace-one", self.audit)
        self.assertEqual((result, self.audit), (None, []))


if __name__ == "__main__":
    unittest.main()
