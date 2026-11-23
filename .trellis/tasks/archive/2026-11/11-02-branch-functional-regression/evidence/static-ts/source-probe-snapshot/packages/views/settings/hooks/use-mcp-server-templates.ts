"use client";

import { useQuery } from "@tanstack/react-query";
import { mcpServerTemplateListOptions } from "@multica/core/workspace/queries";
import { useLocale } from "../../i18n";
import { templateLanguageFor } from "../../agents/create/use-role-templates";

/** Locale wrapper; the caller owns market/discovery visibility. */
export function useMcpServerTemplates(wsId: string, { enabled = true, poll = false }: { enabled?: boolean; poll?: boolean } = {}) {
  const locale = useLocale();
  return useQuery({
    ...mcpServerTemplateListOptions(wsId, templateLanguageFor(locale), { poll }),
    enabled: !!wsId && enabled,
  });
}
