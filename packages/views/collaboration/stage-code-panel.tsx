import { useEffect, useId, useState } from "react";
import { useIsMutating, useQuery, useQueryClient } from "@tanstack/react-query";
import { deriveCollaborationStageCodeState, useCollaborationStageCodeCandidate, canPrepareCollaborationTest, collaborationCodeTestSubject, collaborationStageSourceDenied,
  collaborationKeys, collaborationMentionTargetsOptions, useCollaborationDeliveryForm, useCollaborationPendingDrafts,
  type CollaborationIssue, type CollaborationScope, type CollaborationStageView } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { StageBlockers } from "./stage-blockers";
import { useCollaborationStageLabels } from "./stage-labels";
import type { StageCodeFormIntent } from "./stage-code-forms";
import { StageTestReportHistory } from "./stage-test-report";
import { StageExecutionHistory } from "./stage-execution-history";

const actionClass = "h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11";
export function useStageClock() {
  const [now, setNow] = useState(Date.now);
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, []);
  return now;
}
export function StageCodePanel({ scope, issue, view, locked, selectedId, onSelect, onOpen, accountName }: {
  scope: CollaborationScope; issue: CollaborationIssue; view: CollaborationStageView; locked: boolean; selectedId: string | null;
  accountName: (userId: string) => string; onSelect: (id: string) => void; onOpen: (form: StageCodeFormIntent, trigger: HTMLElement) => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const client = useQueryClient();
  const id = useId();
  const now = useStageClock();
  const [reportRun, setReportRun] = useState<string | null>(null);
  const candidateId = selectedId ?? view.code_candidates?.items[0]?.candidate_id ?? null;
  const query = useCollaborationStageCodeCandidate(scope, issue.id, candidateId, issue.source_workspace.id, view.code_candidates);
  const record = !query.error && query.data?.state === "available" ? query.data.value : null;
  const actions = deriveCollaborationStageCodeState(issue, view, record, scope.userId, now);
  const delivery = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "comment");
  const pending = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const mutating = useIsMutating({ mutationKey: collaborationKeys.issue(scope, issue.id) });
  const mentions = useQuery({ ...collaborationMentionTargetsOptions(client, scope, issue.id), enabled: issue.capabilities.comment === true && view.stage?.current === "qa" });
  const canPrepare = canPrepareCollaborationTest(issue, view, record, scope.userId, !mentions.error && mentions.data?.can_submit_commands === true, undefined, now);
  const preparationLocked = locked || mutating > 0 || pending.comments.length > 0 || pending.error || Boolean(delivery.error) || delivery.uploads.length > 0 || delivery.form.testSubject !== undefined;
  const source = actions.readable ? record?.candidate.baseline : null;
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-4">
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
      <h4 id={`${id}-title`} className="text-body font-semibold">{t(($) => $.stage.code.title)}</h4>
      {actions.writable && <Button type="button" variant="outline" disabled={locked} className={actionClass}
        onClick={(event) => onOpen({ kind: "code", action: "create" }, event.currentTarget)}>{t(($) => $.stage.code.create)}</Button>}
    </div>
    {view.code_candidates === undefined && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.discoveryUnavailable)}</p>}
    {view.code_candidates && view.code_candidates.items.length === 0 && candidateId === null && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.empty)}</p>}
    {view.code_candidates && view.code_candidates.items.length > 0 && <div className="space-y-1">
      <label htmlFor={`${id}-candidate`} className="block text-caption">{t(($) => $.stage.code.candidate)}</label>
      <select id={`${id}-candidate`} className="min-h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 text-body pointer-coarse:min-h-11"
        value={candidateId ?? ""} onChange={(event) => onSelect(event.target.value)}>
        {selectedId && !view.code_candidates.items.some((candidate) => candidate.candidate_id === selectedId) && <option value={selectedId}>{selectedId}</option>}
        {view.code_candidates.items.map((candidate) => <option key={candidate.candidate_id} value={candidate.candidate_id}>{t(($) => $.stage.code.version, { version: candidate.version })} · {candidate.candidate_id} · {labels.codeCandidateState(candidate.state)}</option>)}
      </select>
      {view.code_candidates.truncated && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.truncated)}</p>}
    </div>}
    {candidateId && query.isPending && <p role="status" className="text-caption">{t(($) => $.stage.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && query.data.state !== "available" && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.code.unavailable)}</p>}
    {(collaborationStageSourceDenied(view) || (record && !source)) && <p role="status" className="text-body text-muted-foreground">{t(($) => $.stage.code.sourceUnavailable)}</p>}
    {record && <>
      <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.version, { version: record.candidate.version })} · {labels.codeCandidateState(record.candidate.state)}</p>
      <div className="flex flex-wrap gap-2">{actions.returns.map((step) => <Button key={step} type="button" variant="outline" className={actionClass} disabled={locked}
            onClick={(event) => onOpen({ kind: "code", action: "decide", candidateId: record.candidate.candidate_id, step, decision: "return" }, event.currentTarget)}>{t(($) => $.stage.code.return[step])}</Button>)}</div>
      {source && <>
        <dl className="min-w-0 space-y-2 text-caption [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
          <div><dt className="text-muted-foreground">{t(($) => $.stage.code.pullRequest)}</dt><dd>{source.pull_request.provider} · {source.pull_request.pull_request_id}</dd><dd>{source.pull_request.head_sha}</dd><dd>{labels.pullRequestState(record.observed.pull_request.state)}</dd></div>
          <div><dt className="text-muted-foreground">{t(($) => $.stage.code.ci)}</dt><dd>{labels.ciState(record.observed.ci.state === "passed" && record.observed.ci.observed_at !== null && now - Date.parse(record.observed.ci.observed_at) > 30 * 60_000 ? "stale" : record.observed.ci.state)}</dd></div>
          <div><dt className="text-muted-foreground">{t(($) => $.stage.code.build)}</dt><dd>{source.artifact?.artifact_digest ?? t(($) => $.stage.code.noBuild)}</dd></div>
        </dl>
        <div className="flex flex-wrap gap-2">
          {canPrepare && <Button type="button" variant="outline" className={actionClass} disabled={preparationLocked}
            onClick={() => { if (!preparationLocked && record) delivery.update({ testSubject: collaborationCodeTestSubject(record) }); }}>{t(($) => $.stage.testInput.prepare)}</Button>}
          {actions.writable && (record.candidate.state === "current" || record.candidate.state === "stale") && <Button type="button" variant="outline" className={actionClass} disabled={locked}
            onClick={(event) => onOpen({ kind: "code", action: "create", candidateId: record.candidate.candidate_id }, event.currentTarget)}>{t(($) => $.stage.code.revise)}</Button>}
          {actions.approve.map((step) => <Button key={step} type="button" className={actionClass} disabled={locked}
            onClick={(event) => onOpen({ kind: "code", action: "decide", candidateId: record.candidate.candidate_id, step, decision: "approve" }, event.currentTarget)}>{t(($) => $.stage.code.approve[step])}</Button>)}

          {actions.advances.map((move) => <Button key={move.stage} type="button" variant="outline" className={actionClass} disabled={locked}
            onClick={(event) => onOpen({ kind: "code", action: "advance", candidateId: record.candidate.candidate_id, targetStage: move.stage, decisionIds: move.decisionIds }, event.currentTarget)}>{t(($) => $.stage.code.advance, { stage: labels.stage(move.stage) })}</Button>)}
        </div>
        {record.signable.filter((entry) => entry.blockers.length > 0).map((entry) => <StageBlockers key={entry.step} blockers={entry.blockers} />)}
        <section aria-label={t(($) => $.stage.code.reports)} className="space-y-2">
          <h5 className="text-body font-medium">{t(($) => $.stage.code.reports)}</h5>
          <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.reportHint)}</p>
          {record.test_reports.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.noReports)}</p>}
          {record.test_reports.map((report) => <div key={report.report_id} className="min-w-0 space-y-1 text-caption [overflow-wrap:anywhere]">
            <p>{labels.reportResult(report.reported_result)} · {labels.reportApplicability(report.applicability)}</p>
            <p>{t(($) => $.stage.code.reportCounts, { total: report.case_count, failed: report.failed_case_count })}</p>
            <p className="text-muted-foreground">{t(($) => $.stage.code.reportRun, { run: report.run_id, attempt: report.attempt })}</p>
          </div>)}
          {[...new Set(record.test_reports.map((report) => report.run_id))].map((runId) => <Button key={runId} type="button" variant="outline" size="sm" className={actionClass}
            onClick={() => setReportRun(runId)}>{t(($) => $.stage.report.browseRun, { run: runId })}</Button>)}
          {reportRun && <StageTestReportHistory key={reportRun} scope={scope} issue={issue} runId={reportRun} />}
        </section>
        <section aria-label={t(($) => $.stage.code.decisions)} className="space-y-2">
          <h5 className="text-body font-medium">{t(($) => $.stage.code.decisions)}</h5>
          {record.decisions.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.noDecisions)}</p>}
          {record.decisions.map((decision) => <div key={decision.decision_id} className="min-w-0 space-y-1 text-caption [overflow-wrap:anywhere]">
            <p>{labels.codeStep(decision.step)} · {labels.codeDecision(decision.decision)} · {labels.codeDecisionState(decision.state === "valid" && decision.expires_at !== null && Date.parse(decision.expires_at) <= now ? "expired" : decision.state)}</p>
            <p>{decision.reason}</p>
            <p className="text-muted-foreground">{t(($) => $.stage.code.signedBy, { actor: accountName(decision.actor_user_id) })}</p>
            {decision.expires_at && <p className="text-muted-foreground">{t(($) => $.stage.code.expires, { time: decision.expires_at })}</p>}
            {decision.revocation && <p>{t(($) => $.stage.code.revokedReason, { reason: decision.revocation.reason })}</p>}
            {actions.revocable.includes(decision.decision_id) && <Button type="button" variant="outline" className={actionClass} disabled={locked}
              onClick={(event) => onOpen({ kind: "code", action: "revoke", candidateId: record.candidate.candidate_id, decisionId: decision.decision_id }, event.currentTarget)}>{t(($) => $.stage.code.revoke)}</Button>}
          </div>)}
        </section>
        <StageExecutionHistory record={record} now={now} />
      </>}
    </>}
  </section>;
}
