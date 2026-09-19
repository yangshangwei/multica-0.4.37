"use client";

import { useMemo } from "react";
import { Plus } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import type { McpServerTemplate } from "@multica/core/types";
import {
  BuiltinTemplateCatalog,
  BuiltinTemplateRow,
} from "../../common/builtin-template-catalog";
import { useT } from "../../i18n";
import { useMcpServerTemplates } from "../hooks/use-mcp-server-templates";

/**
 * The built-in MCP catalog above the workspace's own library. Selecting a
 * template pre-fills the shared add dialog; nothing is saved until the user
 * confirms, and the saved entry is an ordinary write-only workspace server.
 *
 * A template already present in the library (matched by name) is shown as
 * "added" with its button disabled — the pre-fill would only be rejected by the
 * dialog's duplicate-name check. Because saved names are renameable, a renamed
 * copy is no longer matched here; that is acceptable, and the dialog remains the
 * authoritative duplicate guard.
 */
export function McpBuiltinCatalog({
  wsId,
  existingNames,
  onAdd,
}: {
  wsId: string;
  existingNames: Set<string>;
  onAdd: (preset: { name: string; config: Record<string, unknown> }) => void;
}) {
  const { t } = useT("settings");
  const templates = useMcpServerTemplates(wsId);

  // A template with no usable config would render a button that fills nothing.
  const usable = useMemo(
    () =>
      (templates.data ?? []).filter(
        (template: McpServerTemplate) =>
          !!template.key && Object.keys(template.config).length > 0,
      ),
    [templates.data],
  );

  return (
    <BuiltinTemplateCatalog
      className="rounded-lg border px-4 py-2"
      copy={{
        title: t(($) => $.mcp.builtin_title),
        description: t(($) => $.mcp.builtin_description),
        loading: t(($) => $.mcp.builtin_loading),
        error: t(($) => $.mcp.builtin_error),
        empty: t(($) => $.mcp.builtin_empty),
        retry: t(($) => $.mcp.builtin_retry),
      }}
      count={usable.length}
      loading={templates.isPending}
      failed={templates.isError}
      empty={usable.length === 0}
      onRetry={() => void templates.refetch()}
    >
      {usable.map((template) => {
        const added = existingNames.has(template.key);
        return (
          <BuiltinTemplateRow
            key={template.key}
            title={template.title || template.key}
            description={template.description}
            actions={
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={added}
                onClick={() =>
                  onAdd({ name: template.key, config: template.config })
                }
              >
                {added ? null : <Plus className="h-4 w-4" />}
                {added
                  ? t(($) => $.mcp.builtin_added)
                  : t(($) => $.mcp.builtin_add)}
              </Button>
            }
          />
        );
      })}
    </BuiltinTemplateCatalog>
  );
}
