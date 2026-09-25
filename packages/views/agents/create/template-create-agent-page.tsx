"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { runtimeDisplayLabel } from "@multica/core/runtimes";
import { isKnownAutonomyLevel } from "@multica/core/types";
import type { AgentRoleTemplate } from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
import { cn } from "@multica/ui/lib/utils";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { AppLink, useBackOrReplace, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { useRestoredScrollRef, useRestoredViewState, useViewStateWriter } from "../../platform/scroll-restoration";
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
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const backOrReplace = useBackOrReplace();
  const squadId = navigation.searchParams.get("squad");
  const templateKey = navigation.searchParams.get("template");

  const { data: templates, isPending, isError, refetch } = useRoleTemplates();
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
          key={wsId}
          templates={(templates ?? []).filter((role) => role.key)}
          loading={isPending}
          failed={isError}
          onRetry={() => void refetch()}
          squadId={squadId}
        />
      )}
    </AgentCreateShell>
  );
}

function RoleTemplatePicker({
  templates,
  loading,
  failed,
  onRetry,
  squadId,
}: {
  templates: AgentRoleTemplate[];
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  squadId: string | null;
}) {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const {
    data: agents = [],
    isPending: agentsPending,
    isError: agentsFailed,
    refetch: refetchAgents,
  } = useQuery(agentListOptions(wsId));
  const instancesReady = !agentsPending && !agentsFailed;
  const restoredScrollTop = useRestoredViewState("agent-role-template-scroll");
  const writeViewState = useViewStateWriter();
  const restoreScroll = useRestoredScrollRef("agent-role-templates");
  const scrollElement = useRef<HTMLElement | null>(null);
  const didRestore = useRef(false);
  const attachScroll = useCallback((element: HTMLElement | null) => {
    scrollElement.current = element;
    // Both catalogs change the gallery height. Wait for the real instance
    // links before restoring, or skeleton/empty content clamps the offset.
    if (!element || loading || failed || !instancesReady || didRestore.current) return;
    didRestore.current = true;
    const savedTop = restoredScrollTop === undefined ? undefined : Number(restoredScrollTop);
    if (savedTop !== undefined && Number.isFinite(savedTop) && savedTop >= 0) element.scrollTop = savedTop;
    else restoreScroll(element);
  }, [failed, instancesReady, loading, restoredScrollTop, restoreScroll]);
  const rememberScroll = () => {
    // The configure step shares this pathname and can replace ordinary scroll
    // capture. Keep the gallery position in the platform's view-state channel.
    writeViewState("agent-role-template-scroll", String(scrollElement.current?.scrollTop ?? 0));
  };

  return (
    <main ref={attachScroll} data-tab-scroll-root="agent-role-templates" aria-busy={loading || agentsPending} className="min-h-0 flex-1 overflow-y-auto px-5 py-8 sm:px-8">
      <div className="mx-auto w-full max-w-5xl">
        <div className="max-w-2xl">
          <h2 className="text-balance text-title-lg font-semibold">
            {t(($) => $.role_templates.title)}
          </h2>
          <p className="mt-2 text-pretty text-body text-muted-foreground">
            {t(($) => $.role_templates.description)}
          </p>
        </div>

        {loading ? (
          <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {[0, 1, 2, 3, 4, 5].map((key) => (
              <Skeleton key={key} className="h-40 rounded-xl" />
            ))}
          </div>
        ) : failed ? (
          <div role="alert" className="mt-8 flex flex-wrap items-center gap-3 text-body text-muted-foreground">
            <p>{t(($) => $.role_templates.load_failed)}</p>
            <Button type="button" variant="outline" onClick={onRetry}>
              {t(($) => $.role_templates.retry)}
            </Button>
          </div>
        ) : templates.length === 0 ? (
          <p className="mt-8 text-body text-muted-foreground">
            {t(($) => $.role_templates.empty)}
          </p>
        ) : (
          <>
            {agentsPending ? (
              <p role="status" className="mt-6 text-caption text-muted-foreground">
                {t(($) => $.role_templates.instances_loading)}
              </p>
            ) : agentsFailed ? (
              <div role="alert" className="mt-6 flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
                <p>{t(($) => $.role_templates.instances_load_failed)}</p>
                <Button type="button" size="sm" variant="outline" onClick={() => void refetchAgents()}>
                  {t(($) => $.role_templates.retry)}
                </Button>
              </div>
            ) : null}
            <ul className="mt-6 grid gap-x-8 sm:grid-cols-2 lg:grid-cols-3">
              {templates.map((template) => {
                const title = template.title || template.name;
                const instances = instancesReady
                  ? agents.filter((agent) => !agent.archived_at && agent.template_key === template.key)
                  : [];
                return (
                  <li key={template.key} aria-label={title} className="flex min-w-0 flex-col items-start gap-3 border-t border-border py-5">
                    <div className="flex w-full flex-wrap items-start justify-between gap-2">
                      <h3 className="min-w-0 break-words text-body font-semibold">
                        {template.avatar_emoji && <span aria-hidden="true" className="mr-2">{template.avatar_emoji}</span>}
                        {title}
                      </h3>
                      <AutonomyBadge level={template.autonomy_level} />
                    </div>
                    <p className="break-words text-caption leading-5 text-muted-foreground">{template.description}</p>
                    {instancesReady && (
                      <div className="w-full min-w-0 text-caption">
                        <p className="text-muted-foreground">
                          {instances.length > 0
                            ? t(($) => $.role_templates.instances_count, { count: instances.length })
                            : t(($) => $.role_templates.instances_none)}
                        </p>
                        {instances.map((instance) => (
                          <AppLink
                            key={instance.id}
                            href={paths.agentDetail(instance.id)}
                            newTabTitle={instance.name}
                            onClick={rememberScroll}
                            onAuxClick={rememberScroll}
                            className="flex min-h-11 min-w-0 max-w-full items-center rounded-sm py-1 font-medium underline decoration-muted-foreground/50 underline-offset-4 hover:decoration-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:min-h-9"
                          >
                            <span className="min-w-0 break-words">{t(($) => $.role_templates.open_instance, { name: instance.name })}</span>
                          </AppLink>
                        ))}
                      </div>
                    )}
                    <AppLink
                      href={createPathWithParams(paths.newAgentTemplate(), { squad: squadId, template: template.key })}
                      onClick={rememberScroll}
                      onAuxClick={rememberScroll}
                      className={cn(
                        "mt-auto flex min-h-11 items-center rounded-md px-3 py-2 text-caption font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:min-h-9",
                        instances.length > 0
                          ? "text-muted-foreground hover:bg-surface-hover hover:text-foreground"
                          : "border border-border hover:bg-surface-hover",
                      )}
                    >
                      {instances.length > 0
                        ? t(($) => $.role_templates.create_another)
                        : t(($) => $.role_templates.create_agent)}
                    </AppLink>
                  </li>
                );
              })}
            </ul>
          </>
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
