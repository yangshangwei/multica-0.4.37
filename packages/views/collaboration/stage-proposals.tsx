import { useId, useState } from "react";
import {
  collaborationStageProposalActions, useCollaborationStageProposal, useCollaborationStageProposals,
  type CollaborationIssue, type CollaborationScope, type CollaborationStageCandidate, type CollaborationStageCandidateRef, type CollaborationStageProposalContent,
  type CollaborationStageProposalDetail, type CollaborationStageProposalListItem, type CollaborationStageState,
} from "@multica/core/collaboration";
import { accountPath, paths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "../i18n";
import { AppLink } from "../navigation";
import { CollaborationQueryState } from "./query-state";
import { useCollaborationStageLabels } from "./stage-labels";

const actionClass = "h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11";
const linkClass = "inline-flex min-h-8 items-center text-caption text-muted-foreground underline underline-offset-4 pointer-coarse:min-h-11";

export type StageProposalIntent =
  | { kind: "revise" | "modify"; proposalId: string; base: CollaborationStageCandidate; stale: boolean }
  | { kind: "decide"; proposalId: string; candidate: CollaborationStageCandidateRef; decision: "accept" | "reject" };

/** The exact content of one immutable version. Proposal text is untrusted data: it is rendered as plain text, never as markup. */
export function StageProposalContent({ content }: { content: CollaborationStageProposalContent }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  return <div className="min-w-0 space-y-3">
    <p className="max-w-[75ch] whitespace-pre-wrap text-body [overflow-wrap:anywhere]">{content.summary.trim() ? content.summary : t(($) => $.stage.proposals.noSummary)}</p>
    <div className="min-w-0 space-y-2"><h5 className="text-caption font-medium">{t(($) => $.stage.proposals.children)}</h5>
      <ol className="min-w-0 list-decimal space-y-2 pl-5 text-body">{content.children.map((child, index) => <li key={index} className="min-w-0 [overflow-wrap:anywhere]">
        <span className="font-medium">{child.title}</span>
        <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.stage.proposals.priority, { priority: labels.priority(child.priority) })}</span>
        {child.description.trim() && <p className="mt-1 max-w-[75ch] whitespace-pre-wrap text-caption text-muted-foreground">{child.description}</p>}
      </li>)}</ol>
    </div>
  </div>;
}

export function StageProposals({ scope, issue, state, accountName, locked, onCreate, onIntent }: {
  scope: CollaborationScope; issue: CollaborationIssue; state: CollaborationStageState; accountName: (userId: string) => string; locked: boolean;
  onCreate: (trigger: HTMLElement) => void; onIntent: (intent: StageProposalIntent, trigger: HTMLElement) => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const list = useCollaborationStageProposals(scope, issue.id, state.availability === "available");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const first = list.data?.pages[0];
  const items = list.data?.pages.flatMap((page) => page.state === "available" ? page.value.items : []) ?? [];
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-3">
    <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 space-y-1"><h4 id={`${id}-title`} className="text-body font-semibold">{t(($) => $.stage.proposals.title)}</h4>
        <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.proposals.intro)}</p></div>
      {state.can.createProposal && <Button type="button" variant="outline" className={actionClass} disabled={locked} onClick={(event) => onCreate(event.currentTarget)}>{t(($) => $.stage.proposals.create)}</Button>}
    </div>
    {list.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {list.error && <CollaborationQueryState error={list.error} retry={() => void list.refetch()} />}
    {first && first.state !== "available" && <p role="status" className="text-body text-muted-foreground">{t(($) => $.stage.proposals.unavailable)}</p>}
    {first?.state === "available" && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.stage.proposals.empty)}</p>}
    <ul className="min-w-0 space-y-2">{items.map((item) => <li key={item.proposal_id} className="min-w-0">
      <StageProposalRow item={item} selected={selectedId === item.proposal_id} onToggle={() => setSelectedId(selectedId === item.proposal_id ? null : item.proposal_id)} stateLabel={labels.proposalState(item.state)} />
      {selectedId === item.proposal_id && <StageProposalDetail scope={scope} issue={issue} state={state} proposalId={item.proposal_id} accountName={accountName} locked={locked} onIntent={onIntent} />}
    </li>)}</ul>
    {list.hasNextPage && <Button type="button" variant="outline" className={actionClass} disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>{t(($) => $.stage.proposals.more)}</Button>}
  </section>;
}

