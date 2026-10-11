export type OrderDirection = -1 | 1;

/** Only reorder inside a loaded group, with truthful anchors on both sides. */
export function adjacentOrder(
  ids: readonly string[],
  id: string,
  direction: OrderDirection,
  hasMore = false,
): string[] | null {
  const from = ids.indexOf(id);
  const to = from + direction;
  if (from < 0 || to < 0 || to >= ids.length) return null;
  if (hasMore && to === ids.length - 1) return null;
  const next = [...ids];
  next.splice(from, 1);
  next.splice(to, 0, id);
  return next;
}
