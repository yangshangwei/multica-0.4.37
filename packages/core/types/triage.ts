import type { Issue, IssueAssigneeType, IssuePriority } from "./issue";

export type AdmissionStatus = "not_required" | "pending" | "accepted" | "rejected" | "duplicate" | (string & {});
export type TriageActionName = "accept" | "accept_and_execute" | "reject" | "duplicate" | "snooze" | "unsnooze" | "reopen" | "assign_reviewer";
export type TriageBatchActionName = "accept" | "reject" | "snooze" | "assign_reviewer";
export type TriageResponsibilityMode = "none" | "notify" | "assign";

export interface TriageSettings {
  supported: boolean;
  enabled: boolean;
  acceptance_status: string;
  require_priority: boolean;
  responsibility_mode: TriageResponsibilityMode;
  /** User UUID, not the workspace-member row UUID. */
  responsibility_member_id: string | null;
  revision: number;
}
export interface UpdateTriageSettingsInput {
  enabled: boolean;
  acceptance_status: string;
  require_priority: boolean;
  responsibility_mode: TriageResponsibilityMode;
  responsibility_member_id: string | null;
  expected_revision: number;
}
export interface TriageFields {
  title?: string;
  description?: string;
  status?: string;
  priority?: IssuePriority;
  project_id?: string | null;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  label_ids?: string[];
  start_date?: string | null;
  due_date?: string | null;
}
export interface TriageItem {
  issue: Issue & { admission_status: AdmissionStatus; revision: number };
  candidate_project_id: string | null;
  candidate_assignee_type: IssueAssigneeType | null;
  candidate_assignee_id: string | null;
  reviewer_id: string | null;
  reviewer_valid: boolean;
  round: number;
  first_entered_at: string;
  entered_at: string;
  snoozed_until: string | null;
  duplicate_issue_id: string | null;
  duplicate_identifier: string | null;
  source: "manual" | "csv" | (string & {});
  source_url: string | null;
  external_id: string | null;
  batch_id: string | null;
  filename: string | null;
  row_number: number | null;
}
export interface TriageAction {
  id: string;
  issue_id: string;
  actor_id: string;
  action: TriageActionName | (string & {});
  round: number;
  reason: string | null;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
  created_at: string;
  execution_status: "not_requested" | "pending" | "queued" | "failed" | (string & {});
  task_id: string | null;
  execution_error: string | null;
}
export interface TriageActionInput {
  /** Create once for a user intention; preserve this value on transport retry. */
  request_id: string;
  expected_revision: number;
  action: TriageActionName;
  reason?: string;
  duplicate_issue_id?: string;
  snoozed_until?: string;
  reviewer_id?: string | null;
  fields?: TriageFields;
}
export interface TriageActionResult { item: TriageItem; action: TriageAction }
export interface TriageCounts { pending: number; ready: number; snoozed: number }
export interface TriageListParams {
  view?: "ready" | "all" | "snoozed" | "history";
  q?: string;
  source?: string;
  priority?: string;
  project_id?: string;
  label_id?: string;
  reviewer_id?: string;
  creator_id?: string;
  entered_after?: string;
  entered_before?: string;
  result?: string;
  processed_by?: string;
  processed_after?: string;
  processed_before?: string;
  sort?: "oldest" | "newest" | "priority";
  limit?: number;
  offset?: number;
}
export interface TriageListResponse { items: TriageItem[]; total: number; counts: TriageCounts; limit: number; offset: number }
export interface CreateTriageItemInput {
  request_id: string;
  title: string;
  description?: string;
  priority?: IssuePriority;
  candidate_project_id?: string | null;
  candidate_assignee_type?: IssueAssigneeType | null;
  candidate_assignee_id?: string | null;
  label_ids?: string[];
  start_date?: string | null;
  due_date?: string | null;
  attachment_ids?: string[];
  source_url?: string;
}
export interface TriageItemHistory { events: TriageAction[] }
export interface TriageHistoryParams {
  q?: string;
  result?: string;
  processed_by?: string;
  processed_after?: string;
  processed_before?: string;
  source?: string;
  limit?: number;
  offset?: number;
}
export interface TriageImportCounts { created: number; skipped: number; failed: number }
export interface TriageHistoryEntry {
  id: string;
  kind: "action" | "import" | (string & {});
  issue_id: string | null;
  identifier: string | null;
  title: string;
  action: string;
  actor_id: string;
  created_at: string;
  reason: string | null;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
  batch_id: string | null;
  filename: string | null;
  counts: TriageImportCounts | null;
}
export interface TriageHistoryResponse { entries: TriageHistoryEntry[]; total: number; limit: number; offset: number }
export interface TriageBatchPreviewInput {
  items: { issue_id: string; expected_revision: number }[];
  action: TriageBatchActionName;
  reason?: string;
  snoozed_until?: string;
  reviewer_id?: string | null;
  fields?: TriageFields;
}
export interface TriageBatchPreview { items: { issue_id: string; expected_revision: number; valid: boolean; error: string | null }[]; valid_count: number }
export interface TriageBatchInput { items: (Omit<TriageActionInput, "action"> & { issue_id: string; action: TriageBatchActionName })[] }
export interface TriageBatchRowResult {
  issue_id: string;
  status: "success" | "conflict" | "invalid" | "forbidden" | "failed" | (string & {});
  result?: TriageActionResult;
  error?: string;
}
export interface TriageBatchResult { results: TriageBatchRowResult[]; success_count: number }
export interface TriageImportPreviewInput { request_id: string; filename: string; csv: string; mapping?: Record<string, string> }
export interface TriageImportRow {
  row_number: number;
  values: Record<string, string>;
  warnings: string[];
  errors: string[];
  duplicate: boolean;
  duplicate_issue_id: string | null;
  similar_issue_ids: string[];
  status: "ready" | "created" | "skipped" | "failed" | (string & {});
  issue_id: string | null;
  error: string | null;
}
export interface TriageImportPreview {
  batch_id: string;
  filename: string;
  headers: string[];
  mapping: Record<string, string>;
  rows: TriageImportRow[];
  counts: { valid: number; warning: number; error: number; duplicate: number };
  limits: { max_rows: number; max_bytes: number };
}
export interface TriageImportCommitInput { rows: { row_number: number; import_duplicate: boolean }[] }
export interface TriageImportResult extends TriageImportCounts { batch_id: string; results: TriageImportRow[] }
export interface TriageUpdatedPayload { workspace_id: string; issue_id?: string; batch_id?: string; settings_changed?: boolean }
