"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { McpServerTemplate } from "@multica/core/types";
import { useLocale } from "../../i18n";
import { templateLanguageFor } from "../../agents/create/use-role-templates";

/**
 * The built-in MCP catalog, in the reader's language.
 *
 * Templates ship with the backend binary, so the only thing that invalidates
 * this is a backend deploy or a language change — hence the long stale time and
 * the locale in the key. It is deliberately NOT workspace-scoped: the catalog is
 * identical for every workspace on the server. `wsId` only satisfies the
 * workspace-scoped route and gates the query until one is known.
 */
export function useMcpServerTemplates(wsId: string) {
  const locale = useLocale();
  const language = templateLanguageFor(locale);
  return useQuery({
    queryKey: ["mcp-server-templates", language],
    queryFn: ({ signal }): Promise<McpServerTemplate[]> =>
      api.listMcpServerTemplates(wsId, language, signal),
    enabled: !!wsId,
    staleTime: 30 * 60 * 1000,
  });
}
