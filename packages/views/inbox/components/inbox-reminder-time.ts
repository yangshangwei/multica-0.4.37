/** Calendar time is display-only; Core owns preset arithmetic and validation. */
export function formatInboxReminderTime(
  value: string,
  locale: string,
  includeOffset = false,
): string {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    ...(includeOffset ? { timeZoneName: "shortOffset" as const } : {}),
  }).format(date);
}
