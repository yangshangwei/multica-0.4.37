"use client";

import { useCallback, useId, useRef, useState } from "react";
import {
  ArrowRight,
  BookOpen,
  Bug,
  CalendarRange,
  ChartNoAxesCombined,
  Clock,
  TimerReset,
  Files,
  Flag,
  GitPullRequest,
  ListChecks,
  PackageSearch,
  Plus,
  ScanLine,
  ScanSearch,
  Zap,
  type LucideIcon,
} from "lucide-react";
import type { AutopilotTemplate } from "@multica/core/types";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { IconTile } from "../../common/icon-tile";
import { browserTimezone } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import {
  useRestoredScrollRef,
  useRestoredViewState,
  useViewStateWriter,
} from "../../platform/scroll-restoration";
import { parseCron } from "./schedule-editor/cron-mapping";
import { useDescribeSchedule } from "./schedule-editor/describe";

const GROUPS = ["all", "maintenance", "collaboration", "reporting"] as const;
type TemplateGroup = (typeof GROUPS)[number];

const CATALOG_FOCUS = "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-foreground focus-visible:ring-offset-2 focus-visible:ring-offset-background";

// Same column rhythm as the capability market's skill catalog.
const GRID_CLASS = "grid grid-cols-1 gap-3 @2xl:grid-cols-2 @5xl:grid-cols-3 @7xl:grid-cols-4";

// Reuse the skill library's category palette and light/dark theme tokens.
const GROUP_ICON_TONES: Record<TemplateGroup, string> = {
  all: "bg-skill-other/12 text-skill-other",
  maintenance: "bg-skill-operations/12 text-skill-operations",
  collaboration: "bg-skill-engineering/12 text-skill-engineering",
  reporting: "bg-skill-data/12 text-skill-data",
};

// Display groups are intentionally independent of the server's categories.
// Unrecognized templates stay available in All, in the server's original order.
const PRESENTATION: Record<string, { group: TemplateGroup; icon: LucideIcon } | undefined> = {
  "workday-repo-audit": { group: "maintenance", icon: ScanLine },
  "release-readiness": { group: "collaboration", icon: Flag },
  "daily-change-review": { group: "collaboration", icon: ScanSearch },
  "hourly-queue-check": { group: "maintenance", icon: ListChecks },
  "stale-pr-reminder": { group: "collaboration", icon: GitPullRequest },
  "bug-triage": { group: "collaboration", icon: Bug },
  "daily-progress-report": { group: "reporting", icon: ChartNoAxesCombined },
  "weekly-progress-report": { group: "reporting", icon: CalendarRange },
  "dependency-audit": { group: "maintenance", icon: PackageSearch },
  "documentation-check": { group: "maintenance", icon: Files },
};

const PATROL_KEYS = new Set([
  "workday-repo-audit", "hourly-queue-check", "stale-pr-reminder",
  "dependency-audit", "documentation-check",
]);

const WELCOME_TEMPLATE_KEYS = new Set([
  "daily-change-review", "stale-pr-reminder", "bug-triage",
  "weekly-progress-report", "dependency-audit", "documentation-check",
]);

const WELCOME_GRID_CLASS = "grid grid-cols-1 gap-3 @2xl:grid-cols-2";

