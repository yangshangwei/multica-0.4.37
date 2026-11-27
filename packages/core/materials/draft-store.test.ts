// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { clearMaterialDrafts, materialDraftKey, retireMaterialPendingRequest, setMaterialEditorDraft, setMaterialPendingRequest, useMaterialDraftStore } from "./draft-store";
import { decodeMaterialCommand } from "./schema";

beforeEach(() => useMaterialDraftStore.getState().clearDraft());
describe("material intent and editor ownership", () => {
  it("retains the exact request and isolates viewers/projects during cleanup", () => {
    const current = materialDraftKey("alice", "ws", "project", "material");
    const otherProject = materialDraftKey("alice", "ws", "other", "material");
    const otherWorkspace = materialDraftKey("bob", "elsewhere", "project", "material");
    const request = { action: "start-plan", data: { request_id: "original", expected_version: 5 } };
    for (const key of [current, otherProject, otherWorkspace]) setMaterialPendingRequest(key, request);
    setMaterialEditorDraft(current, { version: 5, content: "Keep this draft", planContent: "", acceptance: "", blockers: "", ready: false, sources: [] });
    expect(decodeMaterialCommand(useMaterialDraftStore.getState().draft.requests[current])).toEqual(request);
    clearMaterialDrafts("ws", "project");
    expect(useMaterialDraftStore.getState().draft.editors[current]).toBeUndefined();
    expect(useMaterialDraftStore.getState().draft.requests[current]).toBeUndefined();
    expect(useMaterialDraftStore.getState().draft.requests[otherProject]).toEqual(request);
    expect(useMaterialDraftStore.getState().draft.requests[otherWorkspace]).toEqual(request);
    clearMaterialDrafts("ws");
    expect(useMaterialDraftStore.getState().draft.requests[otherProject]).toBeUndefined();
    expect(useMaterialDraftStore.getState().draft.requests[otherWorkspace]).toEqual(request);
  });

  it("retains retired request bytes until their project loses access", () => {
    const key = materialDraftKey("alice", "ws", "project", "settings");
    const request = { request_id: "legacy", connection_id: "original", plan_agent_id: null, auto_start_plan: false };
    setMaterialPendingRequest(key, request);
    retireMaterialPendingRequest(key);
    expect(useMaterialDraftStore.getState().draft.requests[key]).toBeUndefined();
    expect(useMaterialDraftStore.getState().draft.retiredRequests[key]).toEqual([request]);
    retireMaterialPendingRequest(key);
    expect(useMaterialDraftStore.getState().draft.retiredRequests[key]).toEqual([request]);
    clearMaterialDrafts("ws", "project");
    expect(useMaterialDraftStore.getState().draft.retiredRequests[key]).toBeUndefined();
  });

  it("does not recover malformed stored approval into an authorized command", () => {
    expect(decodeMaterialCommand({ action: "decisions", data: { request_id: "key", decision: "approve" } })).toBeNull();
    expect(decodeMaterialCommand({ action: "retry", data: { request_id: "key", expected_version: "5" } })).toBeNull();
  });
});
