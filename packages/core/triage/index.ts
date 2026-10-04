export * from "./queries";
export * from "./mutations";
export type * from "../types/triage";

/** Missing only means pre-triage server; malformed and future states fail closed. */
export function isFormalAdmission(status: unknown): boolean {
  return status === undefined || status === "not_required" || status === "accepted";
}

/** Call when creating an intention, never inside a mutation function/retry. */
export function createTriageRequestId(): string {
  return crypto.randomUUID();
}
