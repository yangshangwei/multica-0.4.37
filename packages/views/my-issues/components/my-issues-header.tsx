"use client";

import { useEffect, useState } from "react";
import type {
  Issue,
  IssueTableFacetSpec,
  IssueTableFacetsResponse,
  WorkingAgentSummary,
} from "@multica/core/types";
import { type MyIssuesScope } from "@multica/core/issues/stores/my-issues-view-store";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { useT } from "../../i18n";
import { WorkspaceAgentWorkingChip } from "../../issues/components/workspace-agent-working-chip";
import {
  IssueDisplayControls,
  ViewRefreshIndicator,
} from "../../issues/components/issues-header";
import { cn } from "@multica/ui/lib/utils";
import { PAGE_GUTTER } from "../../layout/page-header";
import { FilterChipsBar } from "../../issues/components/filter-chips-bar";
import { toast } from "sonner";
import { SaveViewDialog, type SaveViewScope } from "../../issues/components/save-view-dialog";
import { useActiveIssueView } from "@multica/core/issue-views/use-active-view";
import { useWorkspaceId } from "@multica/core/hooks";
import { baselineFromQuery } from "@multica/core/issue-views/baseline";
import { ViewBar } from "../../issues/components/view-bar";
import { CompactViewSelector } from "../../issues/components/compact-view-selector";
import type { IssueView } from "@multica/core/api/schemas";

/** My Issues tab → saved-view scope_variant (API vocabulary). */
const SAVE_VARIANT: Record<MyIssuesScope, Extract<SaveViewScope, { kind: "my" }>["variant"]> = {
  all: "any",
  assigned: "assigned",
  created: "created",
  agents: "involved",
};

export function MyIssuesHeader({
  allIssues,
  workingAgents,
  scope,
  onScopeChange,
  isRefreshing = false,
  facetCountsExact = true,
  tableFacetCounts,
  onTableFacetChange,
}: {
  allIssues: Issue[];
  /** See IssueSurfaceController.workingAgents. My Issues used to ask the
   *  working-agents endpoint for its own relation-scoped count; the surface
   *  projection now covers the relation AND every active filter. */
  workingAgents: WorkingAgentSummary[] | undefined;
  scope: MyIssuesScope;
  onScopeChange: (scope: MyIssuesScope) => void;
  isRefreshing?: boolean;
  /** See IssueDisplayControls.facetCountsExact. */
  facetCountsExact?: boolean;
  tableFacetCounts?: IssueTableFacetsResponse;
  onTableFacetChange: (facet: IssueTableFacetSpec | null) => void;
}) {
  const { t } = useT("my-issues");
  const { t: tIssues } = useT("issues");
  const [saveViewOpen, setSaveViewOpen] = useState(false);
  const saveScope: SaveViewScope = { kind: "my", variant: SAVE_VARIANT[scope] };
  const wsId = useWorkspaceId();
  const { activeView, views, viewsReady, setActive, missing } = useActiveIssueView(
    wsId,
    { scope_type: "my" },
  );
  useEffect(() => {
    if (missing) {
      setActive(null);
      toast.info(tIssues(($) => $.view_selector.unavailable));
    }
  }, [missing, setActive, tIssues]);
  const viewBaseline = activeView ? baselineFromQuery(activeView.query) : undefined;
  const [editTarget, setEditTarget] = useState<{
    view: IssueView;
    fromDefinition: boolean;
  } | null>(null);
  const SCOPES: { value: MyIssuesScope; label: string; description: string }[] = [
    { value: "all", label: t(($) => $.header.scope.all_label), description: t(($) => $.header.scope.all_description) },
    { value: "assigned", label: t(($) => $.header.scope.assigned_label), description: t(($) => $.header.scope.assigned_description) },
    { value: "created", label: t(($) => $.header.scope.created_label), description: t(($) => $.header.scope.created_description) },
    { value: "agents", label: t(($) => $.header.scope.agents_label), description: t(($) => $.header.scope.agents_description) },
  ];
  const agentRunningFilter = useViewStore((s) => s.agentRunningFilter);
  const toggleAgentRunningFilter = useViewStore(
    (s) => s.toggleAgentRunningFilter,
  );
  const builtins = SCOPES.map((s) => ({
  key: s.value,
  label: s.label,
  description: s.description,
  active: !activeView && scope === s.value,
  onSelect: () => {
    if (activeView) setActive(null);
    onScopeChange(s.value);
  },
}));

  return (
    <>
    <div className={cn("min-h-12 shrink-0 py-2 [-webkit-overflow-scrolling:touch]", PAGE_GUTTER)}>
      <div className="flex w-full min-w-0 flex-wrap items-start justify-between gap-2">
        <div className="hidden min-w-0 flex-1 md:block">
          <ViewBar
            wsId={wsId}
            scope={{ scope_type: "my" }}
            builtins={builtins}
            views={views}
            viewsReady={viewsReady}
            activeView={activeView}
            onSelectView={(view) => setActive(view ? view.id : null)}
            onNewView={() => {
              setEditTarget(null);
              setSaveViewOpen(true);
            }}
            onEditView={(view) => {
              setEditTarget({ view, fromDefinition: true });
              setSaveViewOpen(true);
            }}
          />
        </div>

        <CompactViewSelector
          builtins={builtins}
          views={views}
          activeView={activeView}
          onSelectView={(view) => setActive(view.id)}
        />

        <div className="flex max-w-full flex-wrap items-center justify-end gap-1">
          {agentRunningFilter && (
            <span className="mr-1 hidden text-caption text-muted-foreground md:inline">
              {tIssues(($) => $.agent_activity.filter_active_label)}
            </span>
          )}
          <WorkspaceAgentWorkingChip
            value={agentRunningFilter}
            onToggle={toggleAgentRunningFilter}
            agents={workingAgents}
          />
          <IssueDisplayControls
            scopedIssues={allIssues}
            facetCountsExact={facetCountsExact}
            tableFacetCounts={tableFacetCounts}
            onTableFacetChange={onTableFacetChange}
            viewBaseline={viewBaseline}
          />
          <ViewRefreshIndicator active={isRefreshing} />
        </div>
      </div>
    </div>
    <FilterChipsBar
      viewBaseline={viewBaseline}
      saveLabel={activeView ? tIssues(($) => $.filters.chip_edit) : undefined}
      onSave={() => {
        setEditTarget(
          activeView ? { view: activeView, fromDefinition: false } : null,
        );
        setSaveViewOpen(true);
      }}
    />
    <SaveViewDialog
      open={saveViewOpen}
      onOpenChange={setSaveViewOpen}
      scope={saveScope}
      editView={editTarget?.view ?? null}
      seedFromDefinition={editTarget?.fromDefinition ?? false}
    />
    </>
  );
}
