export function decodeTriageCsv(buffer: ArrayBuffer): string {
  return new TextDecoder("utf-8", { fatal: true }).decode(buffer);
}

export function duplicateReference(
  value: string,
  workspaceSlug: string,
): string {
  const input = value.trim();
  if (!input.includes("/")) return input;
  const url = new URL(input, "https://multica.invalid");
  const parts = url.pathname.split("/").filter(Boolean).map(decodeURIComponent);
  if (
    parts.length !== 3 ||
    parts[0] !== workspaceSlug ||
    parts[1] !== "issues"
  ) {
    throw new Error("invalid_duplicate_link");
  }
  return parts[2]!;
}

export function snoozePresets(now = new Date()) {
  const tomorrow = new Date(now);
  tomorrow.setDate(tomorrow.getDate() + 1);
  tomorrow.setHours(9, 0, 0, 0);
  const week = new Date(now);
  // Next Monday is always in the following calendar week.
  week.setDate(now.getDate() + (now.getDay() === 0 ? 1 : 8 - now.getDay()));
  week.setHours(9, 0, 0, 0);
  return { hour: new Date(now.getTime() + 3600000), tomorrow, week };
}

export function nextTriageSelection(ids: string[], current: string): string {
  const index = ids.indexOf(current);
  return ids[index + 1] ?? ids[index - 1] ?? "";
}

export const TRIAGE_CONTROL =
  "min-h-11 md:min-h-8 [@media(pointer:coarse)]:min-h-11";

export const TRIAGE_HISTORY_FIELDS = [
  "title",
  "description",
  "status",
  "priority",
  "project_id",
  "assignee_id",
  "labels",
  "start_date",
  "due_date",
  "admission_status",
  "reviewer_id",
  "snoozed_until",
  "duplicate_identifier",
  "candidate_project_id",
  "candidate_assignee_id",
] as const;
export type TriageHistoryField = (typeof TRIAGE_HISTORY_FIELDS)[number];

function snapshotFields(
  snapshot: Record<string, unknown>,
): Record<string, unknown> {
  const issue = snapshot.issue;
  return {
    ...snapshot,
    ...(issue !== null && typeof issue === "object" && !Array.isArray(issue)
      ? issue
      : {}),
  };
}

export function triageSnapshotChanges(
  before: Record<string, unknown>,
  after: Record<string, unknown>,
) {
  const previous = snapshotFields(before);
  const next = snapshotFields(after);
  return TRIAGE_HISTORY_FIELDS.flatMap((field) =>
    JSON.stringify(previous[field] ?? null) ===
    JSON.stringify(next[field] ?? null)
      ? []
      : [{ field, before: previous[field], after: next[field] }],
  );
}
