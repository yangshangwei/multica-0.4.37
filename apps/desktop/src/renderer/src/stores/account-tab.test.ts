import { beforeEach, describe, expect, it } from "vitest";
import { accountPath, accountRouteKey } from "@multica/core/paths";
import { getActiveTab, mergePersistedTabs, useTabStore } from "./tab-store";

const a = accountPath({ issueId: "11111111-1111-4111-8111-111111111111" });
const b = accountPath({ view: "participating", issueId: "22222222-2222-4222-8222-222222222222" });
beforeEach(() => useTabStore.getState().reset());

describe("persistent account tab", () => {
  it("keeps history traversal transient through persistence, rehydration and owner reset", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchAccount(a);
    store.switchAccount(b);
    store.goBack();
    expect(useTabStore.getState().isHistoryTraversal).toBe(true);
    const serialized = JSON.parse(localStorage.getItem("multica_tabs")!).state;
    expect(serialized.isHistoryTraversal).toBeUndefined();
    expect(mergePersistedTabs(serialized, useTabStore.getState()).isHistoryTraversal).toBe(false);
    store.setSessionOwner("server:other-viewer");
    expect(useTabStore.getState().isHistoryTraversal).toBe(false);
    store.switchAccount(a);
    store.switchAccount(b);
    store.goBack();
    store.reset();
    expect(useTabStore.getState().isHistoryTraversal).toBe(false);
  });
  it("restores notification filters, exact selections and separate list/detail mementos through history and scope resume", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchWorkspace("acme", "/acme/projects/project-a");
    const first = "/account?view=notifications&filter=unread&notification=33333333-3333-4333-8333-333333333333&issue=11111111-1111-4111-8111-111111111111#description";
    const second = "/account?view=notifications&filter=unread&notification=44444444-4444-4444-8444-444444444444&issue=11111111-1111-4111-8111-111111111111#comment-55555555-5555-4555-8555-555555555555";
    const archived = "/account?view=notifications&filter=archived";
    store.switchAccount(first);
    const id = getActiveTab(useTabStore.getState())!.id;
    store.commitScrollMemento(id, accountRouteKey(first)!, { "account-notifications-list": { top: 240, height: 1800 }, "account-detail": { top: 620, height: 2400 } });
    store.switchAccount(second);
    store.commitScrollMemento(id, accountRouteKey(second)!, { "account-notifications-list": { top: 240, height: 1800 }, "account-detail": { top: 920, height: 2400 } });
    store.switchAccount(archived);
    expect(useTabStore.getState().accountGroup?.tabs).toHaveLength(1);
    store.goBack();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(second);
    store.goBack();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(first);
    store.goForward();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(second);
    store.switchWorkspace("acme");
    expect(getActiveTab(useTabStore.getState())?.url).toBe("/acme/projects/project-a");
    store.switchAccount();
    expect(getActiveTab(useTabStore.getState())).toMatchObject({ id, url: second, history: { stack: [first, second, archived], index: 1 } });
    expect(useTabStore.getState().isScopeResume).toBe(true);
    const persisted = JSON.parse(localStorage.getItem("multica_tabs")!).state;
    const restored = mergePersistedTabs(persisted, useTabStore.getState());
    expect(getActiveTab(restored)).toMatchObject({ id, url: second, history: { stack: [first, second, archived], index: 1 } });
    const scroll = getActiveTab(restored)!.memento.scroll;
    expect(scroll[`${accountRouteKey(first)}::account-detail`]?.top).toBe(620);
    expect(scroll[`${accountRouteKey(second)}::account-detail`]?.top).toBe(920);
    expect(scroll[`${accountRouteKey(second)}::account-notifications-list`]?.top).toBe(240);
  });

  it("resumes the most recent workspace and account details without replacing their state", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchWorkspace("acme");
    store.switchWorkspace("butter", "/butter/projects/project-b?tab=overview");
    const workspaceTab = getActiveTab(useTabStore.getState())!;
    store.switchAccount(b);
    const accountTab = getActiveTab(useTabStore.getState())!;
    store.commitViewState(accountTab.id, b, "account-search", "needle");
    store.commitScrollMemento(accountTab.id, b, { "account-detail": { top: 480, height: 1500 } });
    const account = useTabStore.getState().accountGroup;

    expect(useTabStore.getState().lastWorkspaceSlug).toBe("butter");
    store.switchWorkspace(useTabStore.getState().lastWorkspaceSlug!);
    expect(getActiveTab(useTabStore.getState())).toBe(workspaceTab);
    store.switchAccount();
    expect(getActiveTab(useTabStore.getState())).toBe(account?.tabs[0]);
    expect(useTabStore.getState().isScopeResume).toBe(true);
    const before = useTabStore.getState();
    store.switchAccount();
    expect(useTabStore.getState()).toBe(before);
  });

  it("persists the last workspace while account-active and clears invalid or previous-owner pointers", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchWorkspace("acme");
    store.switchWorkspace("butter");
    store.switchAccount(b);
    const serialized = JSON.parse(localStorage.getItem("multica_tabs")!).state;
    expect(serialized.lastWorkspaceSlug).toBe("butter");
    expect(serialized.isScopeResume).toBeUndefined();
    expect(serialized.navigationKey).toBeUndefined();
    const restored = mergePersistedTabs(serialized, useTabStore.getState());
    expect(restored.lastWorkspaceSlug).toBe("butter");
    expect(restored.activeWorkspaceSlug).toBeNull();
    const invalid = mergePersistedTabs({ ...serialized, lastWorkspaceSlug: "gone" }, useTabStore.getState());
    expect(invalid.lastWorkspaceSlug).toBeNull();

    store.validateWorkspaceSlugs(new Set(["acme"]));
    expect(useTabStore.getState().lastWorkspaceSlug).toBeNull();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(b);
    store.switchWorkspace("acme");
    store.setSessionOwner("other-server:viewer");
    expect(useTabStore.getState().lastWorkspaceSlug).toBeNull();
    store.switchWorkspace("acme");
    store.reset();
    expect(useTabStore.getState().lastWorkspaceSlug).toBeNull();
  });

  it("adopts a legacy active workspace and records cross-workspace tab activation", () => {
    const store = useTabStore.getState();
    store.switchWorkspace("acme");
    const acme = getActiveTab(useTabStore.getState())!;
    store.switchWorkspace("butter");
    store.setActiveTab(acme.id);
    expect(useTabStore.getState().lastWorkspaceSlug).toBe("acme");
    const legacy = { activeWorkspaceSlug: "butter", byWorkspace: useTabStore.getState().byWorkspace };
    expect(mergePersistedTabs(legacy, useTabStore.getState()).lastWorkspaceSlug).toBe("butter");
    store.validateWorkspaceSlugs(new Set(["butter"]));
    expect(useTabStore.getState().lastWorkspaceSlug).toBe("butter");
  });

  it("reuses one account tab while updating query selection and preserving workspace pins/MRU", () => {
    const store = useTabStore.getState();
    store.switchWorkspace("acme");
    store.togglePin(getActiveTab(useTabStore.getState())!.id);
    store.openTab("/acme/projects", "Projects", { activate: true });
    const workspace = useTabStore.getState().byWorkspace.acme;
    store.switchAccount(a);
    const id = getActiveTab(useTabStore.getState())!.id;
    store.switchWorkspace("butter");
    store.switchAccount(b);
    const state = useTabStore.getState();
    expect(state.activeScope).toBe("account");
    expect(state.activeWorkspaceSlug).toBeNull();
    expect(state.accountGroup?.tabs).toHaveLength(1);
    expect(getActiveTab(state)).toMatchObject({ id, url: b, history: { stack: [a, b], index: 1 } });
    expect(state.byWorkspace.acme).toBe(workspace);
    store.goBack();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(a);
    store.goForward();
    expect(getActiveTab(useTabStore.getState())?.url).toBe(b);
  });

  it("workspace eviction does not evict or select over the account group", () => {
    const store = useTabStore.getState();
    store.switchWorkspace("acme");
    store.switchAccount(a);
    store.validateWorkspaceSlugs(new Set(["butter"]));
    expect(getActiveTab(useTabStore.getState())?.url).toBe(a);
    expect(useTabStore.getState().activeWorkspaceSlug).toBeNull();
    store.validateWorkspaceSlugs(new Set());
    expect(getActiveTab(useTabStore.getState())?.url).toBe(a);
  });

  it("migrates v4 workspace state and roundtrips account history/mementos", async () => {
    const store = useTabStore.getState();
    store.switchWorkspace("acme");
    store.openTab("/acme/projects", "Projects", { activate: true });
    store.togglePin(getActiveTab(useTabStore.getState())!.id);
    const workspace = useTabStore.getState().byWorkspace;
    const migrated = mergePersistedTabs({ activeWorkspaceSlug: "acme", byWorkspace: workspace }, useTabStore.getState());
    expect(migrated.activeScope).toBe("workspace");
    expect(migrated.byWorkspace).toEqual(workspace);
    store.setSessionOwner("server:viewer");
    store.switchAccount(a);
    store.commitViewState(getActiveTab(useTabStore.getState())!.id, a, "thread", "open");
    store.switchAccount(b);
    const serialized = JSON.parse(localStorage.getItem("multica_tabs")!);
    expect(serialized.version).toBe(5);
    store.reset();
    const restored = mergePersistedTabs(serialized.state, useTabStore.getState());
    expect(getActiveTab(restored)?.history.stack).toEqual([a, b]);
    expect(getActiveTab(restored)?.memento.view[`${a}::thread`]).toBe("open");
    expect(restored.byWorkspace).toEqual(workspace);
    expect(restored.sessionOwner).toBe("server:viewer");
  });

  it("clears previous server/viewer tabs on identity change", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer-a");
    store.switchWorkspace("acme");
    store.switchAccount(a);
    store.setSessionOwner("server:viewer-b");
    expect(getActiveTab(useTabStore.getState())).toBeNull();
    expect(useTabStore.getState().accountGroup).toBeNull();
    expect(useTabStore.getState().byWorkspace).toEqual({});
  });

  it("restores the saved history position when the same account URL was visited twice", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchAccount(a);
    store.switchAccount(b);
    store.switchAccount(a);
    store.goBack();
    store.goBack();
    const persisted = JSON.parse(localStorage.getItem("multica_tabs")!).state;

    const restored = mergePersistedTabs(persisted, useTabStore.getState());

    expect(getActiveTab(restored)).toMatchObject({
      url: a,
      history: { stack: [a, b, a], index: 0 },
    });
  });

  it("preserves the saved position when rejected history entries precede it", () => {
    const store = useTabStore.getState();
    store.setSessionOwner("server:viewer");
    store.switchAccount(a);
    const state = useTabStore.getState();
    const restored = mergePersistedTabs({
      ...state,
      accountGroup: {
        ...state.accountGroup,
        tabs: [{
          ...getActiveTab(state),
          history: { stack: ["/acme/issues", a, b, a], index: 1 },
        }],
      },
    }, state);

    expect(getActiveTab(restored)?.history).toEqual({ stack: [a, b, a], index: 0 });
  });

  it("rejects malformed account paths and account history containing workspace paths", () => {
    const store = useTabStore.getState();
    store.switchAccount("/account?issue=bad");
    expect(getActiveTab(useTabStore.getState())).toBeNull();
    store.setSessionOwner("server:viewer");
    store.switchAccount(a);
    const state = useTabStore.getState();
    const restored = mergePersistedTabs({ ...state, accountGroup: { ...state.accountGroup, tabs: [{ ...getActiveTab(state), history: { stack: ["/acme/issues", a], index: 1 } }] } }, state);
    expect(getActiveTab(restored)?.history.stack).toEqual([a]);
  });
});
