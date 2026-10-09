import { queryOptions } from "@tanstack/react-query";
import { api, ApiError } from "../api";
import { issueStatusCategory } from "../issues/status-category";
import type { Issue, IssueStatusCategory } from "../types";

export type IterationCandidateBlock =
  | "member"
  | "cancelled"
  | "triage"
  | "closed_source"
  | "completed_needs_active";

export interface IterationCandidateState {
  /** Why the task cannot join the target; null when it can. */
  blocked: IterationCandidateBlock | null;
  /** The planned/active iteration the task would leave, if any. */
  sourceId: string | null;
  /** Leaving another iteration requires a reason on the server. */
  needsReason: boolean;
  /** A completed task joins only an active target, with explicit consent. */
  needsCompletedConfirmation: boolean;
}

type CandidateIssue = Pick<
  Issue,
  "status" | "status_category" | "admission_status" | "current_iteration_id"
>;

/**
 * Mirrors the server's move validation (iteration_lifecycle_preview.go) so a
 * picker can explain each task before the preview runs. The preview remains
 * authoritative; this only decides what to offer and which inputs to ask for.
 *
 * `openIterationIds` is null while the catalogue loads: membership of an
 * unknown iteration is then treated as movable and left to the preview.
 */
export function classifyIterationCandidate(
  issue: CandidateIssue,
  target: { id: string; status: string },
  openIterationIds: ReadonlySet<string> | null,
): IterationCandidateState {
  const sourceId = issue.current_iteration_id ?? null;
  const category = issueStatusCategory(issue);
  const state = (blocked: IterationCandidateBlock | null): IterationCandidateState => ({
    blocked,
    sourceId,
    needsReason: blocked === null && sourceId !== null,
    needsCompletedConfirmation: blocked === null && category === "done",
  });
  if (sourceId === target.id) return state("member");
  if (category === "cancelled") return state("cancelled");
  if (![undefined, "not_required", "accepted"].includes(issue.admission_status))
    return state("triage");
  if (sourceId !== null && openIterationIds !== null && !openIterationIds.has(sourceId))
    return state("closed_source");
  if (category === "done" && target.status !== "active")
    return state("completed_needs_active");
  return state(null);
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const IDENTIFIER = /^[A-Za-z][A-Za-z0-9]*-\d+$/;
export const ISSUE_REFERENCE_LIMIT = 50;

/**
 * A query made only of task identifiers or UUIDs is a pasted list to resolve
 * exactly; anything else is a text search. Returns null for a text search.
 */
export function parseIssueReferences(
  query: string,
): { tokens: string[]; truncated: boolean } | null {
  const raw = query.split(/[\s,，;；、]+/).filter(Boolean);
  if (raw.length === 0 || raw.some((token) => !UUID.test(token) && !IDENTIFIER.test(token)))
    return null;
  const seen = new Set<string>();
  const tokens: string[] = [];
  for (const token of raw) {
    const key = token.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    tokens.push(token);
  }
  return {
    tokens: tokens.slice(0, ISSUE_REFERENCE_LIMIT),
    truncated: tokens.length > ISSUE_REFERENCE_LIMIT,
  };
}

export interface IterationCandidateResults {
  issues: Issue[];
  /** Pasted references that did not resolve to a task in this workspace. */
  missing: string[];
  truncated: boolean;
}

export const iterationCandidateSearchOptions = (wsId: string, query: string) =>
  queryOptions({
    // Under the issues prefix so task writes invalidate stale candidates.
    queryKey: ["issues", wsId, "iteration-candidates", "search", query],
    enabled: query.trim() !== "",
    queryFn: async ({ signal }): Promise<IterationCandidateResults> => {
      const references = parseIssueReferences(query);
      if (!references) {
        const { issues } = await api.searchIssues({
          q: query.trim(),
          limit: 20,
          include_closed: true,
          signal,
        });
        return {
          issues: issues.filter((issue) => issue.workspace_id === wsId),
          missing: [],
          truncated: false,
        };
      }
      const settled = await Promise.allSettled(
        references.tokens.map((token) => api.getIssue(token, { signal })),
      );
      const issues: Issue[] = [];
      const missing: string[] = [];
      const seen = new Set<string>();
      settled.forEach((result, index) => {
        if (result.status === "rejected") {
          // Only "no such task" is an answer; a failed lookup is not.
          if (!(result.reason instanceof ApiError && result.reason.status === 404)) throw result.reason;
          missing.push(references.tokens[index]!);
        } else if (result.value.workspace_id !== wsId) {
          missing.push(references.tokens[index]!);
        } else if (!seen.has(result.value.id)) {
          seen.add(result.value.id);
          issues.push(result.value);
        }
      });
      return { issues, missing, truncated: references.truncated };
    },
    retry: false,
  });

export const UNPLANNED_CANDIDATE_LIMIT = 30;
const OPEN_CATEGORIES: IssueStatusCategory[] = ["backlog", "todo", "in_progress", "in_review", "blocked"];

/**
 * Recently updated open work outside every iteration: what a planner adds by
 * default. The server filters and counts; the client re-checks because an
 * older backend ignores the facets and would return planned or closed work.
 */
export const iterationUnplannedCandidatesOptions = (wsId: string) =>
  queryOptions({
    queryKey: ["issues", wsId, "iteration-candidates", "unplanned"],
    queryFn: async () => {
      const { issues, total } = await api.listIssues({
        include_no_iteration: true,
        status_categories: OPEN_CATEGORIES,
        sort_by: "updated_at",
        sort_direction: "desc",
        limit: UNPLANNED_CANDIDATE_LIMIT,
      });
      const open = issues.filter((issue) => {
        const category = issueStatusCategory(issue);
        return issue.workspace_id === wsId && !issue.current_iteration_id && category !== "done" && category !== "cancelled";
      });
      return { issues: open, more: total > issues.length };
    },
    retry: false,
  });
