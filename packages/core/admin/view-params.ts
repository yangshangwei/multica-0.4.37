import { z } from "zod";

const instant = z.iso.datetime({ offset: true });
const dayMs = 86_400_000;
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
const utf8 = new TextEncoder();
export type AdminFilterError = "invalidTime" | "invalidId" | "invalidTimezone" | "invalidRange" | "rangeTooLong" | "futureTime" | "tooLong";
export interface AdminFilterValidationOptions {
  maxWindowDays?: number | null;
  now?: Date;
}

function canonicalTimezone(value: string): string | null {
  if (value === "Local" || /^[+-]/.test(value)) return null;
  try { return new Intl.DateTimeFormat("en", { timeZone: value }).resolvedOptions().timeZone; }
  catch { return null; }
}

/** Native date/time controls are explicitly labelled UTC, never browser-local. */
export function adminInstantInput(value: string | null): string {
  if (!value || !instant.safeParse(value).success) return value ?? "";
  return new Date(value).toISOString().slice(0, 19);
}

export function normalizeAdminFilters(values: FormData, { dateOnly = false, now = new Date() }: { dateOnly?: boolean; now?: Date } = {}): URLSearchParams {
  const params = new URLSearchParams();
  for (const [name, raw] of values) {
    if (typeof raw !== "string" || !raw.trim()) continue;
    let value = raw.trim();
    if (name === "timezone") value = canonicalTimezone(value) ?? value;
    if (name === "time_from" || name === "time_to") {
      if (dateOnly && /^\d{4}-\d{2}-\d{2}$/.test(value)) {
        const candidate = `${value}T00:00:00Z`;
        if (instant.safeParse(candidate).success) {
          const boundary = Date.parse(candidate) + (name === "time_to" ? dayMs : 0);
          value = new Date(name === "time_to" && value === now.toISOString().slice(0, 10)
            ? Math.min(boundary, now.getTime()) : boundary).toISOString();
        }
      } else if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d{1,3})?)?$/.test(value)) {
        value = `${value.length === 16 ? `${value}:00` : value}Z`;
      }
    }
    params.set(name, value);
  }
  return params;
}

export function validateAdminFilters(params: URLSearchParams, { maxWindowDays = 31, now = new Date() }: AdminFilterValidationOptions = {}): Record<string, AdminFilterError> {
  const errors: Record<string, AdminFilterError> = {};
  for (const [name, value] of params) {
    if (!value || name === "cursor" || name === "return_to") continue;
    if (name.endsWith("_id") && !uuid.test(value)) errors[name] = "invalidId";
    if (utf8.encode(value).length > 128 || value.includes("\0")) errors[name] = "tooLong";
  }
  const zone = params.get("timezone");
  if (zone && !canonicalTimezone(zone)) errors.timezone = "invalidTimezone";
  for (const name of ["time_from", "time_to"]) {
    const value = params.get(name);
    if (!value) continue;
    if (!instant.safeParse(value).success) errors[name] = "invalidTime";
    else if (Date.parse(value) > now.getTime() + 60_000) errors[name] = "futureTime";
  }
  if (!errors.time_from && !errors.time_to && params.has("time_from")) {
    const from = Date.parse(params.get("time_from")!);
    const to = params.has("time_to") ? Date.parse(params.get("time_to")!) : now.getTime();
    if (from >= to) errors.time_to = "invalidRange";
    else if (maxWindowDays !== null && to - from > maxWindowDays * dayMs) errors.time_from = "rangeTooLong";
  }
  return errors;
}

/** Only list context is stored; detail routes never send return_to to the API. */
export function adminDetailHref(detailPath: string, listPath: string, params: URLSearchParams): string {
  const listParams = new URLSearchParams(params);
  listParams.delete("return_to");
  const query = new URLSearchParams({ timezone: params.get("timezone") || "UTC" });
  query.set("return_to", `${listPath}${listParams.size ? `?${listParams}` : ""}`);
  return `${detailPath}?${query}`;
}

export function adminReturnHref(params: URLSearchParams, fallbackPath: string): string {
  const target = params.get("return_to");
  if (!target || !target.startsWith("/") || target.startsWith("//") || /[\\\r\n]/.test(target)) return fallbackPath;
  const pathname = target.split(/[?#]/, 1)[0];
  const allowed = pathname === fallbackPath || fallbackPath === "/admin/users" && pathname === "/admin/administrators";
  return allowed ? target : fallbackPath;
}

/** A detail page's validated list context, overlaid with its selected zone, for links to further details. */
export function adminListParams(params: URLSearchParams, listPath: string): URLSearchParams {
  const list = new URL(adminReturnHref(params, listPath), "https://admin.invalid").searchParams;
  const zone = params.get("timezone");
  if (zone) list.set("timezone", zone);
  return list;
}
