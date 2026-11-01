import { useEffect, useId, useRef, useState } from "react";
import { prepareKnowledgeDecision, useCollaborationKnowledgeCandidate, type CollaborationIssue, type CollaborationKnowledgeCandidate,
  type CollaborationScope, type KnowledgeIntent } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useLocale, useT } from "../i18n";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { KnowledgeSourceLabel, KnowledgeState } from "./knowledge-controls";
import type { KnowledgeProposalMode } from "./knowledge-proposal";

export function KnowledgeReview({ scope, candidateId, issue, canPropose, locked, onSubmit, onProposal, onSelect, onClose }: {
  scope: CollaborationScope; candidateId: string; issue?: CollaborationIssue; canPropose: boolean; locked: boolean;
  onSubmit: (intent: KnowledgeIntent) => Promise<void>; onProposal: (mode: KnowledgeProposalMode) => void;
  onSelect: (id: string) => void; onClose: () => void;
}) {
  const { t } = useT("collaboration"); const locale = useLocale();
  const root = useRef<HTMLElement>(null);
  useEffect(() => { root.current?.focus(); }, [candidateId]);
  const query = useCollaborationKnowledgeCandidate(scope, candidateId);
  const candidate = query.data?.value.candidate;
  const actorLabel = (userId: string) => issue?.creator.user_id === userId && issue.creator.name.trim() ? issue.creator.name
    : issue?.handler.user_id === userId && issue.handler.name.trim() ? issue.handler.name : userId;
  const decisionRepeatsApproval = candidate?.availability !== "restricted" && candidate?.decision !== null && candidate?.decision !== undefined &&
    candidate.decision.actor_user_id === candidate.approved_by_user_id && candidate.decision.at === candidate.approved_at;
  const identity = candidate ? JSON.stringify([candidate.candidate_id, candidate.state_revision, candidate.availability,
    candidate.availability === "available" ? candidate.content_sha256 : null, candidate.availability !== "restricted" ? candidate.version : null]) : null;
  // Human input survives lease erasure and an unchanged refetch; a different reviewed tuple starts a fresh decision.
  const [decisionKey, setDecisionKey] = useState<string | null>(identity);
  if (identity !== null && identity !== decisionKey) setDecisionKey(identity);
  return <section ref={root} tabIndex={-1} aria-label={t(($) => $.knowledge.review)} className="min-w-0 space-y-4 border-t border-border pt-5 outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <div className="flex flex-wrap items-center justify-between gap-3"><h4 className="text-body font-semibold">{t(($) => $.knowledge.review)}</h4><Button type="button" variant="ghost" size="sm" disabled={locked} onClick={onClose}>{t(($) => $.knowledge.close)}</Button></div>
    {!candidate && <div role={query.error ? "alert" : "status"} className="space-y-2 text-body"><p>{query.error ? t(($) => $.knowledge.loadError) : t(($) => $.knowledge.loading)}</p>
      {query.error && <Button type="button" variant="outline" onClick={() => void query.refetch()}>{t(($) => $.knowledge.refresh)}</Button>}</div>}
    {candidate && <>
      <KnowledgeState candidate={candidate} />
      {candidate.availability === "restricted" ? <p className="text-body">{t(($) => $.knowledge.restricted)}</p> : <>
        <dl className="flex flex-wrap gap-x-6 gap-y-3 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.knowledge.versionLabel)}</dt><dd>{candidate.version}</dd></div>
          <div><dt>{t(($) => $.knowledge.target)}</dt><dd>{t(($) => $.knowledge[candidate.target.kind])} · {candidate.target.kind === "project"
            ? issue?.source_structure?.project?.id === candidate.target.project_id ? issue.source_structure.project.name : candidate.target.project_id
            : issue?.source_workspace.id === candidate.target.workspace_id ? issue.source_workspace.name : candidate.target.workspace_id}</dd></div>
          <div><dt>{t(($) => $.knowledge.proposedBy)}</dt><dd>{actorLabel(candidate.proposed_by_user_id)}</dd></div>
          <div><dt>{t(($) => $.knowledge.proposedAt)}</dt><dd><time dateTime={candidate.proposed_at}>{new Date(candidate.proposed_at).toLocaleString(locale)}</time></dd></div>
          {candidate.approved_by_user_id && <div><dt>{t(($) => $.knowledge.approvedBy)}</dt><dd>{actorLabel(candidate.approved_by_user_id)}</dd></div>}
          {candidate.approved_at && <div><dt>{t(($) => $.knowledge.approvedAt)}</dt><dd><time dateTime={candidate.approved_at}>{new Date(candidate.approved_at).toLocaleString(locale)}</time></dd></div>}
          {candidate.decision && !decisionRepeatsApproval && <>
            <div><dt>{t(($) => $.knowledge.decisionBy)}</dt><dd>{actorLabel(candidate.decision.actor_user_id)}</dd></div>
            <div><dt>{t(($) => $.knowledge.decisionAt)}</dt><dd><time dateTime={candidate.decision.at}>{new Date(candidate.decision.at).toLocaleString(locale)}</time></dd></div>
          </>}
          <div><dt>{t(($) => $.knowledge.expires)}</dt><dd>{candidate.expires_at ? <time dateTime={candidate.expires_at}>{new Date(candidate.expires_at).toLocaleString(locale)}</time> : t(($) => $.knowledge.noExpiry)}</dd></div>
        </dl>
        {candidate.availability === "available" && candidate.decision?.reason && <dl className="space-y-1 text-caption">
          <dt className="text-muted-foreground">{t(($) => $.knowledge.reason)}</dt><dd className="whitespace-pre-wrap text-body [overflow-wrap:anywhere]">{candidate.decision.reason}</dd>
        </dl>}
        {!issue && <AppLink className="inline-block text-caption underline underline-offset-4" href={accountPath({ view: "participating", issueId: candidate.source_issue_id })}>{t(($) => $.knowledge.openSource)}</AppLink>}
      </>}
      {candidate.availability === "source_unavailable" && <p className="text-body text-muted-foreground">{t(($) => $.knowledge.sourceUnavailable)}</p>}
      {candidate.availability === "available" && <>
        <p className="text-body font-medium [overflow-wrap:anywhere]">{candidate.summary}</p>
        <RichContent resourcePolicy="restricted" content={candidate.content} />
        <div className="space-y-2"><h5 className="text-caption font-medium">{t(($) => $.knowledge.sources)}</h5><ul className="space-y-2 text-caption">{candidate.sources.map((source, index) => <li key={index}>
          <AppLink className="inline-block underline underline-offset-4 pointer-coarse:min-h-11 pointer-coarse:py-3" href={accountPath({ view: "participating", issueId: source.issue_id,
            ...(source.kind === "memory" ? { memoryVersion: source.version } : source.kind === "comment" ? { commentId: source.comment_id } : { runId: source.run_id }) })}><KnowledgeSourceLabel source={source} /></AppLink>
        </li>)}</ul></div>
        <div className="flex flex-wrap gap-2 text-caption text-muted-foreground">{candidate.applicability.surfaces.map((surface) => <span key={surface}>{t(($) => $.knowledge.surfaces[surface])}</span>)}</div>
        {candidate.evidence.length > 0 && <div className="space-y-2"><h5 className="text-caption font-medium">{t(($) => $.knowledge.verification)}</h5><ul className="space-y-2 text-body">{candidate.evidence.map((evidence, index) => <li key={index} className="[overflow-wrap:anywhere]">
          <span className="text-caption text-muted-foreground">{t(($) => $.knowledge.evidenceKinds[evidence.kind])} · {t(($) => $.knowledge[evidence.verification])}</span> · {evidence.kind === "run_record" ? evidence.record_id : evidence.reference}
        </li>)}</ul></div>}
        <details className="text-caption"><summary className="cursor-pointer text-muted-foreground">{t(($) => $.knowledge.digest)}</summary><code className="mt-2 block break-all">{candidate.content_sha256}</code></details>
        {canPropose && issue?.id === candidate.source_issue_id && <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" disabled={locked} onClick={() => onProposal({ target: candidate.target, supersedesId: candidate.candidate_id })}>{t(($) => $.knowledge.correct)}</Button>
          {candidate.state === "approved" && candidate.target.kind === "project" && <Button type="button" variant="outline" disabled={locked} onClick={() => onProposal({ target: { kind: "workspace", workspace_id: candidate.target.workspace_id }, derivedId: candidate.candidate_id })}>{t(($) => $.knowledge.derive)}</Button>}
        </div>}
      </>}
    </>}
    {decisionKey !== null && <KnowledgeDecision key={decisionKey} scope={scope} candidate={identity === decisionKey ? candidate : undefined} locked={locked} onSubmit={onSubmit} />}
    {candidate && candidate.availability !== "restricted" && <details className="space-y-3"><summary className="cursor-pointer text-body font-medium">{t(($) => $.knowledge.history)}</summary>
      <ul className="space-y-2">{query.data?.value.history.map((version) => <li key={version.candidate_id}><Button type="button" variant="ghost" size="sm" onClick={() => onSelect(version.candidate_id)}>{version.availability === "restricted" ? t(($) => $.knowledge.restricted) : t(($) => $.knowledge.version, { version: version.version })} · {t(($) => $.knowledge.states[version.state])}</Button></li>)}</ul>
      {query.data?.value.history_truncated === true && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.historyTruncated)}</p>}
    </details>}
  </section>;
}

