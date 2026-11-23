// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

const child = { id: "00000000-0000-4000-8000-000000000002", identifier: "SRC-2", title: "Ordinary child", can_open: false };
const external = { items: [child], source_workspace_slug: null };
afterEach(() => setSchemaLogger(noopLogger));

describe("ordinary collaboration child references", () => {
  it("retains the complete ordered child list and the current source-member route", () => {
    const items = Array.from({ length: 205 }, (_, index) => ({ ...child,
      id: `00000000-0000-4000-8000-${(index + 2).toString(16).padStart(12, "0")}`,
      identifier: `SRC-${index + 2}`, can_open: true,
    }));
    const ordinary_children = { items, source_workspace_slug: "current-source" };
    // Opening is source membership, not creator/handler source-edit authority.
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, ordinary_children })?.ordinary_children).toEqual(ordinary_children);
  });

  it("exposes only minimal unopenable external references, distinguishing old-server absence from an empty list", () => {
    const parsed = parseCollaborationIssue({ ...collaborationIssueFixture,
      ordinary_children: { ...external, items: [{ ...child, description: "PRIVATE_CHILD", workspace_id: child.id, status: "todo", assignee: "PRIVATE_ACCOUNT" }] },
    });
    expect(parsed?.ordinary_children).toEqual(external);
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE_CHILD");
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE_ACCOUNT");
    expect(parseCollaborationIssue(collaborationIssueFixture)?.ordinary_children).toBeUndefined();
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, ordinary_children: { ...external, items: [] } })?.ordinary_children?.items).toEqual([]);
  });

  it.each([
    null,
    {},
    { ...external, items: null },
    { ...external, items: [child, child] },
    { ...external, items: [{ ...child, id: "not-a-uuid" }] },
    { ...external, items: [{ ...child, id: collaborationIssueFixture.id }] },
    { ...external, items: [{ ...child, identifier: "" }] },
    { ...external, items: [{ ...child, title: null }] },
    { ...external, items: [{ ...child, can_open: undefined }] },
    { ...external, items: [{ ...child, can_open: "true" }] },
    { ...external, items: [{ ...child, can_open: true }] },
    { ...external, source_workspace_slug: "source" },
    { ...external, source_workspace_slug: "source", items: [child, { ...child, id: "00000000-0000-4000-8000-000000000003", can_open: true }] },
    ...["", "../other", "source/issues", "source?next=other", "https://example.com", "source%2Fother", "account"].map((source_workspace_slug) => ({ items: [{ ...child, can_open: true }], source_workspace_slug })),
  ])("fails closed for malformed or inconsistent child authority: %j", (ordinary_children) => {
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, ordinary_children })).toBeNull();
  });

  it("redacts malformed child response values from schema logs", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationIssue({ ...collaborationIssueFixture, ordinary_children: {
      ...external, items: [{ ...child, title: { private: "PRIVATE_CHILD_SENTINEL" } }],
    } })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_CHILD_SENTINEL");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
