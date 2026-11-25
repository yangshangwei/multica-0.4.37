import { issueStatusCategory } from "@multica/core/issues";
import {
  isIssueDirectHit,
  isProjectDirectHit,
} from "@multica/core/search/cancelled-rank";
import type { Issue, IssueStatus } from "@multica/core/types";
import type { MentionItem } from "./mention-suggestion";

export function isDemotedCancelled(item: MentionItem, query: string): boolean {
  if (isPinnedAboveTruncation(item, query)) return false;
  if (item.type === "issue") return item.statusCategory === "cancelled";
  if (item.type === "project") return item.projectStatus === "cancelled";
  return false;
}

/**
 * Rows that must survive the MAX_ITEMS truncation: curated context and direct
 * hits. Exempting a direct hit from the demotion is not enough on its own —
 * `slice(0, MAX_ITEMS)` runs on the merged list, so a direct hit sitting behind
 * 20 cached candidates would still be cut. Pinning it to the front is what
 * actually keeps the record the user typed in full reachable, and it mirrors the
 * server, which already ranks direct hits first.
 *
 * Issue mention rows carry the identifier in `label` and the title in
 * `description`; project rows carry the title in `label`.
 */
function isPinnedAboveTruncation(item: MentionItem, query: string): boolean {
  if (item.group === "current" || item.group === "recent") return true;
  if (!query) return false;
  if (item.type === "issue") {
    return isIssueDirectHit(
      { identifier: item.label, title: item.description },
      query,
    );
  }
  if (item.type === "project") {
    return isProjectDirectHit({ title: item.label }, query);
  }
  return false;
}

/**
 * Stable three-tier partition applied BEFORE the MAX_ITEMS truncation, so
 * cancelled rows give up their slot rather than merely their position:
 *
 *   pinned (curated context + direct hits) → live → cancelled
 *
 * Relative order within each tier is untouched, leaving the backend ranking in
 * charge of everything else. groupItems() then renders the cancelled tier as the
 * trailing section.
 */
export function demoteCancelledItems(
  items: MentionItem[],
  query: string,
): MentionItem[] {
  const pinned: MentionItem[] = [];
  const live: MentionItem[] = [];
  const cancelled: MentionItem[] = [];

  for (const item of items) {
    if (isPinnedAboveTruncation(item, query)) pinned.push(item);
    else if (isDemotedCancelled(item, query)) cancelled.push(item);
    else live.push(item);
  }

  return pinned.length > 0 || cancelled.length > 0
    ? [...pinned, ...live, ...cancelled]
    : items;
}

export function issueToMention(
  i: Pick<Issue, "id" | "identifier" | "title" | "status"> &
    Partial<Pick<Issue, "status_category">>,
): MentionItem {
  return {
    id: i.id,
    label: i.identifier,
    type: "issue" as const,
    description: i.title,
    status: i.status as IssueStatus,
    statusCategory: issueStatusCategory(i) ?? undefined,
  };
}
