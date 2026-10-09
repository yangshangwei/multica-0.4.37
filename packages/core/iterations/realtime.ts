import type { QueryClient } from "@tanstack/react-query";
import { triageKeys } from "../triage/queries";
import { projectP1Keys } from "../projects/p1-queries";

/** Settings feed triage assignment and the shared project planning timezone. */
export async function refreshIterationSettingsDependents(client: QueryClient, wsId: string) {
  await Promise.all([
    client.invalidateQueries({ queryKey: triageKeys.settings(wsId) }),
    client.invalidateQueries({ queryKey: projectP1Keys.timezone(wsId) }),
  ]);
}

/** Inputs to live iteration scope/statistics and protected historical summaries. */
export function iterationManagementEvent(type: string): boolean {
  const [domain] = type.split(":");
  if (domain === "task")
    return type !== "task:message" && type !== "task:progress";
  return [
    "issue",
    "label",
    "project",
    "member",
    "agent",
    "squad",
    "issue_status",
    "workspace",
    "triage",
  ].includes(domain ?? "");
}
