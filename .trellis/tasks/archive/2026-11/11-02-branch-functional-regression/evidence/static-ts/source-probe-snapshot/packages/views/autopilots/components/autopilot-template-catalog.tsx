"use client";

import { useCallback, useId, useRef, useState } from "react";
import {
  ArrowRight,
  Bug,
  CalendarRange,
  ChartNoAxesCombined,
  Clock,
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
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { IconTile } from "../../common/icon-tile";
import { browserTimezone } from "../../common/timezone-select";
import { useT } from "../../i18n";
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

export function AutopilotTemplateCatalog({
  templates,
  loading,
  failed,
  onRetry,
  onPick,
  onStartBlank,
}: {
  templates: AutopilotTemplate[];
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onPick: (key: string) => void;
  onStartBlank?: () => void;
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
  const group = counts[selectedGroup] > 0 ? selectedGroup : "all";
  const visibleTemplates = templates.filter(
    (template) => group === "all" || PRESENTATION[template.key]?.group === group,
  );

  return (
    <section ref={attachScroll} data-tab-scroll-root="autopilot-templates" aria-labelledby={headingId} aria-busy={loading} className="min-h-0 flex-1 overflow-y-auto @container">
      <div className="px-4 py-8 @sm:px-8">
        <div className="flex flex-wrap items-start justify-between gap-4">
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
        </div>

        {failed ? (
          <div role="alert" className="mt-8 flex flex-wrap items-center gap-3 text-body text-muted-foreground">
            <p>{t(($) => $.template_picker.load_failed)}</p>
            <Button type="button" variant="outline" onClick={onRetry} className={CATALOG_FOCUS}>
              {t(($) => $.page.retry)}
            </Button>
          </div>
        ) : loading ? (
          <div role="status" className={cn("mt-8", GRID_CLASS)}>
            <span className="sr-only">{commonT(($) => $.loading)}</span>
            {[0, 1, 2, 3].map((key) => <Skeleton key={key} className="h-52 rounded-lg" />)}
          </div>
        ) : templates.length === 0 ? (
          <p className="mt-8 text-body text-muted-foreground">{t(($) => $.template_picker.empty)}</p>
        ) : (
          <>
            <div role="group" aria-label={t(($) => $.catalog.filter_label)} className="mb-5 mt-6 flex flex-wrap gap-1.5">
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
            </div>
            <ul className={GRID_CLASS}>
              {visibleTemplates.map((template, index) => {
                const presentation = PRESENTATION[template.key];
                // The schema defaults a missing title to "", like the MCP market.
                const title = template.title || template.key;
                const recommended = template.key === "workday-repo-audit";
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
                      className={cn("flex w-full min-w-0 flex-col gap-3 rounded-lg border bg-card p-4 text-left transition-colors hover:bg-accent/40 motion-reduce:transition-none", CATALOG_FOCUS)}
                    >
                      <span className="flex min-w-0 items-center gap-3">
                        <IconTile
                          icon={presentation?.icon ?? Zap}
                          tone={GROUP_ICON_TONES[presentation?.group ?? "all"]}
                        />
                        <span className="min-w-0 break-words">
                          <span className="block text-title-sm font-medium">{title}</span>
                          {recommended && (
                            <span id={`${cardId}-recommended`} className="mt-0.5 block text-caption font-medium text-foreground">{t(($) => $.catalog.recommended)}</span>
                          )}
                        </span>
                      </span>
                      <span id={`${cardId}-description`} className="line-clamp-2 min-h-[2lh] break-words text-body text-muted-foreground" title={template.description}>
                        {template.description}
                      </span>
                      <span className="flex flex-col gap-1.5 text-caption text-muted-foreground">
                        <span className="flex items-start gap-1.5">
                          <Clock className="mt-0.5 size-3 shrink-0" aria-hidden="true" />
                          <span id={`${cardId}-schedule`} className="min-w-0 break-words">{describe(parseCron(template.cron_expression, browserTimezone())) ?? template.cron_expression}</span>
                        </span>
                        {output && <span id={`${cardId}-output`}>{output}</span>}
                      </span>
                      <span className="mt-auto flex items-center gap-1.5 pt-1 text-body font-medium">
                        {t(($) => $.catalog.view_template)}
                        <ArrowRight className="size-3.5" aria-hidden="true" />
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </div>
    </section>
  );
}
