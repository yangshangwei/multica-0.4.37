"use client";

import { useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { findSourceIssue, sourceCollaborationAction, type SourceCollaborationAction } from "@multica/core/issues";
import { toast } from "sonner";
import type { UpdateIssueRequest } from "@multica/core/types";
import {
  useBatchDeleteIssues,
  useBatchUpdateIssues,
  useUpdateIssue,
} from "@multica/core/issues/mutations";
import { errorCode } from "@multica/core/api";
import { useModalStore } from "@multica/core/modals";
import {
  type IssueSurfaceActions,
  type IssueSurfaceMutationOptions,
} from "./actions-context";
import type { IssueCreateDefaults } from "./types";
import { mergeIssueCreateDefaults } from "./create-defaults";
import { useT } from "../../i18n";

export type MoveIssueUpdates = Pick<
  UpdateIssueRequest,
  | "status"
  | "assignee_type"
  | "assignee_id"
  | "position"
  | "parent_issue_id"
  | "project_id"
> & {
  before_id: string | null;
  after_id: string | null;
};

export interface IssueSurfaceActionController {
  collaborationAction: SourceCollaborationAction | null;
  dismissCollaborationAction: () => void;
  actions: IssueSurfaceActions;
  openCreateIssue: (defaults?: IssueCreateDefaults) => void;
  moveIssue: (
    issueId: string,
    updates: MoveIssueUpdates,
    onSettled?: (error?: unknown) => void,
  ) => void;
}

export function useIssueSurfaceActions({
  createDefaults,
}: {
  createDefaults: IssueCreateDefaults;
}): IssueSurfaceActionController {
  const { t } = useT("projects");
  const { t: tIssues } = useT("issues");
  const { t: tCollaboration } = useT("collaboration");
  const client = useQueryClient();
  const wsId = useWorkspaceId();
  const { statuses } = useIssueStatuses(wsId);
  const [collaborationAction, setCollaborationAction] = useState<SourceCollaborationAction | null>(null);
  const updateIssueMutation = useUpdateIssue();
  const batchUpdateMutation = useBatchUpdateIssues();
  const batchDeleteMutation = useBatchDeleteIssues();

  const updateIssue = useCallback(
    (
      issueId: string,
      updates: Partial<UpdateIssueRequest>,
      options?: IssueSurfaceMutationOptions,
    ) => {
      const issue = findSourceIssue(client, wsId, issueId);
      if (issue?.collaboration) {
        const category = statuses.find((status) => status.key === updates.status)?.category;
        const action = sourceCollaborationAction(issue, updates, category);
        if (action) {
          setCollaborationAction(action);
          options?.onSettled?.();
          return;
        }
        if (updates.position === undefined) { options?.onSettled?.(); return; }
        updates = { position: updates.position, expected_revision: issue.revision };
      }
      updateIssueMutation.mutate(
        { id: issueId, ...updates },
        {
          onSuccess: (issue) => options?.onSuccess?.(issue),
          onError: (err) => {
            toast.error(
              errorCode(err) === "revision_conflict"
                ? tIssues(($) => $.revision.conflict)
                : err instanceof Error && err.message
                ? err.message
                : (options?.errorMessage ??
                    t(($) => $.detail.toast_move_issue_failed)),
            );
            options?.onError?.(err);
          },
          onSettled: () => options?.onSettled?.(),
        },
      );
    },
    [client, wsId, statuses, t, tIssues, updateIssueMutation],
  );

  const moveIssue = useCallback(
    (
      issueId: string,
      updates: MoveIssueUpdates,
      onSettled?: (error?: unknown) => void,
    ) => {
      const { before_id, after_id, ...optimisticUpdates } = updates;
      if (findSourceIssue(client, wsId, issueId)?.collaboration) {
        let error: unknown;
        updateIssue(issueId, optimisticUpdates, { onError: (err) => { error = err; }, onSettled: () => onSettled?.(error) });
        return;
      }
      updateIssueMutation.mutate(
        {
          id: issueId,
          ...optimisticUpdates,
          move_intent: { before_id, after_id },
        },
        {
          onError: (err) => {
            toast.error(
              errorCode(err) === "revision_conflict"
                ? tIssues(($) => $.revision.conflict)
                : err instanceof Error && err.message
                ? err.message
                : t(($) => $.detail.toast_move_issue_failed),
            );
          },
          onSettled: (_issue, error) => onSettled?.(error),
        },
      );
    },
    [client, wsId, updateIssue, t, tIssues, updateIssueMutation],
  );

  const openCreateIssue = useCallback(
    (defaults?: IssueCreateDefaults) => {
      useModalStore
        .getState()
        .open("create-issue", mergeIssueCreateDefaults(createDefaults, defaults));
    },
    [createDefaults],
  );

  const actions = useMemo<IssueSurfaceActions>(
    () => ({
      openCollaboration: setCollaborationAction,
      isPending:
        updateIssueMutation.isPending ||
        batchUpdateMutation.isPending ||
        batchDeleteMutation.isPending,
      createIssue: openCreateIssue,
      updateIssue,
      moveIssue: (issueId, updates, options) =>
        updateIssue(issueId, updates, {
          errorMessage: t(($) => $.detail.toast_move_issue_failed),
          ...options,
        }),
      batchUpdate: async (issueIds, updates) => {
        if (issueIds.some((id) => findSourceIssue(client, wsId, id)?.collaboration)) {
          throw new Error(tCollaboration(($) => $.source.batchUnsupported));
        }
        await batchUpdateMutation.mutateAsync({ ids: issueIds, updates });
      },
      batchDelete: async (issueIds) => {
        if (issueIds.some((id) => findSourceIssue(client, wsId, id)?.collaboration)) {
          throw new Error(tCollaboration(($) => $.source.batchUnsupported));
        }
        await batchDeleteMutation.mutateAsync(issueIds);
      },
    }),
    [
      batchDeleteMutation,
      batchUpdateMutation,
      openCreateIssue,
      t,
      updateIssue,
      updateIssueMutation.isPending,
      client,
      wsId,
      tCollaboration,
    ],
  );

  return { actions, openCreateIssue, moveIssue, collaborationAction, dismissCollaborationAction: () => setCollaborationAction(null) };
}
