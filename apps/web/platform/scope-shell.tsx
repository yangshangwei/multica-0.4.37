"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { navigationScope, resolveScopePath, useScopeHistoryStore } from "@multica/core/navigation";
import { useWorkspaceList } from "@multica/core/workspace";
import { GlobalScopeNavigation } from "@multica/views/layout";
import { currentPath, useNavigation } from "@multica/views/navigation";
import { useT } from "@multica/views/i18n";
import { Button } from "@multica/ui/components/ui/button";
import { useWebNavigationOwner } from "./navigation-owner";
import { useWebScopeResume } from "./navigation";

/** The two application scopes share chrome, not their workspace providers. */
export function WebScopeShell({ children }: { children: ReactNode }) {
  const owner = useWebNavigationOwner();
  const navigation = useNavigation();
  const path = currentPath(navigation);
  const scope = navigationScope(path)?.scope;
  const resume = useWebScopeResume();
  const list = useWorkspaceList({ enabled: owner !== null && scope !== undefined });
  const { t } = useT("layout");
  const { t: collaboration } = useT("collaboration");
  const request = useRef(0);

  useEffect(() => {
    useScopeHistoryStore.getState().setOwner(owner);
    if (owner) useScopeHistoryStore.getState().record(owner, path);
    request.current += 1;
  }, [owner, path]);

  const openWorkspace = async () => {
    if (!owner || scope === "workspace") return;
    const state = useScopeHistoryStore.getState();
    const destination = resolveScopePath(state, owner, "workspace", list.ready ? list.workspaces.map((workspace) => workspace.slug) : undefined);
    if (destination) { resume(destination); return; }
    const attempt = ++request.current;
    const result = await list.refetch();
    if (attempt !== request.current || useScopeHistoryStore.getState().owner !== owner || !result.data) return;
    const loaded = resolveScopePath(useScopeHistoryStore.getState(), owner, "workspace", result.data.map((workspace) => workspace.slug));
    if (loaded) resume(loaded);
  };

  // One element tree for every state. Returning bare children until the viewer
  // or scope resolves would remount the whole page when the wrapper appears.
  const chrome = owner !== null && scope !== undefined;
  return <div className={chrome ? "flex h-svh min-h-0 flex-col overflow-hidden bg-app-shell [&_[data-slot=sidebar-container]]:top-12 [&_[data-slot=sidebar-container]]:h-[calc(100svh-3rem)]" : "contents"}>
    {chrome && <header className="z-20 flex h-12 shrink-0 items-center px-3">
      <GlobalScopeNavigation activeScope={scope} onWorkspace={() => void openWorkspace()} onCollaboration={() => {
        if (!owner || scope === "account") return;
        request.current += 1;
        const destination = resolveScopePath(useScopeHistoryStore.getState(), owner, "account");
        if (destination) resume(destination);
      }}>
        {scope === "account" && list.unavailable && <Button variant="ghost" size="sm" disabled={list.isFetching} onClick={() => void list.refetch()}>{collaboration(($) => $.page.refresh)}</Button>}
        {scope === "account" && list.ready && list.workspaces.length === 0 && <Button variant="ghost" size="sm" onClick={() => navigation.push("/workspaces/new")}>{t(($) => $.sidebar.create_workspace)}</Button>}
      </GlobalScopeNavigation>
    </header>}
    <div className={chrome ? "min-h-0 flex-1" : "contents"}>{children}</div>
  </div>;
}
