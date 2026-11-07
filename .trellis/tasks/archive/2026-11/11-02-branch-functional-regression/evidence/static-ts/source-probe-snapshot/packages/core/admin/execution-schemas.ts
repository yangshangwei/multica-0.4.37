import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { adminVersionSchema, executionFenceSchema } from "./control-schema";
export const executionStatuses = ["queued", "preparing", "dispatched", "running", "waiting_local_directory", "completed", "failed", "cancelled", "deferred", "unknown"] as const;
export const executionSources = ["issue", "autopilot_issue", "chat", "autopilot", "quick_create", "unknown"] as const;
const time = z.iso.datetime({ offset: true });
const optionalTime = time.nullish().transform((v) => v ?? null);
const optionalId = z.uuid().nullish().transform((v) => v ?? null);
const contentUrl = z.string().regex(/^\/[a-z0-9-]+\/(?:issues\/[0-9a-f-]{36}|chat\?session=[0-9a-f-]{36})$/).nullish().transform((v) => v ?? null);
const content = {
  title: z.string().nullable().optional(), content_access: z.boolean().default(false), content_url: contentUrl,
};
const usage = z.object({
  input_tokens: z.number().int().nonnegative(), output_tokens: z.number().int().nonnegative(), cache_read_tokens: z.number().int().nonnegative(), cache_write_tokens: z.number().int().nonnegative()
}).transform((v) => ({
  inputTokens: v.input_tokens, outputTokens: v.output_tokens, cacheReadTokens: v.cache_read_tokens, cacheWriteTokens: v.cache_write_tokens
}));
const taskSchema = z.object({
  id: z.uuid(), workspace_id: z.uuid(), agent_id: z.uuid(), issue_id: optionalId, runtime_id: optionalId, chat_session_id: optionalId,
  autopilot_run_id: optionalId, parent_task_id: optionalId, retry_of_task_id: optionalId, rerun_of_task_id: optionalId,
  accountable_user_id: optionalId, submitted_installation_id: optionalId, execution_installation_id: optionalId,
  status: z.enum(executionStatuses).catch("unknown"), source: z.enum(executionSources).catch("unknown"), attempt: z.number().int().positive(),
  created_at: time, dispatched_at: optionalTime, started_at: optionalTime, completed_at: optionalTime,
  provider: z.string().nullable().optional(), model: z.string().nullable().optional(), failure_code: z.string().nullable().optional(),
  usage: usage.nullish().transform((v) => v ?? null), ...content,
  state_version: adminVersionSchema.nullish().transform(v => v ?? null),
  execution_fence: executionFenceSchema.nullish().transform(v => v ?? null),
  allowed_actions: z.array(z.string()).nullish().transform(v => v ?? []),
}).transform((v) => ({
  id: v.id, workspaceId: v.workspace_id, agentId: v.agent_id, issueId: v.issue_id, runtimeId: v.runtime_id,
  chatSessionId: v.chat_session_id, autopilotRunId: v.autopilot_run_id, parentTaskId: v.parent_task_id,
  retryOfTaskId: v.retry_of_task_id, rerunOfTaskId: v.rerun_of_task_id, accountableUserId: v.accountable_user_id,
  submittedInstallationId: v.submitted_installation_id, executionInstallationId: v.execution_installation_id,
  status: v.status, source: v.source, attempt: v.attempt, createdAt: v.created_at, dispatchedAt: v.dispatched_at,
  startedAt: v.started_at, completedAt: v.completed_at, provider: v.provider ?? null, model: v.model ?? null,
  failureCode: v.failure_code ?? null, usage: v.usage,
  stateVersion: v.state_version, executionFence: v.execution_fence, allowedActions: v.allowed_actions,
  contentAccess: v.content_access === true && v.content_url !== null,
  title: v.content_access === true && v.content_url !== null ? v.title ?? null : null, contentUrl: v.content_access === true ? v.content_url : null,
}));
const issueSchema = z.object({
  id: z.uuid(), workspace_id: z.uuid(), number: z.number().int().nonnegative(), identifier: z.string(), status: z.string(), created_at: time, updated_at: time, execution_count: z.number().int().nonnegative(), ...content
}).transform((v) => ({
  id: v.id, workspaceId: v.workspace_id, number: v.number, identifier: v.identifier, status: v.status, createdAt: v.created_at, updatedAt: v.updated_at, executionCount: v.execution_count, contentAccess: v.content_access === true && v.content_url !== null, title: v.content_access === true && v.content_url !== null ? v.title ?? null : null, contentUrl: v.content_access === true ? v.content_url : null
}));
const envelope = {
  next_cursor: z.string().nullable().default(null), as_of: time, scope: z.uuid(), time_from: optionalTime, time_to: optionalTime
};
const tasksSchema = z.object({ items: z.array(taskSchema), ...envelope }).transform((v) => ({
  items: v.items, nextCursor: v.next_cursor, asOf: v.as_of, scope: v.scope, timeFrom: v.time_from, timeTo: v.time_to
}));
const issuesSchema = z.object({ items: z.array(issueSchema), ...envelope }).transform((v) => ({
  items: v.items, nextCursor: v.next_cursor, asOf: v.as_of, scope: v.scope, timeFrom: v.time_from, timeTo: v.time_to
}));
export type AdminExecution = z.output<typeof taskSchema>;
export type AdminIssue = z.output<typeof issueSchema>;
export type AdminExecutionList = z.output<typeof tasksSchema>;
export type AdminIssueList = z.output<typeof issuesSchema>;
export const parseAdminExecution = (raw: unknown): AdminExecution | null => parseWithFallback(raw, taskSchema, null, { endpoint: "GET /api/admin/tasks/:id", redact: true });
export const parseAdminExecutionList = (raw: unknown): AdminExecutionList | null => parseWithFallback(raw, tasksSchema, null, { endpoint: "GET /api/admin/tasks", redact: true });
export const parseAdminIssueList = (raw: unknown): AdminIssueList | null => parseWithFallback(raw, issuesSchema, null, { endpoint: "GET /api/admin/issues", redact: true });
