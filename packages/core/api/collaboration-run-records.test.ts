// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { noopLogger } from "../logger";
import { setSchemaLogger } from "./schema";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { collaborationRunRecordsFixture as fixture } from "../collaboration/run-records-test-fixtures";

const issueId = fixture.issue_id;
const runId = fixture.run.id;
const otherId = "00000000-0000-4000-8000-0000000000ab";
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); setSchemaLogger(noopLogger); });

describe("collaboration run-record transport", () => {
  it("uses only the protected run path, omits ambient workspace headers and preserves an opaque cursor", async () => {
    setCurrentWorkspace("ambient", otherId);
    const fetch = vi.fn(async () => json(fixture));
    vi.stubGlobal("fetch", fetch);
    const signal = new AbortController().signal;
    await expect(new ApiClient("https://server.example").getCollaborationRunRecords(issueId, runId, { cursor: "opaque+/position=2", limit: 50 }, { signal })).resolves.toEqual(fixture);
    expect(fetch).toHaveBeenCalledOnce();
    const [url, init] = fetch.mock.calls[0]! as unknown as [string, RequestInit];
    expect(url).toBe(`https://server.example/api/collaboration/issues/${issueId}/runs/${runId}/records?limit=50&cursor=opaque%2B%2Fposition%3D2`);
    expect(new Headers(init.headers).has("X-Workspace-ID")).toBe(false);
    expect(new Headers(init.headers).has("X-Workspace-Slug")).toBe(false);
    expect(init.cache).toBe("no-store");
  });

  it("rejects another issue/run, a repeated cursor and an overfull page", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(json({ ...fixture, issue_id: otherId }))
      .mockResolvedValueOnce(json({ ...fixture, run: { ...fixture.run, id: otherId } }))
      .mockResolvedValueOnce(json({ ...fixture, next_cursor: "same-cursor" }))
      .mockResolvedValueOnce(json(fixture));
    vi.stubGlobal("fetch", fetch);
    const api = new ApiClient("https://server.example");
    await expect(api.getCollaborationRunRecords(issueId, runId)).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId)).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId, { cursor: "same-cursor" })).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId, { limit: 1 })).resolves.toBeNull();
  });

  it("validates identities and page bounds before transport", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    const api = new ApiClient("https://server.example");
    for (const params of [{ limit: 0 }, { limit: 101 }, { limit: 1.5 }, { cursor: "" }, { cursor: "x".repeat(1025) }]) {
      await expect(api.getCollaborationRunRecords(issueId, runId, params)).rejects.toThrow();
    }
    await expect(api.getCollaborationRunRecords("SRC-1", runId)).rejects.toThrow();
    await expect(api.getCollaborationRunRecords(issueId, "../messages")).rejects.toThrow();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("distinguishes an absent route or authorized missing run from a denied issue", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
      .mockResolvedValueOnce(json({ code: "run_not_found" }, 404)).mockResolvedValueOnce(json({ code: "not_found" }, 404)));
    const api = new ApiClient("https://server.example");
    await expect(api.getCollaborationRunRecords(issueId, runId)).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId)).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId)).rejects.toMatchObject({ status: 404 });
  });

  it("redacts private JSON, error bodies and cursors on every failure path", async () => {
    const log = vi.fn();
    const logger = { ...noopLogger, info: log, warn: log, error: log };
    setSchemaLogger(logger);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response('{"PRIVATE_SYNTAX":'))
      .mockResolvedValueOnce(json({ ...fixture, items: [{ kind: "PRIVATE_KIND", body: "PRIVATE_BODY" }] }))
      .mockResolvedValueOnce(json({ error: "PRIVATE_ERROR" }, 500)));
    const api = new ApiClient("https://server.example", { logger });
    await expect(api.getCollaborationRunRecords(issueId, runId, { cursor: "PRIVATE_CURSOR" })).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId)).resolves.toBeNull();
    await expect(api.getCollaborationRunRecords(issueId, runId)).rejects.toThrow();
    expect(log).toHaveBeenCalled();
    expect(JSON.stringify(log.mock.calls)).not.toContain("PRIVATE_");
  });
});
