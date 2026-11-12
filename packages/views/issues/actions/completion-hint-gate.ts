"use client";

import { useCallback, useEffect, useRef } from "react";
import type { Issue, IssueStatusCategory, UpdateIssueRequest } from "@multica/core/types";
import { api, ApiError, errorCode } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { classifyCompletionHintIntent, deliveryCompletionScope, isLegacyCompletionHintError, issueStatusCategory } from "@multica/core/issues";
import { isIssueStatusCategory } from "@multica/core/issue-statuses";
import { useModalStore } from "@multica/core/modals";
import { toast } from "sonner";
import { useT } from "../../i18n";
import type { IssueCompletionHintModalData } from "../../modals/issue-completion-hint";
import type { IssueCompletionAuthorizationModalData } from "../../modals/issue-completion-authorization";

export interface CompletionHintMutationOptions {
  errorMessage?: string;
  onSuccess?: (issue: Issue) => void;
  onError?: (error: unknown) => void;
  onSettled?: () => void;
}

export interface CompletionHintGateIssue {
  issue: Issue;
  targetStatus: string;
  targetCategory?: IssueStatusCategory;
  updates: Partial<UpdateIssueRequest>;
  continueMutation: () => void | Promise<void>;
  options?: CompletionHintMutationOptions;
}

function isStatusOnlyUpdate(updates: Partial<UpdateIssueRequest>): updates is { status: string } {
  const keys = Object.keys(updates);
  return keys.length === 1 && keys[0] === "status" && typeof updates.status === "string";
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    return (errorCode(error) ?? error.message) || fallback;
  }
  return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * Coordinates the non-authorizing completion hint and the real status writer.
 * The continuation is never called until the hint is either not applicable,
 * explicitly confirmed by the user, or unavailable on an old server.
 */
