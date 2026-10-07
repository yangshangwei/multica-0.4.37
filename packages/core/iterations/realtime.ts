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
