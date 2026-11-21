"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, type ReactNode } from "react";
import { navigationScope, resolveScopePath, useScopeHistoryStore } from "@multica/core/navigation";
import { paths } from "@multica/core/paths";
import type { Workspace } from "@multica/core/types";
import { useWorkspaceList } from "@multica/core/workspace";
import { currentPath, useNavigation } from "@multica/views/navigation";
import { useWebNavigationOwner } from "./navigation-owner";
import { useWebScopeResume } from "./navigation";

interface WebSidebarNavigation {
  navigationWorkspace: Workspace | null;
  onWorkspaceSelect(slug?: string): void;
  onCollaborationResume(): void;
  workspaceListUnavailable: boolean;
  onRetryWorkspaces(): void;
}

const SidebarNavigationContext = createContext<WebSidebarNavigation | null>(null);

export function useWebSidebarNavigation() {
  const context = useContext(SidebarNavigationContext);
  if (!context) throw new Error("Web sidebar navigation requires WebScopeShell");
  return context;
}

/** Both route layouts share navigation intent, independently of workspace providers. */
export function WebScopeShell({ children }: { children: ReactNode }) {
  const owner = useWebNavigationOwner();
  const navigation = useNavigation();
  const path = currentPath(navigation);
  const location = navigationScope(path);
  const scope = location?.scope;
  const routeSlug = location?.slug;
  const resume = useWebScopeResume();
  const { ready, workspaces, unavailable, refetch } = useWorkspaceList({ enabled: owner !== null && scope !== undefined });
  const workspacePath = useScopeHistoryStore((state) => state.owner === owner ? state.workspacePath : null);
  const rememberedSlug = workspacePath ? navigationScope(workspacePath)?.slug : undefined;
  // A shortcut may preview a directory member, but it never supplies workspace
  // authority to account content or fabricates a workspace from a saved slug.
  const navigationWorkspace = owner === null ? null :
    workspaces.find((workspace) => workspace.slug === routeSlug) ??
    workspaces.find((workspace) => workspace.slug === rememberedSlug) ??
    workspaces[0] ?? null;
  const selectedSlug = navigationWorkspace?.slug;
  const request = useRef(0);

  useEffect(() => {
    useScopeHistoryStore.getState().setOwner(owner);
    if (owner) useScopeHistoryStore.getState().record(owner, path);
    request.current += 1;
  }, [owner, path]);

  const onWorkspaceSelect = useCallback(async (slug?: string) => {
    const state = useScopeHistoryStore.getState();
    if (!owner || state.owner !== owner || (scope === "workspace" && (!slug || slug === routeSlug))) return;
    request.current += 1;
    // The section heading resumes the displayed workspace. Choosing another
    // directory member is an explicit navigation with its own destination.
    if (slug && slug !== selectedSlug) {
      if (workspaces.some((workspace) => workspace.slug === slug)) navigation.push(paths.workspace(slug).issues());
      return;
    }
    const destination = resolveScopePath(state, owner, "workspace", ready ? workspaces.map((workspace) => workspace.slug) : undefined);
    if (destination) { resume(destination); return; }
    const attempt = request.current;
    const result = await refetch();
    if (attempt !== request.current || useScopeHistoryStore.getState().owner !== owner || !result.data) return;
    const loaded = resolveScopePath(useScopeHistoryStore.getState(), owner, "workspace", result.data.map((workspace) => workspace.slug));
    if (loaded) resume(loaded);
  }, [owner, scope, routeSlug, selectedSlug, ready, workspaces, refetch, navigation, resume]);

  const onCollaborationResume = useCallback(() => {
    const state = useScopeHistoryStore.getState();
    if (!owner || state.owner !== owner || scope === "account") return;
    request.current += 1;
    const destination = resolveScopePath(state, owner, "account");
    if (destination) resume(destination);
  }, [owner, scope, resume]);

  const onRetryWorkspaces = useCallback(() => {
    if (!owner || useScopeHistoryStore.getState().owner !== owner) return;
    request.current += 1;
    void refetch();
  }, [owner, refetch]);
  const sidebar = useMemo<WebSidebarNavigation>(() => ({
    navigationWorkspace,
    onWorkspaceSelect: (slug) => { void onWorkspaceSelect(slug); },
    onCollaborationResume,
    workspaceListUnavailable: unavailable,
    onRetryWorkspaces,
  }), [navigationWorkspace, onWorkspaceSelect, onCollaborationResume, unavailable, onRetryWorkspaces]);

  // One element tree for every state. Viewer discovery must not remount a
  // routed editor, and the route layouts own all sidebar presentation.
  const chrome = owner !== null && scope !== undefined;
  return <SidebarNavigationContext.Provider value={sidebar}>
    <div className={chrome ? "flex h-svh min-h-0 flex-col overflow-hidden bg-app-shell" : "contents"}>
      <div className={chrome ? "min-h-0 flex-1" : "contents"}>{children}</div>
    </div>
  </SidebarNavigationContext.Provider>;
}
