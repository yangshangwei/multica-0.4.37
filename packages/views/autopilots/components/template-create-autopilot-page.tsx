"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ArrowLeft,
  ChevronDown,
  Clock,
  FilePlus2,
  FolderKanban,
  Loader2,
  Play,
  Rocket,
} from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCreateAutopilotFromTemplate } from "@multica/core/autopilots/mutations";
import { isAgentRuntimeBound } from "@multica/core/agents";
import {
  canAssignAgentToIssue,
  useCurrentMember,
} from "@multica/core/permissions";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  agentListOptions,
  memberListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import type {
  AutopilotExecutionMode,
  AutopilotTemplate,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { browserTimezone, timezoneOptions } from "../../common/timezone-select";
import { useT, useLocale } from "../../i18n";
import { useBackOrReplace, useNavigation } from "../../navigation";
import { ProjectIcon } from "../../projects/components/project-icon";
import { ProjectPicker } from "../../projects/components/project-picker";
import { templateLanguageFor } from "../../agents/create/use-role-templates";
import {
  findAutopilotTemplate,
  useAutopilotTemplates,
} from "../use-autopilot-templates";
import {
  autopilotTemplateDefaultsFromSearch,
  autopilotTemplateHref,
  type AutopilotTemplateDefaults,
} from "../template-create-defaults";
import { AutopilotDialog } from "./autopilot-dialog";
import { AutopilotTemplateCatalog } from "./autopilot-template-catalog";
import { AgentPicker, type AssigneeSelection } from "./pickers/agent-picker";
import { TimezonePicker } from "./pickers/timezone-picker";
import { parseCron } from "./schedule-editor/cron-mapping";
import { useDescribeSchedule } from "./schedule-editor/describe";
import type { ScheduleConfig } from "./schedule-editor/model";

/**
 * Adopting a built-in autopilot template.
 *
 * Two steps on one route, selected by `?template=<key>`: pick a template, then
 * say who runs it. It cannot be one click — `assignee_id` is required and a
 * template cannot know which agent a workspace owns — so this mirrors the agent
 * role-template flow rather than inventing a shape of its own.
 *
 * The picked template's prompt, schedule and output mode render read-only: the
 * server takes all three from the template so a client cannot claim a
 * template's provenance while supplying its own brief. Every one of them is
 * editable on the autopilot afterwards.
 */
export function TemplateCreateAutopilotRoute() {
  const navigation = useNavigation();
  return (
    <TemplateCreateAutopilotPage
      {...autopilotTemplateDefaultsFromSearch(navigation.searchParams)}
    />
  );
}

export function TemplateCreateAutopilotPage(defaults: AutopilotTemplateDefaults) {
  const { t } = useT("autopilots");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const backOrReplace = useBackOrReplace();
  const templateKey = navigation.searchParams.get("template");
  const pickerHref = defaults.initialReturnTo === "autopilots"
    ? paths.autopilots()
    : autopilotTemplateHref(paths.newAutopilotTemplate(), defaults);

  const { data: templates, isLoading, isError, refetch } = useAutopilotTemplates();
  const template = findAutopilotTemplate(templates, templateKey);
  // A deep link to a key this server does not ship — an honest dead end beats
  // a picker that silently pretends the link never happened. Only verdict-able
  // once the list has actually loaded: while loading or after a failed load
  // the picker's own states speak, and a missing key over an unread list
  // would report the load, not the key.
  const templateMissing =
    Boolean(templateKey) &&
    !isLoading &&
    !isError &&
    templates !== undefined &&
    template === null;

  return (
    <AutopilotCreateShell
      step={
        template
          ? t(($) => $.template_picker.step_configure)
          : t(($) => $.template_picker.step_pick)
      }
      // Return to the originating gallery so its route-scoped browsing state
      // restores, including when the gallery was the empty automation list.
      onBack={() =>
        template
          ? navigation.replace(pickerHref)
          : backOrReplace(paths.autopilots())
      }
    >
      {templateMissing ? (
        <TemplateNotFoundStep pickerHref={pickerHref} />
      ) : template ? (
        <TemplateConfigureStep key={wsId} template={template} {...defaults} />
      ) : (
        <AutopilotTemplatePicker
          templates={templates ?? []}
          loading={isLoading}
          failed={isError}
          onRetry={() => void refetch()}
          onPick={(key) =>
            navigation.push(
              autopilotTemplateHref(paths.newAutopilotTemplate(), defaults, key),
            )
          }
        />
      )}
    </AutopilotCreateShell>
  );
}

/**
 * The `?template=` deep link named a key the server does not ship — an older
 * binary, an offline deployment, a typo. Centered, with one way out: back to
 * the picker, where every template the server does ship is one click away.
 */
function TemplateNotFoundStep({ pickerHref }: { pickerHref: string }) {
  const { t } = useT("autopilots");
  const navigation = useNavigation();
  return (
    <main className="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 py-10">
      <div className="m-auto flex flex-col items-center gap-3 text-center">
        <p className="text-body text-muted-foreground">
          {t(($) => $.template_picker.not_found)}
        </p>
        <Button
          size="sm"
          variant="outline"
          onClick={() => navigation.replace(pickerHref)}
        >
          {t(($) => $.template_picker.back)}
        </Button>
      </div>
    </main>
  );
}

/** Chrome shared by both steps: back control, flow title, current step. */
function AutopilotCreateShell({
  step,
  onBack,
  children,
}: {
  step: string;
  onBack: () => void;
  children: React.ReactNode;
}) {
  const { t } = useT("autopilots");
  return (
    <div className="flex min-h-0 flex-1 flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b px-5">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={onBack}
          aria-label={t(($) => $.template_picker.back)}
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
        </Button>
        <div className="min-w-0">
          <h1 className="truncate text-body font-semibold">
            {t(($) => $.page.new_autopilot)}
          </h1>
          <p className="truncate text-caption text-muted-foreground">{step}</p>
        </div>
      </header>
      {children}
    </div>
  );
}

/**
 * A template's schedule in plain language, falling back to the raw expression
 * for anything the structured model cannot describe.
 *
 * The readback carries no timezone — the zone is stated as its own field — so
 * which one `parseCron` hydrates the config with does not reach the text.
 */
function scheduleText(
  describe: (config: ScheduleConfig) => string | null,
  cron: string,
  timezone: string,
): string {
  return describe(parseCron(cron, timezone)) ?? cron;
}

/**
 * The output mode as a value this client can describe, or null.
 *
 * A mode shipped by a newer server renders as nothing rather than as its raw
 * wire value: the label would be untranslated, and naming a behaviour we cannot
 * describe is worse than staying quiet about it.
 */
function knownExecutionMode(mode: string): AutopilotExecutionMode | null {
  return mode === "create_issue" || mode === "run_only" ? mode : null;
}

const OUTPUT_MODE_ICONS: Record<AutopilotExecutionMode, typeof FilePlus2> = {
  create_issue: FilePlus2,
  run_only: Play,
};

function AutopilotTemplatePicker({
  templates,
  loading,
  failed,
  onRetry,
  onPick,
}: {
  templates: AutopilotTemplate[];
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onPick: (key: string) => void;
}) {
  const [blankOpen, setBlankOpen] = useState(false);
  return (
    <>
      <AutopilotTemplateCatalog
        templates={templates}
        loading={loading}
        failed={failed}
        onRetry={onRetry}
        onPick={onPick}
        onStartBlank={() => setBlankOpen(true)}
      />
      {blankOpen && (
        <AutopilotDialog
          mode="create"
          open={blankOpen}
          onOpenChange={setBlankOpen}
        />
      )}
    </>
  );
}

function TemplateConfigureStep({
  template,
  ...initialDefaults
}: { template: AutopilotTemplate } & AutopilotTemplateDefaults) {
  const { t } = useT("autopilots");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const describe = useDescribeSchedule();

  const [defaults] = useState(() => ({
    projectId: initialDefaults.initialProjectId,
    assignee:
      initialDefaults.initialAssigneeType && initialDefaults.initialAssigneeId
        ? {
            type: initialDefaults.initialAssigneeType,
            id: initialDefaults.initialAssigneeId,
          }
        : null,
  }));
  const suggestedAssignee = defaults.assignee;
  // Undefined means untouched while the workspace choices are loading. Any
  // user selection, including clearing the project, ends default seeding.
  const [assignee, setAssignee] = useState<AssigneeSelection | null>();
  const [projectId, setProjectId] = useState<string | null>();
  const [timezone, setTimezone] = useState(() => browserTimezone());

  const agentsQuery = useQuery(agentListOptions(wsId));
  const squadsQuery = useQuery(squadListOptions(wsId));
  const projectsQuery = useQuery(projectListOptions(wsId));
  const membersQuery = useQuery(memberListOptions(wsId));
  const { userId, role } = useCurrentMember(wsId);
  const agents = agentsQuery.data ?? [];
  const squads = squadsQuery.data ?? [];
  const projects = projectsQuery.data ?? [];
  const assigneeChoicesReady =
    agentsQuery.isSuccess && squadsQuery.isSuccess && membersQuery.isSuccess &&
    userId !== null && role !== null;
  const canSelectAssignee = (selection: AssigneeSelection | null | undefined) => {
    if (!selection || !assigneeChoicesReady || role === null) return false;
    const squad =
      selection.type === "squad"
        ? squads.find((item) =>
            item.id === selection.id && item.workspace_id === wsId && !item.archived_at,
          )
        : null;
    const agentId = selection.type === "agent" ? selection.id : squad?.leader_id;
    const agent = agents.find((item) =>
      item.id === agentId && item.workspace_id === wsId && !item.archived_at,
    );
    return !!agent && isAgentRuntimeBound(agent) &&
      canAssignAgentToIssue(agent, { userId, role }).allowed;
  };
  const suggestedAssigneeAllowed = canSelectAssignee(suggestedAssignee);
  const suggestedProject = projects.find((project) =>
    project.id === defaults.projectId && project.workspace_id === wsId,
  );

  useEffect(() => {
    if (assignee !== undefined || !assigneeChoicesReady) return;
    setAssignee(suggestedAssigneeAllowed ? suggestedAssignee : null);
  }, [assignee, assigneeChoicesReady, suggestedAssignee, suggestedAssigneeAllowed]);

  useEffect(() => {
    if (projectId !== undefined || !projectsQuery.isSuccess) return;
    setProjectId(suggestedProject?.id ?? null);
  }, [projectId, projectsQuery.isSuccess, suggestedProject?.id]);

  const assigneeName =
    !assignee
      ? null
      : (assignee.type === "squad"
          ? squads.find((squad) => squad.id === assignee.id)?.name
          : agents.find((agent) => agent.id === assignee.id)?.name) ?? null;
  const selectedProject =
    projects.find((project) =>
      project.id === projectId && project.workspace_id === wsId,
    ) ?? null;
  const assigneeAvailable = canSelectAssignee(assignee);
  const projectAvailable = projectId === null || selectedProject !== null;
  const choicesFailed =
    agentsQuery.isError || squadsQuery.isError || projectsQuery.isError || membersQuery.isError;
  const assigneeUnavailable =
    assigneeChoicesReady && !assigneeAvailable && !!(assignee || suggestedAssignee);
  const projectUnavailable = projectsQuery.isSuccess && (
    projectId ? !selectedProject : !!defaults.projectId && !suggestedProject
  );

  const create = useCreateAutopilotFromTemplate();
  const mode = knownExecutionMode(template.execution_mode);
  const ModeIcon = mode ? OUTPUT_MODE_ICONS[mode] : null;

  // A response the schema could not read is not an exception: parseWithFallback
  // returns the conservative empty shape so the UI keeps rendering, which means
  // a drifted backend reaches here as a RESOLVED mutation carrying no id. That
  // has to be reported rather than celebrated, and it cannot ride on
  // `create.isError` — the mutation succeeded.
  const [unreadableResponse, setUnreadableResponse] = useState(false);

  const formError = create.isError
    ? create.error instanceof Error && create.error.message
      ? create.error.message
      : t(($) => $.dialog.toast_create_failed)
    : unreadableResponse
      ? t(($) => $.dialog.toast_create_failed)
      : null;

  const handleCreate = async () => {
    if (!assignee || !assigneeAvailable || !projectAvailable || create.isPending) {
      return;
    }
    setUnreadableResponse(false);
    try {
      const created = await create.mutateAsync({
        template_key: template.key,
        assignee_id: assignee.id,
        assignee_type: assignee.type,
        project_id: projectId ?? null,
        timezone,
        // Selects which localized title lands on the row. The prompt is
        // English on every server, by design.
        language: templateLanguageFor(locale),
      });
      // `id === ""` is the schema fallback's marker for "this payload did not
      // parse". Navigating on it would land on a detail route with no id, under
      // a success toast for an automation we cannot confirm exists.
      if (!created.autopilot.id) {
        setUnreadableResponse(true);
        return;
      }
      toast.success(t(($) => $.dialog.toast_created));
      navigation.push(paths.autopilotDetail(created.autopilot.id));
    } catch {
      // Rendered inline by `formError`; the footer is where the user is
      // looking, and a toast would scroll away from the button that failed.
    }
  };

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto w-full max-w-3xl px-5 py-8 sm:px-8">
        <div className="flex items-start gap-3 rounded-lg border bg-card px-4 py-3">
          <span
            className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-title-sm"
            aria-hidden="true"
          >
            {template.avatar_emoji}
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-body font-semibold">{template.title}</span>
              <span className="text-micro text-muted-foreground">
                {t(($) => $.template_picker.version, {
                  version: template.version,
                })}
              </span>
            </div>
            <p className="mt-1 text-caption leading-5 text-muted-foreground">
              {template.description}
            </p>
          </div>
          <span className="ml-auto hidden shrink-0 self-center rounded-full bg-muted px-2 py-1 text-caption text-muted-foreground sm:block">
            {template.category_label}
          </span>
        </div>

        {/* What the template decides, stated as facts rather than as controls:
            the create request does not carry any of them. */}
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          <ReadonlyFact
            icon={Clock}
            label={t(($) => $.template_picker.schedule_label)}
            value={scheduleText(describe, template.cron_expression, timezone)}
            hint={t(($) => $.catalog.default_schedule_note)}
          />
          {mode && ModeIcon && (
            <ReadonlyFact
              icon={ModeIcon}
              label={t(($) => $.dialog.section_output_mode)}
              value={t(($) => $.dialog.output_modes[mode].label)}
              hint={t(($) => $.dialog.output_modes[mode].description)}
            />
          )}
        </div>

        <section className="mt-4">
          <div className="flex items-baseline justify-between gap-3">
            <span className="text-caption font-medium">
              {t(($) => $.template_picker.prompt_label)}
            </span>
            <span className="text-micro text-muted-foreground">
              {t(($) => $.template_picker.prompt_note)}
            </span>
          </div>
          <pre className="mt-2 max-h-72 overflow-y-auto whitespace-pre-wrap rounded-md border bg-muted/40 p-3 font-mono text-micro leading-5 text-muted-foreground">
            {template.prompt}
          </pre>
        </section>

        <section className="mt-4 space-y-4 rounded-lg border bg-card p-4">
          <div>
            <FieldLabel required>
              {t(($) => $.dialog.section_assignee)}
            </FieldLabel>
            <p className="mb-2 text-micro text-muted-foreground">
              {t(($) => $.template_picker.assignee_hint)}
            </p>
            <AgentPicker
              assignee={assignee ?? null}
              onChange={setAssignee}
              canSelect={canSelectAssignee}
              align="start"
              triggerRender={
                <button
                  type="button"
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-md border bg-background px-3 py-2 text-left",
                    "cursor-pointer transition-colors hover:bg-accent/40",
                  )}
                >
                  {assignee ? (
                    <ActorAvatar
                      actorType={assignee.type}
                      actorId={assignee.id}
                      size="md"
                      showStatusDot={assignee.type === "agent"}
                    />
                  ) : (
                    <span className="inline-flex size-7 items-center justify-center rounded-full bg-muted text-muted-foreground">
                      <Rocket className="size-3.5" aria-hidden="true" />
                    </span>
                  )}
                  <span className="min-w-0 flex-1 truncate text-body font-medium">
                    {assigneeName ?? t(($) => $.dialog.select_assignee)}
                  </span>
                  <ChevronDown
                    className="size-3.5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </button>
              }
            />
            {assigneeUnavailable && (
              <p className="mt-2 text-caption text-destructive">
                {t(($) => $.template_picker.assignee_unavailable)}
              </p>
            )}
          </div>

          <div>
            <FieldLabel>{t(($) => $.dialog.section_project)}</FieldLabel>
            <p className="mb-2 text-micro text-muted-foreground">
              {t(($) => $.dialog.project_hint)}
            </p>
            <ProjectPicker
              projectId={projectId ?? null}
              onUpdate={(updates) => setProjectId(updates.project_id ?? null)}
              align="start"
              triggerRender={
                <button
                  type="button"
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-md border bg-background px-3 py-2 text-left",
                    "cursor-pointer transition-colors hover:bg-accent/40",
                  )}
                >
                  {selectedProject ? (
                    <ProjectIcon project={selectedProject} size="md" />
                  ) : (
                    <span className="inline-flex size-5 items-center justify-center rounded-md bg-muted text-muted-foreground">
                      <FolderKanban className="size-3.5" aria-hidden="true" />
                    </span>
                  )}
                  <span className="min-w-0 flex-1 truncate text-body font-medium">
                    {selectedProject?.title ?? t(($) => $.dialog.no_project)}
                  </span>
                  <ChevronDown
                    className="size-3.5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                </button>
              }
            />
            {projectUnavailable && (
              <p className="mt-2 text-caption text-destructive">
                {t(($) => $.template_picker.project_unavailable)}
              </p>
            )}
          </div>

          <div>
            <FieldLabel>
              {t(($) => $.schedule_editor.timezone_label)}
            </FieldLabel>
            <TimezonePicker
              value={timezone}
              onChange={setTimezone}
              options={timezoneOptions(timezone)}
              ariaLabel={t(($) => $.schedule_editor.a11y.timezone)}
            />
          </div>
        </section>

        <p className="mt-4 text-caption text-muted-foreground">
          {t(($) => $.template_picker.enable_hint)}
        </p>
        {choicesFailed && (
          <div className="mt-3 flex items-center gap-3">
            <p role="alert" className="text-caption text-destructive">
              {t(($) => $.template_picker.choices_load_failed)}
            </p>
            <Button size="sm" variant="outline" onClick={() => {
              void agentsQuery.refetch();
              void squadsQuery.refetch();
              void projectsQuery.refetch();
              void membersQuery.refetch();
            }}>
              {t(($) => $.page.retry)}
            </Button>
          </div>
        )}
      </div>

      <div className="pe-chat-launcher sticky bottom-0 mt-8 flex items-center justify-between gap-3 border-t bg-background/95 py-3 pl-5 backdrop-blur">
        {formError ? (
          <p
            role="alert"
            className="min-w-0 flex-1 break-words text-body text-destructive"
          >
            {formError}
          </p>
        ) : null}
        <Button
          type="button"
          className="ml-auto shrink-0"
          onClick={() => void handleCreate()}
          disabled={!assigneeAvailable || !projectAvailable || create.isPending}
          aria-busy={create.isPending || undefined}
        >
          {create.isPending && <Loader2 className="size-4 animate-spin" />}
          {create.isPending
            ? t(($) => $.template_picker.enabling)
            : t(($) => $.template_picker.enable)}
        </Button>
      </div>
    </div>
  );
}

function FieldLabel({
  children,
  required,
}: {
  children: React.ReactNode;
  /** The sighted user's advance warning that this field blocks enabling. */
  required?: boolean;
}) {
  return (
    <div className="mb-2 text-micro font-semibold uppercase tracking-[0.08em] text-muted-foreground">
      {children}
      {required === true && (
        <span aria-hidden className="ml-0.5 text-destructive">
          *
        </span>
      )}
    </div>
  );
}

function ReadonlyFact({
  icon: Icon,
  label,
  value,
  hint,
}: {
  icon: typeof Clock;
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="flex items-start gap-2.5 rounded-lg border bg-card px-3 py-2.5">
      <Icon
        className="mt-0.5 size-4 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <div className="min-w-0">
        <div className="text-micro font-semibold uppercase tracking-[0.08em] text-muted-foreground">
          {label}
        </div>
        <div className="mt-1 text-body font-medium">{value}</div>
        {hint ? (
          <div className="mt-0.5 text-caption text-muted-foreground">
            {hint}
          </div>
        ) : null}
      </div>
    </div>
  );
}
