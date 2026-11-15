import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { collaborationTextByteLength } from "./text-limits";
import { positiveVersion } from "./version";
import { CollaborationTestSubjectSchema } from "./stage-test-subject-schema";
const uuid = z.uuid().transform((value) => value.toLowerCase()).refine((value) => value !== "00000000-0000-0000-0000-000000000000");
const digest = z.string().regex(/^[a-f0-9]{64}$/);
const timestamp = z.string().datetime({ offset: true });
const text = (maximum: number, minimum = 0) => z.string().refine((value) => {
  try { encodeURIComponent(value); } catch { return false; }
  return !value.includes("\0") && collaborationTextByteLength(value) >= minimum && collaborationTextByteLength(value) <= maximum;
});
const summary = z.object({
  report_id: uuid, report_digest: digest, agent_id: uuid, attempt: positiveVersion, source_kind: z.literal("runtime_reported"),
  subject: CollaborationTestSubjectSchema.extend({ binding_digest: digest, source_revision: z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/), artifact_digest: digest.nullable() }),
  reported_result: z.enum(["passed", "failed", "incomplete"]), applicability: z.enum(["usable", "stale", "execution_failed", "execution_unsettled"]),
  case_count: z.number().int().min(1).max(256), failed_case_count: z.number().int().min(0).max(256), received_at: timestamp,
}).refine((v) => v.failed_case_count <= v.case_count && (v.reported_result === "failed" ? v.failed_case_count > 0 : v.failed_case_count === 0));
export type CollaborationTestReportSummary = z.infer<typeof summary>;
export const CollaborationTestReportsSchema = z.object({
  version: z.literal(1), issue_id: uuid, run_id: uuid, items: z.array(summary).max(50), next_cursor: z.string().min(1).nullable(),
}).refine((v) => new Set(v.items.map((r) => r.report_id)).size === v.items.length && (v.items.length > 0 || v.next_cursor === null));
export type CollaborationTestReports = z.infer<typeof CollaborationTestReportsSchema>;
const log = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("record"), event_id: uuid, message_index: z.number().int().nonnegative().safe(), body_sha256: digest, body: text(65536) }),
  z.strictObject({ kind: z.literal("inline"), body_sha256: digest, body: text(65536) }),
]);
export const CollaborationTestReportSchema = summary.safeExtend({
  version: z.literal(1), issue_id: uuid, run_id: uuid, suite_id: text(128, 1),
  cases: z.array(z.strictObject({ id: text(256, 1), outcome: z.enum(["passed", "failed", "error", "skipped"]), log_index: z.number().int().nonnegative().safe() })).min(1).max(256),
  logs: z.array(log).min(1).max(64),
}).refine((v) => v.cases.length === v.case_count && new Set(v.cases.map((c) => c.id)).size === v.cases.length && v.cases.every((c) => c.log_index < v.logs.length))
  .refine((v) => {
    const failed = v.cases.filter((c) => c.outcome === "failed" || c.outcome === "error").length;
    return failed === v.failed_case_count && v.reported_result === (failed ? "failed" : v.cases.some((c) => c.outcome === "skipped") ? "incomplete" : "passed");
  }).refine((v) => v.logs.filter((l) => l.kind === "inline").reduce((total, l) => total + collaborationTextByteLength(l.body), 0) <= 65536);
export type CollaborationTestReport = z.infer<typeof CollaborationTestReportSchema>;
export const parseCollaborationTestReports = (raw: unknown): CollaborationTestReports | null => parseWithFallback(raw, CollaborationTestReportsSchema, null, { endpoint: "GET collaboration test reports", redact: true });
export const parseCollaborationTestReport = (raw: unknown): CollaborationTestReport | null => parseWithFallback(raw, CollaborationTestReportSchema, null, { endpoint: "GET collaboration test report", redact: true });
/** Resolve no paths or URLs; verify only the redacted bytes returned in this protected response. */
export async function verifyCollaborationTestReportLogs(report: CollaborationTestReport, signal?: AbortSignal): Promise<boolean> {
  for (const log of report.logs) {
    signal?.throwIfAborted();
    const value = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(log.body));
    signal?.throwIfAborted();
    const hash = Array.from(new Uint8Array(value), (byte) => byte.toString(16).padStart(2, "0")).join("");
    if (hash !== log.body_sha256) return false;
  }
  return true;
}
export type CollaborationTestReportRead<T> = { state: "available"; value: T } | { state: "not_found" } | { state: "unsupported" } |
  { state: "unavailable"; reason: "malformed" | "log_not_retained" | "storage" };
