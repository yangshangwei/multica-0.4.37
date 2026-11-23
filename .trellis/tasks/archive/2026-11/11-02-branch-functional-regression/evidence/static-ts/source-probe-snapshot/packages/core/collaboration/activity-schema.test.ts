// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationActivity } from "./activity-schema";

const id = "00000000-0000-4000-8000-000000000001";
const other = "00000000-0000-4000-8000-000000000002";
const entry = { id: other, issue_id: id, event_seq: "9007199254740993", kind: "issue_edited", actor: null,
  created_at: "2026-10-18T00:00:00Z", details_available: true, target: null, handoff: null, comment_id: null,
  changes: [{ field: "title", before: "Before", after: "After" }] };
afterEach(() => { setSchemaLogger(noopLogger); vi.unstubAllGlobals(); });

it("checks complete scoped activity, exact sequence and explicitly unavailable details", () => {
  const page = { issue_id: id, items: [entry], next_cursor: null };
  expect(parseCollaborationActivity(page)?.items[0]?.event_seq).toBe("9007199254740993");
  expect(parseCollaborationActivity({ ...page, items: [{ ...entry, kind: "other", details_available: false, changes: [] }] })).not.toBeNull();
  for (const patch of [{ issue_id: other }, { event_seq: 1 }, { changes: undefined }, { details_available: false }, { kind: "other" }]) {
    expect(parseCollaborationActivity({ ...page, items: [{ ...entry, ...patch }] })).toBeNull();
  }
  expect(parseCollaborationActivity({ ...page, items: [entry, entry] })).toBeNull();
});

it("does not log malformed history values or accept another issue's page", async () => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationActivity({ issue_id: id, items: [{ ...entry, changes: [{ field: "title", before: null, after: { secret: "PRIVATE_ACTIVITY" } }] }], next_cursor: null })).toBeNull();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_ACTIVITY");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ issue_id: other, items: [], next_cursor: null }))));
  await expect(new ApiClient("https://server.example").getCollaborationActivity(id)).resolves.toBeNull();
});

it("distinguishes an absent optional history endpoint from structured object denial", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found" }), { status: 404 })));
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationActivity(id)).resolves.toBeNull();
  await expect(api.getCollaborationActivity(id)).rejects.toMatchObject({ status: 404 });
});
