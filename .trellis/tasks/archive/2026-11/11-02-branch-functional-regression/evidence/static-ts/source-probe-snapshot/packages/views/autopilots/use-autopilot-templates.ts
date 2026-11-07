"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { AutopilotTemplate } from "@multica/core/types";
import { templateLanguageFor } from "../agents/create/use-role-templates";
import { useLocale } from "../i18n";

/**
 * The built-in autopilot templates, in the reader's language.
 *
 * Templates ship with the backend binary, so the only thing that invalidates
 * this is a backend deploy or a language change — hence the long stale time and
 * the locale in the key. It is deliberately NOT workspace-scoped: the answer is
 * identical for every workspace on the server, and keying it per workspace would
 * refetch the same payload on every switch.
 *
 * Lives in views rather than core because the language comes from `useLocale`,
 * and core may not depend on views' i18n. Same reason `useRoleTemplates` is
 * here; the language mapping is imported from it rather than re-derived.
 */
export function useAutopilotTemplates() {
  const locale = useLocale();
  const language = templateLanguageFor(locale);
  return useQuery({
    queryKey: ["autopilot-templates", language],
    queryFn: () => api.listAutopilotTemplates(language),
    staleTime: 30 * 60 * 1000,
  });
}

/** Finds a template by key in a possibly-still-loading list. */
export function findAutopilotTemplate(
  templates: AutopilotTemplate[] | undefined,
  key: string | null,
): AutopilotTemplate | null {
  if (!key) return null;
  return templates?.find((template) => template.key === key) ?? null;
}
