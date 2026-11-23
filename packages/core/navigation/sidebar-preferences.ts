"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { createPersistStorage } from "../platform/persist-storage";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";

export type SidebarGroup = "work" | "ai" | "resources" | "collaboration";
const defaults: Record<SidebarGroup, boolean> = { work: true, ai: false, resources: false, collaboration: false };

interface SidebarPreferences {
  groups: Record<SidebarGroup, boolean>;
  setGroupOpen(group: SidebarGroup, open: boolean): void;
}

function readGroups(value: unknown): Record<SidebarGroup, boolean> {
  const result = { ...defaults };
  if (!value || typeof value !== "object" || !("groups" in value)) return result;
  const groups = value.groups;
  if (!groups || typeof groups !== "object" || Array.isArray(groups)) return result;
  for (const group of Object.keys(defaults) as SidebarGroup[]) {
    const candidate = Object.entries(groups).find(([key]) => key === group)?.[1];
    if (typeof candidate === "boolean") result[group] = candidate;
  }
  return result;
}

/** Only durable disclosure choices are stored; active-route reveal is view state. */
export function createSidebarPreferencesStore(adapter: StorageAdapter = defaultStorage) {
  const storage = createPersistStorage({
    getItem: (key) => { try { return adapter.getItem(key); } catch { return null; } },
    setItem: (key, value) => { try { adapter.setItem(key, value); } catch { /* Keep the session preference. */ } },
    removeItem: (key) => { try { adapter.removeItem(key); } catch { /* Storage may be unavailable. */ } },
  });
  return create<SidebarPreferences>()(persist((set) => ({
    groups: { ...defaults },
    setGroupOpen: (group, open) => set((state) => ({ groups: { ...state.groups, [group]: open } })),
  }), {
    name: "multica_sidebar_preferences",
    storage: createJSONStorage(() => storage),
    skipHydration: true,
    partialize: (state) => ({ groups: state.groups }),
    merge: (persisted, current) => ({ ...current, groups: readGroups(persisted) }),
  }));
}

export const useSidebarPreferences = createSidebarPreferencesStore();
