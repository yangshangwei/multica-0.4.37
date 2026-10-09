import type { TriageHistoryEntry } from "@multica/core/triage";
import { formatInTimeZone } from "../common/format-in-time-zone";

interface TriageHistoryDay {
  key: string;
  dayKey: string | null;
  dateLabel: string | null;
  events: {
    entry: TriageHistoryEntry;
    time: string | null;
    fullTime: string | null;
  }[];
}

/** Group only adjacent entries: the server owns event order and page boundaries. */
export function groupTriageHistoryEntries(
  entries: readonly TriageHistoryEntry[],
  locale: string,
  timeZone?: string,
): TriageHistoryDay[] {
  const groups: TriageHistoryDay[] = [];
  const dayOccurrences = new Map<string | null, number>();
  const dateOptions: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: undefined,
    minute: undefined,
  };
  for (const entry of entries) {
    const valid = !Number.isNaN(new Date(entry.created_at).getTime());
    const dayKey = valid
      ? formatInTimeZone(entry.created_at, timeZone, "en-CA", {
          ...dateOptions,
          month: "2-digit",
          day: "2-digit",
          calendar: "gregory",
          numberingSystem: "latn",
        })
      : null;
    let group = groups.at(-1);
    if (!group || group.dayKey !== dayKey) {
      // A new first event must not remount that day's open disclosures.
      // Occurrences also keep noncontiguous groups of the same date distinct.
      const occurrence = dayOccurrences.get(dayKey) ?? 0;
      dayOccurrences.set(dayKey, occurrence + 1);
      group = {
        key: `${dayKey ?? "unknown"}-${occurrence}`,
        dayKey,
        dateLabel: valid
          ? formatInTimeZone(entry.created_at, timeZone, locale, dateOptions)
          : null,
        events: [],
      };
      groups.push(group);
    }
    group.events.push({
      entry,
      time: valid
        ? formatInTimeZone(entry.created_at, timeZone, locale, {
            month: undefined,
            day: undefined,
          })
        : null,
      fullTime: valid
        ? formatInTimeZone(entry.created_at, timeZone, locale, {
            year: "numeric",
            month: "long",
            second: "2-digit",
            timeZoneName: "short",
          })
        : null,
    });
  }
  return groups;
}
