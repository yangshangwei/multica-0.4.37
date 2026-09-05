"use client";

import { useEffect, useState } from "react";
import { ChevronRight } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { runtimeDisplayLabel } from "@multica/core/runtimes";
import { isKnownAutonomyLevel } from "@multica/core/types";
import type { AgentRoleTemplate } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useBackOrReplace, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { AgentConfigurationPanel } from "./agent-configuration-panel";
import { CreateAgentFooter } from "./create-agent-footer";
import { AgentCreateChip, AgentCreateShell } from "./create-shell";
import { useCreateAgentForm } from "./use-create-agent-form";
import { useCreateTemplateAgentSubmit } from "./use-create-template-agent-submit";
import { findRoleTemplate, useRoleTemplates } from "./use-role-templates";
import { createPathWithParams, withSquadParam } from "./squad-param";

/**
 * Creating an agent from a built-in role template.
 *
 * Two steps on one route, selected by `?template=<key>`: pick a role, then review
 * and configure it. The second step is the SAME configuration panel the blank
 * flow uses — the two produce the same kind of agent, so they should not look
 * like different features — with the instructions and skills shown read-only
 * because the backend owns those for a template create.
 */
export function TemplateCreateAgentPage() {
  const { t } = useT("agents");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const backOrReplace = useBackOrReplace();
  const squadId = navigation.searchParams.get("squad");
  const templateKey = navigation.searchParams.get("template");

  const { data: templates, isLoading, isError } = useRoleTemplates();
  const template = findRoleTemplate(templates, templateKey);

  return (
    <AgentCreateShell
      title={
        squadId
          ? t(($) => $.creation_studio.squad_title)
          : t(($) => $.creation_studio.title)
      }
      step={
        template
          ? t(($) => $.creation_studio.step_configure)
          : t(($) => $.role_templates.step_pick)
      }
      // Back from the configure step returns to the role list, not out of the
      // flow: picking the wrong role is the likely reason someone goes back.
      onBack={() =>
        template
          ? navigation.replace(withSquadParam(paths.newAgentTemplate(), squadId))
          : backOrReplace(paths.newAgent())
      }
      chips={
        <AgentCreateChip>
          {t(($) => $.creation_studio.modes.template.title)}
        </AgentCreateChip>
      }
    >
      {template ? (
        <TemplateConfigureStep template={template} squadId={squadId} />
      ) : (
        <RoleTemplatePicker
          templates={templates ?? []}
          loading={isLoading}
          failed={isError}
          onPick={(key) =>
            navigation.push(
              createPathWithParams(paths.newAgentTemplate(), {
                squad: squadId,
                template: key,
              }),
            )
          }
        />
      )}
    </AgentCreateShell>
  );
}

function RoleTemplatePicker({
  templates,
  loading,
  failed,
  onPick,
}: {
  templates: AgentRoleTemplate[];
  loading: boolean;
  failed: boolean;
  onPick: (key: string) => void;
}) {
  const { t } = useT("agents");
  return (
    <main className="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 py-10">
      <div className="m-auto w-full max-w-5xl">
        <div className="mx-auto max-w-2xl text-center">
          <div className="text-caption font-medium uppercase tracking-wider text-muted-foreground">
            {t(($) => $.creation_studio.eyebrow)}
          </div>
          <h2 className="mt-2 text-balance text-display-sm font-semibold tracking-tight">
            {t(($) => $.role_templates.title)}
          </h2>
          <p className="mt-3 text-pretty text-body text-muted-foreground">
            {t(($) => $.role_templates.description)}
          </p>
        </div>

        {loading ? (
          <div className="mt-9 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {[0, 1, 2, 3, 4, 5].map((key) => (
              <Skeleton key={key} className="h-40 rounded-xl" />
            ))}
          </div>
        ) : failed || templates.length === 0 ? (
          <p className="mt-9 text-center text-body text-muted-foreground">
            {failed
              ? t(($) => $.role_templates.load_failed)
              : t(($) => $.role_templates.empty)}
          </p>
        ) : (
          <div className="mt-9 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {templates.map((template) => (
              <button
                key={template.key}
                type="button"
                onClick={() => onPick(template.key)}
                className={cn(
                  "group flex h-full flex-col items-start rounded-xl border bg-card p-4 text-left",
                  "transition-[border-color,background-color,transform] hover:-translate-y-0.5 hover:border-primary/40 hover:bg-accent/30",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                )}
              >
                <div className="flex w-full items-start justify-between gap-2">
                  <span
                    className="flex size-9 items-center justify-center rounded-lg bg-muted text-title-sm"
                    aria-hidden="true"
                  >
                    {template.avatar_emoji}
                  </span>
                  <AutonomyBadge level={template.autonomy_level} />
                </div>
                <span className="mt-3 text-body font-semibold">
                  {template.title}
                </span>
                <span className="mt-1.5 text-caption leading-5 text-muted-foreground">
                  {template.description}
                </span>
                <span className="mt-auto flex items-center gap-1 pt-4 text-caption font-medium text-foreground">
                  {t(($) => $.creation_studio.continue)}
                  <ChevronRight
                    className="size-3.5 transition-transform group-hover:translate-x-0.5"
                    aria-hidden="true"
                  />
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </main>
  );
}

