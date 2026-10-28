"use client";

import { create } from "zustand";
import { accountPath, isReservedSlug, parseAccountPath, paths } from "../paths";

export type NavigationScope = "workspace" | "account";

/** Only internal destinations belong to the two resumable application scopes. */
export function navigationScope(path: string): { scope: NavigationScope; slug?: string } | null {
  if (!path.startsWith("/") || path.startsWith("//") || path.includes("\\") || [...path].some((character) => character.charCodeAt(0) <= 0x20)) return null;
  const url = new URL(path, "https://navigation.invalid");
  if (parseAccountPath(path)) return { scope: "account" };
  const slug = url.pathname.split("/")[1];
  if (!slug || isReservedSlug(slug) || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) return null;
  return { scope: "workspace", slug };
}

interface ScopeHistoryState {
  owner: string | null;
  workspacePath: string | null;
  accountPath: string | null;
  setOwner(owner: string | null): void;
  record(owner: string, path: string): void;
}

/** Browser-tab lifetime only. Server data and navigation mementos live elsewhere. */
export const useScopeHistoryStore = create<ScopeHistoryState>((set) => ({
  owner: null,
  workspacePath: null,
  accountPath: null,
  setOwner: (owner) => set((state) => state.owner === owner ? state : { owner, workspacePath: null, accountPath: null }),
  record: (owner, path) => {
    const location = navigationScope(path);
    if (!location) return;
    set((state) => {
      if (state.owner !== owner) return state;
      const key = location.scope === "workspace" ? "workspacePath" : "accountPath";
      return state[key] === path ? state : { [key]: path };
    });
  },
}));

/** Undefined membership means unknown, while [] is a confirmed empty account. */
export function resolveScopePath(
  state: Pick<ScopeHistoryState, "owner" | "workspacePath" | "accountPath">,
  owner: string,
  scope: NavigationScope,
  workspaceSlugs?: readonly string[],
): string | null {
  const remembered = state.owner === owner ? state : null;
  if (scope === "account") return remembered?.accountPath ?? accountPath();
  const previous = remembered?.workspacePath;
  const slug = previous ? navigationScope(previous)?.slug : undefined;
  if (previous && slug && (workspaceSlugs === undefined || workspaceSlugs.includes(slug))) return previous;
  if (workspaceSlugs === undefined) return null;
  const available = workspaceSlugs.find((value) => navigationScope(`/${value}/issues`)?.slug === value);
  return available ? paths.workspace(available).issues() : "/workspaces/new";
}
