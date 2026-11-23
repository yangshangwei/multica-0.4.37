// @vitest-environment node
import { createHash } from "node:crypto";
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { clearCollaborationAccess, collaborationScope } from "./access";
import { buildKnowledgeSources } from "./knowledge-sources";
import { parseCollaborationMemoryVersion } from "./memory-schema";
import { parseCollaborationComment, parseCollaborationCommentHistory } from "./content-schema";
import { parseCollaborationRunRecords } from "./run-records-schema";
import { collaborationRunRecordsFixture } from "./run-records-test-fixtures";

const actor = "00000000-0000-4000-8000-000000000001", commentId = "00000000-0000-4000-8000-000000000002";
const issueId = collaborationRunRecordsFixture.issue_id;
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).forEach((dispose) => dispose()); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://sources.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); cleanups.push(() => client.clear());
  return { api, client, scope: collaborationScope(actor) };
}
function memory(content: string | null) {
  return parseCollaborationMemoryVersion({ issue_id: issueId, version: "7", current_version: "9", bytes: 12, max_bytes: 65536,
    author: null, created_at: "2026-11-01T00:00:00Z", restored_from_version: null, purged: content === null,
    purged_at: content === null ? "2026-11-01T00:00:00Z" : null, purged_by: null, purged_by_admin: false, content });
}
function comment(deleted = false) {
  return parseCollaborationComment({ id: commentId, issue_id: issueId, author_type: "member", author: null, content: deleted ? "" : "Latest text", type: "comment",
    parent_id: null, comment_revision: "2", created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:01Z",
    deleted_at: deleted ? "2026-11-01T00:00:02Z" : null, resolved_at: null, resolved_by: null, can_edit: false, can_resolve: false, reactions: [] });
}

it("hashes the selected historical memory version exactly when latest has advanced", async () => {
  const f = setup(), content = "  Exact 文本\n";
  const read = vi.spyOn(f.api, "getCollaborationMemoryVersion").mockResolvedValue(memory(content));
  const result = await buildKnowledgeSources(f.client, f.scope, issueId, [{ kind: "memory", issue_id: issueId, version: "7" }]);
  expect(read.mock.calls[0]?.slice(0, 2)).toEqual([issueId, "7"]);
  expect(result).toEqual([{ kind: "memory", issue_id: issueId, version: "7", sha256: createHash("sha256").update(content).digest("hex") }]);
});

it("uses retained comment revision text, while a tombstone blocks every old revision", async () => {
  const f = setup(); const read = vi.spyOn(f.api, "getCollaborationComment").mockResolvedValue(comment());
  vi.spyOn(f.api, "getCollaborationCommentHistory").mockResolvedValue(parseCollaborationCommentHistory({ issue_id: issueId, comment_id: commentId, items: [{
    event_id: actor, event_seq: "2", kind: "comment_edited", actor: null, created_at: "2026-11-01T00:00:01Z", comment_revision: "2", before: "Original", after: "Latest text",
  }] }));
  const selection = [{ kind: "comment" as const, issue_id: issueId, comment_id: commentId, version: "1" }];
  expect((await buildKnowledgeSources(f.client, f.scope, issueId, selection))[0]?.sha256).toBe(createHash("sha256").update("Original").digest("hex"));
  read.mockResolvedValue(comment(true));
  await expect(buildKnowledgeSources(f.client, f.scope, issueId, selection)).rejects.toThrow();
});

it("finds the exact run-record identity across pages without depending on memory", async () => {
  const f = setup(); const runId = collaborationRunRecordsFixture.run.id;
  const read = vi.spyOn(f.api, "getCollaborationRunRecords").mockResolvedValueOnce(parseCollaborationRunRecords({ ...collaborationRunRecordsFixture, items: [], next_cursor: "more" }))
    .mockResolvedValueOnce(parseCollaborationRunRecords(collaborationRunRecordsFixture));
  const item = collaborationRunRecordsFixture.items[1]!;
  const result = await buildKnowledgeSources(f.client, f.scope, issueId, [{ kind: "run_record", issue_id: issueId, run_id: runId, record_id: item.id }]);
  expect(read).toHaveBeenCalledTimes(2); expect(result[0]?.sha256).toBe(createHash("sha256").update(item.body).digest("hex"));
});

it("refuses purged versions, foreign issue references and late reads after access loss", async () => {
  const f = setup(); vi.spyOn(f.api, "getCollaborationMemoryVersion").mockResolvedValueOnce(memory(null));
  const source = { kind: "memory" as const, issue_id: issueId, version: "7" };
  await expect(buildKnowledgeSources(f.client, f.scope, issueId, [source])).rejects.toThrow();
  await expect(buildKnowledgeSources(f.client, f.scope, issueId, [{ ...source, issue_id: actor }])).rejects.toThrow();
  let finish!: (value: ReturnType<typeof memory>) => void;
  vi.mocked(f.api.getCollaborationMemoryVersion).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const pending = buildKnowledgeSources(f.client, f.scope, issueId, [source]).catch((error: unknown) => error);
  clearCollaborationAccess(f.client, f.scope, issueId); finish(memory("Secret"));
  expect(await pending).toBeInstanceOf(Error);
});

it("bounds stalled source discovery with the existing deadline and does not impose a page-count cutoff", async () => {
  const f = setup(); let page = 0;
  const runId = collaborationRunRecordsFixture.run.id, record = collaborationRunRecordsFixture.items[1]!;
  vi.spyOn(f.api, "getCollaborationRunRecords").mockImplementation(async () => parseCollaborationRunRecords({ ...collaborationRunRecordsFixture,
    items: ++page === 35 ? [record] : [], next_cursor: page === 35 ? null : String(page) }));
  expect((await buildKnowledgeSources(f.client, f.scope, issueId, [{ kind: "run_record", issue_id: issueId, run_id: runId, record_id: record.id }]))[0]?.kind).toBe("run_record");
  expect(page).toBe(35);
  vi.useFakeTimers();
  try {
    vi.spyOn(f.api, "getCollaborationMemoryVersion").mockImplementation(() => new Promise(() => {}));
    const pending = buildKnowledgeSources(f.client, f.scope, issueId, [{ kind: "memory", issue_id: issueId, version: "7" }]).catch((error: unknown) => error);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(await pending).toMatchObject({ name: "TimeoutError" });
  } finally { vi.useRealTimers(); }
});
