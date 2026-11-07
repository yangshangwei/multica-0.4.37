// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { projectProgressDraftKey, useProjectProgressDraftStore, writeProjectProgressDraft, prepareProjectUpdateIntent, clearProjectProgressDraft, emptyProjectUpdateDraft } from "./progress-draft-store";
import { p1Preview } from "./test-fixtures/p1";
beforeEach(() => useProjectProgressDraftStore.getState().clearDraft());
describe("project publication intent persistence", () => {
  it("isolates workspace, server, project and correction identities", () => {
    const keys = [projectProgressDraftKey("a", "w", "p"), projectProgressDraftKey("b", "w", "p"), projectProgressDraftKey("a", "x", "p"), projectProgressDraftKey("a", "w", "q"), projectProgressDraftKey("a", "w", "p", "u")];
    expect(new Set(keys).size).toBe(5);
  });
  it("reuses the same request for retries and does not clear a newer draft", () => {
    const key = projectProgressDraftKey("a", "w", "p");
    writeProjectProgressDraft(key, { ...emptyProjectUpdateDraft(), body: "First" });
    const preview = { ...p1Preview, draft: { ...emptyProjectUpdateDraft(), body: "First" } };
    const first = prepareProjectUpdateIntent(key, preview);
    expect(prepareProjectUpdateIntent(key, preview).request_id).toBe(first.request_id);
    writeProjectProgressDraft(key, { ...emptyProjectUpdateDraft(), body: "Second" });
    clearProjectProgressDraft(key, first.request_id);
    expect(useProjectProgressDraftStore.getState().draft.entries[key]?.draft.body).toBe("Second");
  });
});
