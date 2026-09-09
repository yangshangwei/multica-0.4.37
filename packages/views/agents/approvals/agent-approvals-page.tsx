"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ShieldCheck } from "lucide-react";
import { agentApprovalListOptions } from "@multica/core/agent-approvals";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { AgentApproval, ApprovalStatus } from "@multica/core/types";
import { isApprovalPending } from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import {
  CollectionPageHeader,
  CollectionPageState,
} from "../../layout/collection-page";
import { AgentApprovalCard } from "./agent-approval-card";

/**
 * The workspace's human approval queue.
 *
 * This page is the missing half of the approval boundary: the agent side has a
 * CLI (`multica approval request`), and until this existed a request nobody could
 * reach was the same as no boundary at all. Deliberately human-only — the server
 * refuses a machine credential on the decision endpoint, and the CLI has no
 * `approve` command precisely so that refusal is never reached by accident.
 *
 * Two filters, not five. "Waiting" is the working view; "All" is the audit trail,
 * where a decided request still shows who decided it and what the agent reported.
 * A per-status picker would be four more strings for a distinction the status
 * badge already makes.
 */
export function AgentApprovalsPage() {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const [onlyWaiting, setOnlyWaiting] = useState(true);

  const status: ApprovalStatus | undefined = onlyWaiting ? "pending" : undefined;
  const {
    data: approvals = [],
    isLoading,
    isError,
    refetch,
  } = useQuery(agentApprovalListOptions(wsId, status));

  // Agent names come from the workspace's agent list, which every dashboard route
  // already holds: the approval payload carries agent_id alone, and "Release
  // Engineer wants to deploy" is the sentence a reviewer needs.
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const agentNames = useMemo(() => {
    const byId = new Map<string, string>();
    for (const agent of agents) byId.set(agent.id, agent.name);
    return byId;
  }, [agents]);

  const paths = useWorkspacePaths();

  const waitingCount = useMemo(
    () => approvals.filter((entry) => isApprovalPending(entry.status)).length,
    [approvals],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <CollectionPageHeader
        icon={ShieldCheck}
        title={t(($) => $.approvals.title)}
        count={waitingCount}
        description={t(($) => $.approvals.tagline)}
        learnMore={{
          href: paths.docsPage(DOCS_SLUGS.agents),
          label: t(($) => $.approvals.learn_more),
        }}
        actions={
          <QueueFilter value={onlyWaiting} onChange={setOnlyWaiting} />
        }
      />

      {isLoading ? (
        <QueueSkeleton />
      ) : isError ? (
        <CollectionPageState
          role="alert"
          tone="destructive"
          icon={AlertCircle}
          title={t(($) => $.approvals.load_failed)}
          actions={
            <Button size="sm" variant="outline" onClick={() => refetch()}>
              {t(($) => $.approvals.retry)}
            </Button>
          }
        />
      ) : approvals.length === 0 ? (
        <CollectionPageState
          icon={ShieldCheck}
          title={
            onlyWaiting
              ? t(($) => $.approvals.empty_pending)
              : t(($) => $.approvals.empty_all)
          }
          description={
            onlyWaiting ? t(($) => $.approvals.empty_pending_hint) : undefined
          }
        />
      ) : (
        <QueueList approvals={approvals} agentNames={agentNames} />
      )}
    </div>
  );
}

/**
 * Waiting / All.
 *
 * The selected side is expressed with weight and text colour, which hover does not
 * touch, so hovering the active filter cannot visually demote it to a plain hover
 * state.
 */
function QueueFilter({
  value,
  onChange,
}: {
  value: boolean;
  onChange: (onlyWaiting: boolean) => void;
}) {
  const { t } = useT("agents");
  return (
    <div
      role="group"
      className="flex items-center gap-0.5 rounded-md border border-border p-0.5"
    >
      {[true, false].map((waiting) => (
        <Button
          key={String(waiting)}
          type="button"
          size="sm"
          variant={value === waiting ? "secondary" : "ghost"}
          aria-pressed={value === waiting}
          className={
            value === waiting
              ? "h-7 px-2.5 text-caption font-medium text-foreground"
              : "h-7 px-2.5 text-caption text-muted-foreground"
          }
          onClick={() => onChange(waiting)}
        >
          {waiting
            ? t(($) => $.approvals.filter_pending)
            : t(($) => $.approvals.filter_all)}
        </Button>
      ))}
    </div>
  );
}

function QueueList({
  approvals,
  agentNames,
}: {
  approvals: AgentApproval[];
  agentNames: Map<string, string>;
}) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {approvals.map((approval) => (
        <AgentApprovalCard
          key={approval.id}
          approval={approval}
          agentName={agentNames.get(approval.agent_id) ?? null}
        />
      ))}
    </div>
  );
}

function QueueSkeleton() {
  return (
    <div className="space-y-4 p-4">
      {[0, 1, 2].map((row) => (
        <div key={row} className="space-y-2">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-4 w-2/3" />
        </div>
      ))}
    </div>
  );
}
