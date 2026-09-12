"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { AgentRoleTemplate } from "@multica/core/types";
import { useLocale } from "../../i18n";

/**
 * The built-in role templates, in the reader's language.
 *
 * Templates ship with the backend binary, so the only thing that invalidates
 * this is a backend deploy or a language change — hence the long stale time and
 * the locale in the key. It is deliberately NOT workspace-scoped: the answer is
 * identical for every workspace on the server, and keying it per workspace would
 * refetch the same payload on every switch.
 */
export function useRoleTemplates() {
  const locale = useLocale();
  const language = templateLanguageFor(locale);
  return useQuery({
    queryKey: ["agent-role-templates", language],
    queryFn: () => api.listAgentRoleTemplates(language),
    staleTime: 30 * 60 * 1000,
  });
}

/** The squad templates, same caching rationale as the role templates. */
export function useSquadTemplates() {
  const locale = useLocale();
  const language = templateLanguageFor(locale);
  return useQuery({
    queryKey: ["squad-templates", language],
    queryFn: () => api.listSquadTemplates(language),
    staleTime: 30 * 60 * 1000,
  });
}

/**
 * Maps an app locale onto the language the backend has template copy for.
 *
 * Regional locale variants share one supported language. Unknown locales use
 * the backend's English fallback and the same catalog cache entry.
 */
export function templateLanguageFor(locale: string): "en" | "zh" | "ja" | "ko" {
  const language = locale.trim().toLowerCase().split(/[-_]/)[0];
  return language === "zh" || language === "ja" || language === "ko" ? language : "en";
}

/** Finds a template by key in a possibly-still-loading list. */
export function findRoleTemplate(
  templates: AgentRoleTemplate[] | undefined,
  key: string | null,
): AgentRoleTemplate | null {
  if (!key) return null;
  return templates?.find((template) => template.key === key) ?? null;
}
