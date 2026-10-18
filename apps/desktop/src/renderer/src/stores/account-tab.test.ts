import { beforeEach, describe, expect, it } from "vitest";
import { accountPath } from "@multica/core/paths";
import { getActiveTab, mergePersistedTabs, useTabStore } from "./tab-store";

const a = accountPath({ issueId: "11111111-1111-4111-8111-111111111111" });
const b = accountPath({ view: "participating", issueId: "22222222-2222-4222-8222-222222222222" });
beforeEach(() => useTabStore.getState().reset());

describe("persistent account tab", () => {
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
