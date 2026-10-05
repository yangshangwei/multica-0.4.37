/** @vitest-environment jsdom */
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError } from "../api";
import { clearClientSessionData } from "../platform/session-cleanup";
import type { StorageAdapter } from "../types/storage";
import { projectKeys } from "./queries";
import { useProjectAccessStore, protectProjectRequest, isProjectAccessLost, beginProjectDelete, isProjectDeletePending, registerProjectLocalTextFlush, markProjectDeleted } from "./access";
const storage: StorageAdapter = { getItem: () => null, setItem: () => {}, removeItem: () => {}, keys: () => [] };
afterEach(() => vi.unstubAllGlobals());
beforeEach(() => useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }));
it("recognizes the real workspace middleware404 and erases its protected cache", async () => {
  const qc = new QueryClient(); qc.setQueryData(projectKeys.detail("w", "p"), { description: "private" });
  const error = new ApiError("workspace not found or access denied", 404, "Not Found", { code: "workspace_access_denied" });
  await expect(protectProjectRequest(qc, "w", "p", async () => { throw error; })).rejects.toMatchObject({ status: 404 });
  expect(isProjectAccessLost(error)).toBe(true);
  expect(qc.getQueryData(projectKeys.detail("w", "p"))).toBeUndefined(); qc.clear();
});
it.each([
  [404, "project_update_not_found"], [404, "execution_not_found"], [403, "project_evidence_forbidden"],
  [403, "project_permission_denied"], [503, "project_health_unavailable"],
])("does not erase workspace content for HTTP%s %s", async (status, code) => {
  const qc = new QueryClient(); const data = { description: "retained" }; qc.setQueryData(projectKeys.detail("w", "p"), data);
  const error = new ApiError("request failed", Number(status), "Error", { code });
  await expect(protectProjectRequest(qc, "w", "p", async () => { throw error; })).rejects.toBe(error);
  expect(qc.getQueryData(projectKeys.detail("w", "p"))).toEqual(data); expect(useProjectAccessStore.getState().denied).toEqual({}); qc.clear();
});
it("actual session cleanup removes denied and copy-only state before a new account reads", async () => {
  const qc = new QueryClient();
  useProjectAccessStore.setState({ denied: { '["w","*"]': true }, deleted: { '["w","p"]': ["Previous account text"] } });
  clearClientSessionData(qc, storage);
  expect(useProjectAccessStore.getState().denied).toEqual({}); expect(useProjectAccessStore.getState().deleted).toEqual({});
  let networkCalls = 0;
  await expect(protectProjectRequest(qc, "w", "p", async () => { networkCalls++; return "new account"; })).resolves.toBe("new account");
  expect(networkCalls).toBe(1);
});
it("session cleanup for logout or server reset rejects an earlier request even after epochs clear", async () => {
  const qc = new QueryClient(); let finish!: (value: string) => void;
  const pending = protectProjectRequest(qc, "w", "p", () => new Promise<string>((resolve) => { finish = resolve; }));
  clearClientSessionData(qc, storage);
  await expect(protectProjectRequest(qc, "w", "p", async () => "new connection")).resolves.toBe("new connection");
  finish("previous account protected result");
  await expect(pending).rejects.toThrow(); expect(useProjectAccessStore.getState().denied).toEqual({});
});

it("preserves the middleware denial code through a real API error response", async () => {
  const qc = new QueryClient(); qc.setQueryData(projectKeys.detail("w", "p"), { description: "private" });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "workspace not found or access denied", code: "workspace_access_denied" }), { status: 404, headers: { "Content-Type": "application/json" } })));
  const client = new ApiClient("https://server.test");
  await expect(protectProjectRequest(qc, "w", "p", () => client.getProject("p", { workspaceId: "w" }))).rejects.toMatchObject({ status: 404, body: { code: "workspace_access_denied" } });
  expect(qc.getQueryData(projectKeys.detail("w", "p"))).toBeUndefined(); qc.clear();
});
it("old-session deletion cleanup cannot clear the new session's pending deletion", () => {
  const qc = new QueryClient(); const finishOld = beginProjectDelete("w", "p");
  clearClientSessionData(qc, storage); const finishNew = beginProjectDelete("w", "p");
  finishOld(); expect(isProjectDeletePending("w", "p")).toBe(true); finishNew();
});
it("old-editor cleanup cannot remove a newly registered deletion text flusher", () => {
  const qc = new QueryClient(); const removeOld = registerProjectLocalTextFlush("w", "p", () => {});
  clearClientSessionData(qc, storage); const flushNew = vi.fn(); const removeNew = registerProjectLocalTextFlush("w", "p", flushNew);
  removeOld(); markProjectDeleted(qc, "w", "p"); expect(flushNew).toHaveBeenCalledOnce(); removeNew(); qc.clear();
});