export function useCompletionHintGate(ownerKey?: string | null) {
  const { t } = useT("issues");
  const openModal = useModalStore((state) => state.open);
  const pendingIssueIds = useRef(new Set<string>());
  const controllers = useRef(new Map<string, AbortController>());
  const authorizationSettlers = useRef(new Map<string, () => void>());

  const clearPending = useCallback((issueId: string) => {
    pendingIssueIds.current.delete(issueId);
    controllers.current.get(issueId)?.abort();
    controllers.current.delete(issueId);
  }, []);

  const run = useCallback(
    ({ issue, targetStatus, targetCategory, updates, continueMutation, options }: CompletionHintGateIssue) => {
      if (!isStatusOnlyUpdate(updates) || updates.status === issue.status) {
        void continueMutation();
        return;
      }

      // A project-less issue and a target outside the done category never
      // need a completion hint. Decide these locally before the core helper's
      // revision guard so older projections keep their existing behavior.
      if (issue.collaboration || !issue.project_id || issueStatusCategory(issue) === "done") {
        void continueMutation();
        return;
      }
      const resolvedTargetCategory =
        targetCategory ?? (isIssueStatusCategory(targetStatus) ? targetStatus : undefined);
      if (resolvedTargetCategory && resolvedTargetCategory !== "done") {
        void continueMutation();
        return;
      }

      const intent = classifyCompletionHintIntent(issue, targetStatus, resolvedTargetCategory);
      if (intent.kind === "not_applicable") {
        void continueMutation();
        return;
      }
      if (intent.kind === "unknown") {
        toast.error(t(($) => $.detail.completion_hint.unknown_description));
        return;
      }
      if (pendingIssueIds.current.has(issue.id)) return;
      if (issue.revision == null) {
        toast.error(t(($) => $.detail.completion_hint.unknown_description));
        return;
      }

      const controller = new AbortController();
      let consumed = false;
      pendingIssueIds.current.add(issue.id);
      controllers.current.set(issue.id, controller);

      const showModal = (data: IssueCompletionHintModalData) => {
        if (controller.signal.aborted) return;
        openModal("issue-completion-hint", data as unknown as Record<string, unknown>);
      };

      const retry = async () => {
        if (controller.signal.aborted) return;
        try {
          const currentIssue = await api.getIssue(issue.id, {
            workspaceId: issue.workspace_id,
            signal: controller.signal,
          });
          if (controller.signal.aborted) return;
          clearPending(issue.id);
          run({ issue: currentIssue, targetStatus, targetCategory, updates, continueMutation, options });
        } catch (error) {
          if (controller.signal.aborted) return;
          showModal({
            issueId: issue.id,
            issueRevision: issue.revision!,
            targetStatus,
            state: "unknown",
            message: errorMessage(error, t(($) => $.detail.completion_hint.unknown_description)),
            onRetry: retry,
            onCancel: () => clearPending(issue.id),
          });
        }
      };

      const confirm = async () => {
        if (controller.signal.aborted || consumed) return;
        consumed = true;
        try {
          await continueMutation();
        } finally {
          clearPending(issue.id);
        }
      };

      const request = async () => {
        try {
          const response = await api.getDeliveryCompletionHint(
            issue.id,
            {
              expected_issue_revision: issue.revision!,
              target_status: targetStatus,
              evaluation_key: null,
            },
            { workspaceId: issue.workspace_id, signal: controller.signal },
          );
          if (controller.signal.aborted) return;
          if (response.policy_mode === "enforce" && response.target_category === "done") {
            const userId = useAuthStore.getState().user?.id;
            if (!userId || useAuthStore.getState().user?.id !== userId) throw new Error(t($ => $.detail.completion_authorization.session_changed));
            let settled = false;
            const settle = () => {
              if (settled) return;
              settled = true;
              authorizationSettlers.current.delete(issue.id);
              options?.onSettled?.();
            };
            authorizationSettlers.current.set(issue.id, settle);
            const data: IssueCompletionAuthorizationModalData = {
              issue,
              scope: deliveryCompletionScope(userId, issue.workspace_id, issue.id),
              targetStatus,
              ownerSignal: controller.signal,
              onRefresh: retry,
              onCancel: () => {
                clearPending(issue.id);
                settle();
              },
              onCompleted: (completedIssue) => {
                if (controller.signal.aborted || consumed || useAuthStore.getState().user?.id !== userId) return;
                consumed = true;
                // The core completion action already submitted the bound PUT.
                // Forward settlement only; never invoke the old plain writer.
                try { options?.onSuccess?.(completedIssue); }
                finally { settle(); }
              },
            };
            openModal("issue-completion-authorization", data as unknown as Record<string, unknown>);
            return;
          }
          if (response.hint === "not_applicable") {
            await confirm();
            return;
          }
          if (
            (response.hint === "attention" || response.hint === "matching") &&
            response.target_category === "done" &&
            response.enforced === false &&
            response.usable_for_completion === false
          ) {
            showModal({
              issueId: issue.id,
              issueRevision: issue.revision!,
              targetStatus,
              state: response.hint,
              message:
                response.hint === "matching"
                  ? t(($) => $.detail.completion_hint.matching_description)
                  : t(($) => $.detail.completion_hint.attention_description),
              onConfirm: confirm,
              onCancel: () => clearPending(issue.id),
            });
            return;
          }
          showModal({
            issueId: issue.id,
            issueRevision: issue.revision!,
            targetStatus,
            state: "unknown",
            message: t(($) => $.detail.completion_hint.unknown_description),
            onRetry: retry,
            onCancel: () => clearPending(issue.id),
          });
        } catch (error) {
          if (controller.signal.aborted) return;
          if (isLegacyCompletionHintError(error)) {
            await confirm();
            return;
          }
          const refusal = error instanceof ApiError && error.status >= 400 && error.status < 500;
          showModal({
            issueId: issue.id,
            issueRevision: issue.revision!,
            targetStatus,
            state: refusal ? "refusal" : "unknown",
            message: errorMessage(
              error,
              refusal
                ? t(($) => $.detail.completion_hint.refusal_description)
                : t(($) => $.detail.completion_hint.unknown_description),
            ),
            onRetry: refusal ? undefined : retry,
            onCancel: () => clearPending(issue.id),
          });
        }
      };

      void request();
    },
    [clearPending, openModal, t],
  );

  useEffect(
    () => () => {
      for (const controller of controllers.current.values()) controller.abort();
      controllers.current.clear();
      pendingIssueIds.current.clear();
      for (const settle of authorizationSettlers.current.values()) settle();
      authorizationSettlers.current.clear();
    },
    [ownerKey],
  );

  return run;
}
