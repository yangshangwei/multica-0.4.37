"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { createPersistStorage } from "../platform/persist-storage";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { isWorkspaceNameSelection, type WorkspaceNameSelection } from "./workspace-names";

interface WorkspaceNamePreferences {
  seriesByUser: Record<string, WorkspaceNameSelection>;
  setSeries: (userId: string | null | undefined, series: WorkspaceNameSelection) => void;
}

function readPreferences(value: unknown): Record<string, WorkspaceNameSelection> {
  if (typeof value !== "object" || value === null || !("seriesByUser" in value)) return {};
  const entries = value.seriesByUser;
  if (typeof entries !== "object" || entries === null || Array.isArray(entries)) return {};
  return Object.fromEntries(
    Object.entries(entries).filter(
      (entry): entry is [string, WorkspaceNameSelection] =>
        entry[0].trim().length > 0 && isWorkspaceNameSelection(entry[1]),
    ),
  );
}

export function createWorkspaceNamePreferencesStore(adapter: StorageAdapter = defaultStorage) {
  // A blocked device store must not prevent naming or creating a workspace.
  const storage = createPersistStorage({
    getItem: (key) => {
      try { return adapter.getItem(key); } catch { return null; }
    },
    setItem: (key, value) => {
      try { adapter.setItem(key, value); } catch { /* Keep the session preference. */ }
    },
    removeItem: (key) => {
      try { adapter.removeItem(key); } catch { /* Storage may be unavailable. */ }
    },
  });

  const store = create<WorkspaceNamePreferences>()(
    persist(
      (set) => ({
        seriesByUser: {},
        setSeries: (userId, series) => {
          if (!userId?.trim() || !isWorkspaceNameSelection(series)) return;
          if (!store.persist.hasHydrated()) void store.persist.rehydrate();
          set((state) => ({ seriesByUser: { ...state.seriesByUser, [userId]: series } }));
        },
      }),
      {
        name: "multica_workspace_name_preferences",
        storage: createJSONStorage(() => storage),
        skipHydration: true,
        partialize: (state) => ({ seriesByUser: state.seriesByUser }),
        merge: (persisted, current) => ({
          ...current,
          seriesByUser: { ...readPreferences(persisted), ...current.seriesByUser },
        }),
      },
    ),
  );

  return store;
}

// Hydrate from a client effect so the server and first client render agree.
export const useWorkspaceNamePreferences = createWorkspaceNamePreferencesStore();
