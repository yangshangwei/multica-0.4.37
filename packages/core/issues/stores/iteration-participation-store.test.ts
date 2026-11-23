// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  createIterationParticipationStore,
  registerIterationParticipationStore,
  resetIterationParticipationStores,
  type IterationParticipationScope,
  type IterationParticipationStore,
} from "./iteration-participation-store";

const a = {
  workspaceId: "10000000-0000-4000-8000-000000000001",
  issueId: "20000000-0000-4000-8000-000000000001",
  sourcePath: "/acme/issues/20000000-0000-4000-8000-000000000001",
  canonicalPath: "/acme/issues/ACME-1",
};
const b = {
  ...a,
  issueId: "20000000-0000-4000-8000-000000000002",
  sourcePath: "/acme/issues/20000000-0000-4000-8000-000000000002",
  canonicalPath: "/acme/issues/ACME-2",
};

function scope(store: IterationParticipationStore, issue = a): IterationParticipationScope {
  return { ...issue, generation: store.getState().generation };
}

describe("current issue iteration participation intent", () => {
  it("retains an early expansion through both spellings of the same issue", () => {
    const store = createIterationParticipationStore(a.sourcePath);
    const original = scope(store);
    // A real toggle may arrive before the route's passive selection effect.
    store.getState().setOpen(original, true);
    store.getState().selectIssue(original);
    store.getState().syncPath(a.canonicalPath);
    store.getState().selectIssue({ ...original, sourcePath: a.canonicalPath });
    expect(store.getState().current?.open).toBe(true);
    expect(store.getState().generation).toBe(original.generation);

    store.getState().syncPath(a.sourcePath);
    expect(store.getState().current?.open).toBe(true);
    expect(store.getState().generation).toBe(original.generation);
  });

  it("starts another issue closed and ignores outgoing issue callbacks", () => {
    const store = createIterationParticipationStore(a.sourcePath);
    const original = scope(store);
    store.getState().setOpen(original, true);
    store.getState().syncPath(b.sourcePath);
    const next = scope(store, b);
    store.getState().selectIssue(next);

    store.getState().setOpen(original, true);
    store.getState().selectIssue(original);
    store.getState().clearIssue(a.sourcePath, original.generation);
    expect(store.getState().current?.issueId).toBe(b.issueId);
    expect(store.getState().current?.open).toBe(false);

    store.getState().setOpen(next, true);
    store.getState().syncPath(b.canonicalPath);
    expect(store.getState().current?.open).toBe(true);
  });

  it("does not remember visited issues after leaving or visiting another issue", () => {
    for (const destination of ["/acme/issues", "/login", b.sourcePath]) {
      const store = createIterationParticipationStore(a.sourcePath);
      store.getState().setOpen(scope(store), true);
      store.getState().syncPath(destination);
      expect(store.getState().current).toBeNull();
      store.getState().syncPath(a.canonicalPath);
      store.getState().selectIssue(scope(store, { ...a, sourcePath: a.canonicalPath }));
      expect(store.getState().current?.open).toBe(false);
    }
  });

  it("separates workspace identity even when an address and issue UUID are reused", () => {
    const store = createIterationParticipationStore(a.sourcePath);
    const original = scope(store);
    store.getState().setOpen(original, true);
    const otherWorkspace = { ...a, workspaceId: "10000000-0000-4000-8000-000000000002" };
    store.getState().selectIssue(scope(store, otherWorkspace));
    store.getState().setOpen(original, true);
    expect(store.getState().current?.workspaceId).toBe(otherWorkspace.workspaceId);
    expect(store.getState().current?.open).toBe(false);
    expect(store.getState().generation).toBeGreaterThan(original.generation);
  });

  it("fences an unavailable current issue without letting its old toggle reactivate it", () => {
    const store = createIterationParticipationStore(a.sourcePath);
    const original = scope(store);
    store.getState().setOpen(original, true);
    store.getState().syncPath(a.canonicalPath);
    store.getState().clearIssue(a.canonicalPath, original.generation);
    store.getState().setOpen(original, true);
    store.getState().selectIssue(original);
    expect(store.getState().current).toBeNull();
    const generation = store.getState().generation;
    store.getState().clearIssue(a.canonicalPath, generation);
    expect(store.getState().generation).toBe(generation);
  });

  it("does not let an old unavailable route clear the new current issue", () => {
    const store = createIterationParticipationStore(a.sourcePath);
    const original = scope(store);
    store.getState().selectIssue(original);
    store.getState().syncPath(b.sourcePath);
    store.getState().setOpen(scope(store, b), true);
    store.getState().clearIssue(b.sourcePath, original.generation);
    expect(store.getState().current?.issueId).toBe(b.issueId);
    expect(store.getState().current?.open).toBe(true);
  });

  it("keeps independent mounted navigation surfaces independent", () => {
    const first = createIterationParticipationStore(a.sourcePath);
    const second = createIterationParticipationStore(a.sourcePath);
    first.getState().setOpen(scope(first), true);
    second.getState().selectIssue(scope(second));
    expect(second.getState().current?.open).toBe(false);
    second.getState().syncPath(b.sourcePath);
    expect(first.getState().current?.open).toBe(true);
  });

  it("resets only registered live surfaces and fences old session callbacks", () => {
    const live = createIterationParticipationStore(a.sourcePath);
    const unmounted = createIterationParticipationStore(a.sourcePath);
    const previousSession = scope(live);
    live.getState().setOpen(previousSession, true);
    unmounted.getState().setOpen(scope(unmounted), true);
    const unregister = registerIterationParticipationStore(live);
    try {
      resetIterationParticipationStores();
      live.getState().setOpen(previousSession, true);
      live.getState().selectIssue(previousSession);
      expect(live.getState().current).toBeNull();
      expect(live.getState().generation).toBeGreaterThan(previousSession.generation);
      expect(unmounted.getState().current?.open).toBe(true);
      live.getState().selectIssue(scope(live));
      expect(live.getState().current?.open).toBe(false);

      unregister();
      live.getState().setOpen(scope(live), true);
      resetIterationParticipationStores();
      expect(live.getState().current?.open).toBe(true);
    } finally {
      unregister();
    }
  });
});
