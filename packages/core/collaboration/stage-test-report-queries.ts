import { infiniteQueryOptions, queryOptions, replaceEqualDeep, useInfiniteQuery, useQuery, useQueryClient, type InfiniteData, type QueryClient, type QueryKey } from "@tanstack/react-query";
import { getApi } from "../api";
import { CollaborationUnavailableError, type CollaborationScope } from "./access";
import { collaborationIssueOptions } from "./queries";
import { collaborationStageKeys } from "./stage-queries";
import { eraseCollaborationStageSource, protectCollaborationStageSource } from "./stage-source-cache";
import type { CollaborationTestReportRead, CollaborationTestReports } from "./stage-test-report-schema";

const freshness = { staleTime: 0, retry: false, refetchOnMount: "always" as const, refetchOnWindowFocus: "always" as const, refetchOnReconnect: "always" as const };
export const collaborationTestReportKeys = {
  run: (scope: CollaborationScope, issueId: string, workspaceId: string, runId: string) => [...collaborationStageKeys.root(scope, issueId), "test-reports", workspaceId.toLowerCase(), runId.toLowerCase()] as const,
  detail: (scope: CollaborationScope, issueId: string, workspaceId: string, runId: string, reportId: string) => [...collaborationTestReportKeys.run(scope, issueId, workspaceId, runId), reportId.toLowerCase()] as const,
};

async function read<T>(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string, queryKey: QueryKey, request: () => Promise<CollaborationTestReportRead<T>>) {
  const result = await protectCollaborationStageSource(client, scope, issueId, workspaceId, request);
  if (result.state === "not_found") {
    // T4 deliberately conflates source/issue denial with missing reports. Only
    // the authoritative issue re-read may retire independent issue access.
    eraseCollaborationStageSource(client, scope, workspaceId, queryKey);
    await client.fetchQuery(collaborationIssueOptions(client, scope, issueId));
  }
  return result;
}

type Pages = InfiniteData<CollaborationTestReportRead<CollaborationTestReports>, string | undefined>;
function sharePages(previous: unknown, next: unknown): unknown {
  if (next === undefined) return undefined;
  const data = next as Pages;
  const ids = new Set<string>(), cursors = new Set<string>();
  let identity: string | undefined;
  for (const [index, page] of data.pages.entries()) {
    const cursor = data.pageParams[index];
    if (index > 0 && (cursor === undefined || cursors.has(cursor))) throw new CollaborationUnavailableError();
    const prior = data.pages[index - 1];
    if (prior && (prior.state !== "available" || prior.value.next_cursor !== cursor)) throw new CollaborationUnavailableError();
    if (cursor !== undefined) cursors.add(cursor);
    if (page.state !== "available") { if (index > 0) throw new CollaborationUnavailableError(); continue; }
    const current = `${page.value.issue_id}:${page.value.run_id}`;
    if (identity !== undefined && identity !== current) throw new CollaborationUnavailableError();
    identity = current;
    if (page.value.next_cursor !== null && cursors.has(page.value.next_cursor)) throw new CollaborationUnavailableError();
    for (const item of page.value.items) {
      if (ids.has(item.report_id)) throw new CollaborationUnavailableError();
      ids.add(item.report_id);
    }
  }
  return replaceEqualDeep(previous, data);
}
export function collaborationTestReportsOptions(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string, runId: string) {
  const queryKey = collaborationTestReportKeys.run(scope, issueId, workspaceId, runId);
  return infiniteQueryOptions({ queryKey, ...freshness, initialPageParam: undefined as string | undefined, structuralSharing: sharePages,
    queryFn: ({ signal, pageParam }) => read(client, scope, issueId, workspaceId, queryKey, () => getApi().getCollaborationTestReports(issueId, runId, { limit: 20, cursor: pageParam }, { signal })),
    getNextPageParam: (page) => page.state === "available" ? page.value.next_cursor ?? undefined : undefined,
  });
}
export function collaborationTestReportOptions(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string, runId: string, reportId: string) {
  const queryKey = collaborationTestReportKeys.detail(scope, issueId, workspaceId, runId, reportId);
  return queryOptions({ queryKey, ...freshness, refetchInterval: 15_000, refetchIntervalInBackground: false,
    queryFn: ({ signal }) => read(client, scope, issueId, workspaceId, queryKey, () => getApi().getCollaborationTestReport(issueId, runId, reportId, { signal })),
  });
}
export function useCollaborationTestReport(scope: CollaborationScope, issueId: string, workspaceId: string, report: { run_id: string; report_id: string } | null) {
  const client = useQueryClient();
  return useQuery({ ...collaborationTestReportOptions(client, scope, issueId, workspaceId, report?.run_id ?? "", report?.report_id ?? ""), enabled: report !== null && scope.userId !== "" });
}
export function useCollaborationTestReports(scope: CollaborationScope, issueId: string, workspaceId: string, runId: string | null) {
  const client = useQueryClient();
  return useInfiniteQuery({ ...collaborationTestReportsOptions(client, scope, issueId, workspaceId, runId ?? ""), enabled: runId !== null && scope.userId !== "" });
}
