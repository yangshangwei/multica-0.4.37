import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationExecutionRunSchema } from "./execution-schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const recordId = z.string().min(1).max(200);
const recordedName = z.string().min(1).refine((value) => collaborationTextByteLength(value) <= 128);
const attachments = z.array(uuid).refine((items) => new Set(items).size === items.length);

export const CollaborationRunRecordItemSchema = z.discriminatedUnion("kind", [
  z.object({ id: recordId, kind: z.literal("input"), command_id: uuid, author_user_id: uuid, body: z.string(), attachment_ids: attachments }),
  z.object({ id: recordId, kind: z.literal("message"), role: z.enum(["assistant", "user", "system"]), body: z.string() }),
  z.object({ id: recordId, kind: z.literal("tool_call"), call_id: recordedName, name: recordedName, body: z.string() }),
  z.object({ id: recordId, kind: z.literal("tool_result"), call_id: recordedName, body: z.string(), is_error: z.boolean() }),
  z.object({ id: recordId, kind: z.literal("comment"), comment_id: uuid, body: z.string() }),
  z.object({ id: recordId, kind: z.literal("squad_activity"), outcome: z.enum(["action", "no_action", "failed"]), body: z.string() }),
  z.object({ id: recordId, kind: z.literal("result"), outcome: z.enum(["success", "failure"]), body: z.string() }),
]);

const usage = z.object({
  input_tokens: nonnegativeVersion.nullable(), output_tokens: nonnegativeVersion.nullable(),
  cache_read_tokens: nonnegativeVersion.nullable(), cache_creation_tokens: nonnegativeVersion.nullable(),
  total_tokens: nonnegativeVersion.nullable(),
  cost_usd: z.string().regex(/^(?:0|[1-9][0-9]{0,18})(?:\.[0-9]{1,10})?$/).nullable(),
});

/** Source-owned records are separate from mutable execution/control snapshots. */
export const CollaborationRunRecordsSchema = z.object({
  protocol_version: z.literal(1), issue_id: uuid, run: CollaborationExecutionRunSchema,
  resource: z.object({
    agent_id: uuid, runtime_id: uuid, resource_owner_user_id: uuid, execution_workspace_id: uuid, grant_id: uuid,
    provider: recordedName.nullable(), model: recordedName.nullable(), account_source: z.literal("runtime_configuration").nullable(),
  }),
  recording: z.object({ status: z.enum(["not_recorded", "partial", "complete"]), inputs_recorded: z.boolean() }),
  usage: usage.nullable(), items: z.array(CollaborationRunRecordItemSchema).max(100),
  next_cursor: z.string().min(1).max(1024).nullable(),
}).refine((page) => {
  const { run, resource, recording, items } = page;
  const terminal = run.settled_at !== null;
  if (resource.agent_id !== run.agent_id || terminal !== (run.progress_state === "settled") ||
    (terminal && run.proof_state === "unknown") || run.parent_run_id === run.id) return false;
  if ((run.step_kind === "agent" || run.step_kind === "leader") !== (run.parent_run_id === null)) return false;
  if (run.recording_incomplete && recording.status !== "partial") return false;
  if (recording.status === "not_recorded" && (page.usage !== null ||
    items.some((item) => item.kind === "message" || item.kind === "tool_call" || item.kind === "tool_result"))) return false;
  if (new Set(items.map((item) => item.id)).size !== items.length) return false;
  let eventsStarted = false;
  const commands = new Set<string>();
  for (const item of items) {
    if (item.kind !== "input") { eventsStarted = true; continue; }
    if (!recording.inputs_recorded || eventsStarted || commands.has(item.command_id)) return false;
    commands.add(item.command_id);
  }
  return true;
}, "Recorded run identity, input order and recording state must be consistent");

export type CollaborationRunRecordItem = z.infer<typeof CollaborationRunRecordItemSchema>;
export type CollaborationRunRecords = z.infer<typeof CollaborationRunRecordsSchema>;

export function parseCollaborationRunRecords(data: unknown): CollaborationRunRecords | null {
  return parseWithFallback<CollaborationRunRecords | null>(data, CollaborationRunRecordsSchema, null, {
    endpoint: "collaboration run records", redact: true,
  });
}
