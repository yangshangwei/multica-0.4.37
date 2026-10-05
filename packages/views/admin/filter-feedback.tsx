"use client";

import { useId, useState } from "react";
import { adminInstantInput, validateAdminFilters, type AdminFilterError, type AdminFilterValidationOptions } from "@multica/core/admin";
import { useT } from "../i18n";

export function useAdminFilterFeedback(params: URLSearchParams) {
  const { t } = useT("admin");
  const id = useId();
  const query = params.toString();
  const [state, setState] = useState<{ query: string; errors: Record<string, AdminFilterError>; days: number }>({ query, errors: {}, days: 31 });
  // Feedback belongs to the current draft, just like the form keyed by this query.
  if (state.query !== query) setState({ query, errors: {}, days: 31 });
  return {
    reset() { setState({ query, errors: {}, days: 31 }); },
    validate(params: URLSearchParams, form: HTMLFormElement, options?: AdminFilterValidationOptions) {
      const next = validateAdminFilters(params, options);
      setState({ query, errors: next, days: options?.maxWindowDays ?? 31 });
      const first = Object.keys(next)[0];
      if (!first) return true;
      requestAnimationFrame(() => {
        const field = form.elements.namedItem(first);
        if (!(field instanceof HTMLElement)) return;
        let details = field.closest("details");
        while (details) { details.open = true; details = details.parentElement?.closest("details") ?? null; }
        field.focus();
      });
      return false;
    },
    fieldProps(name: string) {
      return { "aria-invalid": state.errors[name] ? true : undefined, "aria-describedby": state.errors[name] ? `${id}-${name}` : undefined };
    },
    message(name: string) {
      const code = state.errors[name];
      if (!code) return null;
      const text = code === "rangeTooLong" ? t($ => $.filters.errors.rangeTooLong, { days: state.days }) : t($ => $.filters.errors[code]);
      return <p id={`${id}-${name}`} role="alert" className="text-caption text-destructive">{text}</p>;
    },
  };
}

/** Native selects share the Input density; the admin shell raises both for touch. */
export const adminSelectClass = "h-8 w-full min-w-0 rounded-lg border border-input bg-background px-2.5 text-body outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive dark:bg-input/30";

let zones: string[] | undefined;
/** Suggestions for an IANA time zone text field; validation still decides what is accepted. */
export function AdminTimezoneOptions({ id }: { id: string }) {
  zones ??= ["UTC", ...(typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone").filter(zone => zone !== "UTC") : [])];
  return <datalist id={id}>{zones.map(zone => <option key={zone} value={zone} />)}</datalist>;
}

/** Time fields use native date-time controls labelled UTC; the zone field suggests IANA names. */
export function adminFilterInputProps(name: string, params: URLSearchParams, zoneList: string) {
  if (name === "time_from" || name === "time_to")
    return { type: "datetime-local", step: 1, defaultValue: adminInstantInput(params.get(name)) };
  if (name === "timezone")
    return { list: zoneList, autoComplete: "off", spellCheck: false, placeholder: "UTC", defaultValue: params.get(name) ?? "" };
  return { autoComplete: "off", spellCheck: false, defaultValue: params.get(name) ?? "" };
}
