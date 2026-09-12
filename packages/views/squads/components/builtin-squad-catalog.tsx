"use client";

import { useState } from "react";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Squad, SquadTemplate } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useSquadTemplates } from "../../agents/create/use-role-templates";
import { BuiltinTemplateCatalog, BuiltinTemplateRow } from "../../common/builtin-template-catalog";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { UseSquadForProjectDialog } from "../../projects/components/use-squad-for-project-dialog";

export function BuiltinSquadCatalog({ squads }: { squads: readonly Squad[] }) {
  const { t } = useT("squads");
  const catalog = useSquadTemplates();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const [selected, setSelected] = useState<SquadTemplate | null>(null);
  const templates = (catalog.data ?? []).filter((template) => template.key);

  return (
    <>
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
          const instance = squads.find((squad) => !squad.archived_at && squad.template_key === template.key);
          return (
            <BuiltinTemplateRow key={template.key} title={template.title || template.name} description={template.description}
              actions={<>
                <Button type="button" size="sm" variant="outline" onClick={() => setSelected(template)}>{t(($) => $.catalog.use_for_project)}</Button>
                {instance && (
                  <Button type="button" size="sm" variant="ghost" onClick={() => navigation.push(paths.squadDetail(instance.id))}>
                    {t(($) => $.catalog.open_instance)}
                  </Button>
                )}
              </>}
            />
          );
        })}
      </BuiltinTemplateCatalog>
      {selected && <UseSquadForProjectDialog template={selected} onClose={() => setSelected(null)} />}
    </>
  );
}
