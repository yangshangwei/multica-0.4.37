import { useEffect, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  collaborationIssueAccessDenied, collaborationRunRecordsOptions, isCollaborationDenied,
  type CollaborationExecutionGroup, type CollaborationExecutionRun, type CollaborationIssue, type CollaborationRunRecordItem,
  type CollaborationRunRecords, type CollaborationScope,
} from "@multica/core/collaboration";
import { accountPath, parseAccountPath, paths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink, currentPath, useNavigation } from "../navigation";
import { RichContent } from "../rich-content";
import { useT } from "../i18n";
import { CollaborationCommentAttachments } from "./attachment-panel";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationPreparationSummary } from "./preparation-summary";

export function CollaborationRunDisclosure({ scope, issue, run, group, selectedRunId }: {
  scope: CollaborationScope; issue: CollaborationIssue; run: CollaborationExecutionRun;
  group: CollaborationExecutionGroup; selectedRunId?: string | null;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const navigation = useNavigation();
  const issueId = issue.id;
  const [open, setOpen] = useState(selectedRunId === run.id);
  useEffect(() => { if (selectedRunId === run.id) setOpen(true); }, [selectedRunId, run.id]);
  function toggle() {
    setOpen(!open);
    if (!open || selectedRunId !== run.id) return;
    const account = parseAccountPath(currentPath(navigation));
    if (account?.issueId === issueId && account.runId === run.id) {
      navigation.replace(accountPath({ ...account, runId: null }));
      return;
    }
    const slug = issue.ordinary_children?.source_workspace_slug;
    if (slug && [paths.workspace(slug).issueDetail(issueId), paths.workspace(slug).issueDetail(issue.source_workspace.identifier)].includes(navigation.pathname)
      && parseAccountPath(`/account?issue=${issueId}${navigation.hash}`)?.runId === run.id) {
      navigation.replace(currentPath({ ...navigation, hash: "" }));
    }
  }
  return <li id={`run-${run.id}`} tabIndex={-1} className="min-w-0 space-y-3 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 text-caption text-muted-foreground">
      <span className="font-medium text-foreground">{t(($) => $.execution[run.step_kind])}</span>
      <span className="[overflow-wrap:anywhere]">{run.agent_id}</span>
      {run.execution_mode === "host_process_supervision_v1" && <span>{t(($) => $.host.mode)}</span>}
      <span>{labels.execution(run.progress_state)}</span>
      <span>{labels.proof(run.proof_state, run.execution_mode, run.forced_disposition)}</span>
      {run.recording_incomplete === true && <span>{run.execution_mode === "host_process_supervision_v1" && run.proof_state === "host_exit_observed" ? t(($) => $.host.incomplete) : t(($) => $.execution.incomplete)}</span>}
      <RunMemoryStatus run={run} issueId={issueId} />
      <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" aria-expanded={open} aria-controls={`records-${run.id}`}
        aria-label={t(($) => open ? $.records.closeLabel : $.records.openLabel, { id: run.id })} onClick={toggle}>
        {open ? t(($) => $.records.close) : t(($) => $.records.open)}
      </Button>
    </div>
    {run.execution_mode === "host_process_supervision_v1" && <p className="text-caption text-muted-foreground"><RunCleanup run={run} /></p>}
    {open && <RunRecords key={`${scope.apiScope}:${scope.userId}:${issueId}:${run.id}`} scope={scope} issueId={issueId} runId={run.id} group={group} />}
  </li>;
}

function RunRecords({ scope, issueId, runId, group }: { scope: CollaborationScope; issueId: string; runId: string; group: CollaborationExecutionGroup }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const query = useInfiniteQuery(collaborationRunRecordsOptions(client, scope, issueId, runId));
  const denied = collaborationIssueAccessDenied(client, scope, issueId);
  if (denied || isCollaborationDenied(query.error)) return <CollaborationQueryState denied error={query.error} />;
  const page = query.data?.pages.at(-1);
  const items = [...new Map((query.data?.pages.flatMap((value) => value.items) ?? []).map((item) => [item.id, item])).values()];
  const run = page?.run;
  return <section id={`records-${runId}`} aria-label={t(($) => $.records.title)} className="min-w-0 space-y-5 border-l-2 border-border pl-3 sm:pl-4">
    {query.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.records.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())} />}
    {page && run && <>
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-body sm:grid-cols-2 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.records.groupStatus)}</dt><dd>{labels.execution(group.state)}</dd></div>
        <div><dt>{t(($) => $.records.outcome)}</dt><dd>{run.execution_mode !== "host_process_supervision_v1" && run.forced_disposition !== null
          ? run.settled_at === null ? t(($) => $.execution.stopRequested) : labels.execution(run.forced_disposition)
          : run.business_outcome === "success" ? t(($) => $.execution.succeeded) : run.business_outcome === "failure" ? t(($) => $.execution.failed) : t(($) => $.records.noOutcome)}</dd></div>
        <div><dt>{t(($) => $.records.recording)}</dt><dd>{t(($) => $.records.coverage[page.recording.status])}</dd></div>
        <div><dt>{run.execution_mode === "host_process_supervision_v1" ? t(($) => $.host.termination) : t(($) => $.records.termination)}</dt><dd>{labels.proof(run.proof_state, run.execution_mode, run.forced_disposition)}</dd></div>
        {run.execution_mode === "host_process_supervision_v1" && <>
          <div><dt>{t(($) => $.host.modeLabel)}</dt><dd>{t(($) => $.host.mode)}</dd></div>
          <div><dt>{t(($) => $.host.cleanup)}</dt><dd><RunCleanup run={run} /></dd></div>
          <div><dt>{t(($) => $.host.cleanupScope)}</dt><dd>{run.descendant_cleanup ? t(($) => $.host.scopes[run.descendant_cleanup!.scope]) : t(($) => $.records.notRecorded)}</dd></div>
        </>}
      </dl>
      {run.launch_count !== undefined && <p className="text-caption text-muted-foreground">{t(($) => $.host.launches, { launches: run.launch_count })}</p>}
      {!page.recording.inputs_recorded && <p className="text-caption text-muted-foreground">{t(($) => $.records.inputsMissing)}</p>}
      <AppLink href={accountPath({ view: "participating", issueId, runId })} className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t(($) => $.records.permalink)}</AppLink>
      <RecordedResource page={page} groupId={group.id} />
      <CollaborationPreparationSummary preparation={page.preparation} />
      <RecordedUsage usage={page.usage} />
      {items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.records.empty)}</p>}
      <ol className="space-y-6">{items.map((item) => <RecordedEntry key={item.id} item={item} scope={scope} issueId={issueId} />)}</ol>
    </>}
    {query.hasNextPage && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{t(($) => $.records.more)}</Button>}
  </section>;
}

