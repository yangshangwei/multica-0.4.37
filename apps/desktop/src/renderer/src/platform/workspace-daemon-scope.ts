/**
 * Keeps the managed daemon bound to every workspace the signed-in account has.
 *
 * Desktop main creates a workspace's managed binding, not the daemon, and the
 * daemon refuses to register runtimes for a workspace without one ("managed
 * workspace has no verified binding"). A binding used to be requested only when
 * a session went from zero workspaces to one, so a workspace created or joined
 * later had no runtimes until 刷新 restarted the daemon.
 *
 * This tracks the account's workspace ids across renders and reports what the
 * daemon needs. An account switch reseeds instead of acting: daemon-manager
 * already restarts on that via syncToken.
 */
export interface WorkspaceScope {
  userId: string;
  workspaceIds: ReadonlySet<string>;
  /** The session started with no workspace and has not restarted for one yet. */
  awaitingFirst: boolean;
}

export type WorkspaceScopeAction = "none" | "first_workspace" | "workspace_added";

export function nextWorkspaceScope(
  scope: WorkspaceScope | null,
  observed: { userId: string | null; listReady: boolean; workspaceIds: readonly string[] },
): { scope: WorkspaceScope | null; action: WorkspaceScopeAction } {
  if (!observed.userId) return { scope: null, action: "none" };
  if (!observed.listReady) return { scope: scope?.userId === observed.userId ? scope : null, action: "none" };
  const ids = new Set(observed.workspaceIds);
  if (!scope || scope.userId !== observed.userId) {
    return { scope: { userId: observed.userId, workspaceIds: ids, awaitingFirst: ids.size === 0 }, action: "none" };
  }
  const added = [...ids].some((id) => !scope.workspaceIds.has(id));
  if (!added) return { scope: { ...scope, workspaceIds: ids }, action: "none" };
  if (scope.awaitingFirst) return { scope: { userId: scope.userId, workspaceIds: ids, awaitingFirst: false }, action: "first_workspace" };
  return { scope: { ...scope, workspaceIds: ids }, action: "workspace_added" };
}

export interface WorkspaceScopeDaemonAPI {
  getStatus: () => Promise<{ state: string }>;
  refreshManagedWorkspaces?: () => Promise<{ managed: boolean; accepted: boolean }>;
  restart: () => Promise<unknown>;
}

export async function applyWorkspaceScopeAction(api: WorkspaceScopeDaemonAPI, action: WorkspaceScopeAction): Promise<void> {
  if (action === "first_workspace") {
    // Unchanged from before: a legacy daemon restarts to pick the workspace up
    // now rather than on its next workspace-sync tick.
    const result = await api.refreshManagedWorkspaces?.();
    if (!result?.managed) await api.restart();
    return;
  }
  if (action !== "workspace_added") return;
  // Only a running daemon: the refresh IPC marks the daemon as desired-running,
  // which would undo a member's explicit Stop, and any later start already binds
  // every workspace. Main retries a busy daemon from its own poll loop.
  if ((await api.getStatus()).state !== "running") return;
  await api.refreshManagedWorkspaces?.();
}
