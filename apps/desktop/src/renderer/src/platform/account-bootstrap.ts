import type { TabScope } from "../stores/tab-store";

/**
 * Workspace query failures cannot take over an authenticated account surface.
 * A fresh session ("none") waits for the list instead of defaulting to account:
 * with workspaces it opens the first one, as before account tabs existed. The
 * account navigation bar stays reachable meanwhile, and deep links still win.
 */
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
  return workspaceListUnavailable || (workspaceListReady && workspaceCount === 0) ? "account" : null;
}