function RunMemoryStatus({ run, issueId }: { run: CollaborationExecutionRun; issueId: string }) {
  const { t } = useT("collaboration");
  const memory = run.memory;
  if (!memory) return null;
  if (memory.support === "unsupported") return <span>{t(($) => $.records.memoryUnsupported)}</span>;
  if (memory.support !== "supported") return null;
  const base = memory.base_version === null ? t(($) => $.records.notRecorded)
    : memory.base_version === "0" ? t(($) => $.records.memoryBaseEmpty)
      : <AppLink className="underline underline-offset-4" href={accountPath({ view: "participating", issueId, memoryVersion: memory.base_version })}>{t(($) => $.records.memoryBase, { version: memory.base_version })}</AppLink>;
  if (memory.written_version === null) return <span>{base} · {t(($) => $.records.memoryNotUpdated)}</span>;
  return <span>{base} · <AppLink className="underline underline-offset-4" href={accountPath({ view: "participating", issueId, memoryVersion: memory.written_version })}>{t(($) => $.records.memoryWrote, { version: memory.written_version })}</AppLink></span>;
}

function RunCleanup({ run }: { run: CollaborationExecutionRun }) {
  const { t } = useT("collaboration");
  switch (run.descendant_cleanup?.state) {
    case "tracked_scope_complete": return t(($) => $.host.cleanupTracked);
    case "best_effort": return run.proof_state === "host_exit_observed" ? t(($) => $.host.cleanupBestEffort) : t(($) => $.host.cleanupAttempted);
    case "unverified": default: return t(($) => $.host.cleanupUnverified);
  }
}

