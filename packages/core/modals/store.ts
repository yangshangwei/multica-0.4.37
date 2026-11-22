"use client";

import { create } from "zustand";
import type { ApiClient } from "../api/client";

type ModalType =
  | "create-issue"
  | "quick-create-issue"
  | "create-project"
  | "feedback"
  | "issue-set-parent"
  | "issue-add-child"
  | "issue-delete-confirm"
  | "issue-run-confirm"
  | "issue-completion-hint"
  | "issue-completion-authorization"
  | null;

export interface WorkspaceIssueCreationIntent {
  api: ApiClient;
  sessionScope: string;
  userId: string;
  workspaceId: string;
  workspaceSlug: string;
  originPath: string;
  targetPath: string;
}

export interface WorkspaceIssueCreationContext {
  api: ApiClient;
  sessionScope: string;
  userId: string | null;
  path: string;
  /** Supplied only after the real route provider and API mirror agree. */
  workspaceId?: string | null;
  workspaceSlug?: string | null;
}

interface ModalStore {
  modal: ModalType;
  data: Record<string, unknown> | null;
  issueLimitRecoveryWorkspaceId: string | null;
  workspaceIssueCreation: WorkspaceIssueCreationIntent | null;
  open: (modal: NonNullable<ModalType>, data?: Record<string, unknown> | null) => void;
  close: () => void;
  queueWorkspaceIssueCreation: (intent: WorkspaceIssueCreationIntent) => void;
  cancelWorkspaceIssueCreation: () => void;
  reconcileWorkspaceIssueCreation: (
    intent: WorkspaceIssueCreationIntent,
    context: WorkspaceIssueCreationContext,
  ) => boolean;
  showIssueLimitRecovery: (workspaceId: string) => void;
  dismissIssueLimitRecovery: () => void;
}

export const useModalStore = create<ModalStore>((set, get) => ({
  modal: null,
  data: null,
  issueLimitRecoveryWorkspaceId: null,
  workspaceIssueCreation: null,
  open: (modal, data = null) => set({ modal, data, workspaceIssueCreation: null }),
  close: () => set({ modal: null, data: null, workspaceIssueCreation: null }),
  queueWorkspaceIssueCreation: (intent) =>
    set({ workspaceIssueCreation: intent, modal: null, data: null }),
  cancelWorkspaceIssueCreation: () => {
    if (get().workspaceIssueCreation) set({ workspaceIssueCreation: null });
  },
  reconcileWorkspaceIssueCreation: (intent, context) => {
    // An outgoing route effect must not retire a newer explicit action.
    if (get().workspaceIssueCreation !== intent) return false;
    if (context.api !== intent.api || context.sessionScope !== intent.sessionScope
      || context.userId !== intent.userId
      || (context.path !== intent.originPath && context.path !== intent.targetPath)) {
      set({ workspaceIssueCreation: null });
      return false;
    }
    if (context.path === intent.targetPath && context.workspaceId === intent.workspaceId
      && context.workspaceSlug === intent.workspaceSlug) {
      // Consume before opening the preference-backed form (including StrictMode).
      set({ workspaceIssueCreation: null });
      return true;
    }
    return false;
  },
  showIssueLimitRecovery: (workspaceId) =>
    set({ issueLimitRecoveryWorkspaceId: workspaceId }),
  dismissIssueLimitRecovery: () =>
    set({ issueLimitRecoveryWorkspaceId: null }),
}));
