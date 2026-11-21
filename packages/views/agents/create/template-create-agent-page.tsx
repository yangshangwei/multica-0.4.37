"use client";

import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";

import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { runtimeDisplayLabel } from "@multica/core/runtimes";
import { isKnownAutonomyLevel } from "@multica/core/types";
import type { AgentRoleTemplate } from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
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
import { AutonomyBadge, RoleTemplateCard } from "./role-template-card";

export { AutonomyBadge } from "./role-template-card";

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
  const fromMarket = !squadId && navigation.searchParams.get("source") === "marketplace";

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
      // The market returns to discovery; the original flow keeps role selection
      // and its squad context when backing out of configuration.
      onBack={() => {
        if (fromMarket) backOrReplace(paths.marketplaceTab("agent-templates"));
        else if (template) navigation.replace(withSquadParam(paths.newAgentTemplate(), squadId));
        else backOrReplace(paths.newAgent());
      }}
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
    // Loading/error layouts can clamp scrollTop before restoration is possible.
    if (!didRestore.current || loading || failed || !instancesReady) return;
    // The configure step shares this pathname and can replace ordinary scroll
    // capture. Track scrolling too, so sidebar/tab navigation keeps the latest
    // gallery position rather than the last card click.
    writeViewState("agent-role-template-scroll", String(scrollElement.current?.scrollTop ?? 0));
  };

  return (
    <main ref={attachScroll} onScroll={rememberScroll} data-tab-scroll-root="agent-role-templates" aria-busy={loading || agentsPending} className="min-h-0 flex-1 overflow-y-auto px-5 py-8 sm:px-8">
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
                const instances = instancesReady
                  ? agents.filter((agent) => !agent.archived_at && agent.template_key === template.key)
                  : [];
                return (
                  <RoleTemplateCard
                    key={template.key}
                    template={template}
                    createHref={createPathWithParams(paths.newAgentTemplate(), { squad: squadId, template: template.key })}
                    createLabel={instances.length > 0 ? t(($) => $.role_templates.create_another) : t(($) => $.role_templates.create_agent)}
                    secondaryAction={instances.length > 0}
                    onNavigate={rememberScroll}
                  >
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
                  </RoleTemplateCard>
                );
              })}
            </ul>
          </>
        )}
      </div>
    </main>
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
          <ActorAvatar name={template.title} initials="" avatarUrl={`emoji:${template.avatar_emoji}`} isAgent size="xl" />
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
