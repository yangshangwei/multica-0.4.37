"use client";

import { useState } from "react";
import { X, Plus, Filter, CircleQuestionMark } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import { DOCS_ANCHORS, DOCS_SLUGS } from "@multica/core/docs";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import type { WebhookEventFilter } from "@multica/core/types";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";

interface WebhookEventFilterSectionProps {
  filters: WebhookEventFilter[];
  onChange: (filters: WebhookEventFilter[]) => void;
}

export function WebhookEventFilterSection({
  filters,
  onChange,
}: WebhookEventFilterSectionProps) {
  const { t } = useT("autopilots");
  const [newEvent, setNewEvent] = useState("");
  const [newActions, setNewActions] = useState("");
  // Was `#事件过滤` in Chinese and `#event-filters` in English, and neither has
  // ever existed — the heading reads "过滤事件". The anchor now comes from
  // DOCS_ANCHORS, which docs-anchor-parity.test.ts checks against the bundle.
  //
  // Nullable slug rather than useWorkspacePaths(): this section renders inside
  // the autopilot dialog, which the tests mount bare and which is not
  // guaranteed to sit under a workspace route. The link drops instead of
  // throwing, matching the slack/dingtalk/telegram tabs.
  const workspaceSlug = useWorkspaceSlug();
  const docsHref = workspaceSlug
    ? paths
        .workspace(workspaceSlug)
        .docsPage(DOCS_SLUGS.autopilots, DOCS_ANCHORS.webhookEventFilters)
    : null;

  const addFilter = () => {
    const event = newEvent.trim();
    if (!event) return;
    const actions = newActions
      .split(",")
      .map((a) => a.trim())
      .filter((a) => a.length > 0);
    const next: WebhookEventFilter = { event };
    if (actions.length > 0) next.actions = actions;
    onChange([...filters, next]);
    setNewEvent("");
    setNewActions("");
  };

  const removeFilter = (idx: number) => {
    onChange(filters.filter((_, i) => i !== idx));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-1.5 text-micro font-semibold tracking-[0.08em] text-muted-foreground uppercase">
        <Filter className="size-3" />
        {t(($) => $.dialog.event_filter_label)}
        {docsHref ? (
          <AppLink
            href={docsHref}
            aria-label={t(($) => $.dialog.event_filter_docs_link_label)}
            title={t(($) => $.dialog.event_filter_docs_link_label)}
            className="ml-0.5 inline-flex items-center text-faint-foreground hover:text-foreground transition-colors"
          >
            {/* Not an external-link glyph any more: this opens the documentation
                inside the app rather than handing the reader to a browser. */}
            <CircleQuestionMark className="size-3" />
          </AppLink>
        ) : null}
      </div>

      {filters.length > 0 && (
        <div className="space-y-1">
          {filters.map((f, idx) => (
            <div
              key={idx}
              className="flex items-center gap-2 rounded-md border bg-background px-2.5 py-1.5 text-caption"
            >
              <span className="font-mono font-medium text-foreground">
                {f.event}
              </span>
              {f.actions && f.actions.length > 0 && (
                <span className="text-muted-foreground">
                  : {f.actions.join(", ")}
                </span>
              )}
              <button
                type="button"
                onClick={() => removeFilter(idx)}
                aria-label={t(($) => $.dialog.event_filter_remove_label)}
                title={t(($) => $.dialog.event_filter_remove_label)}
                className="ml-auto rounded p-0.5 text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
              >
                <X className="size-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-2">
        <input
          type="text"
          value={newEvent}
          onChange={(e) => setNewEvent(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              addFilter();
            }
          }}
          placeholder={t(($) => $.dialog.event_filter_event_placeholder)}
          className="flex-1 min-w-0 rounded-md border bg-background px-2.5 py-1.5 text-caption font-mono outline-none focus:ring-1 focus:ring-ring"
        />
        <input
          type="text"
          value={newActions}
          onChange={(e) => setNewActions(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              addFilter();
            }
          }}
          placeholder={t(($) => $.dialog.event_filter_actions_placeholder)}
          className="w-28 rounded-md border bg-background px-2.5 py-1.5 text-caption outline-none focus:ring-1 focus:ring-ring"
        />
        <button
          type="button"
          onClick={addFilter}
          disabled={!newEvent.trim()}
          className={cn(
            "inline-flex shrink-0 items-center justify-center gap-1 rounded-md border px-2.5 py-1.5 text-caption font-medium transition-colors",
            newEvent.trim()
              ? "border-primary bg-primary text-primary-foreground hover:bg-primary/90 cursor-pointer"
              : "bg-muted text-muted-foreground cursor-not-allowed",
          )}
        >
          <Plus className="size-3.5" />
          {t(($) => $.dialog.event_filter_add)}
        </button>
      </div>
      <p className="text-micro text-muted-foreground">
        {t(($) => $.dialog.event_filter_hint)}
      </p>
    </div>
  );
}
