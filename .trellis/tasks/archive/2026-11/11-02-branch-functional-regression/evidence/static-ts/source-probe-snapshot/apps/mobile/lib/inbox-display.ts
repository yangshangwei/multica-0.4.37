/**
 * Inbox title display helpers.
 *
 * Mirrors packages/views/inbox/components/inbox-display.ts. Keeping behavior
 * identical is required by apps/mobile/CLAUDE.md "Behavioral parity":
 * the title a user sees in the mobile inbox MUST match what they see on
 * web for the same item. When the web version changes, sync this file.
 */
import type { InboxItem } from "@multica/core/types";

/** Project updates have no issue_id. Open the existing project detail;
 * mobile keeps health/progress/acceptance editing on Web and Desktop. */
export function getInboxProjectTarget(
  item: Pick<InboxItem, "workspace_id" | "details"> & { type: string },
  workspaceId: string | null,
): string | null {
  if (!workspaceId || item.workspace_id !== workspaceId || item.type !== "project_update") return null;
  const projectId = item.details?.project_id;
  return projectId && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(projectId) ? projectId : null;
}

/** Mirrors Web/Desktop's iteration notice destinations; mobile explains
 * that the supported clients are required instead of opening an editor. */
export function getInboxIterationTarget(
  item: Pick<InboxItem, "workspace_id" | "details"> & { type: string },
  workspaceId: string | null,
): { kind: "list" } | { kind: "detail"; iterationId: string } | null {
  if (!workspaceId || item.workspace_id !== workspaceId || item.type !== "iteration") return null;
  switch (item.details?.kind) {
    case "disable":
      return { kind: "list" };
    case "start":
    case "end":
    case "cancel":
    case "dates_changed":
    case "overdue": {
      const iterationId = item.details.iteration_id;
      return iterationId && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(iterationId)
        ? { kind: "detail", iterationId }
        : null;
    }
    default:
      return null;
  }
}

function singleLine(value: string | null | undefined): string {
  return (value ?? "").replace(/\s+/g, " ").trim();
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function stripQuickCreatePrefix(
  title: string,
  identifier?: string,
): string {
  const normalized = singleLine(title);
  if (!normalized) return "";
  if (identifier) {
    const exactPrefix = new RegExp(
      `^Created\\s+${escapeRegExp(identifier)}:\\s*`,
      "i",
    );
    const withoutExactPrefix = normalized.replace(exactPrefix, "");
    if (withoutExactPrefix !== normalized) return withoutExactPrefix.trim();
  }
  return normalized.replace(/^Created\s+[A-Z][A-Z0-9]*-\d+:\s*/i, "").trim();
}

export function getInboxDisplayTitle(item: InboxItem): string {
  const details = item.details ?? {};
  if (item.type === "quick_create_done") {
    const cleanedTitle = stripQuickCreatePrefix(item.title, details.identifier);
    if (cleanedTitle) return cleanedTitle;
    const prompt = singleLine(details.original_prompt);
    if (prompt) return prompt;
  }
  // Both non-success quick-create outcomes surface the user's original input
  // as the row title. Mirrors isQuickCreateOutcome in
  // packages/views/inbox/components/inbox-display.ts.
  if (item.type === "quick_create_failed" || item.type === "quick_create_unconfirmed") {
    const prompt = singleLine(details.original_prompt);
    if (prompt) return prompt;
  }
  return item.title;
}

/**
 * Deduplicate inbox items by issue_id (Linear-style: one entry per issue).
 *
 * Mirrors packages/core/inbox/queries.ts deduplicateInboxItems. **MUST stay
 * aligned with that function** — see the inbox dedup incident in this file's
 * companion `apps/mobile/CLAUDE.md` "Behavioral parity" section. Skipping
 * this step makes the same workspace/user show different unread counts on
 * mobile vs web.
 *
 * Steps:
 *   1. Drop archived rows (these never appear in web's inbox view).
 *   2. Group by `issue_id` (fall back to `id` for items with no issue
 *      attached — e.g. quick_create_failed).
 *   3. In each group, keep the newest by `created_at`.
 *   4. Preserve the newest grouped `comment_id` anchor when the newest row
 *      is a later status/metadata event for the same issue.
 *   5. Sort the result newest-first.
 */
export function deduplicateInboxItems(items: InboxItem[]): InboxItem[] {
  const active = items.filter((i) => !i.archived);
  const groups = new Map<string, InboxItem[]>();
  for (const item of active) {
    const key = item.issue_id ?? item.id;
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  const merged: InboxItem[] = [];
  for (const group of groups.values()) {
    group.sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    const newest = group[0];
    if (!newest) continue;

    const commentId =
      newest.details?.comment_id ??
      group.find((item) => item.details?.comment_id)?.details?.comment_id;

    if (commentId && newest.details?.comment_id !== commentId) {
      merged.push({
        ...newest,
        details: { ...(newest.details ?? {}), comment_id: commentId },
      });
      continue;
    }

    merged.push(newest);
  }
  return merged.sort(
    (a, b) =>
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
}
