"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { SkillSummary } from "@multica/core/types";
import { skillTemplateListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { BuiltinTemplateCatalog, BuiltinTemplateRow } from "../../common/builtin-template-catalog";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { useSkillPresentation } from "../hooks/use-skill-presentation";
import { getBuiltinRoleSkillPresentation } from "../lib/skill-presentation";

export function BuiltinSkillCatalog({ skills, onView }: {
  skills: readonly SkillSummary[];
  onView: (templateName: string) => void;
}) {
  const { t } = useT("skills");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const presentSkill = useSkillPresentation();
  const catalog = useQuery(skillTemplateListOptions(wsId));
  const templates = (catalog.data ?? []).filter((template) => template.name);

  return (
    <BuiltinTemplateCatalog
      copy={{
        title: t(($) => $.catalog.title), description: t(($) => $.catalog.description),
        loading: t(($) => $.catalog.loading), error: t(($) => $.catalog.error),
        empty: t(($) => $.catalog.empty), retry: t(($) => $.catalog.retry),
      }}
      count={templates.length} loading={catalog.isPending} failed={catalog.isError} empty={templates.length === 0}
      onRetry={() => void catalog.refetch()}
    >
      {templates.map((template) => {
        const presentation = getBuiltinRoleSkillPresentation(template.name, t, template.description);
        const instance = skills.find((skill) => {
          if (skill.name === template.name && presentSkill(skill).isBuiltin) return true;
          const source = skill.config?.template_source;
          return !!source && typeof source === "object" && "name" in source && source.name === template.name;
        });
        return (
          <BuiltinTemplateRow key={template.name} title={presentation?.name ?? template.name} description={presentation?.description ?? template.description}
            actions={<>
              <Button type="button" size="sm" variant="outline" onClick={() => onView(template.name)}>{t(($) => $.catalog.view_template)}</Button>
              {instance && (
                <Button type="button" size="sm" variant="ghost" onClick={() => navigation.push(paths.skillDetail(instance.id))}>
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
