import type { Iteration } from "../api/iteration-schemas";

export interface IterationTimelineFilters {
  search?: string;
  from?: string;
  to?: string;
  status?: string;
}

const knownStatuses = new Set(["planned", "active", "completed", "cancelled"]);
const descending = (a: Iteration, b: Iteration) =>
  b.start_date.localeCompare(a.start_date) || b.id.localeCompare(a.id);

function calendarDay(value: string): number | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const time = Date.parse(`${value}T00:00:00Z`);
  if (!Number.isFinite(time) || new Date(time).toISOString().slice(0, 10) !== value) return null;
  return time / 86_400_000;
}

/** Derive the timeline from a complete catalogue, never a server page.
 * Callers must hide gaps while the catalogue is refreshing or has an error.
 */
export function iterationTimeline(items: readonly Iteration[], filters: IterationTimelineFilters = {}) {
  const search = filters.search?.trim().toLocaleLowerCase() ?? "";
  const isKnown = (item: Iteration) => item.mode === "manual" && knownStatuses.has(item.status);
  const sorted = [...items].sort(descending);
  const upcomingId = sorted.findLast(item => isKnown(item) && item.status === "planned")?.id ?? null;
  const visible = sorted.filter(item =>
    (!search || item.name.toLocaleLowerCase().includes(search)) &&
    (!filters.from || item.end_date >= filters.from) &&
    (!filters.to || item.start_date <= filters.to) &&
    (!filters.status || item.status === filters.status),
  );
  const planned = visible.filter(item => isKnown(item) && item.status === "planned");
  const active = visible.filter(item => isKnown(item) && item.status === "active");
  const history = visible.filter(item => isKnown(item) && ["completed", "cancelled"].includes(item.status));
  const unknown = visible.filter(item => !isKnown(item));
  const gaps = new Map<string, number>();

  if (!search && !filters.from && !filters.to && !filters.status) {
    const intervals = items.map(item => ({ item, start: calendarDay(item.start_date), end: calendarDay(item.end_date) }));
    const comparable = intervals.every(({ start, end }) => start !== null && end !== null && start <= end)
      && new Set(items.map(item => item.workspace_id)).size <= 1
      && new Set(items.map(item => item.id)).size === items.length;
    if (comparable) {
      // Group order is part of the display contract. Never connect two rows
      // merely because filtering or grouping made them adjacent on screen.
      const rows = [...planned, ...active];
      for (let index = 0; index < rows.length - 1; index++) {
        const later = rows[index]!;
        const earlier = rows[index + 1]!;
        if (later.timezone !== earlier.timezone) continue;
        const start = calendarDay(earlier.start_date)!;
        const end = calendarDay(later.end_date)!;
        const days = calendarDay(later.start_date)! - calendarDay(earlier.end_date)! - 1;
        if (days <= 0) continue;
        // A cancelled/unknown period or overlapping interval makes this pair
        // ambiguous. Suppress it instead of inventing occupancy semantics.
        const intervening = intervals.some(({ item, start: otherStart, end: otherEnd }) =>
          item.id !== later.id && item.id !== earlier.id && otherStart! <= end && otherEnd! >= start,
        );
        if (!intervening) gaps.set(later.id, days);
      }
    }
  }
  return { planned, active, history, unknown, upcomingId, gaps };
}
