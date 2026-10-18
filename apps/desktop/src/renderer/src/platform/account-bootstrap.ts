import type { TabScope } from "../stores/tab-store";

/** Workspace query failures cannot take over an authenticated account surface. */
export function accountBootstrapDestination({
  authenticated, restricted, activeScope, workspaceListReady, workspaceListUnavailable, workspaceCount,
}: {
  authenticated: boolean;
  restricted: boolean;
  activeScope: TabScope;
  workspaceListReady: boolean;
  workspaceListUnavailable: boolean;
  workspaceCount: number;
}): "account" | null {
  if (!authenticated || restricted || activeScope === "account") return null;
  return activeScope === "none" || workspaceListUnavailable || (workspaceListReady && workspaceCount === 0) ? "account" : null;
}
