import type { QueryClient } from "@tanstack/react-query";
import type { Issue, UpdateIssueRequest } from "../types";
import { issueKeys } from "./queries";
import { bucketedListEntries, flatListEntries, issueArrayEntries, tableRowEntries } from "./cache-coordinator";

export type SourceCollaborationAction = { issueId: string; action: "transfer" | "change_status" | "edit" | "lifecycle"; status?: string };

/** Source cards carry display data; workflow authority is reloaded in the dialog. */
export function sourceCollaborationAction(issue: Issue, updates: Partial<UpdateIssueRequest>, targetCategory?: string): SourceCollaborationAction | null {
  if (!issue.collaboration) return null;
  if ((updates.assignee_id !== undefined && updates.assignee_id !== issue.assignee_id)
    || (updates.assignee_type !== undefined && updates.assignee_type !== issue.assignee_type)) {
    return { issueId: issue.id, action: "transfer" };
  }
  if (updates.status !== undefined && updates.status !== issue.status) {
    return { issueId: issue.id, action: ["done", "cancelled"].includes(targetCategory ?? updates.status) ? "lifecycle" : "change_status", status: updates.status };
  }
  const changed = Object.entries(updates).some(([key, value]) => !["position", "expected_revision", "before_id", "after_id"].includes(key)
    && value !== undefined && value !== issue[key as keyof Issue]);
  return changed ? { issueId: issue.id, action: "edit" } : null;
}

/** Reuse the shape-checked cache readers used by ordinary mutation reconciliation. */
export function findSourceIssue(client: QueryClient, wsId: string, issueId: string): Issue | undefined {
  const matches: Issue[] = [];
  const detail = client.getQueryData<Issue>(issueKeys.detail(wsId, issueId));
  if (detail) matches.push(detail);
  for (const [, data] of tableRowEntries(client, wsId)) for (const row of data.rows) if (row.issue.id === issueId) matches.push(row.issue);
  for (const [, data] of bucketedListEntries(client, wsId)) for (const bucket of Object.values(data.byStatus)) {
    const issue = bucket?.issues.find((item) => item.id === issueId);
    if (issue) matches.push(issue);
  }
  for (const [, data] of flatListEntries(client, wsId)) for (const page of data.pages) {
    const issue = page.issues.find((item) => item.id === issueId);
    if (issue) matches.push(issue);
  }
  for (const [, data] of issueArrayEntries(client, issueKeys.all(wsId))) {
    const issue = data.find((item) => item.id === issueId);
    if (issue) matches.push(issue);
  }
  // The immutable relationship cannot disappear in a partial/older projection.
  const shared = matches.filter((issue) => issue.collaboration != null);
  return (shared.length ? shared : matches).sort((a, b) => (b.revision ?? 0) - (a.revision ?? 0))[0];
}