function StageProposalRow({ item, selected, onToggle, stateLabel }: { item: CollaborationStageProposalListItem; selected: boolean; onToggle: () => void; stateLabel: string }) {
  const { t } = useT("collaboration");
  const summary = item.current.summary.trim() || t(($) => $.stage.proposals.noSummary);
  // Selection is carried by weight and aria-pressed, so hovering a selected row does not downgrade it.
  return <button type="button" aria-pressed={selected} aria-label={t(($) => $.stage.proposals.open, { summary: summary.slice(0, 80) })} onClick={onToggle}
    className={`flex w-full min-w-0 flex-wrap items-start justify-between gap-2 rounded-md px-2 py-2 text-left text-body hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11 ${selected ? "bg-accent font-medium" : ""}`}>
    <span className="min-w-0 flex-1 [overflow-wrap:anywhere]"><span className="line-clamp-2">{summary}</span>
      <span className="mt-1 block text-caption font-normal text-muted-foreground">{t(($) => $.stage.proposals.version, { version: item.current.version })} · {t(($) => $.stage.proposals.childCount, { count: item.current.child_count })}
        {item.current.stale === true && <> · {t(($) => $.stage.proposals.stale)}</>}</span></span>
    <span className="shrink-0 text-caption font-normal text-muted-foreground">{stateLabel}</span>
  </button>;
}

function StageProposalDetail({ scope, issue, state, proposalId, accountName, locked, onIntent }: {
  scope: CollaborationScope; issue: CollaborationIssue; state: CollaborationStageState; proposalId: string; accountName: (userId: string) => string; locked: boolean;
  onIntent: (intent: StageProposalIntent, trigger: HTMLElement) => void;
}) {
  const { t } = useT("collaboration");
  const query = useCollaborationStageProposal(scope, issue.id, proposalId);
  if (query.error) return <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />;
  if (!query.data) return <p role="status" className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>;
  if (query.data.state !== "available") return <p role="status" className="px-2 py-2 text-body text-muted-foreground">{query.data.state === "not_found" ? t(($) => $.stage.proposals.notFound) : t(($) => $.stage.proposals.unavailable)}</p>;
  return <StageProposalRecord issue={issue} state={state} detail={query.data.value} accountName={accountName} locked={locked} onIntent={onIntent} />;
}

