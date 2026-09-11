"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ArrowLeft,
  ChevronDown,
  ChevronRight,
  Clock,
  FilePlus2,
  FolderKanban,
  Loader2,
  Play,
  Plus,
  Rocket,
} from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCreateAutopilotFromTemplate } from "@multica/core/autopilots/mutations";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  agentListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import type {
  AutopilotExecutionMode,
  AutopilotTemplate,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
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
import { AutopilotDialog } from "./autopilot-dialog";
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
export function TemplateCreateAutopilotPage() {
  const { t } = useT("autopilots");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const backOrReplace = useBackOrReplace();
  const templateKey = navigation.searchParams.get("template");

  const { data: templates, isLoading, isError } = useAutopilotTemplates();
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
      // Back from the configure step returns to the template list, not out of
      // the flow: picking the wrong template is the likely reason to go back.
      onBack={() =>
        template
          ? navigation.replace(paths.newAutopilotTemplate())
          : backOrReplace(paths.autopilots())
      }
    >
      {templateMissing ? (
        <TemplateNotFoundStep />
      ) : template ? (
        <TemplateConfigureStep template={template} />
      ) : (
        <AutopilotTemplatePicker
          templates={templates ?? []}
          loading={isLoading}
          failed={isError}
          onPick={(key) =>
            navigation.push(
              `${paths.newAutopilotTemplate()}?template=${encodeURIComponent(key)}`,
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
function TemplateNotFoundStep() {
  const { t } = useT("autopilots");
  const paths = useWorkspacePaths();
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
          onClick={() => navigation.replace(paths.newAutopilotTemplate())}
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
  onPick,
}: {
  templates: AutopilotTemplate[];
  loading: boolean;
  failed: boolean;
  onPick: (key: string) => void;
}) {
  const { t } = useT("autopilots");
  const describe = useDescribeSchedule();
  const [blankOpen, setBlankOpen] = useState(false);

  return (
    <main className="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 py-10">
      <div className="m-auto w-full max-w-5xl">
        <div className="mx-auto max-w-2xl text-center">
          <div className="text-caption font-medium uppercase tracking-wider text-muted-foreground">
            {t(($) => $.template_picker.eyebrow)}
          </div>
          <h2 className="mt-2 text-balance text-display-sm font-semibold tracking-tight">
            {t(($) => $.template_picker.title)}
          </h2>
          <p className="mt-3 text-pretty text-body text-muted-foreground">
            {t(($) => $.template_picker.description)}
          </p>
        </div>

        {loading ? (
          <div className="mt-9 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {[0, 1, 2, 3].map((key) => (
              <Skeleton key={key} className="h-40 rounded-xl" />
            ))}
          </div>
        ) : failed || templates.length === 0 ? (
          <p className="mt-9 text-center text-body text-muted-foreground">
            {failed
              ? t(($) => $.template_picker.load_failed)
              : t(($) => $.template_picker.empty)}
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
                <span
                  className="flex size-9 items-center justify-center rounded-lg bg-muted text-title-sm"
                  aria-hidden="true"
                >
                  {template.avatar_emoji}
                </span>
                <span className="mt-3 text-caption font-medium uppercase tracking-wider text-muted-foreground">
                  {template.category_label}
                </span>
                <span className="mt-1 text-body font-semibold">
                  {template.title}
                </span>
                <span className="mt-1.5 text-caption leading-5 text-muted-foreground">
                  {template.description}
                </span>
                <span className="mt-auto flex w-full items-center gap-2 pt-4 text-caption">
                  <Clock
                    className="size-3.5 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 truncate text-muted-foreground">
                    {scheduleText(
                      describe,
                      template.cron_expression,
                      browserTimezone(),
                    )}
                  </span>
                  <span className="ml-auto flex shrink-0 items-center gap-1 font-medium text-foreground">
                    {t(($) => $.template_picker.continue)}
                    <ChevronRight
                      className="size-3.5 transition-transform group-hover:translate-x-0.5"
                      aria-hidden="true"
                    />
                  </span>
                </span>
              </button>
            ))}
          </div>
        )}

        {/* The blank flow is kept reachable from here, because this screen is
            now what "New autopilot" opens. It is the same dialog the list used
            to open, not a second creation path. */}
        <div className="mt-9 flex flex-col items-center gap-2">
          <p className="text-caption text-muted-foreground">
            {t(($) => $.template_picker.blank_hint)}
          </p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setBlankOpen(true)}
          >
            <Plus className="mr-1 size-3.5" aria-hidden="true" />
            {t(($) => $.page.start_blank)}
          </Button>
        </div>
      </div>

      {blankOpen && (
        <AutopilotDialog
          mode="create"
          open={blankOpen}
          onOpenChange={setBlankOpen}
        />
      )}
    </main>
  );
}

function TemplateConfigureStep({ template }: { template: AutopilotTemplate }) {
  const { t } = useT("autopilots");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const describe = useDescribeSchedule();

  const [assignee, setAssignee] = useState<AssigneeSelection | null>(null);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [timezone, setTimezone] = useState(() => browserTimezone());

  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));
  const { data: projects = [] } = useQuery(projectListOptions(wsId));

  const assigneeName =
    assignee === null
      ? null
      : (assignee.type === "squad"
          ? squads.find((squad) => squad.id === assignee.id)?.name
          : agents.find((agent) => agent.id === assignee.id)?.name) ?? null;
  const selectedProject =
    projects.find((project) => project.id === projectId) ?? null;

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
    if (assignee === null || create.isPending) return;
    setUnreadableResponse(false);
    try {
      const created = await create.mutateAsync({
        template_key: template.key,
        assignee_id: assignee.id,
        assignee_type: assignee.type,
        project_id: projectId,
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
              assignee={assignee}
              onChange={setAssignee}
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
          </div>

          <div>
            <FieldLabel>{t(($) => $.dialog.section_project)}</FieldLabel>
            <p className="mb-2 text-micro text-muted-foreground">
              {t(($) => $.dialog.project_hint)}
            </p>
            <ProjectPicker
              projectId={projectId}
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
          disabled={assignee === null || create.isPending}
          aria-busy={create.isPending || undefined}
        >
          {create.isPending && <Loader2 className="size-4 animate-spin" />}
          {create.isPending
            ? t(($) => $.dialog.creating)
            : t(($) => $.dialog.create)}
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
  /** The sighted user's advance warning that this field blocks Create. */
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
