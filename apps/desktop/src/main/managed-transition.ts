import { randomUUID } from "node:crypto";
import { ManagementControlError } from "./managed-control";
import type { ManagedDaemonScope, OwnedManagedDaemon } from "./managed-session";

export type ManagedPendingReason = "busy" | "switching" | "unassociated";
export type ManagedTransitionResult = { accepted: true } | { accepted: false; reason: ManagedPendingReason };
interface TransitionSession {
 connectRunningDaemon(port: number): Promise<OwnedManagedDaemon | null>;
 matchesDaemonScope(scope: ManagedDaemonScope): boolean;
 createHandoff(knownWorkspaceIds?: ReadonlySet<string>): Promise<string>;
 acknowledgeHandoff?(value: string): Promise<void>;
}
export interface ManagedTransitionRecovery { stopRequested: boolean; intentId?: string }

/** Pause new claims immediately; finish a switch only after the old process exits. */
export async function transitionManagedDaemon(options: {
 session: TransitionSession; port: number; isCurrent(): boolean;
 recovery: ManagedTransitionRecovery; isStopped(): Promise<boolean>;
 waitForStop(): Promise<boolean>; start(handoff: string): Promise<{ success: boolean }>;
}): Promise<ManagedTransitionResult> {
 const { session, recovery } = options;
 const pending = (reason: ManagedPendingReason = "switching"): ManagedTransitionResult => ({ accepted: false, reason });
 const startCurrent = async (): Promise<ManagedTransitionResult> => {
  if (!options.isCurrent()) return pending();
  const handoff = await session.createHandoff();
  if (!options.isCurrent()) return pending();
  const result = await options.start(handoff);
  if (!options.isCurrent()) return pending();
  if (!result.success) return pending();
  await session.acknowledgeHandoff?.(handoff);
  recovery.stopRequested = false; delete recovery.intentId;
  return { accepted: true };
 };
 if (!options.isCurrent()) return pending();
 try {
  // Recovery survives a new login. No live peer is needed once its actual
  // process exit is confirmed, and only the current session can start.
  if (await options.isStopped()) return await startCurrent();
  const owned = await session.connectRunningDaemon(options.port);
  if (!options.isCurrent()) return pending();
  if (!owned) return pending("unassociated");
  const sameScope = session.matchesDaemonScope(owned.scope);
  if (sameScope && !owned.scope.drain_intent_id && !recovery.stopRequested) {
   const handoff = await session.createHandoff(new Set(owned.scope.workspace_ids));
   if (!options.isCurrent()) return pending();
   const parsed: unknown = JSON.parse(handoff);
   if (!parsed || typeof parsed !== "object" || !("bindings" in parsed) || !Array.isArray(parsed.bindings)) throw new Error("Invalid prepared management handoff");
   if (parsed.bindings.length === 0) return { accepted: true };
   if (owned.scope.active_task_count !== 0) return pending("busy");
   const response: unknown = await owned.connection.request("POST", "/management/handoff", handoff);
   if (!options.isCurrent()) return pending();
   if (!response || typeof response !== "object" || !("accepted" in response) || response.accepted !== true) return pending();
   await session.acknowledgeHandoff?.(handoff); return { accepted: true };
  }
  // Ensure the desired credentials are available before draining the old scope.
  await session.createHandoff();
  if (!options.isCurrent()) return pending();
  recovery.intentId ??= randomUUID();
  const body = JSON.stringify({ intent_id: recovery.intentId, expected_intent_id: owned.scope.drain_intent_id ?? null });
  recovery.stopRequested = true;
  try {
   const response = await owned.connection.request("POST", "/shutdown", body);
   if (!response || typeof response !== "object" || !("accepted" in response) || response.accepted !== true || !("intent_id" in response) || response.intent_id !== recovery.intentId) return pending();
  } catch (error) {
   if (error instanceof ManagementControlError) return pending(error.status === 409 ? "busy" : "switching");
   // Unknown acknowledgement never authorizes a kill or resuming old claims.
   // An observed process exit is sufficient for the current intent to continue.
  }
  if (!options.isCurrent()) return pending();
  if (!await options.waitForStop()) return pending(owned.scope.active_task_count > 0 ? "busy" : "switching");
  return await startCurrent();
 } catch (error) { return pending(error instanceof ManagementControlError && error.status === 409 ? "busy" : "switching"); }
}
