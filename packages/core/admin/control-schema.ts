import { z } from "zod";

// PostgreSQL bigint versions cross the wire as decimal strings.
export const adminVersionSchema = z.union([
  z.string().regex(/^[1-9]\d{0,18}$/).refine(value => value.length < 19 || value <= "9223372036854775807"),
  z.number().int().positive().max(Number.MAX_SAFE_INTEGER).transform(String),
]);
export const executionFenceSchema = z.object({
  runtime_id: z.uuid().nullable(),
  dispatched_at: z.iso.datetime({ offset: true }).nullable(),
  target_version: adminVersionSchema,
}).transform(value => ({ runtimeId: value.runtime_id, dispatchedAt: value.dispatched_at, targetVersion: value.target_version }));
export type AdminExecutionFence = z.output<typeof executionFenceSchema>;
export interface AdminAdmissionChange {
  admission: "accepting" | "stopped";
  expectedAdmissionVersion: string;
  reason: string;
}
export interface AdminExecutionCancellation {
  expectedExecutionFence: AdminExecutionFence;
  reason: string;
}
export function canControlAdmission(role: string | undefined, target: {
  lifecycle: string; admission: string; allowedActions?: readonly string[]; admissionVersion?: string;
}): "stopped" | "accepting" | null {
  if (role !== "super_admin" || target.lifecycle !== "active" || !adminVersionSchema.safeParse(target.admissionVersion).success) return null;
  if (target.admission === "accepting" && target.allowedActions?.includes("stop_admission")) return "stopped";
  if (target.admission === "stopped" && target.allowedActions?.includes("resume_admission")) return "accepting";
  return null;
}
export function canCancelExecution(role: string | undefined, target: {
  status: string; allowedActions?: readonly string[]; executionFence?: AdminExecutionFence | null;
}): boolean {
  return role === "super_admin" && target.allowedActions?.includes("cancel") === true && !!target.executionFence &&
    ["queued", "deferred", "dispatched", "running", "waiting_local_directory"].includes(target.status);
}
