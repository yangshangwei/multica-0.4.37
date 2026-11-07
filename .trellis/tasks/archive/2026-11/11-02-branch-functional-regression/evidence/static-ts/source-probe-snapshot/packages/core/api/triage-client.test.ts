// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { IssueSchema } from "./schemas";
import { isFormalAdmission } from "../triage";

const settings = { supported: true, enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none" as const, responsibility_member_id: null, revision: 1 };
const issue = { id: "issue-1", workspace_id: "ws-1", number: 1, identifier: "T-1", title: "Input", description: null, status: "backlog", admission_status: "pending", priority: "none", assignee_type: null, assignee_id: null, creator_type: "member", creator_id: "user-1", parent_issue_id: null, project_id: null, position: 1, start_date: null, due_date: null, created_at: "2026-10-04T00:00:00Z", updated_at: "2026-10-04T00:00:00Z", revision: 1 };
const item = { issue, candidate_project_id: null, candidate_assignee_type: null, candidate_assignee_id: null, reviewer_id: null, reviewer_valid: false, round: 1, first_entered_at: issue.created_at, entered_at: issue.created_at, snoozed_until: null, duplicate_issue_id: null, duplicate_identifier: null, source: "manual", source_url: null, external_id: null, batch_id: null, filename: null, row_number: null };
const action = { id: "action-1", issue_id: issue.id, actor_id: "user-1", action: "accept", round: 1, reason: null, before: {}, after: {}, created_at: issue.created_at, execution_status: "not_requested", task_id: null, execution_error: null };
const row = { row_number: 1, values: { title: "Input" }, warnings: [], errors: [], duplicate: false, duplicate_issue_id: null, similar_issue_ids: [], status: "ready", issue_id: null, error: null };
const preview = { batch_id: "batch-1", filename: "input.csv", headers: ["title"], mapping: { title: "title" }, rows: [row], counts: { valid: 1, warning: 0, error: 0, duplicate: 0 }, limits: { max_rows: 1000, max_bytes: 5242880 } };
const input = { request_id: "614c5ae7-4255-4d92-a3b8-8801adbd4ce4", expected_revision: 1, action: "accept" as const };
const client = () => new ApiClient("https://example.test");
const respond = (body: unknown, status = 200) => vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } })));
afterEach(() => vi.unstubAllGlobals());

