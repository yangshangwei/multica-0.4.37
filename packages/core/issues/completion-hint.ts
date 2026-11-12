import type {
  DeliveryCompletionHintRequest,
  Issue,
  IssueStatus,
} from "../types";
import { isIssueStatusCategory } from "../issue-statuses";
import { issueStatusCategory } from "./status-category";

export type CompletionHintIntent =
  | {
      kind: "requires_hint";
      request: DeliveryCompletionHintRequest;
    }
  | {
      kind: "not_applicable";
      reason: "already_done" | "target_not_done";
    }
  | {
      kind: "unknown";
      reason: "missing_revision" | "missing_source_category" | "missing_target_category";
    };

/**
 * Classify one status-only transition before any optimistic mutation starts.
 * The server remains authoritative for policy and evidence; this helper only
 * decides whether the caller has enough local facts to request a hint.
 */
export function classifyCompletionHintIntent(
  issue: Pick<Issue, "status" | "status_category" | "revision">,
  targetStatus: IssueStatus,
  targetCategory?: string,
): CompletionHintIntent {
  const revision = issue.revision;
  if (revision === undefined || !Number.isSafeInteger(revision) || revision < 0) {
    return { kind: "unknown", reason: "missing_revision" };
  }

  const sourceCategory = issueStatusCategory(issue);
  if (!sourceCategory) {
    return { kind: "unknown", reason: "missing_source_category" };
  }
  if (sourceCategory === "done") {
    return { kind: "not_applicable", reason: "already_done" };
  }

  if (!targetCategory || !isIssueStatusCategory(targetCategory)) {
    return { kind: "unknown", reason: "missing_target_category" };
  }
  if (targetCategory !== "done") {
    return { kind: "not_applicable", reason: "target_not_done" };
  }

  return {
    kind: "requires_hint",
    request: {
      expected_issue_revision: revision,
      target_status: targetStatus,
      evaluation_key: null,
    },
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasStructuredDeliveryError(body: unknown): boolean {
  if (!isRecord(body)) return false;
  return [
    "code",
    "error_code",
    "resource",
    "resource_type",
    "resource_id",
    "permission",
    "permission_code",
    "required_role",
    "delivery",
    "delivery_code",
  ].some((key) => key in body);
}

/**
 * Only an unstructured 404/405 may mean that the hint route is absent on an
 * older server. Structured resource, permission and delivery errors remain
 * real refusals and must never be bypassed.
 */
export function isLegacyCompletionHintError(error: unknown): boolean {
  if (!isRecord(error)) return false;
  const status = error.status;
  if (status !== 404 && status !== 405) return false;
  return !hasStructuredDeliveryError(error.body);
}