function StageProposalRecord({ issue, state, detail, accountName, locked, onIntent }: {
  issue: CollaborationIssue; state: CollaborationStageState; detail: CollaborationStageProposalDetail; accountName: (userId: string) => string; locked: boolean;
  onIntent: (intent: StageProposalIntent, trigger: HTMLElement) => void;
}) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const labels = useCollaborationStageLabels();
  const actions = collaborationStageProposalActions(state, detail, issue);
  const newest = detail.versions[0]!;
  const slug = issue.ordinary_children?.source_workspace_slug ?? null;
  const time = (value: string) => <time dateTime={value}>{new Date(value).toLocaleString(locale)}</time>;
  return <div className="min-w-0 space-y-4 border-l border-border py-2 pl-4" data-proposal-id={detail.proposal_id}>
    {actions.stale && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.stale)}</p>}
    {/* Accept, modify and reject are separate, explicit affordances; each pins the newest version as displayed. */}
    {(actions.canRevise || actions.canDecide) && <div className="flex min-w-0 flex-wrap gap-2">
      {actions.canDecide && <Button type="button" className={actionClass} disabled={locked} onClick={(event) => onIntent({ kind: "decide", proposalId: detail.proposal_id, candidate: actions.newest, decision: "accept" }, event.currentTarget)}>{t(($) => $.stage.proposals.actions.accept)}</Button>}
      {actions.canDecide && actions.canRevise && <Button type="button" variant="outline" className={actionClass} disabled={locked} onClick={(event) => onIntent({ kind: "modify", proposalId: detail.proposal_id, base: newest, stale: actions.stale }, event.currentTarget)}>{t(($) => $.stage.proposals.actions.modifyAccept)}</Button>}
      {actions.canDecide && <Button type="button" variant="outline" className={actionClass} disabled={locked} onClick={(event) => onIntent({ kind: "decide", proposalId: detail.proposal_id, candidate: actions.newest, decision: "reject" }, event.currentTarget)}>{t(($) => $.stage.proposals.actions.reject)}</Button>}
      {actions.canRevise && <Button type="button" variant="ghost" className={actionClass} disabled={locked} onClick={(event) => onIntent({ kind: "revise", proposalId: detail.proposal_id, base: newest, stale: actions.stale }, event.currentTarget)}>{t(($) => $.stage.proposals.actions.revise)}</Button>}
    </div>}
    {detail.decisions.length > 0 && <div className="min-w-0 space-y-2"><h5 className="text-caption font-medium">{t(($) => $.stage.proposals.decisions.title)}</h5>
      <ul className="min-w-0 space-y-3">{detail.decisions.map((decision) => <li key={decision.decision_id} className="min-w-0 space-y-2 rounded-md bg-muted/30 p-3">
        <p className="text-body font-medium">{decision.decision === "accepted" ? decision.accepted_with_modification === true ? t(($) => $.stage.proposals.decisions.acceptedModified, { version: decision.version })
          : t(($) => $.stage.proposals.decisions.accepted, { version: decision.version }) : decision.decision === "rejected" ? t(($) => $.stage.proposals.decisions.rejected, { version: decision.version })
          : t(($) => $.stage.proposals.decisions.unknown, { version: decision.version })}</p>
        <dl className="flex min-w-0 flex-wrap gap-x-6 gap-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.stage.proposals.decisions.by)}</dt><dd>{accountName(decision.actor_user_id)}</dd></div>
          <div><dt>{t(($) => $.stage.proposals.decisions.at)}</dt><dd>{time(decision.created_at)}</dd></div>
        </dl>
        {decision.reason.trim() && <dl className="space-y-1 text-caption"><dt className="text-muted-foreground">{t(($) => $.stage.proposals.decisions.reason)}</dt>
          <dd className="max-w-[75ch] whitespace-pre-wrap text-body [overflow-wrap:anywhere]">{decision.reason}</dd></dl>}
      </li>)}</ul>
    </div>}
    {detail.children.length > 0 && <div className="min-w-0 space-y-2"><h5 className="text-caption font-medium">{t(($) => $.stage.proposals.createdChildren)}</h5>
      <ol className="min-w-0 space-y-1 text-body">{detail.children.map((child) => <li key={child.issue_id} className="min-w-0 [overflow-wrap:anywhere]">
        {child.can_open === true && slug ? <AppLink href={paths.workspace(slug).issueDetail(child.issue_id)} className="inline-flex min-h-8 min-w-0 flex-wrap items-baseline gap-x-2 rounded-md underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
          <span className="text-caption text-muted-foreground">{child.identifier}</span><span>{child.title}</span></AppLink>
          : <span className="inline-flex min-w-0 flex-wrap items-baseline gap-x-2"><span className="text-caption text-muted-foreground">{child.identifier}</span><span>{child.title}</span></span>}
        {child.attached !== true && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.stage.scope.detached)}</span>}
      </li>)}</ol>
    </div>}
    <div className="min-w-0 space-y-3"><h5 className="text-caption font-medium">{t(($) => $.stage.proposals.versions)}</h5>
      <ol className="min-w-0 space-y-4">{detail.versions.map((version) => <li key={version.candidate_id} className="min-w-0 space-y-2" data-candidate-id={version.candidate_id}>
        <p className="text-body font-medium">{t(($) => $.stage.proposals.version, { version: version.version })}<span className="ml-2 text-caption font-normal text-muted-foreground">{labels.proposalState(version.state)}</span></p>
        <dl className="flex min-w-0 flex-wrap gap-x-6 gap-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.stage.proposals.submittedBy)}</dt><dd>{accountName(version.submitted_by)}</dd></div>
          <div><dt>{t(($) => $.stage.proposals.submittedAt)}</dt><dd>{time(version.created_at)}</dd></div>
          <div><dt>{t(($) => $.stage.proposals.source)}</dt><dd>{version.source.kind === "manual" ? t(($) => $.stage.proposals.sources.manual)
            : version.source.kind === "comment" ? version.source.author_type === "agent" ? t(($) => $.stage.proposals.sources.commentAgent) : t(($) => $.stage.proposals.sources.comment)
            : version.source.kind === "run" ? t(($) => $.stage.proposals.sources.run) : t(($) => $.stage.proposals.sources.unknown)}</dd></div>
        </dl>
        {version.source.kind === "comment" && <AppLink href={accountPath({ view: "participating", issueId: issue.id, commentId: version.source.comment_id })} className={linkClass}>{t(($) => $.stage.proposals.openSourceComment)}</AppLink>}
        {version.source.kind === "run" && <AppLink href={accountPath({ view: "participating", issueId: issue.id, runId: version.source.run_id })} className={linkClass}>{t(($) => $.stage.proposals.openSourceRun)}</AppLink>}
        <StageProposalContent content={version.content} />
      </li>)}</ol>
      {detail.versions_truncated === true && <p className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.versionsTruncated)}</p>}
    </div>
  </div>;
}
