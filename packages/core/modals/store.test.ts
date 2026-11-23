// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { ApiClient } from "../api/client";
import { useModalStore } from "./store";

const api = new ApiClient("https://api.example.test");

function intent(workspaceId = "ws-acme", workspaceSlug = "acme") {
  return {
    api,
    sessionScope: api.getSessionScope(),
    userId: "viewer-1",
    workspaceId,
    workspaceSlug,
    originPath: "/account?view=participating&issue=issue-1#description",
    targetPath: `/${workspaceSlug}/issues`,
  };
}

function context(path = "/acme/issues") {
  return {
    api,
    sessionScope: api.getSessionScope(),
    userId: "viewer-1",
    path,
    workspaceId: "ws-acme",
    workspaceSlug: "acme",
  };
}

beforeEach(() => useModalStore.getState().close());

describe("workspace issue creation handoff", () => {
  it("queues an explicit target without publishing a workspace modal", () => {
    const request = intent();
    useModalStore.getState().queueWorkspaceIssueCreation(request);
    expect(useModalStore.getState().workspaceIssueCreation).toBe(request);
    expect(useModalStore.getState().modal).toBeNull();
  });

  it("waits at the source and at a target whose real workspace is not ready", () => {
    const request = intent();
    useModalStore.getState().queueWorkspaceIssueCreation(request);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, {
      ...context(request.originPath), workspaceId: null, workspaceSlug: null,
    })).toBe(false);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, {
      ...context(), workspaceId: null, workspaceSlug: null,
    })).toBe(false);
    expect(useModalStore.getState().workspaceIssueCreation).toBe(request);
  });

  it("consumes only the exact current target once", () => {
    const request = intent();
    useModalStore.getState().queueWorkspaceIssueCreation(request);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, context())).toBe(true);
    expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, context())).toBe(false);
  });

  it.each([
    { name: "a different API instance", owner: { api: new ApiClient("https://api.example.test") } },
    { name: "new credentials on the same API", owner: { sessionScope: "changed-session" } },
    { name: "a different viewer", owner: { userId: "viewer-2" } },
    { name: "an unauthenticated viewer", owner: { userId: null } },
    { name: "another committed destination", owner: { path: "/account?view=current" } },
  ])("retires the request after $name", ({ owner }) => {
    const request = intent();
    useModalStore.getState().queueWorkspaceIssueCreation(request);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, { ...context(), ...owner })).toBe(false);
    expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
  });

  it("does not open under another workspace even if the target URL is already published", () => {
    const request = intent();
    useModalStore.getState().queueWorkspaceIssueCreation(request);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(request, {
      ...context(), workspaceId: "ws-old", workspaceSlug: "old",
    })).toBe(false);
    expect(useModalStore.getState().workspaceIssueCreation).toBe(request);
  });

  it("rejects an obsolete observer without clearing a newer request", () => {
    const previous = intent();
    const current = intent("ws-other", "other");
    useModalStore.getState().queueWorkspaceIssueCreation(previous);
    useModalStore.getState().queueWorkspaceIssueCreation(current);
    expect(useModalStore.getState().reconcileWorkspaceIssueCreation(previous, context())).toBe(false);
    expect(useModalStore.getState().workspaceIssueCreation).toBe(current);
  });

  it.each(["open", "close"] as const)("retires pending creation when another modal action calls %s", (action) => {
    useModalStore.getState().queueWorkspaceIssueCreation(intent());
    if (action === "open") useModalStore.getState().open("feedback");
    else useModalStore.getState().close();
    expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
  });
});
