export function iterationLocalDate(timezone: string, now = new Date()): string {
  const parts = new Intl.DateTimeFormat("en", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(now);
  const part = (type: string) =>
    parts.find((value) => value.type === type)!.value;
  return `${part("year")}-${part("month")}-${part("day")}`;
}
export function iterationDefaultDates(timezone: string, now = new Date()) {
  const start = iterationLocalDate(timezone, now);
  const end = new Date(`${start}T00:00:00Z`);
  end.setUTCDate(end.getUTCDate() + 13);
  return { start, end: end.toISOString().slice(0, 10) };
}
export function iterationIsOverdue(
  status: string,
  endDate: string,
  timezone: string,
  now = new Date(),
): boolean {
  return status === "active" && endDate < iterationLocalDate(timezone, now);
}
