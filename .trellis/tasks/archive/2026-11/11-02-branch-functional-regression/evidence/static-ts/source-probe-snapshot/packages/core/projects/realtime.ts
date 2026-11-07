/** Inputs of the project snapshot digest. Streaming text is not an input. */
export function projectManagementEvent(type: string): boolean {
  return ["project", "issue", "triage", "issue_status", "member", "agent", "squad", "workspace", "daemon", "runtime"].includes(type.split(":")[0] ?? "") && type !== "daemon:heartbeat";
}
