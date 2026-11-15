import type { CollaborationComment } from "./content-schema";

export interface CollaborationCommentThread {
  /** Topmost known ancestor id; the missing parent id when that ancestor is not loaded. */
  id: string;
  /** Loaded root comment, or null when the root is outside the loaded pages. */
  root: CollaborationComment | null;
  /** Depth-first, siblings by created_at then id; the root comes first at depth 0. */
  items: { comment: CollaborationComment; depth: number }[];
  /** Loaded comment that carries the thread's resolution. */
  resolvedBy: CollaborationComment | null;
  /** Comment hosting the one visible Resolve control; null once resolved or when nothing can be resolved. */
  resolveTarget: CollaborationComment | null;
  /** Unresolved discussion: no loaded resolution and at least one comment that is not deleted. */
  pending: boolean;
}

const time = (comment: CollaborationComment) => new Date(comment.created_at).getTime();
const ascending = (a: CollaborationComment, b: CollaborationComment) => time(a) - time(b) || a.id.localeCompare(b.id);

/**
 * Groups already deduplicated, loaded comments into discussion threads. A
 * thread is a root with all of its descendants, matching the server's
 * resolution scope. Only loaded comments are known here, so `pending` and
 * `resolvedBy` describe the loaded pages, never the whole issue.
 */
export function deriveCollaborationCommentThreads(comments: CollaborationComment[]): CollaborationCommentThread[] {
  const byId = new Map(comments.map((comment) => [comment.id, comment]));
  const children = new Map<string, CollaborationComment[]>();
  const roots: CollaborationComment[] = [];
  const orphans = new Map<string, CollaborationComment[]>();
  const assigned = new Set<string>();
  const ordered = [...byId.values()].sort(ascending);
  for (const comment of ordered) {
    if (comment.parent_id === null) roots.push(comment);
    else if (!byId.has(comment.parent_id)) orphans.set(comment.parent_id, [...orphans.get(comment.parent_id) ?? [], comment]);
    else children.set(comment.parent_id, [...children.get(comment.parent_id) ?? [], comment]);
  }
  function collect(comment: CollaborationComment, depth: number, items: CollaborationCommentThread["items"]) {
    if (assigned.has(comment.id)) return;
    assigned.add(comment.id);
    items.push({ comment, depth });
    for (const child of children.get(comment.id) ?? []) collect(child, depth + 1, items);
  }
  function thread(id: string, root: CollaborationComment | null, items: CollaborationCommentThread["items"]): CollaborationCommentThread {
    const resolvedBy = items.find((item) => item.comment.resolved_at !== null)?.comment ?? null;
    const resolveTarget = resolvedBy ? null
      : root?.can_resolve === true ? root : items.find((item) => item.comment.can_resolve === true)?.comment ?? null;
    return { id, root, items, resolvedBy, resolveTarget, pending: resolvedBy === null && items.some((item) => item.comment.deleted_at === null) };
  }
  const threads: CollaborationCommentThread[] = [];
  for (const root of roots) {
    const items: CollaborationCommentThread["items"] = [];
    collect(root, 0, items);
    threads.push(thread(root.id, root, items));
  }
  for (const [parentId, replies] of orphans) {
    const items: CollaborationCommentThread["items"] = [];
    for (const reply of replies) collect(reply, 1, items);
    threads.push(thread(parentId, null, items));
  }
  // Anything still unassigned sits on a parent cycle. The earliest member of
  // each cycle becomes its root so every comment appears exactly once.
  for (const comment of ordered) {
    if (assigned.has(comment.id)) continue;
    const items: CollaborationCommentThread["items"] = [];
    collect(comment, 0, items);
    threads.push(thread(comment.id, comment, items));
  }
  return threads.sort((a, b) => ascending(a.items[0]!.comment, b.items[0]!.comment));
}