function RecordedResource({ page, groupId }: { page: CollaborationRunRecords; groupId: string }) {
  const { t } = useT("collaboration");
  const resource = page.resource;
  const references = [
    [t(($) => $.records.runId), page.run.id], [t(($) => $.records.groupId), groupId],
    [t(($) => $.execution.agent), resource.agent_id], [t(($) => $.resources.runtime), resource.runtime_id],
    ...(resource.agent_owner_user_id ? [[t(($) => $.host.agentOwner), resource.agent_owner_user_id]] : []),
    [t(($) => $.resources.owner), resource.resource_owner_user_id], [t(($) => $.resources.workspace), resource.execution_workspace_id],
    [t(($) => $.records.grant), resource.grant_id], [t(($) => $.records.provider), resource.provider], [t(($) => $.records.model), resource.model],
    [t(($) => $.records.accountSource), resource.account_source === "runtime_configuration" ? t(($) => $.records.runtimeAccount) : null],
  ];
  return <details className="min-w-0 space-y-3">
    <summary className="cursor-pointer py-2 text-body font-medium pointer-coarse:min-h-11">{t(($) => $.records.resource)}</summary>
    <dl className="grid min-w-0 gap-x-6 gap-y-3 text-caption sm:grid-cols-2 [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
      {references.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value ?? t(($) => $.records.notRecorded)}</dd></div>)}
    </dl>
  </details>;
}

function RecordedUsage({ usage }: { usage: CollaborationRunRecords["usage"] }) {
  const { t } = useT("collaboration");
  return <section aria-label={t(($) => $.records.usage)} className="min-w-0 space-y-3">
    <h5 className="text-body font-medium">{t(($) => $.records.usage)}</h5>
    {!usage ? <p className="text-caption text-muted-foreground">{t(($) => $.records.usageMissing)}</p> : <dl className="grid min-w-0 grid-cols-2 gap-3 text-caption sm:grid-cols-3 [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:tabular-nums [&_dd]:[overflow-wrap:anywhere]">
      {(["input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "total_tokens", "cost_usd"] as const).map((key) => <div key={key}>
        <dt>{t(($) => $.records.usageFields[key])}</dt>
        <dd>{usage[key] === null ? t(($) => $.records.notRecorded) : key === "cost_usd" ? t(($) => $.records.usd, { value: usage[key] }) : usage[key]}</dd>
      </div>)}
    </dl>}
  </section>;
}

function RecordedEntry({ item, scope, issueId }: { item: CollaborationRunRecordItem; scope: CollaborationScope; issueId: string }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  function title() {
    switch (item.kind) {
      case "input": return t(($) => $.records.input);
      case "message": return t(($) => $.records.roles[item.role]);
      case "tool_call": return t(($) => $.records.toolCall, { name: item.name });
      case "tool_result": return item.is_error ? t(($) => $.records.toolError) : t(($) => $.records.toolResult);
      case "comment": return t(($) => $.records.comment);
      case "squad_activity": return t(($) => $.records.squadActivity);
      case "result": return t(($) => $.records.result);
      default: return t(($) => $.page.unavailable);
    }
  }
  return <li className="min-w-0 space-y-2" data-record-id={item.id}>
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 text-caption [overflow-wrap:anywhere]">
      <h6 className="font-semibold">{title()}</h6>
      {item.kind === "result" && <span className="text-muted-foreground">{labels.execution(item.outcome === "success" ? "succeeded" : "failed")}</span>}
      {item.kind === "squad_activity" && <span className="text-muted-foreground">{t(($) => $.records.squadOutcomes[item.outcome])}</span>}
      {(item.kind === "tool_call" || item.kind === "tool_result") && <span className="text-muted-foreground">{t(($) => $.records.callId, { id: item.call_id })}</span>}
    </div>
    {item.kind === "input" && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.execution.author)}: {item.author_user_id === scope.userId ? t(($) => $.execution.you) : item.author_user_id}</p>}
    {item.kind === "input" && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.records.inputId)}: {item.command_id}</p>}
    {item.kind === "tool_call" || item.kind === "tool_result"
      ? <pre className="max-w-full whitespace-pre-wrap rounded-md bg-muted p-3 text-caption [overflow-wrap:anywhere]">{item.body}</pre>
      : <RichContent resourcePolicy="restricted" content={item.body} />}
    {item.kind === "input" && item.attachment_ids.length > 0 && <div className="space-y-2">
      <p className="text-caption text-muted-foreground">{t(($) => $.records.attachments)}</p>
      <ul className="space-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">{item.attachment_ids.map((id) => <li key={id}>{id}</li>)}</ul>
      <CollaborationCommentAttachments scope={scope} issueId={issueId} attachmentIds={item.attachment_ids} />
    </div>}
    {item.kind === "comment" && <AppLink href={accountPath({ view: "participating", issueId, commentId: item.comment_id })} className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t(($) => $.activity.openComment)}</AppLink>}
  </li>;
}