describe("triage response and transport contracts", () => {
  const endpoints = [
    { name: "settings", call: (c: ApiClient) => c.getTriageSettings("ws-1"), good: settings },
    { name: "settings mutation", call: (c: ApiClient) => c.updateTriageSettings("ws-1", { ...settings, expected_revision: 1 }), good: settings },
    { name: "queue", call: (c: ApiClient) => c.listTriageItems("ws-1"), good: { items: [item], total: 1, counts: { pending: 1, ready: 1, snoozed: 0 }, limit: 50, offset: 0 } },
    { name: "detail", call: (c: ApiClient) => c.getTriageItem("ws-1", "issue-1"), good: item },
    { name: "create", call: (c: ApiClient) => c.createTriageItem("ws-1", { request_id: input.request_id, title: "Input" }), good: item },
    { name: "item history", call: (c: ApiClient) => c.getTriageItemHistory("ws-1", "issue-1"), good: { events: [action] } },
    { name: "global history", call: (c: ApiClient) => c.listTriageHistory("ws-1"), good: { entries: [{ id: "entry-1", kind: "import", issue_id: null, identifier: null, title: "Import", action: "import", actor_id: "user-1", created_at: issue.created_at, reason: null, before: {}, after: {}, batch_id: "batch-1", filename: "input.csv", counts: { created: 1, skipped: 0, failed: 0 } }], total: 1, limit: 50, offset: 0 } },
    { name: "action", call: (c: ApiClient) => c.performTriageAction("ws-1", "issue-1", input), good: { item, action } },
    { name: "execution retry", call: (c: ApiClient) => c.retryTriageExecution("ws-1", "action-1"), good: { item, action } },
    { name: "batch preview", call: (c: ApiClient) => c.previewTriageBatch("ws-1", { items: [{ issue_id: "issue-1", expected_revision: 1 }], action: "accept" }), good: { items: [{ issue_id: "issue-1", expected_revision: 1, valid: true, error: null }], valid_count: 1 } },
    { name: "batch commit", call: (c: ApiClient) => c.commitTriageBatch("ws-1", { items: [{ issue_id: "issue-1", ...input }] }), good: { results: [{ issue_id: "issue-1", status: "success", result: { item, action } }], success_count: 1 } },
    { name: "import preview", call: (c: ApiClient) => c.previewTriageImport("ws-1", { request_id: input.request_id, filename: "input.csv", csv: "title\nInput" }), good: preview },
    { name: "import get", call: (c: ApiClient) => c.getTriageImport("ws-1", "batch-1"), good: preview },
    { name: "import commit", call: (c: ApiClient) => c.commitTriageImport("ws-1", "batch-1", { rows: [{ row_number: 1, import_duplicate: false }] }), good: { batch_id: "batch-1", results: [{ ...row, status: "skipped" }], created: 0, skipped: 1, failed: 0 } },
  ];
  it.each(endpoints)("parses $name with explicit workspace", async ({ call, good }) => {
    respond(good);
    const result = await call(client());
    expect(result).toMatchObject(good);
    expect(vi.mocked(fetch).mock.calls[0]?.[1]?.headers).toMatchObject({ "X-Workspace-ID": "ws-1" });
  });
  it.each(endpoints)("rejects malformed $name instead of inventing success", async ({ call }) => {
    respond({});
    await expect(call(client())).rejects.toThrow(/malformed/i);
  });
  it("rejects action receipts whose issue identity does not match", async () => {
    respond({ item, action: { ...action, issue_id: "another-issue" } });
    await expect(client().performTriageAction("ws-1", "issue-1", input)).rejects.toThrow(/malformed/i);
  });
  it("rejects queued execution without its durable task identity", async () => {
    respond({ item, action: { ...action, execution_status: "queued", task_id: null } });
    await expect(client().retryTriageExecution("ws-1", "action-1")).rejects.toThrow(/malformed/i);
  });
  it("rejects a successful batch row with no committed receipt", async () => {
    respond({ results: [{ issue_id: "issue-1", status: "success" }], success_count: 1 });
    await expect(client().commitTriageBatch("ws-1", { items: [{ ...input, issue_id: "issue-1" }] })).rejects.toThrow(/malformed/i);
  });
  it("rejects an import claiming creation without an issue identity", async () => {
    respond({ batch_id: "batch-1", results: [{ ...row, status: "created" }], created: 1, skipped: 0, failed: 0 });
    await expect(client().commitTriageImport("ws-1", "batch-1", { rows: [{ row_number: 1, import_duplicate: false }] })).rejects.toThrow(/malformed/i);
  });
  it.each(["detail", "create", "list", "action", "retry", "batch"])("rejects a foreign-workspace %s response", async (kind) => {
    const foreign = { ...item, issue: { ...issue, workspace_id: "ws-2" } };
    const c = client();
    if (kind === "detail") { respond(foreign); await expect(c.getTriageItem("ws-1", "issue-1")).rejects.toThrow(/malformed/i); }
    if (kind === "create") { respond(foreign); await expect(c.createTriageItem("ws-1", { request_id: input.request_id, title: "Input" })).rejects.toThrow(/malformed/i); }
    if (kind === "list") { respond({ items: [foreign], total: 1, counts: { pending: 1, ready: 1, snoozed: 0 }, limit: 50, offset: 0 }); await expect(c.listTriageItems("ws-1")).rejects.toThrow(/malformed/i); }
    if (kind === "action") { respond({ item: foreign, action }); await expect(c.performTriageAction("ws-1", "issue-1", input)).rejects.toThrow(/malformed/i); }
    if (kind === "retry") { respond({ item: foreign, action }); await expect(c.retryTriageExecution("ws-1", "action-1")).rejects.toThrow(/malformed/i); }
    if (kind === "batch") { respond({ results: [{ issue_id: "issue-1", status: "success", result: { item: foreign, action } }], success_count: 1 }); await expect(c.commitTriageBatch("ws-1", { items: [{ ...input, issue_id: "issue-1" }] })).rejects.toThrow(/malformed/i); }
  });
  it("rejects a different issue even if the receipt internally agrees", async () => {
    respond(item);
    await expect(client().getTriageItem("ws-1", "wrong-issue")).rejects.toThrow(/malformed/i);
    respond({ item, action });
    await expect(client().performTriageAction("ws-1", "wrong-issue", input)).rejects.toThrow(/malformed/i);
  });
  it("accepts detail/decision responses resolved from a requested identifier", async () => {
    respond(item);
    await expect(client().getTriageItem("ws-1", "T-1")).resolves.toMatchObject(item);
    respond({ item, action });
    await expect(client().performTriageAction("ws-1", "T-1", input)).resolves.toMatchObject({ item, action });
  });
  it("rejects history actions attached to another issue", async () => {
    respond({ events: [action] });
    await expect(client().getTriageItemHistory("ws-1", "wrong-issue")).rejects.toThrow(/malformed/i);
  });
  it("rejects execution retry responses for another action", async () => {
    respond({ item, action });
    await expect(client().retryTriageExecution("ws-1", "wrong-action")).rejects.toThrow(/malformed/i);
  });
  it.each(["get", "commit"])("rejects %s import responses for another batch", async (kind) => {
    if (kind === "get") { respond(preview); await expect(client().getTriageImport("ws-1", "wrong-batch")).rejects.toThrow(/malformed/i); }
    else { respond({ batch_id: "batch-1", results: [row], created: 0, skipped: 1, failed: 0 }); await expect(client().commitTriageImport("ws-1", "wrong-batch", { rows: [{ row_number: 1, import_duplicate: false }] })).rejects.toThrow(/malformed/i); }
  });
  it("rejects a batch row whose outer and nested issue identities differ", async () => {
    respond({ results: [{ issue_id: "issue-2", status: "success", result: { item, action } }], success_count: 1 });
    await expect(client().commitTriageBatch("ws-1", { items: [{ ...input, issue_id: "issue-2" }] })).rejects.toThrow(/malformed/i);
  });
  it.each(["preview", "commit"])("rejects unselected and duplicate rows in batch %s", async (kind) => {
    for (const ids of [["issue-2"], ["issue-1", "issue-1"]]) {
      if (kind === "preview") {
        respond({ items: ids.map(issue_id => ({ issue_id, expected_revision: 1, valid: true, error: null })), valid_count: ids.length });
        await expect(client().previewTriageBatch("ws-1", { items: [{ issue_id: "issue-1", expected_revision: 1 }], action: "accept" })).rejects.toThrow(/malformed/i);
      } else {
        respond({ results: ids.map(issue_id => ({ issue_id, status: "conflict", error: "Changed" })), success_count: 0 });
        await expect(client().commitTriageBatch("ws-1", { items: [{ ...input, issue_id: "issue-1" }] })).rejects.toThrow(/malformed/i);
      }
    }
  });
  it("rejects batch preview revision substitution", async () => {
    respond({ items: [{ issue_id: "issue-1", expected_revision: 99, valid: true, error: null }], valid_count: 1 });
    await expect(client().previewTriageBatch("ws-1", { items: [{ issue_id: "issue-1", expected_revision: 1 }], action: "accept" })).rejects.toThrow(/malformed/i);
  });
  it("rejects unselected and duplicate import row results", async () => {
    for (const rows of [[{ ...row, row_number: 2 }], [row, row]]) {
      respond({ batch_id: "batch-1", results: rows, created: 0, skipped: rows.length, failed: 0 });
      await expect(client().commitTriageImport("ws-1", "batch-1", { rows: [{ row_number: 1, import_duplicate: false }] })).rejects.toThrow(/malformed/i);
    }
  });
  it("rejects a different action than the requested decision", async () => {
    respond({ item, action: { ...action, action: "reject" } });
    await expect(client().performTriageAction("ws-1", "issue-1", input)).rejects.toThrow(/malformed/i);
  });
  it.each([0, -1, 1.5, "1", Number.MAX_SAFE_INTEGER + 1])("rejects invalid triage revision %s", async (revision) => {
    respond({ ...item, issue: { ...issue, revision } });
    await expect(client().getTriageItem("ws-1", "issue-1")).rejects.toThrow(/malformed/i);
  });
  it("rejects preview counts inconsistent with row validity", async () => {
    respond({ items: [{ issue_id: "issue-1", expected_revision: 1, valid: false, error: "Changed" }], valid_count: 1 });
    await expect(client().previewTriageBatch("ws-1", { items: [{ issue_id: "issue-1", expected_revision: 1 }], action: "accept" })).rejects.toThrow(/malformed/i);
  });
  it("rejects batch success counts without matching successful receipts", async () => {
    respond({ results: [{ issue_id: "issue-1", status: "conflict", error: "Changed" }], success_count: 1 });
    await expect(client().commitTriageBatch("ws-1", { items: [{ ...input, issue_id: "issue-1" }] })).rejects.toThrow(/malformed/i);
  });
  it("rejects import result counts inconsistent with selected row outcomes", async () => {
    respond({ batch_id: "batch-1", results: [{ ...row, status: "failed", error: "Invalid" }], created: 1, skipped: 0, failed: 0 });
    await expect(client().commitTriageImport("ws-1", "batch-1", { rows: [{ row_number: 1, import_duplicate: false }] })).rejects.toThrow(/malformed/i);
  });
  it("requires a revision and admission on a triage item", async () => {
    respond({ ...item, issue: { ...issue, revision: undefined, admission_status: undefined } });
    await expect(client().getTriageItem("ws-1", "issue-1")).rejects.toThrow(/malformed/i);
  });
  it("only a settings 404 means unsupported", async () => {
    respond({ error: "not found" }, 404);
    await expect(client().getTriageSettings("ws-1")).resolves.toMatchObject({ supported: false, enabled: false });
    respond({ error: "forbidden" }, 403);
    await expect(client().getTriageSettings("ws-1")).rejects.toMatchObject({ status: 403 });
  });
  it("retains request identity and payload after an uncertain transport failure", async () => {
    const mock = vi.fn().mockRejectedValueOnce(new TypeError("Network lost")).mockResolvedValueOnce(new Response(JSON.stringify({ item, action })));
    vi.stubGlobal("fetch", mock);
    const c = client();
    await expect(c.performTriageAction("ws-1", "issue-1", input)).rejects.toThrow("Network lost");
    await c.performTriageAction("ws-1", "issue-1", input);
    expect(mock.mock.calls.map(([, init]) => JSON.parse(init.body))).toEqual([input, input]);
  });
  it("serializes every queue filter without dropping offset zero", async () => {
    respond(endpoints[2]!.good);
    await client().listTriageItems("ws-1", { view: "snoozed", q: "need & work", reviewer_id: "user-1", offset: 0, entered_after: "2026-10-01", sort: "priority" });
    const url = new URL(String(vi.mocked(fetch).mock.calls[0]?.[0]));
    expect(Object.fromEntries(url.searchParams)).toEqual({ view: "snoozed", q: "need & work", reviewer_id: "user-1", offset: "0", entered_after: "2026-10-01", sort: "priority" });
  });
  it("downloads CSV through authorized transport", async () => {
    const csv = 'title,error\n"hello, world",invalid';
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(csv, { headers: { "Content-Type": "text/csv" } })));
    const blob = await client().downloadTriageFailures("ws-1", "batch/1");
    expect(await blob.text()).toBe(csv);
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe("https://example.test/api/triage/imports/batch%2F1/failures");
  });
  it("rejects an HTML error page disguised as a successful CSV download", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("<html>Error</html>", { headers: { "Content-Type": "text/html" } })));
    await expect(client().downloadTriageFailures("ws-1", "batch-1")).rejects.toThrow(/CSV/);
  });
});

describe("admission compatibility", () => {
  it("defaults only absent admission to not_required on older responses", () => {
    const parsed = IssueSchema.parse({ ...issue, admission_status: undefined });
    expect(parsed.admission_status).toBe("not_required");
  });
  it.each(["pending", "rejected", "duplicate", "future_state", null, 12])("fails closed for %s", (value) => {
    expect(isFormalAdmission(value)).toBe(false);
  });
  it.each([undefined, "not_required", "accepted"])("permits formal %s", (value) => {
    expect(isFormalAdmission(value)).toBe(true);
  });
});
