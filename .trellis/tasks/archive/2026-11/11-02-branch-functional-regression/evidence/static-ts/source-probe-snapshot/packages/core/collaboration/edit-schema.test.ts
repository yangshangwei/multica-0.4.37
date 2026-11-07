// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

afterEach(() => setSchemaLogger(noopLogger));

it("preserves optional date-only details without inventing values for older servers", () => {
  const dated = { ...collaborationIssueFixture, start_date: "2026-10-14", due_date: "2028-02-29" };
  expect(parseCollaborationIssue(dated)).toMatchObject({ start_date: "2026-10-14", due_date: "2028-02-29" });
  expect(parseCollaborationIssue({ ...dated, start_date: null, due_date: null })).toMatchObject({ start_date: null, due_date: null });
  const older = parseCollaborationIssue(collaborationIssueFixture);
  expect(older).not.toHaveProperty("start_date");
  expect(older).not.toHaveProperty("due_date");
});

it.each(["", "2026-02-29", "2026-10-14T00:00:00Z", "2026-10-14\n", "PRIVATE_DATE", 1, {}])("rejects malformed present dates without logging private values", (date) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationIssue({ ...collaborationIssueFixture, start_date: date })).toBeNull();
  expect(parseCollaborationIssue({ ...collaborationIssueFixture, due_date: date })).toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});