export function AutopilotTemplateCatalog({
  templates,
  loading,
  failed,
  onRetry,
  onPick,
  onStartBlank,
  welcome,
}: {
  templates: AutopilotTemplate[];
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onPick: (key: string) => void;
  onStartBlank?: () => void;
  welcome?: { docsHref: string; templatesHref: string };
}) {
  const { t } = useT("autopilots");
  const { t: commonT } = useT("common");
  const headingId = useId();
  const describe = useDescribeSchedule();
  const restoredGroup = useRestoredViewState("autopilot-template-group");
  const restoredScrollTop = useRestoredViewState("autopilot-template-scroll");
  const pendingTemplateKey = useRestoredViewState("autopilot-template-focus");
  const writeViewState = useViewStateWriter();
  const restoreScroll = useRestoredScrollRef("autopilot-templates");
  const scrollElement = useRef<HTMLElement | null>(null);
  const templateButtons = useRef(new Map<string, HTMLButtonElement>());
  const selectedFilter = useRef<HTMLButtonElement | null>(null);
  const blankButton = useRef<HTMLButtonElement | null>(null);
  const didRestore = useRef(false);
  const attachScroll = useCallback((element: HTMLElement | null) => {
    scrollElement.current = element;
    // Wait for real content: restoring against skeletons clamps the offset.
    if (!element || loading || failed || didRestore.current) return;
    didRestore.current = true;
    const savedTop = restoredScrollTop === undefined ? undefined : Number(restoredScrollTop);
    if (savedTop !== undefined && Number.isFinite(savedTop) && savedTop >= 0) element.scrollTop = savedTop;
    else restoreScroll(element);

    if (!pendingTemplateKey) return;
    // Consume the handoff even if a dialog or another control now owns focus.
    // Refetches and later visits must not revive this return target.
    writeViewState("autopilot-template-focus", undefined);
    const doc = element.ownerDocument;
    if (doc.activeElement && doc.activeElement !== doc.body) return;
    const target = templateButtons.current.get(pendingTemplateKey) ?? selectedFilter.current ?? blankButton.current;
    target?.focus({ preventScroll: true });
  }, [loading, failed, restoredScrollTop, restoreScroll, pendingTemplateKey, writeViewState]);
  const [selectedGroup, setSelectedGroup] = useState(
    () => GROUPS.find((group) => group === restoredGroup) ?? "all",
  );
  const counts = {
    all: templates.length,
    maintenance: 0,
    collaboration: 0,
    reporting: 0,
  };
  for (const template of templates) {
    const group = PRESENTATION[template.key]?.group;
    if (group && group !== "all") counts[group]++;
  }
  // A backend deployment can remove a whole group while this view is open.
  const group = !welcome && counts[selectedGroup] > 0 ? selectedGroup : "all";
  const featuredKeys = new Set(templates.filter((template) => WELCOME_TEMPLATE_KEYS.has(template.key)).map((template) => template.key));
  // Partial or newer registries still get a useful preview. Membership is
  // curated, while the final filter keeps the server's original ordering.
  if (welcome) {
    for (const template of templates) {
      if (featuredKeys.size >= 6) break;
      featuredKeys.add(template.key);
    }
  }
  const visibleTemplates = templates.filter((template) => welcome
    ? featuredKeys.has(template.key)
    : group === "all" || PRESENTATION[template.key]?.group === group,
  );
  const gridClass = welcome ? WELCOME_GRID_CLASS : GRID_CLASS;

  return (
    <section ref={attachScroll} data-tab-scroll-root="autopilot-templates" aria-labelledby={headingId} aria-busy={loading} className="min-h-0 flex-1 overflow-y-auto @container">
      <div className={welcome
        ? "mx-auto grid min-h-full w-full max-w-[1440px] content-center gap-10 px-5 py-10 @sm:px-8 @4xl:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] @4xl:items-center @4xl:gap-10 @4xl:py-16 @6xl:gap-16 @6xl:px-12"
        : "px-4 py-8 @sm:px-8"}>
        {welcome ? (
          <div className="min-w-0">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-info/10 px-3 py-1.5 text-label font-medium text-info-foreground">
              <TimerReset className="size-3.5" aria-hidden="true" />
              {t(($) => $.page.title)}
            </span>
            <h2 id={headingId} className="mt-5 max-w-[20ch] text-balance text-display-sm font-semibold tracking-tight @sm:text-display">
              {t(($) => $.welcome.title)}
            </h2>
            <p className="mt-4 max-w-[48ch] text-pretty text-body-lg leading-relaxed text-muted-foreground">
              {t(($) => $.welcome.description)}
            </p>
            <div className="mt-7 flex flex-wrap items-center gap-3">
              {onStartBlank && (
                <Button ref={blankButton} type="button" onClick={onStartBlank} className={cn("min-h-11 gap-2 px-4", CATALOG_FOCUS)}>
                  <Plus className="size-4" aria-hidden="true" />
                  {t(($) => $.page.new_autopilot)}
                </Button>
              )}
              <AppLink href={welcome.docsHref} className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 gap-2 px-3", CATALOG_FOCUS)}>
                <BookOpen className="size-4" aria-hidden="true" />
                {t(($) => $.welcome.read_docs)}
              </AppLink>
            </div>
          </div>
        ) : <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 max-w-[70ch]">
            <h2 id={headingId} className="text-display-sm font-semibold tracking-tight">
              {t(($) => $.template_picker.title)}
            </h2>
            <p className="mt-2 text-body text-muted-foreground">
              {t(($) => $.template_picker.description)}
            </p>
          </div>
          {onStartBlank && (
            <Button ref={blankButton} type="button" variant="outline" onClick={onStartBlank} className={cn("min-h-11 @lg:pointer-fine:min-h-9", CATALOG_FOCUS)}>
              <Plus className="size-3.5" aria-hidden="true" />
              {t(($) => $.page.start_blank)}
            </Button>
          )}
        </div>}

        <div className="min-w-0">
          {welcome && (
            <div className="mb-5">
              <h3 className="text-body font-medium text-muted-foreground">{t(($) => $.template_picker.title)}</h3>
              <p className="mt-1.5 text-caption text-muted-foreground">{t(($) => $.welcome.template_hint)}</p>
            </div>
          )}

          {failed ? (
            <div role="alert" className={cn("flex flex-wrap items-center gap-3 text-body text-muted-foreground", !welcome && "mt-8")}>
              <p>{t(($) => $.template_picker.load_failed)}</p>
              <Button type="button" variant="outline" onClick={onRetry} className={cn("min-h-11", CATALOG_FOCUS)}>
                {t(($) => $.page.retry)}
              </Button>
            </div>
          ) : loading ? (
            <div role="status" className={cn(!welcome && "mt-8", gridClass)}>
              <span className="sr-only">{commonT(($) => $.loading)}</span>
              {Array.from({ length: welcome ? 6 : 4 }, (_, key) => <Skeleton key={key} className={welcome ? "h-36 rounded-xl" : "h-52 rounded-lg"} />)}
            </div>
          ) : templates.length === 0 ? (
            <p className={cn("text-body text-muted-foreground", !welcome && "mt-8")}>{t(($) => $.template_picker.empty)}</p>
          ) : (
            <>
              {!welcome && <div role="group" aria-label={t(($) => $.catalog.filter_label)} className="mb-5 mt-6 flex flex-wrap gap-1.5">
                {GROUPS.filter((key) => key === "all" || counts[key] > 0).map((key) => (
                  <Button
                    key={key}
                    type="button"
                    size="sm"
                    variant={group === key ? "secondary" : "ghost"}
                    ref={group === key ? selectedFilter : undefined}
                    aria-label={`${t(($) => $.catalog.groups[key])} ${counts[key]}`}
                    aria-pressed={group === key}
                    onClick={() => {
                      setSelectedGroup(key);
                      writeViewState("autopilot-template-group", key);
                    }}
                    // The market's chip size, kept a full touch target on coarse
                    // pointers and narrow containers.
                    className={cn(
                      "min-h-11 @sm:pointer-fine:min-h-7",
                      CATALOG_FOCUS,
                      group === key ? "font-semibold" : "font-normal text-muted-foreground",
                    )}
                  >
                    <span>{t(($) => $.catalog.groups[key])}</span>
                    <span className="text-caption font-normal tabular-nums text-muted-foreground">{counts[key]}</span>
                  </Button>
                ))}
              </div>}
              <ul className={gridClass}>
                {visibleTemplates.map((template, index) => {
                  const presentation = PRESENTATION[template.key];
                  // The schema defaults a missing title to "", like the MCP market.
                  const title = template.title || template.key;
                  const recommended = !welcome && template.key === "workday-repo-audit";
                  const cardId = `${headingId}-card-${index}`;
                  const output = template.execution_mode === "create_issue"
                    ? t(($) => $.catalog.creates_task)
                    : template.execution_mode === "run_only"
                      ? PATROL_KEYS.has(template.key)
                        ? t(($) => $.catalog.follow_up)
                        : t(($) => $.dialog.output_modes.run_only.label)
                      : null;
                  return (
                    <li key={template.key} className="flex min-w-0">
                      <button
                        type="button"
                        // A short name like the market cards; the card's facts
                        // stay available as its description.
                        aria-label={t(($) => $.catalog.view_label, { name: title })}
                        aria-describedby={[
                          recommended ? `${cardId}-recommended` : null,
                          `${cardId}-description`,
                          `${cardId}-schedule`,
                          output ? `${cardId}-output` : null,
                        ].filter(Boolean).join(" ")}
                        ref={(element) => {
                          if (element) templateButtons.current.set(template.key, element);
                          else templateButtons.current.delete(template.key);
                        }}
                        onClick={() => {
                          // The configure step shares this pathname. Desktop's
                          // plain-scroll capture is replaced when that step leaves,
                          // so keep the gallery position in the view-state channel.
                          writeViewState("autopilot-template-group", group);
                          writeViewState("autopilot-template-scroll", String(scrollElement.current?.scrollTop ?? 0));
                          writeViewState("autopilot-template-focus", template.key);
                          onPick(template.key);
                        }}
                        className={cn(
                          "w-full min-w-0 bg-card text-left transition-colors motion-reduce:transition-none",
                          welcome
                            ? "grid grid-cols-[2.5rem_minmax(0,1fr)] content-start gap-x-3 gap-y-1 rounded-xl border border-surface-border p-4 shadow-[var(--surface-shadow)] hover:border-input hover:bg-surface-hover"
                            : "flex flex-col gap-3 rounded-lg border p-4 hover:bg-accent/40",
                          CATALOG_FOCUS,
                        )}
                      >
                        <span className={welcome ? "contents" : "flex min-w-0 items-center gap-3"}>
                          <IconTile
                            icon={presentation?.icon ?? Zap}
                            tone={welcome ? "bg-muted text-foreground" : GROUP_ICON_TONES[presentation?.group ?? "all"]}
                            className={welcome ? "row-span-3 self-start" : undefined}
                          />
                          <span className="min-w-0 break-words">
                            <span className="block text-title-sm font-medium">{title}</span>
                            {recommended && (
                              <span id={`${cardId}-recommended`} className="mt-0.5 block text-caption font-medium text-foreground">{t(($) => $.catalog.recommended)}</span>
                            )}
                          </span>
                        </span>
                        <span id={`${cardId}-description`} className={cn("line-clamp-2 min-h-[2lh] break-words text-body text-muted-foreground", welcome && "col-start-2")} title={template.description}>
                          {template.description}
                        </span>
                        <span className={cn("flex flex-col gap-1.5 text-caption text-muted-foreground", welcome && "col-start-2 mt-2")}>
                          <span className="flex items-start gap-1.5">
                            <Clock className="mt-0.5 size-3 shrink-0" aria-hidden="true" />
                            <span id={`${cardId}-schedule`} className="min-w-0 break-words">{describe(parseCron(template.cron_expression, browserTimezone())) ?? template.cron_expression}</span>
                          </span>
                          {output && <span id={`${cardId}-output`} className={welcome ? "sr-only" : undefined}>{output}</span>}
                        </span>
                        {!welcome && <span className="mt-auto flex items-center gap-1.5 pt-1 text-body font-medium">
                          {t(($) => $.catalog.view_template)}
                          <ArrowRight className="size-3.5" aria-hidden="true" />
                        </span>}
                      </button>
                    </li>
                  );
                })}
              </ul>
            </>
          )}
          {welcome && (
            <AppLink href={welcome.templatesHref} className={cn("mt-4 inline-flex min-h-11 items-center gap-1.5 rounded-md text-body text-muted-foreground underline-offset-4 hover:text-foreground hover:underline", CATALOG_FOCUS)}>
              {t(($) => $.welcome.browse_all)}
              <ArrowRight className="size-3.5" aria-hidden="true" />
            </AppLink>
          )}
        </div>
      </div>
    </section>
  );
}
