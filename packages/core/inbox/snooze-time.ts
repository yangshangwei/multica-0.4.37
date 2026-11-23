export type InboxSnoozePresetKey =
  | "one_hour"
  | "three_hours"
  | "tomorrow"
  | "three_days"
  | "one_week";

export function inboxSnoozePresets(now: Date): { key: InboxSnoozePresetKey; date: Date }[] {
  const calendarDay = (days: number) => {
    const date = new Date(now);
    date.setDate(date.getDate() + days);
    date.setHours(9, 0, 0, 0);
    return date;
  };
  return [
    { key: "one_hour", date: new Date(now.getTime() + 60 * 60 * 1000) },
    { key: "three_hours", date: new Date(now.getTime() + 3 * 60 * 60 * 1000) },
    { key: "tomorrow", date: calendarDay(1) },
    { key: "three_days", date: calendarDay(3) },
    { key: "one_week", date: calendarDay(7) },
  ];
}

export function toInboxLocalDateTime(date: Date): string {
  if (!Number.isFinite(date.getTime())) return "";
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${String(date.getFullYear()).padStart(4, "0")}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function parseInboxSnoozeDateTime(value: string, now: Date): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value);
  if (!match) return null;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6] ?? 0);
  const date = new Date(year, month - 1, day, hour, minute, second, 0);
  if (year < 100) date.setFullYear(year);
  // Comparing local fields rejects invalid dates and nonexistent DST times.
  // The Date constructor resolves repeated wall times to the first occurrence.
  if (
    date.getFullYear() !== year || date.getMonth() !== month - 1 ||
    date.getDate() !== day || date.getHours() !== hour ||
    date.getMinutes() !== minute || date.getSeconds() !== second ||
    date.getTime() <= now.getTime() || !Number.isFinite(now.getTime())
  ) return null;
  return date;
}
