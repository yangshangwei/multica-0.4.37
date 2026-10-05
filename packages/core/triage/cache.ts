import type { QueryClient } from "@tanstack/react-query";
import { parseWithFallback } from "../api/schema";
import { TriageUpdatedPayloadSchema } from "../api/triage-schemas";
import { issueKeys } from "../issues/queries";
import { projectKeys } from "../projects/queries";
import { dashboardKeys } from "../dashboard/queries";
import type { Issue } from "../types/issue";
import type { TriageItem, TriageListResponse, TriageUpdatedPayload } from "../types/triage";
import { triageKeys } from "./queries";

export function invalidateTriage(qc: QueryClient, wsId: string): void {
  if (!wsId) return;
  void qc.invalidateQueries({ queryKey: triageKeys.all(wsId) });
  // Admission changes membership in every normal list/aggregate. Never insert
  // pending rows into them or infer project counts from a filtered triage list.
  void qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
  void qc.invalidateQueries({ queryKey: projectKeys.all(wsId) });
  void qc.invalidateQueries({ queryKey: dashboardKeys.all(wsId) });
}

export function cacheTriageItem(qc: QueryClient, wsId: string, item: TriageItem): void {
  if (item.issue.workspace_id !== wsId) return;
  qc.setQueryData<TriageItem>(triageKeys.detail(wsId, item.issue.id), previous => previous && previous.issue.revision > item.issue.revision ? previous : item);
  qc.setQueryData<Issue>(issueKeys.detail(wsId, item.issue.id), previous => previous && (previous.revision ?? 0) > item.issue.revision ? previous : item.issue);
}

/** Safe content/comment edits share Issue identity and advance its revision.
 * Keep the review form current without refetching immutable history/imports. */
export function invalidateTriageIssue(qc: QueryClient, wsId: string, issueId: string, options: {
  admissionStatus?: string;
  deleted?: boolean;
} = {}): void {
  if (!wsId || !issueId) return;
  const admission = options.admissionStatus ?? qc.getQueryData<Issue>(issueKeys.detail(wsId, issueId))?.admission_status;
  const nonformal = admission !== undefined && admission !== "accepted" && admission !== "not_required";
  const knownItem = qc.getQueryData(triageKeys.detail(wsId, issueId)) !== undefined ||
    qc.getQueriesData<TriageListResponse>({ queryKey: triageKeys.lists(wsId) }).some(([, data]) => data?.items?.some(item => item.issue.id === issueId));
  if (!nonformal && !knownItem) return;
  void qc.invalidateQueries({ queryKey: triageKeys.detail(wsId, issueId) });
  // An edited title/priority/label can enter a previously empty filter.
  void qc.invalidateQueries({ queryKey: triageKeys.lists(wsId) });
  if (options.deleted) void qc.invalidateQueries({ queryKey: triageKeys.counts(wsId) });
}

export function onTriageUpdated(qc: QueryClient, raw: unknown): void {
  const event = parseWithFallback<TriageUpdatedPayload | null>(raw, TriageUpdatedPayloadSchema, null, { endpoint: "WS triage:updated", redact: true });
  if (!event) return;
  invalidateTriage(qc, event.workspace_id);
  if (event.issue_id) {
    void qc.invalidateQueries({ queryKey: issueKeys.timeline(event.issue_id) });
    void qc.invalidateQueries({ queryKey: issueKeys.tasks(event.issue_id) });
  }
}
