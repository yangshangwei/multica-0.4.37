// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { applyWorkspaceScopeAction, nextWorkspaceScope, type WorkspaceScope } from "./workspace-daemon-scope";

const ready = (userId: string | null, workspaceIds: string[], listReady = true) => ({ userId, workspaceIds, listReady });
const step = (scope: WorkspaceScope | null, ...observations: ReturnType<typeof ready>[]) => {
  const actions: string[] = [];
  for (const observation of observations) {
    const next = nextWorkspaceScope(scope, observation);
    scope = next.scope;
    actions.push(next.action);
  }
  return { scope, actions };
};

describe("workspace set the managed daemon must be bound to", () => {
  it("seeds from the first settled list without acting", () => {
    expect(step(null, ready("u1", ["a", "b"])).actions).toEqual(["none"]);
  });

  it("waits for a pending list", () => {
    expect(step(null, ready("u1", [], false), ready("u1", ["a"])).actions).toEqual(["none", "none"]);
  });

  // Regression F1: a workspace created in a session that already had some was
  // never bound, so the daemon logged "managed workspace has no verified
  // binding" and onboarding showed no runtime until 刷新 restarted it.
  it("binds a workspace added to a non-empty session", () => {
    expect(step(null, ready("u1", ["a"]), ready("u1", ["a", "b"])).actions).toEqual(["none", "workspace_added"]);
  });

  it("keeps the first-workspace path for a session that started empty, then binds later ones", () => {
    expect(step(null, ready("u1", []), ready("u1", ["a"]), ready("u1", ["a", "b"])).actions)
      .toEqual(["none", "first_workspace", "workspace_added"]);
  });

  it("does nothing when the set is unchanged or only shrinks", () => {
    expect(step(null, ready("u1", ["a", "b"]), ready("u1", ["b", "a"]), ready("u1", ["a"])).actions)
      .toEqual(["none", "none", "none"]);
  });

  it("binds a workspace that comes back after being removed", () => {
    expect(step(null, ready("u1", ["a", "b"]), ready("u1", ["a"]), ready("u1", ["a", "b"])).actions)
      .toEqual(["none", "none", "workspace_added"]);
  });

  // daemon-manager already restarts on an account change via syncToken.
  it("reseeds on an account switch instead of treating the new account's workspaces as added", () => {
    expect(step(null, ready("u1", ["a"]), ready("u2", ["x", "y"])).actions).toEqual(["none", "none"]);
    expect(step(null, ready("u1", ["a"]), ready(null, []), ready("u2", ["x"])).actions).toEqual(["none", "none", "none"]);
  });

  it("restarts only once for a session that started empty", () => {
    expect(step(null, ready("u1", []), ready("u1", ["a"]), ready("u1", []), ready("u1", ["b"])).actions)
      .toEqual(["none", "first_workspace", "none", "workspace_added"]);
  });
});

describe("applying the action to the daemon", () => {
  const api = (state: string, managed: boolean) => ({
    getStatus: vi.fn(async () => ({ state })),
    refreshManagedWorkspaces: vi.fn(async () => ({ managed, accepted: true })),
    restart: vi.fn(async () => ({ success: true })),
  });

  it("binds an added workspace on a running daemon without restarting it", async () => {
    const daemon = api("running", true);
    await applyWorkspaceScopeAction(daemon, "workspace_added");
    expect(daemon.refreshManagedWorkspaces).toHaveBeenCalledTimes(1);
    expect(daemon.restart).not.toHaveBeenCalled();
  });

  // refreshManagedWorkspaces marks the daemon as desired-running, which would
  // undo a member's explicit Stop; the next start binds every workspace anyway.
  it.each(["stopped", "stopping", "starting", "recovery_paused", "auth_expired", "cli_not_found"])(
    "leaves a %s daemon alone", async (state) => {
      const daemon = api(state, true);
      await applyWorkspaceScopeAction(daemon, "workspace_added");
      expect(daemon.refreshManagedWorkspaces).not.toHaveBeenCalled();
      expect(daemon.restart).not.toHaveBeenCalled();
    },
  );

  it("keeps the legacy first-workspace restart when the session is not managed", async () => {
    const legacy = api("running", false);
    await applyWorkspaceScopeAction(legacy, "first_workspace");
    expect(legacy.refreshManagedWorkspaces).toHaveBeenCalledTimes(1);
    expect(legacy.restart).toHaveBeenCalledTimes(1);
    const managed = api("running", true);
    await applyWorkspaceScopeAction(managed, "first_workspace");
    expect(managed.restart).not.toHaveBeenCalled();
  });

  it("never restarts for an added workspace on a legacy daemon", async () => {
    const legacy = api("running", false);
    await applyWorkspaceScopeAction(legacy, "workspace_added");
    expect(legacy.restart).not.toHaveBeenCalled();
  });
});
