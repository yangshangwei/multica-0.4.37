// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAdminExecution, parseAdminExecutionList, parseAdminIssueList } from "./execution-schemas";
const id = "11111111-1111-4111-8111-111111111111";
const date = "2026-10-01T00:00:00Z";
const task = {
  id, workspace_id: id, agent_id: id, status: "failed", source: "issue", attempt: 2, created_at: date, content_access: false, title: null, content_url: null, usage: null
};
describe("platform execution response boundary", () => {
  it("keeps missing time and usage unknown and strips unapproved raw fields", () => {
    const result = parseAdminExecution({
      ...task, error: "secret", context: { prompt: "secret" }, work_dir: "/private", attempt: 2, retry_of_task_id: id
    });
    expect(result?.usage).toBeNull();
    expect(result?.startedAt).toBeNull();
    expect(result?.retryOfTaskId).toBe(id);
    expect(JSON.stringify(result)).not.toContain("secret");
  });
  it("fails closed for malformed identities and prevents content display without explicit access", () => {
    expect(parseAdminExecution({ ...task, id: "bad" })).toBeNull();
    expect(parseAdminExecution({
      ...task, title: "hidden", content_url: "/workspace/issues/" + id
    })?.title).toBeNull();
    expect(parseAdminExecution({
      ...task, content_access: true, content_url: "https://evil.test"
    })).toBeNull();
    expect(parseAdminExecution({ ...task, title: "hidden", content_access: true, content_url: null })?.title).toBeNull();
  });
  it("preserves new server enums as unknown without inventing a permission", () => {
    const parsed = parseAdminExecution({
      ...task, status: "new_status", source: "future_source"
    });
    expect(parsed?.status).toBe("unknown");
    expect(parsed?.source).toBe("unknown");
    expect(parsed?.contentAccess).toBe(false);
  });
  it("preserves the existing directory-wait state without exposing its private path",()=>{
    const parsed=parseAdminExecution({...task,status:"waiting_local_directory",wait_reason:"PRIVATE DIRECTORY"});
    expect(parsed?.status).toBe("waiting_local_directory");
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE DIRECTORY");
  });
  it("distinguishes an empty list from a malformed server payload", () => {
    expect(parseAdminExecutionList({
      items: [], next_cursor: null, as_of: date, scope: id
    })).toMatchObject({ items: [], asOf: date });
    expect(parseAdminExecutionList({
      items: [{ id }], as_of: date, scope: id
    })).toBeNull();
  });
  it("keeps issue count separate from execution attempts and conditionally displays its title", () => {
    const list = parseAdminIssueList({
      items: [{
        id, workspace_id: id, number: 1, identifier: "ABC-1", status: "todo", created_at: date, updated_at: date, execution_count: 3, title: "Visible", content_access: true, content_url: "/space/issues/" + id
      }], next_cursor: null, as_of: date, scope: id
    });
    expect(list?.items[0]).toMatchObject({
      executionCount: 3, title: "Visible", contentAccess: true
    });
  });
});
