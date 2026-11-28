"use client";

import { ListTodo, Plus, UsersRound } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { SourceCollaborationCreation } from "../../collaboration/source-bridge";
import type {
  Issue,
  IssueTableFacetSpec,
  IssueTableFacetsResponse,
  WorkingAgentSummary,
} from "@multica/core/types";
import { useIssuesScope } from "@multica/core/issues/stores/issues-scope-store";
import { openCreateIssueWithPreference } from "@multica/core/issues/stores/create-mode-store";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { CollectionPageHeader } from "../../layout/collection-page";
import { useT } from "../../i18n";
import { IssueSurface } from "../surface/issue-surface";
import type { IssueSurfaceController } from "../surface/use-issue-surface-controller";
import { useViewBaseline } from "../surface/view-baseline-context";
import { IssueWelcome } from "./issue-welcome";
import { IssuesHeader } from "./issues-header";

function NewIssueAction() {
  const { t } = useT("layout");
  return (
    <Button type="button" className="min-h-11 gap-2 px-4" onClick={() => openCreateIssueWithPreference()}>
      <Plus className="size-4" aria-hidden="true" />
      {t(($) => $.sidebar.new_issue)}
    </Button>
  );
}

function CollaborationBetaAction({ workspaceId, compact = false }: { workspaceId: string; compact?: boolean }) {
  const { t } = useT("collaboration");
  return (
    <SourceCollaborationCreation
      workspaceId={workspaceId}
      renderTrigger={(label) => (
        <Button type="button" variant="ghost" className="min-h-11 gap-2 px-3 text-muted-foreground" aria-label={label} aria-description={t(($) => $.beta.description)} title={compact ? label : undefined}>
          <UsersRound className="size-4" aria-hidden="true" />
          <span className={compact ? "hidden md:inline" : undefined}>{label}</span>
          <span className="rounded bg-muted px-1 py-0.5 text-micro font-normal" aria-hidden="true">
            {t(($) => $.beta.label)}
          </span>
        </Button>
      )}
    />
  );
}

function IssuesEmpty({ controller, confirmedEmpty = controller.isEmptyReadSuccessful }: {
  controller: IssueSurfaceController;
  confirmedEmpty?: boolean;
}) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const baseline = useViewBaseline();

  if (controller.scopeKey === "workspace:all" && !baseline && confirmedEmpty) {
    return (
      <IssueWelcome
        docsHref={paths.docsPage(DOCS_SLUGS.issues)}
        createAction={<NewIssueAction />}
        secondaryAction={<CollaborationBetaAction workspaceId={wsId} />}
      />
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 text-muted-foreground">
      <ListTodo className="h-10 w-10 text-faint-foreground" aria-hidden="true" />
      <p className="text-body">{t(($) => $.page.empty_title)}</p>
      <p className="text-caption">{t(($) => $.page.empty_hint)}</p>
    </div>
  );
}

function IssuesSurfaceHeader({
  issues,
  workingAgents,
  isRefreshing,
  facetCountsExact,
  tableFacetCounts,
  onTableFacetChange,
}: {
  issues: Issue[];
  workingAgents: WorkingAgentSummary[] | undefined;
  isRefreshing: boolean;
  facetCountsExact: boolean;
  tableFacetCounts?: IssueTableFacetsResponse;
  onTableFacetChange: (facet: IssueTableFacetSpec | null) => void;
}) {
  const dateFilter = useViewStore((s) => s.dateFilter);
  const setDateFilter = useViewStore((s) => s.setDateFilter);

  return (
    <IssuesHeader
      scopedIssues={issues}
      workingAgents={workingAgents}
      dateFilter={dateFilter}
      onDateFilterChange={setDateFilter}
      isRefreshing={isRefreshing}
      facetCountsExact={facetCountsExact}
      tableFacetCounts={tableFacetCounts}
      onTableFacetChange={onTableFacetChange}
    />
  );
}

export function IssuesPage() {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const scope = useIssuesScope("issues");

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <CollectionPageHeader
        icon={ListTodo}
        title={t(($) => $.page.breadcrumb_title)}
        description={t(($) => $.page.tagline)}
        learnMore={{
          href: paths.docsPage(DOCS_SLUGS.issues),
          label: t(($) => $.page.learn_more),
        }}
        actions={<><CollaborationBetaAction workspaceId={wsId} compact /><NewIssueAction /></>}
      />

      <IssueSurface
        scope={{ type: "workspace", actorKind: scope }}
        modes={["board", "list", "table", "swimlane"]}
        batchToolbar="list"
        renderHeader={({ controller }) => (
          <IssuesSurfaceHeader
            issues={controller.surfaceIssues}
            workingAgents={controller.workingAgents}
            isRefreshing={controller.isRefreshing}
            facetCountsExact={controller.facetCountsExact}
            tableFacetCounts={controller.tableFacetCounts}
            onTableFacetChange={controller.setActiveTableFacet}
          />
        )}
        renderEmpty={({ controller }) => <IssuesEmpty controller={controller} />}
        renderTableEmpty={({ controller }) => controller.scopeKey === "workspace:all"
          ? <IssuesEmpty controller={controller} confirmedEmpty />
          : null}
      />
    </div>
  );
}
