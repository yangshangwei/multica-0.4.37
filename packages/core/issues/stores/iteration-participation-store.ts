import { createStore } from "zustand/vanilla";

export interface IterationParticipationScope {
  workspaceId: string;
  issueId: string;
  sourcePath: string;
  canonicalPath: string;
  generation: number;
}

interface CurrentParticipation extends IterationParticipationScope {
  open: boolean;
}

export interface IterationParticipationState {
  activePath: string;
  generation: number;
  current: CurrentParticipation | null;
  syncPath: (pathname: string) => void;
  selectIssue: (scope: IterationParticipationScope) => void;
  setOpen: (scope: IterationParticipationScope, open: boolean) => void;
  clearIssue: (pathname: string, generation: number) => void;
  reset: () => void;
}

function containsPath(scope: IterationParticipationScope, pathname: string) {
  return pathname === scope.sourcePath || pathname === scope.canonicalPath;
}

function sameIssue(a: IterationParticipationScope, b: IterationParticipationScope) {
  return a.workspaceId === b.workspaceId && a.issueId === b.issueId;
}

/** One transient disclosure intent for one mounted navigation surface. */
export function createIterationParticipationStore(pathname = "") {
  return createStore<IterationParticipationState>()((set) => ({
    activePath: pathname,
    generation: 0,
    current: null,
    syncPath: (activePath) => set((state) => {
      if (state.activePath === activePath) return state;
      if (state.current && containsPath(state.current, activePath)) {
        return { activePath };
      }
      return { activePath, current: null, generation: state.generation + 1 };
    }),
    selectIssue: (scope) => set((state) => {
      if (scope.generation !== state.generation || !containsPath(scope, state.activePath)) return state;
      if (state.current && sameIssue(state.current, scope)) return state;
      // A changed identity at the same address must also fence its old callbacks.
      const generation = state.generation + (state.current ? 1 : 0);
      return { generation, current: { ...scope, generation, open: false } };
    }),
    setOpen: (scope, open) => set((state) => {
      if (scope.generation !== state.generation || !containsPath(scope, state.activePath)) return state;
      if (state.current && !sameIssue(state.current, scope)) return state;
      if (state.current?.open === open) return state;
      return { current: { ...(state.current ?? scope), open } };
    }),
    clearIssue: (activePath, generation) => set((state) => {
      if (generation !== state.generation || activePath !== state.activePath || !state.current) return state;
      return { current: null, generation: state.generation + 1 };
    }),
    reset: () => set((state) => ({ current: null, generation: state.generation + 1 })),
  }));
}

export type IterationParticipationStore = ReturnType<typeof createIterationParticipationStore>;

const liveStores = new Set<IterationParticipationStore>();

/** Register only after a provider mounts; constructing a store is SSR-pure. */
export function registerIterationParticipationStore(store: IterationParticipationStore) {
  liveStores.add(store);
  return () => { liveStores.delete(store); };
}

export function resetIterationParticipationStores() {
  for (const store of liveStores) store.getState().reset();
}
