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
  const writeViewState = useViewStateWriter();
  const restoreScroll = useRestoredScrollRef("autopilot-templates");
  const scrollElement = useRef<HTMLElement | null>(null);
  const didRestore = useRef(false);
  const attachScroll = useCallback((element: HTMLElement | null) => {
    scrollElement.current = element;
    // Wait for real content: restoring against skeletons clamps the offset.
    if (!element || loading || failed || didRestore.current) return;
    didRestore.current = true;
    const savedTop = restoredScrollTop === undefined ? undefined : Number(restoredScrollTop);
    if (savedTop !== undefined && Number.isFinite(savedTop) && savedTop >= 0) element.scrollTop = savedTop;
    else restoreScroll(element);
  }, [loading, failed, restoredScrollTop, restoreScroll]);
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
      <div className="mx-auto w-full max-w-[70rem] px-4 py-8 @sm:px-8">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 id={headingId} className="text-display-sm font-semibold tracking-tight">
              {t(($) => $.template_picker.title)}
            </h2>
            <p className="mt-2 text-body text-muted-foreground">
              {t(($) => $.template_picker.description)}
            </p>
          </div>
          {onStartBlank && (
            <Button type="button" variant="outline" onClick={onStartBlank} className="min-h-11 @sm:min-h-9">
              <Plus className="size-3.5" aria-hidden="true" />
              {t(($) => $.page.start_blank)}
            </Button>
          )}
        </div>

        {failed ? (
          <div role="alert" className="mt-8 flex flex-wrap items-center gap-3 text-body text-muted-foreground">
            <p>{t(($) => $.template_picker.load_failed)}</p>
            <Button type="button" variant="outline" onClick={onRetry}>
              {t(($) => $.page.retry)}
            </Button>
          </div>
        ) : loading ? (
          <div role="status" className="mt-8 grid gap-4 @xl:grid-cols-2 @4xl:grid-cols-3">
            <span className="sr-only">{commonT(($) => $.loading)}</span>
            {[0, 1, 2, 3].map((key) => <Skeleton key={key} className="h-48 rounded-lg" />)}
          </div>
        ) : templates.length === 0 ? (
          <p className="mt-8 text-body text-muted-foreground">{t(($) => $.template_picker.empty)}</p>
        ) : (
          <>
            <div role="group" aria-label={t(($) => $.catalog.filter_label)} className="mb-5 mt-6 flex flex-wrap gap-1">
              {GROUPS.filter((key) => key === "all" || counts[key] > 0).map((key) => (
                <Button
                  key={key}
                  type="button"
                  variant="ghost"
                  aria-label={`${t(($) => $.catalog.groups[key])} ${counts[key]}`}
                  aria-pressed={group === key}
                  onClick={() => {
                    setSelectedGroup(key);
                    writeViewState("autopilot-template-group", key);
                  }}
                  className={cn(
                    "min-h-11 gap-2 px-3 @sm:min-h-9",
                    group === key
                      ? "bg-surface-selected font-semibold text-foreground hover:bg-surface-selected"
                      : "font-normal text-muted-foreground",
                  )}
                >
                  <span>{t(($) => $.catalog.groups[key])}</span>
                  <span className="text-caption font-normal tabular-nums text-muted-foreground">{counts[key]}</span>
                </Button>
              ))}
            </div>
            <ul className="grid gap-4 @xl:grid-cols-2 @4xl:grid-cols-3">
              {visibleTemplates.map((template) => {
                const Icon = PRESENTATION[template.key]?.icon ?? Zap;
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
                      onClick={() => {
                        // The configure step shares this pathname. Desktop's
                        // plain-scroll capture is replaced when that step leaves,
                        // so keep the gallery position in the view-state channel.
                        writeViewState("autopilot-template-group", group);
                        writeViewState("autopilot-template-scroll", String(scrollElement.current?.scrollTop ?? 0));
                        onPick(template.key);
                      }}
                      className="flex w-full min-w-0 flex-col rounded-lg border border-surface-border bg-surface p-5 text-left transition-colors hover:border-muted-foreground hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 motion-reduce:transition-none"
                    >
                      <span className="flex items-start gap-2.5 text-title-sm font-semibold">
                        <Icon className="mt-1 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                        <span className="min-w-0 break-words">{template.title}</span>
                      </span>
                      <span className="mt-3 line-clamp-2 text-body text-muted-foreground">{template.description}</span>
                      <span className="mt-auto flex w-full flex-col gap-3 pt-4">
                        {template.key === "workday-repo-audit" && (
                          <span className="text-caption text-muted-foreground">{t(($) => $.catalog.recommended)}</span>
                        )}
                        <span className="flex items-start gap-1.5 text-caption text-muted-foreground">
                          <Clock className="mt-0.5 size-3 shrink-0" aria-hidden="true" />
                          <span className="min-w-0 break-words">{describe(parseCron(template.cron_expression, browserTimezone())) ?? template.cron_expression}</span>
                        </span>
                        <span className="flex flex-wrap items-center justify-between gap-2 text-caption">
                          {output && <span className="text-muted-foreground">{output}</span>}
                          <span className="ml-auto flex items-center gap-1 font-medium">
                            {t(($) => $.catalog.view_template)}
                            <ArrowRight className="size-3.5" aria-hidden="true" />
                          </span>
                        </span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
            <p className="mt-6 text-caption text-muted-foreground">{t(($) => $.catalog.hint)}</p>
          </>
        )}
      </div>
    </section>
  );
}
