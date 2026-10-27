"""SYNTHETIC ONLY: deliberately incomplete workspace-scoped read fixture."""

from collections.abc import Mapping
from dataclasses import dataclass


@dataclass(frozen=True)
class IssueRow:
    workspace_id: str
    title: str


def read_issue(
    rows: Mapping[str, IssueRow],
    issue_id: str,
    workspace_id: str,
    audit: list[str],
) -> dict[str, str] | None:
    row = rows.get(issue_id)
    if row is None:
        return None
    audit.append(issue_id)
    return {"id": issue_id, "title": row.title}
