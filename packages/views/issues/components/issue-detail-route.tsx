"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";
import { useStore } from "zustand";
import { useCanonicalIssue } from "@multica/core/issues/canonical-id";
import type { IterationParticipationScope } from "@multica/core/issues/stores/iteration-participation-store";
import { useWorkspaceId } from "@multica/core/hooks";
import { parseAccountPath, useWorkspacePaths } from "@multica/core/paths";
import { useIterationParticipationStoreApi, useNavigation } from "../../navigation";
import { IssueDetail, IssueDetailSkeleton, IssueNotFound, IssueLoadError } from "./issue-detail";

interface IssueDetailRouteProps {
  /**
   * Raw `/{ws}/issues/{segment}` parameter. Either a UUID (older links, and
   * anything the app itself linked before the issue row was in hand) or a
   * human-readable identifier such as `MUL-123`.
   */
  routeId: string;
  onDelete?: () => void;
}

/**
 * Rewrite `/{ws}/issues/{uuid}` to `/{ws}/issues/{identifier}` once the issue
 * is known, so the address bar and any copied URL read as `MUL-123`.
 *
 * A replace, not a push: the UUID URL is the same page, and a history entry
 * for it would make Back bounce the user between two spellings of one issue.
 */
export function useCanonicalIssueUrl(
  routeId: string,
  identifier: string | undefined,
  hash = "",
) {
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const canonicalHref = identifier ? `${paths.issueDetail(identifier)}${hash}` : null;
  // `useWorkspacePaths()` and the navigation adapter are both rebuilt on
  // render, so this ref — not the dependency array — is what guarantees the
  // replace runs once per target instead of on every commit.
  const replacedRef = useRef<string | null>(null);

  useEffect(() => {
    if (!canonicalHref || routeId === identifier) return;
    if (replacedRef.current === canonicalHref) return;
    replacedRef.current = canonicalHref;
    navigation.replace(canonicalHref);
  }, [canonicalHref, identifier, routeId, navigation]);
}

export function parseCommentHighlightHash(hash: string): string | undefined {
  const match = /^#comment-([A-Za-z0-9_-]+)$/.exec(hash);
  return match?.[1];
}

/**
 * Route-level wrapper around `IssueDetail` for `/{ws}/issues/{id}`.
 *
 * Two jobs the panel-embedded `IssueDetail` must not do:
 *  - resolve an identifier segment to the issue's UUID before rendering, so
 *    every cache key below stays UUID-keyed and realtime updates land (see
 *    `useCanonicalIssue`);
 *  - rewrite the address bar to the canonical identifier URL. That belongs to
 *    the route and only the route — the inbox renders `IssueDetail` in a side
 *    panel, where replacing the URL would navigate the user out of the inbox.
 */
export function IssueDetailRoute({ routeId, onDelete }: IssueDetailRouteProps) {
  const wsId = useWorkspaceId();
  const { canonicalId, issue, isResolving, notFound, accessDenied, error, isFetching, refetch } = useCanonicalIssue(wsId, routeId);
  // The adapter owns fragments in both browser and native MemoryRouter routes.
  const { hash = "" } = useNavigation();
  const paths = useWorkspacePaths();
  const participationStore = useIterationParticipationStoreApi();
  const generation = useStore(participationStore, (state) => state.generation);
  const currentParticipation = useStore(participationStore, (state) => state.current);
  const sourcePath = paths.issueDetail(routeId);
  const issueId = issue?.id;
  const canonicalPath = paths.issueDetail(issue?.identifier ?? routeId);
  const participationScope = useMemo<IterationParticipationScope | null>(() => issueId ? {
    workspaceId: wsId, issueId, sourcePath, canonicalPath, generation,
  } : null, [wsId, issueId, sourcePath, canonicalPath, generation]);
  const unavailable = notFound || accessDenied || (!isResolving && !issue);

  // Select before normalizing the URL: native replace can commit synchronously.
  useEffect(() => {
    if (participationScope) participationStore.getState().selectIssue(participationScope);
    else if (unavailable) participationStore.getState().clearIssue(sourcePath, generation);
  }, [participationStore, participationScope, unavailable, sourcePath, generation]);
  const selectedParticipation = participationScope && currentParticipation
    && currentParticipation.generation === generation
    && currentParticipation.workspaceId === wsId && currentParticipation.issueId === issueId
    ? currentParticipation : null;
  const onIterationHistoryOpenChange = useCallback((open: boolean) => {
    if (participationScope) participationStore.getState().setOpen(participationScope, open);
  }, [participationStore, participationScope]);
  const runId = canonicalId && hash.startsWith("#run-")
    ? parseAccountPath(`/account?issue=${canonicalId}${hash}`)?.runId ?? undefined : undefined;

  useCanonicalIssueUrl(routeId, selectedParticipation ? issue?.identifier : undefined, hash);

  if (isResolving) return <IssueDetailSkeleton />;

  // Render not-found here rather than handing the unresolved segment down.
  // `IssueDetail` would mount a second observer on the query that just failed,
  // refetch it, and restart this component's resolve/remount cycle — an
  // unbounded request loop that never settles. See `CanonicalIssue.notFound`.
  if (notFound) return <IssueNotFound showBackLink={!onDelete} />;
  if (accessDenied || !canonicalId) {
    return <IssueLoadError accessDenied={accessDenied} isRetrying={isFetching} onRetry={() => void refetch()} />;
  }

  return (
    <IssueDetail
      issueId={canonicalId}
      iterationHistoryOpen={selectedParticipation?.open ?? false}
      onIterationHistoryOpenChange={onIterationHistoryOpenChange}
      onDelete={onDelete}
      highlightCommentId={parseCommentHighlightHash(hash)}
      highlightRunId={runId}
      resolutionError={error ? { isRetrying: isFetching, onRetry: () => void refetch() } : undefined}
    />
  );
}
