// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import type { BatchUpdateIssuesOptions } from "../types/api";

const ids = ["issue-1", "issue-2", "issue-3"];
const updates = { status: "done" as const };
const fetchMock = vi.fn();
const warn = vi.fn();
const client = new ApiClient("https://batch.test");
const committed = ids.map((issue_id, index) => ({ index, issue_id, state: "committed", code: "updated", revision: index + 1 }));
const partial = {
  code: "batch_update_incomplete", cause_code: "delivery_completion_authorization_required",
  error: "One update committed; refresh before continuing.", message: "One update committed; refresh before continuing.",
  updated: 1,
  results: [committed[0], { index: 1, issue_id: ids[1], state: "rejected", code: "delivery_completion_authorization_required" },
    { index: 2, issue_id: ids[2], state: "not_attempted", code: "previous_item_failed" }],
};

function reply(body: unknown, status = 200) {
  fetchMock.mockResolvedValue(new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
}

beforeEach(() => {
  fetchMock.mockReset(); warn.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  setSchemaLogger({ ...noopLogger, warn });
});
afterEach(() => { vi.unstubAllGlobals(); setSchemaLogger(noopLogger); });

describe("batch issue update response contract", () => {
  it("preserves the legacy request and validates an older full success without inventing results", async () => {
    reply({ updated: 3 });
    await expect(client.batchUpdateIssues(ids, updates)).resolves.toEqual({ updated: 3 });
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[1].body).toBe(JSON.stringify({ issue_ids: ids, updates }));
  });

  it("sends explicit per-issue bindings without a shared grant or automatic request key", async () => {
    const issueID = "11111111-1111-4111-8111-111111111111";
    const binding = { issue_id: issueID, authorization_id: "22222222-2222-4222-8222-222222222222", operation_id: "33333333-3333-4333-8333-333333333333", expected_issue_revision: 0 };
    reply({ updated: 1 });
    await client.batchUpdateIssues([issueID], { status: "done" }, { completion_bindings: [binding] });
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST", redirect: "error", body: JSON.stringify({ issue_ids: [issueID], updates: { status: "done" }, completion_bindings: [binding] }),
    });
    expect(fetchMock.mock.calls[0]?.[1].headers).not.toHaveProperty("Idempotency-Key");
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("rejects malformed, duplicate or unrelated completion controls before transport", async () => {
    const issueID = "11111111-1111-4111-8111-111111111111";
    const binding = { issue_id: issueID, authorization_id: "22222222-2222-4222-8222-222222222222", operation_id: "33333333-3333-4333-8333-333333333333", expected_issue_revision: 0 };
    const bad: unknown[] = [
      { completion_bindings: [binding, binding] },
      { completion_bindings: [{ ...binding, issue_id: "44444444-4444-4444-8444-444444444444" }] },
      { completion_bindings: [{ ...binding, expected_issue_revision: undefined }] },
      { completion_bindings: [{ ...binding, expected_issue_revision: -1 }] },
      { completion_bindings: [{ ...binding, expected_issue_revision: Number.MAX_SAFE_INTEGER + 1 }] },
      { completion_bindings: [{ ...binding, authorization_id: "00000000-0000-0000-0000-000000000000" }] },
      { completion_bindings: [{ ...binding, extra: true }] },
    ];
    for (const options of bad) {
      await expect(client.batchUpdateIssues([issueID], updates, options as BatchUpdateIssuesOptions)).rejects.toMatchObject({
        status: 400, body: { code: "invalid_batch_update_request" },
      });
    }
    await expect(client.batchUpdateIssues([issueID], { status: "done", completion: binding })).rejects.toMatchObject({ status: 400 });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("correlates a delayed response with the sent IDs even if the caller changes its array", async () => {
    let finish!: (response: Response) => void;
    fetchMock.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const selection = [...ids];
    const pending = client.batchUpdateIssues(selection, updates);
    selection.splice(0, selection.length, "new-selection");
    finish(new Response(JSON.stringify({ updated: 3, results: committed }), { status: 200 }));
    await expect(pending).resolves.toEqual({ updated: 3, results: committed });
    expect(fetchMock.mock.calls[0]?.[1].body).toBe(JSON.stringify({ issue_ids: ids, updates }));
  });

  it("returns ordered server results and strips unrelated response fields", async () => {
    reply({ updated: 3, results: committed, private_observation: "must-not-enter-cache" });
    await expect(client.batchUpdateIssues(ids, updates)).resolves.toEqual({ updated: 3, results: committed });
  });

  it.each([0, 1, 2])("refuses a legacy partial count %i as full success", async (updated) => {
    reply({ updated });
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({
      name: "BatchUpdateError", outcome: { updated }, outcomeUnknown: true,
    });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("preserves no-mutation responses without counting control fields as changes", async () => {
    reply({ updated: 0 });
    await expect(client.batchUpdateIssues(ids, { suppress_run: true, expected_revision: 7 })).resolves.toEqual({ updated: 0 });
    reply({ updated: 0, results: ids.map((issue_id, index) => ({ index, issue_id, state: "not_attempted", code: "no_changes" })) });
    await expect(client.batchUpdateIssues(ids, {})).resolves.toMatchObject({ updated: 0 });
  });

  it("does not claim changes when only controls were sent but a server returns a positive count", async () => {
    reply({ updated: 3 });
    await expect(client.batchUpdateIssues(ids, { suppress_run: true })).rejects.toMatchObject({ name: "BatchUpdateError" });
  });

  it("does not treat an explicit nullable mutation as a no-op", async () => {
    reply({ updated: 0 });
    await expect(client.batchUpdateIssues(ids, { project_id: null })).rejects.toMatchObject({ name: "BatchUpdateError" });
  });

  it.each([
    ["missing body", null], ["missing count", {}], ["negative", { updated: -1 }],
    ["excess count", { updated: 4 }], ["fractional", { updated: 1.5 }],
    ["unsafe integer", { updated: Number.MAX_SAFE_INTEGER + 1 }],
    ["truncated results", { updated: 3, results: committed.slice(0, 2) }],
    ["wrong id", { updated: 3, results: [committed[0], { ...committed[1], issue_id: "other" }, committed[2]] }],
    ["wrong index", { updated: 3, results: [committed[0], { ...committed[1], index: 0 }, committed[2]] }],
    ["unknown state", { updated: 3, results: [committed[0], { ...committed[1], state: "maybe" }, committed[2]] }],
    ["inconsistent count", { updated: 2, results: committed }],
    ["unsafe revision", { updated: 3, results: [committed[0], { ...committed[1], revision: Number.MAX_SAFE_INTEGER + 1 }, committed[2]] }],
    ["null results", { updated: 3, results: null }],
    ["empty reason", { updated: 3, results: [committed[0], { ...committed[1], code: "" }, committed[2]] }],
  ])("treats malformed %s as unknown and never retries", async (_label, body) => {
    reply(body);
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({ name: "BatchUpdateError", outcomeUnknown: true });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("never accepts a partial response just because HTTP is 200", async () => {
    reply(partial);
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({
      name: "BatchUpdateError", outcome: { updated: 1, results: partial.results }, outcomeUnknown: false,
    });
  });

  it("preserves authenticated HTTP error details and the known committed prefix", async () => {
    reply(partial, 428);
    let caught: unknown;
    try { await client.batchUpdateIssues(ids, updates); } catch (error) { caught = error; }
    expect(caught).toBeInstanceOf(ApiError);
    expect(caught).toMatchObject({ status: 428, body: partial, outcome: { updated: 1, results: partial.results }, outcomeUnknown: false });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("retains unknown commit outcomes without a second write", async () => {
    const unknown = { ...partial, code: "batch_update_outcome_unknown", cause_code: "commit_unknown",
      results: [committed[0], { index: 1, issue_id: ids[1], state: "unknown", code: "commit_unknown" }, partial.results[2]] };
    reply(unknown, 503);
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({
      status: 503, outcome: { updated: 1, results: unknown.results }, outcomeUnknown: true,
    });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("rejects contradictory post-refusal commits and revision disclosure on refused items", async () => {
    for (const body of [
      { ...partial, updated: 2, results: [{ ...partial.results[1], index: 0, issue_id: ids[0] }, committed[1], committed[2]] },
      { ...partial, results: [committed[0], { ...partial.results[1], revision: 4 }, partial.results[2]] },
      { ...partial, results: [committed[0], partial.results[1], { ...partial.results[2], state: "rejected" }] },
    ]) {
      reply(body, 409);
      await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({ outcome: undefined, outcomeUnknown: true });
    }
  });

  it.each([401, 403])("keeps HTTP %i refusal status when older errors lack item results", async (status) => {
    reply({ error: "Access denied" }, status);
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({ status, body: { error: "Access denied" }, outcomeUnknown: true });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("redacts malformed success payloads and JSON syntax failures", async () => {
    reply({ updated: "private_value_must_not_be_logged" });
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({ outcomeUnknown: true });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("private_value_must_not_be_logged");
    fetchMock.mockResolvedValue(new Response('private_value_must_not_be_logged{', { status: 200 }));
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toThrow(/could not be confirmed/i);
  });

  it("does not retry after a transport failure", async () => {
    fetchMock.mockRejectedValue(new TypeError("Disconnected"));
    await expect(client.batchUpdateIssues(ids, updates)).rejects.toMatchObject({ outcomeUnknown: true });
    expect(fetchMock).toHaveBeenCalledOnce();
  });
});