function KnowledgeDecision({ scope, candidate, locked, onSubmit }: { scope: CollaborationScope; candidate: CollaborationKnowledgeCandidate | undefined; locked: boolean; onSubmit: (intent: KnowledgeIntent) => Promise<void> }) {
  const { t } = useT("collaboration"); const id = useId(); const [reviewed, setReviewed] = useState(false), [reason, setReason] = useState("");
  const [error, setError] = useState(false);
  // Erased or loading: keep the typed state, but render no control derived from an absent projection.
  if (!candidate) return null;
  const canApprove = candidate.availability === "available" && candidate.state === "pending" && candidate.capabilities.can_approve === true;
  const canReject = candidate.availability !== "restricted" && candidate.state === "pending" && candidate.capabilities.can_reject === true;
  const canWithdraw = ["pending", "approved"].includes(candidate.state) && candidate.capabilities.can_withdraw === true;
  if (!canApprove && !canReject && !canWithdraw) return null;
  const submit = (action: "approve" | "reject" | "withdraw") => {
    if (locked) return;
    try { const intent = prepareKnowledgeDecision(scope, candidate, action, reason); setError(false); void onSubmit(intent); }
    catch { setError(true); }
  };
  return <div className="space-y-3">
    {canApprove && <><p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.knowledge.approvalHint)}</p>
      <label className="flex cursor-pointer items-start gap-3 text-body pointer-coarse:min-h-11"><Checkbox checked={reviewed} onCheckedChange={setReviewed} disabled={locked} />{t(($) => $.knowledge.reviewed)}</label>
      <Button type="button" disabled={!reviewed || locked} onClick={() => submit("approve")}>{t(($) => $.knowledge.approve)}</Button></>}
    {(canReject || canWithdraw) && <><label htmlFor={id} className="block text-body font-medium">{t(($) => $.knowledge.reason)}</label>
      <Textarea id={id} value={reason} onChange={(event) => setReason(event.target.value)} disabled={locked} rows={2} maxLength={1000} />
      <div className="flex flex-wrap gap-2">{canReject && <Button type="button" variant="outline" disabled={locked || !reason.trim()} onClick={() => submit("reject")}>{t(($) => $.knowledge.reject)}</Button>}
        {canWithdraw && <Button type="button" variant="outline" disabled={locked || !reason.trim()} onClick={() => submit("withdraw")}>{t(($) => $.knowledge.withdraw)}</Button>}</div></>}
    {error && <p role="alert" className="text-body text-destructive">{t(($) => $.knowledge.invalid)}</p>}
  </div>;
}
