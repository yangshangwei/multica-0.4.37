"use client";

import type { Agent } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { BuiltinTemplateCatalog, BuiltinTemplateRow } from "../../common/builtin-template-catalog";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { createPathWithParams } from "../create/squad-param";
import { useRoleTemplates } from "../create/use-role-templates";

export function BuiltinAgentCatalog({ agents }: { agents: readonly Agent[] }) {
  const { t } = useT("agents");
  const catalog = useRoleTemplates();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const templates = (catalog.data ?? []).filter((template) => template.key);

  return (
    <BuiltinTemplateCatalog
      copy={{
        title: t(($) => $.catalog.title), description: t(($) => $.catalog.description),
        loading: t(($) => $.catalog.loading), error: t(($) => $.catalog.error),
        empty: t(($) => $.catalog.empty), retry: t(($) => $.catalog.retry),
      }}
      loading={catalog.isPending} failed={catalog.isError} empty={templates.length === 0}
      onRetry={() => void catalog.refetch()}
    >
      {templates.map((template) => {
        const instance = agents.find((agent) => !agent.archived_at && agent.template_key === template.key);
        return (
          <BuiltinTemplateRow key={template.key} title={template.title || template.name} description={template.description}
            actions={<>
              <Button type="button" size="sm" variant="outline" onClick={() => navigation.push(createPathWithParams(paths.newAgentTemplate(), { template: template.key }))}>
                {t(($) => $.catalog.view_template)}
              </Button>
              {instance && (
                <Button type="button" size="sm" variant="ghost" onClick={() => navigation.push(paths.agentDetail(instance.id))}>
                  {t(($) => $.catalog.open_instance)}
                </Button>
              )}
            </>}
          />
        );
      })}
    </BuiltinTemplateCatalog>
  );
}
