"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import {
  createWorkspaceAwareStorage,
  registerForWorkspaceRehydration,
} from "../../platform/workspace-storage";
import { defaultStorage } from "../../platform/storage";
import type { SkillCategory } from "../presentation";

// View preferences for the skills list page: sort, column visibility, and
// filters. Persisted per workspace (workspace-aware storage), per user/device
// (localStorage). Search text and row selection are deliberately NOT stored —
// they are session-scoped, and persisting them would greet returning users
// with an inexplicably narrowed list.

export type SkillSortField =
  | "name"
  | "category"
  | "usedBy"
  | "updated"
  | "created";

/** Card grid or the classic table. Persisted per workspace. */
export type SkillViewMode = "card" | "list";

export type SkillSortDirection = "asc" | "desc";

/** Per-field direction applied when the user switches TO that field. */
export const SKILL_SORT_DEFAULT_DIRECTION: Record<
  SkillSortField,
  SkillSortDirection
> = {
  name: "asc",
  category: "asc",
  usedBy: "desc",
  updated: "desc",
  created: "desc",
};

export type SkillOriginType =
  | "manual"
  | "runtime_local"
  | "clawhub"
  | "skills_sh"
  | "github";

/** Multi-select filter state. Empty array per dimension = inactive. */
export interface SkillListFilters {
  usage: ("used" | "unused")[];
  categories: SkillCategory[];
  origins: SkillOriginType[];
  agents: string[];
  creators: string[];
  /** Workspace label ids (`resource_type = "skill"`); a skill matches when it carries any of them. */
  labels: string[];
}

export const EMPTY_SKILL_FILTERS: SkillListFilters = {
  usage: [],
  categories: [],
  origins: [],
  agents: [],
  creators: [],
  labels: [],
};

// User-hideable columns. Name and the structural columns (checkbox, kebab)
// are always visible.
export type SkillColumnKey =
  | "category"
  | "labels"
  | "usedBy"
  | "source"
  | "creator"
  | "updated"
  | "created";

/** Labels, source and created are opt-in: hidden until the user enables them. */
export const DEFAULT_HIDDEN_COLUMNS: SkillColumnKey[] = ["labels", "source", "created"];

export interface SkillsViewState {
  viewMode: SkillViewMode;
  sortField: SkillSortField;
  sortDirection: SkillSortDirection;
  hiddenColumns: SkillColumnKey[];
  filters: SkillListFilters;
  /** Header click: toggles direction on the active field, otherwise switches
   *  to the field with its default direction. */
  toggleSort: (field: SkillSortField) => void;
  /** Display panel select: switches field (default direction), no toggle. */
  setSortField: (field: SkillSortField) => void;
  setSortDirection: (direction: SkillSortDirection) => void;
  toggleColumn: (key: SkillColumnKey) => void;
  toggleFilter: (key: keyof SkillListFilters, value: string) => void;
  /** Sidebar semantics: a category is single-select; picking the active one
   *  clears the dimension. `null` clears explicitly ("All"). */
  selectCategory: (category: SkillCategory | null) => void;
  clearFilters: () => void;
  setViewMode: (mode: SkillViewMode) => void;
}

const DEFAULTS = {
  viewMode: "card" as SkillViewMode,
  sortField: "updated" as SkillSortField,
  sortDirection: SKILL_SORT_DEFAULT_DIRECTION.updated,
  hiddenColumns: DEFAULT_HIDDEN_COLUMNS,
  filters: EMPTY_SKILL_FILTERS,
};

export const useSkillsViewStore = create<SkillsViewState>()(
  persist(
    (set) => ({
      ...DEFAULTS,
      toggleSort: (field) =>
        set((state) =>
          state.sortField === field
            ? {
                sortDirection: state.sortDirection === "asc" ? "desc" : "asc",
              }
            : {
                sortField: field,
                sortDirection: SKILL_SORT_DEFAULT_DIRECTION[field],
              },
        ),
      setSortField: (field) =>
        set((state) =>
          state.sortField === field
            ? {}
            : {
                sortField: field,
                sortDirection: SKILL_SORT_DEFAULT_DIRECTION[field],
              },
        ),
      setSortDirection: (direction) => set({ sortDirection: direction }),
      toggleColumn: (key) =>
        set((state) => ({
          hiddenColumns: state.hiddenColumns.includes(key)
            ? state.hiddenColumns.filter((k) => k !== key)
            : [...state.hiddenColumns, key],
        })),
      toggleFilter: (key, value) =>
        set((state) => {
          const list = state.filters[key] as string[];
          const next = list.includes(value)
            ? list.filter((v) => v !== value)
            : [...list, value];
          return { filters: { ...state.filters, [key]: next } };
        }),
      selectCategory: (category) =>
        set((state) => {
          const current = state.filters.categories;
          const next =
            category === null || (current.length === 1 && current[0] === category)
              ? []
              : [category];
          return { filters: { ...state.filters, categories: next } };
        }),
      clearFilters: () => set({ filters: EMPTY_SKILL_FILTERS }),
      setViewMode: (mode) => set({ viewMode: mode }),
    }),
    {
      name: "multica_skills_view",
      storage: createJSONStorage(() => createWorkspaceAwareStorage(defaultStorage)),
      // v1 added the opt-in `labels` column. A v0 payload persisted
      // `hiddenColumns` before that column existed, so its absence there means
      // "never seen", not "chosen to show": hide it, as a fresh workspace
      // would. The `merge` below cannot tell those two cases apart, which is
      // why this is a versioned migration and not a default backfill.
      version: 1,
      migrate: (persisted, version) => {
        const p = persisted as Partial<SkillsViewState> | undefined;
        if (version === 0 && p?.hiddenColumns && !p.hiddenColumns.includes("labels")) {
          return { ...p, hiddenColumns: [...p.hiddenColumns, "labels"] } as SkillsViewState;
        }
        return persisted as SkillsViewState;
      },
      partialize: (state) => ({
        viewMode: state.viewMode,
        sortField: state.sortField,
        sortDirection: state.sortDirection,
        hiddenColumns: state.hiddenColumns,
        filters: state.filters,
      }),
      // On rehydrate, if the new workspace has no persisted value, reset to
      // the defaults instead of leaving the previous workspace's in-memory
      // view state in place (same rationale as the agents view store).
      merge: (persisted, current) => {
        if (!persisted) return { ...current, ...DEFAULTS };
        const p = persisted as Partial<SkillsViewState>;
        // DEFAULTS sit between `current` and the payload: a key the payload
        // never stored (a v0 payload has no `viewMode`) must fall back to its
        // default, not to the previous workspace's in-memory value — after a
        // migration persist writes the merged state straight back, which
        // would make that leak permanent.
        // Deep-merge filters so a payload persisted before a new filter
        // dimension existed still gets that key's default instead of
        // dropping it to undefined (which crashes `.length` reads).
        return {
          ...current,
          ...DEFAULTS,
          ...p,
          filters: { ...EMPTY_SKILL_FILTERS, ...(p.filters ?? {}) },
        };
      },
    },
  ),
);

registerForWorkspaceRehydration(() => useSkillsViewStore.persist.rehydrate());
