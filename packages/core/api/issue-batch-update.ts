import { z } from "zod";
import type { BatchUpdateIssuesOptions, BatchUpdateIssuesResponse, UpdateIssueRequest } from "../types/api";
import { parseWithFallback } from "./schema";

const endpoint = "POST /api/issues/batch-update";
const safeCount = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const canonicalUUID = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/)
  .refine((id) => id !== "00000000-0000-0000-0000-000000000000");
const completionOptions = z.object({
  completion_bindings: z.array(z.object({
    issue_id: canonicalUUID,
    authorization_id: canonicalUUID,
    operation_id: canonicalUUID,
    expected_issue_revision: safeCount,
  }).strict()).optional(),
}).strict();
const batchResponse = z.object({
  updated: safeCount,
  results: z.array(z.object({
    index: safeCount,
    issue_id: z.string().min(1),
    state: z.enum(["committed", "rejected", "not_attempted", "unknown"]),
    code: z.string().trim().min(1),
    revision: safeCount.optional(),
  })).optional(),
});

/** Keep this whitelist aligned with BatchUpdateIssues' actual mutation fields. */
export function hasBatchIssueMutation(updates: UpdateIssueRequest): boolean {
  return ["title", "description", "status", "priority", "position"].some((key) =>
    Reflect.get(updates, key) != null,
  ) || ["assignee_type", "assignee_id", "start_date", "due_date", "parent_issue_id", "project_id", "stage"].some((key) =>
    Object.prototype.hasOwnProperty.call(updates, key) && Reflect.get(updates, key) !== undefined,
  );
}

/** Validate control bindings before transport; a shared update cannot carry one grant for N items. */
export function validBatchCompletionOptions(
  ids: readonly string[], updates: UpdateIssueRequest, options?: BatchUpdateIssuesOptions,
): boolean {
  if (Object.prototype.hasOwnProperty.call(updates, "completion")) return false;
  if (options === undefined) return true;
  const parsed = completionOptions.safeParse(options);
  if (!parsed.success) return false;
  const seenIssues = new Set<string>();
  const seenAuthorizations = new Set<string>();
  const seenOperations = new Set<string>();
  return (parsed.data.completion_bindings ?? []).every((binding) => {
    if (ids.filter((id) => id === binding.issue_id).length !== 1 || seenIssues.has(binding.issue_id)
      || seenAuthorizations.has(binding.authorization_id) || seenOperations.has(binding.operation_id)) return false;
    seenIssues.add(binding.issue_id); seenAuthorizations.add(binding.authorization_id); seenOperations.add(binding.operation_id);
    return true;
  });
}

/** Schema and request correlation are one boundary for both success and error bodies. */
export function parseBatchUpdateOutcome(
  raw: unknown, ids: readonly string[], hasMutation: boolean,
): BatchUpdateIssuesResponse | undefined {
  const parsed = parseWithFallback<BatchUpdateIssuesResponse | null>(raw, batchResponse, null, { endpoint, redact: true });
  if (parsed === null || parsed.updated > ids.length) return undefined;
  const results = parsed.results;
  if (results === undefined) return parsed;
  if (results.length !== ids.length || new Set(ids).size !== ids.length
    || results.some((item, index) => item.index !== index || item.issue_id !== ids[index]
      || (item.state !== "committed" && item.revision !== undefined))
    || parsed.updated !== results.filter((item) => item.state === "committed").length) return undefined;
  if (results.every((item) => item.state === "committed")) return parsed;
  if (results.every((item) => item.state === "not_attempted" && item.code === "request_rejected")) return parsed;
  if (!hasMutation && results.every((item) => item.state === "not_attempted" && item.code === "no_changes")) return parsed;

  const failure = results.findIndex((item) => item.state !== "committed");
  if (failure < 0 || (results[failure]?.state !== "rejected" && results[failure]?.state !== "unknown")) return undefined;
  if (results.slice(failure + 1).some((item) => item.state !== "not_attempted" || item.code !== "previous_item_failed")) return undefined;
  return parsed;
}

export function isBatchUpdateComplete(outcome: BatchUpdateIssuesResponse, count: number, hasMutation: boolean): boolean {
  if (outcome.results !== undefined) {
    return hasMutation
      ? outcome.results.every((item) => item.state === "committed")
      : outcome.results.every((item) => item.state === "not_attempted" && item.code === "no_changes");
  }
  return hasMutation ? outcome.updated === count : outcome.updated === 0;
}
