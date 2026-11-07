import { match } from "@formatjs/intl-localematcher";
import {
  DEFAULT_LOCALE,
  SUPPORTED_LOCALES,
  type LocaleAdapter,
  type SupportedLocale,
} from "./types";

export function matchLocale(candidates: string[]): SupportedLocale {
  if (candidates.length === 0) return DEFAULT_LOCALE;
  try {
    return match(
      candidates,
      SUPPORTED_LOCALES,
      DEFAULT_LOCALE,
    ) as SupportedLocale;
  } catch {
    return DEFAULT_LOCALE;
  }
}

// Retired explicit choices keep their priority over system preferences.
export function normalizeStoredLocale(locale: string): string {
  return locale === "ja" || locale === "ko" ? DEFAULT_LOCALE : locale;
}

export function pickLocale(adapter: LocaleAdapter): SupportedLocale {
  const choice = adapter.getUserChoice();
  if (choice) return matchLocale([normalizeStoredLocale(choice)]);
  return matchLocale(adapter.getSystemPreferences());
}