/**
 * The autonomy level as a badge.
 *
 * An unrecognised value renders as nothing rather than as raw text: the label
 * would be untranslated, and claiming a limit this client cannot describe is
 * worse than saying nothing. `isKnownAutonomyLevel` also treats an ABSENT level
 * as unknown, which is correct — it means the backend declared no policy.
 */
export function AutonomyBadge({
  level,
  className,
}: {
  level: string | undefined;
  className?: string;
}) {
  const { t } = useT("agents");
  if (!isKnownAutonomyLevel(level)) return null;
  return (
    <span
      className={cn(
        "rounded-full border px-2 py-0.5 text-micro font-medium text-muted-foreground",
        className,
      )}
      title={t(($) => $.role_templates.autonomy_hint[level])}
    >
      {t(($) => $.role_templates.autonomy[level])}
    </span>
  );
}

function TemplateConfigureStep({
  template,
  squadId,
}: {
  template: AgentRoleTemplate;
  squadId: string | null;
}) {
  const { t } = useT("agents");
  const form = useCreateAgentForm();
  const [seededKey, setSeededKey] = useState<string | null>(null);

  // Seed the editable fields from the template once per role. Keyed on the
  // template so switching roles re-seeds, while everything typed afterwards
  // survives a re-render.
  useEffect(() => {
    if (seededKey === template.key) return;
    setSeededKey(template.key);
    form.setDraft((current) => ({
      ...current,
      name: template.name,
      description: template.description,
    }));
  }, [form, seededKey, template.description, template.key, template.name]);

  const submit = useCreateTemplateAgentSubmit({
    templateKey: template.key,
    draft: form.draft,
    runtimeId: form.selectedRuntime?.id ?? null,
    squadId,
  });

  const canCreate =
    form.draft.name.trim().length > 0 && form.draftReady && !submit.creating;

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto w-full max-w-4xl px-5 py-8 sm:px-8">
        <div className="mb-5 flex items-start gap-3 rounded-lg border bg-card px-4 py-3">
          <span
            className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-title-sm"
            aria-hidden="true"
          >
            {template.avatar_emoji}
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-body font-semibold">{template.title}</span>
              <AutonomyBadge level={template.autonomy_level} />
              <span className="text-micro text-muted-foreground">
                {t(($) => $.role_templates.version, {
                  version: template.version,
                })}
              </span>
            </div>
            <p className="mt-1 text-caption leading-5 text-muted-foreground">
              {t(($) => $.role_templates.autonomy_hint[
                isKnownAutonomyLevel(template.autonomy_level)
                  ? template.autonomy_level
                  : "observer"
              ])}
            </p>
          </div>
          {form.selectedRuntime && (
            <span className="ml-auto hidden shrink-0 self-center rounded-full bg-muted px-2 py-1 text-caption text-muted-foreground sm:block">
              {runtimeDisplayLabel(form.selectedRuntime)}
            </span>
          )}
        </div>

        <AgentConfigurationPanel
          draft={form.draft}
          onChange={form.setDraft}
          runtimes={form.runtimes}
          runtimesLoading={form.runtimesLoading}
          members={form.members}
          currentUserId={form.currentUserId}
          nameError={submit.nameError}
          onNameChange={(name) => {
            submit.clearNameError();
            form.setDraft((current) => ({ ...current, name }));
          }}
          roleTemplate={{
            instructions: template.instructions,
            skillNames: template.skill_names,
          }}
        />
      </div>
      <CreateAgentFooter
        canCreate={canCreate}
        creating={submit.creating}
        squad={!!squadId}
        error={submit.formError}
        onCreate={() => void submit.create()}
      />
    </div>
  );
}
