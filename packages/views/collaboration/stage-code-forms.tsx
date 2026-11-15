import { useId, useState, type ReactNode } from "react";
import { CollaborationStageRequestSchemas, deriveCollaborationStageCodeState, useCollaborationStageCodeCandidate,
  useCollaborationTestReport, collaborationTestReportMatchesCandidate, collaborationStageEvidenceMatches, collaborationStageRollbackObservationCurrent,
  type CollaborationIssue, type CollaborationScope, type CollaborationStageActionInput, type CollaborationStageCodeCandidate,
  type CollaborationStageEvidence, type CollaborationStageStep, type CollaborationStageView, type COLLABORATION_STAGES } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { StageFormFrame, type StageSubmit } from "./stage-forms";
import { useCollaborationStageLabels } from "./stage-labels";
import { useStageClock } from "./stage-code-panel";
import { StageTestReportDetail } from "./stage-test-report";

type Stage = typeof COLLABORATION_STAGES[number];
export type StageCodeFormIntent = { kind: "code" } & (
  | { action: "create"; candidateId?: string }
  | { action: "decide"; candidateId: string; step: CollaborationStageStep; decision: "approve" | "return" }
  | { action: "revoke"; candidateId: string; decisionId: string }
  | { action: "advance"; candidateId: string; targetStage: Stage; decisionIds: string[] }
);
interface Props { scope: CollaborationScope; issue: CollaborationIssue; view: CollaborationStageView; locked: boolean; feedback: ReactNode; submit: StageSubmit }
const selectClass = "block min-h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11";
const actionClass = "h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11";
const candidateRef = (record: CollaborationStageCodeCandidate) => ({ candidate_id: record.candidate.candidate_id, version: record.candidate.version, content_digest: record.candidate.content_digest });
const revisions = (view: CollaborationStageView) => JSON.stringify([view.policy.policy_revision, view.role_revision, view.scope_revision, view.lifecycle_epoch, view.stage?.stage_revision]);
const observations = (issue: CollaborationIssue, view: CollaborationStageView) => ({ expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
  expected_account_epoch: issue.write_context?.account_epoch, expected_participant_epoch: issue.write_context?.participant_epoch,
  expected_policy_revision: view.policy.policy_revision, expected_role_revision: view.role_revision, expected_stage_revision: view.stage?.stage_revision });

export function StageCodeForms(props: Props & { intent: StageCodeFormIntent }) {
  const { t } = useT("collaboration");
  const { intent, scope, issue } = props;
  const query = useCollaborationStageCodeCandidate(scope, issue.id, intent.candidateId ?? null, issue.source_workspace.id, props.view.code_candidates);
  const record = query.data?.state === "available" ? query.data.value : null;
  if (intent.candidateId && !record) return <StageFormFrame title={t(($) => $.stage.code.candidate)} issue={issue} feedback={props.feedback} onSubmit={() => undefined}>
    <p role="status">{query.isPending ? t(($) => $.stage.loading) : t(($) => $.stage.code.unavailable)}</p>
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
  </StageFormFrame>;
  if (record?.candidate.baseline === null && !(intent.action === "decide" && intent.decision === "return")) return <StageFormFrame title={t(($) => $.stage.code.candidate)} issue={issue} feedback={props.feedback} onSubmit={() => undefined}>
    <p role="status">{t(($) => $.stage.code.sourceUnavailable)}</p>
  </StageFormFrame>;
  const currentProps = { ...props, locked: props.locked || query.isError,
    feedback: <>{props.feedback}{query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}</> };
  if (intent.action === "create") return <CodeCandidateForm {...currentProps} record={record} />;
  if (!record) return null;
  return intent.action === "decide" ? <CodeDecisionForm key={record.candidate.baseline === null ? "withheld" : "readable"} {...currentProps} intent={intent} record={record} /> : <CodeControlForm {...currentProps} intent={intent} record={record} />;
}

function CodeCandidateForm({ scope, issue, view, locked, feedback, submit, record }: Props & { record: CollaborationStageCodeCandidate | null }) {
  const { t } = useT("collaboration");
  const id = useId();
  const [baseline] = useState(view);
  const [original] = useState(record);
  const [childId, setChild] = useState(original?.candidate.baseline?.child_issue_id ?? "");
  const [prKey, setPR] = useState("");
  const [artifactId, setArtifact] = useState(original?.candidate.baseline?.artifact?.approval_id ?? "");
  const children = baseline.children.items.filter((child) => child.attached && child.can_open && child.content_digest !== null && child.approval_current && child.pull_requests.length > 0);
  const child = children.find((item) => item.issue_id === childId);
  const pr = child?.pull_requests.find((item) => JSON.stringify(item) === prKey);
  const current = view.children.items.find((item) => item.issue_id === childId && item.attached && item.approval_current);
  const stale = revisions(baseline) !== revisions(view) || child && (current?.content_digest !== child.content_digest || pr &&
    !current.pull_requests.some((p) => p.provider === pr.provider && p.pull_request_id === pr.pull_request_id && p.head_sha === pr.head_sha));
  const input = pr ? { action: "create_code_candidate" as const, child_issue_id: childId, supersedes: original ? candidateRef(original) : null, pull_request: pr,
    artifact: artifactId.trim() ? { approval_id: artifactId.trim() } : null } : null;
  const { expected_stage_revision: _stage, ...observed } = observations(issue, view);
  const { action: _action, ...body } = input ?? { action: "create_code_candidate" };
  const ready = !locked && !stale && deriveCollaborationStageCodeState(issue, view, record, scope.userId).writable && input !== null &&
    (original === null || record?.candidate.baseline !== null && (record?.candidate.state === "current" || record?.candidate.state === "stale")) && CollaborationStageRequestSchemas.create_code_candidate.safeParse({ ...observed, ...body }).success;
  return <StageFormFrame title={original ? t(($) => $.stage.code.revise) : t(($) => $.stage.code.create)} issue={issue} feedback={feedback} onSubmit={() => { if (ready && input) submit(input); }}>
    <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.createHint)}</p>
    {stale && <p role="alert" className="text-caption">{t(($) => $.stage.scope.changed)}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-3">
      <div className="space-y-1"><label htmlFor={`${id}-child`} className="block text-caption">{t(($) => $.stage.code.child)}</label>
        <select id={`${id}-child`} className={selectClass} value={childId} onChange={(event) => { setChild(event.target.value); setPR(""); }} disabled={original !== null}>
          <option value="">{t(($) => $.stage.code.chooseChild)}</option>
          {children.map((c) => <option key={c.issue_id} value={c.issue_id}>{c.identifier} · {c.title}</option>)}
        </select></div>
      <div className="space-y-1"><label htmlFor={`${id}-pr`} className="block text-caption">{t(($) => $.stage.code.pullRequestAndHead)}</label>
        <select id={`${id}-pr`} className={selectClass} value={prKey} onChange={(event) => setPR(event.target.value)}>
          <option value="">{t(($) => $.stage.scope.code.choose)}</option>
          {child?.pull_requests.map((p) => <option key={`${p.provider}:${p.pull_request_id}`} value={JSON.stringify(p)}>{p.provider} · {p.pull_request_id} · {p.head_sha}</option>)}
        </select>{pr && <p className="text-caption [overflow-wrap:anywhere]">{pr.head_sha}</p>}</div>
      <div className="space-y-1"><label htmlFor={`${id}-artifact`} className="block text-caption">{t(($) => $.stage.code.artifactApproval)}</label>
        <Input id={`${id}-artifact`} value={artifactId} onChange={(event) => setArtifact(event.target.value)} />
        <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.artifactHint)}</p></div>
    </fieldset>
    <Button type="submit" disabled={!ready} className={actionClass}>{t(($) => $.stage.code.saveCandidate)}</Button>
  </StageFormFrame>;
}

function CodeDecisionForm({ scope, issue, view, locked, feedback, submit, record, intent }: Props & { record: CollaborationStageCodeCandidate; intent: Extract<StageCodeFormIntent, { action: "decide" }> }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const now = useStageClock();
  const [baseline] = useState(record);
  const [originalRevisions] = useState(() => revisions(view));
  const [reason, setReason] = useState("");
  const [reviewed, setReviewed] = useState(false);
  const state = deriveCollaborationStageCodeState(issue, view, record, scope.userId, now);
  const sourceReadable = state.readable && baseline.candidate.baseline !== null;
  const reviewLabel = sourceReadable ? t(($) => $.stage.code.reviewed) : t(($) => $.stage.code.confirmReturn);
  const reasonOver = [...reason.trim()].length > 2000;
  const [reportId, setReport] = useState("");
  const [dependency, setDependency] = useState("");
  const [targetDigest, setTarget] = useState("");
  const [observationId, setObservation] = useState("");
  const [deploymentId, setDeployment] = useState("");
  const max = baseline.signable.find((s) => s.step === intent.step)?.max_expires_at;
  const [expires, setExpires] = useState(() => max ? new Date(max).toISOString().slice(0, 19) : "");
  const preceding = intent.step === "qa" ? "code_review" : intent.step === "release" ? "qa" : intent.step === "rollback" ? "release" : null;
  const deployment = baseline.execution_facts.find((f) => f.fact_id === deploymentId && f.kind === "deployment");
  const dependencies = (intent.decision === "approve" ? state.validDecisions : record.decisions).filter((d) => d.step === preceding && d.decision === "approved" &&
    (intent.step !== "rollback" || !deployment || d.decision_id === deployment.approval_decision_id));
  const dependencyDecision = dependencies.find((d) => d.decision_id === dependency);
  const target = baseline.execution_targets?.find((item) => item.target_digest === targetDigest);
  const metrics = baseline.rollback_observations?.filter((item) => deployment && item.deployment_id === deployment.fact_id &&
    item.target_digest === deployment.target_digest && (intent.decision === "return" || collaborationStageRollbackObservationCurrent(item, deployment, now))) ?? [];
  const observation = metrics.find((item) => item.observation_id === observationId);
  const reports = baseline.test_reports.filter((r) => intent.decision === "return" || r.reported_result === "passed" && r.applicability === "usable");
  const selectedReportId = intent.step === "release" ? dependencyDecision?.evidence.test_report_id : reportId;
  const selectedReport = reports.find((r) => r.report_id === selectedReportId) ?? null;
  const reportQuery = useCollaborationTestReport(scope, issue.id, issue.source_workspace.id, sourceReadable && (intent.step === "qa" || intent.step === "release") ? selectedReport : null);
  const reportDetail = !reportQuery.error && reportQuery.data?.state === "available" ? reportQuery.data.value : null;
  const reportReady = reportDetail !== null && collaborationTestReportMatchesCandidate(record, reportDetail) &&
    reportDetail.reported_result === "passed" && reportDetail.applicability === "usable";
  const evidence: CollaborationStageEvidence = { ci_digest: intent.step === "code_review" ? baseline.observed.ci.ci_digest : null,
    test_report_id: intent.step === "qa" && reportId ? reportId : null,
    build: intent.step === "release" && baseline.candidate.baseline?.artifact && target ? { artifact_digest: baseline.candidate.baseline.artifact.artifact_digest, environment: target.environment, target_digest: target.target_digest } : null,
    deployment_id: intent.step === "rollback" && deployment ? deployment.fact_id : null,
    rollback_target_digest: intent.step === "rollback" && deployment ? deployment.previous_artifact_digest : null,
    ...(intent.step === "rollback" && observation ? { rollback_observation: { observation_id: observation.observation_id, observation_digest: observation.observation_digest } } : {}) };
  const input: Extract<CollaborationStageActionInput, { action: "decide_stage" }> = { action: "decide_stage", record, step: intent.step, decision: intent.decision,
    candidate: candidateRef(baseline), evidence, evidenceReport: reportDetail, depends_on: dependency ? [dependency] : [], reason, expires_at: intent.decision === "approve" && expires ? `${expires}Z` : null };
  const { action: _action, record: _record, evidenceReport: _report, ...body } = input;
  const stale = originalRevisions !== revisions(view) || JSON.stringify(candidateRef(baseline)) !== JSON.stringify(candidateRef(record)) ||
    intent.step === "code_review" && baseline.observed.ci.ci_digest !== record.observed.ci.ci_digest;
  const allowed = (intent.decision === "approve" ? state.approve : state.returns).includes(intent.step);
  const limit = record.signable.find((s) => s.step === intent.step)?.max_expires_at;
  const ready = !locked && !stale && reviewed && allowed && collaborationStageEvidenceMatches(input, state.validDecisions, now) && (intent.step !== "qa" || intent.decision === "return" || reportReady) && CollaborationStageRequestSchemas.decide_stage.safeParse({ ...observations(issue, view), ...body }).success &&
    (intent.decision === "return" || input.expires_at !== null && limit !== null && limit !== undefined && Date.parse(input.expires_at) > now && Date.parse(input.expires_at) <= Date.parse(limit)) &&
    (!dependency || dependencies.some((d) => d.decision_id === dependency)) && (!reportId || record.test_reports.some((r) => r.report_id === reportId &&
      (intent.decision === "return" || r.reported_result === "passed" && r.applicability === "usable")));
  return <StageFormFrame title={intent.decision === "approve" ? t(($) => $.stage.code.approve[intent.step]) : t(($) => $.stage.code.return[intent.step])} issue={issue} feedback={feedback}
    onSubmit={() => { if (ready) submit(input); }}>
    <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.stage.code.version, { version: baseline.candidate.version })} · {baseline.candidate.content_digest}</p>
    <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.approvalHint)}</p>
    {stale && <p role="alert" className="text-caption">{t(($) => $.stage.scope.changed)}</p>}
    {!allowed && <p role="status" className="text-caption">{t(($) => $.stage.code.notSignable)}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-3">
      {sourceReadable && intent.step === "qa" && <div className="space-y-1"><label htmlFor={`${id}-report`} className="block text-caption">{t(($) => $.stage.code.report)}</label>
        <select id={`${id}-report`} className={selectClass} value={reportId} onChange={(event) => setReport(event.target.value)}><option value="">{t(($) => $.stage.code.chooseEvidence)}</option>
          {reports.map((r) => <option key={r.report_id} value={r.report_id}>{r.report_id} · {labels.reportResult(r.reported_result)} · {labels.reportApplicability(r.applicability)}</option>)}</select></div>}
      {sourceReadable && selectedReport && <div className="space-y-2">
        {reportQuery.isPending && <p role="status">{t(($) => $.stage.loading)}</p>}
        {reportQuery.error && <CollaborationQueryState error={reportQuery.error} retry={() => void reportQuery.refetch()} />}
        {reportDetail && <StageTestReportDetail report={reportDetail} />}
        {!reportQuery.isPending && !reportDetail && <p role="status">{t(($) => $.stage.report.unavailable)}</p>}
      </div>}
      {sourceReadable && intent.step === "release" && <><p className="text-caption [overflow-wrap:anywhere]">{baseline.candidate.baseline?.artifact?.artifact_digest ?? t(($) => $.stage.code.noBuild)}</p>
        <div className="space-y-1"><label htmlFor={`${id}-target`} className="block text-caption">{t(($) => $.stage.execution.target)}</label>
          <select id={`${id}-target`} className={selectClass} value={targetDigest} onChange={(event) => setTarget(event.target.value)}><option value="">{t(($) => $.stage.code.chooseEvidence)}</option>
            {baseline.execution_targets?.map((item) => <option key={item.target_digest} value={item.target_digest}>{item.environment} · {item.target_digest}</option>)}</select>
          {target && <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.stage.execution.targetDigest, { digest: target.target_digest })}</p>}</div></>}
      {sourceReadable && intent.step === "rollback" && <div className="space-y-1"><label htmlFor={`${id}-deployment`} className="block text-caption">{t(($) => $.stage.code.deployment)}</label>
        <select id={`${id}-deployment`} className={selectClass} value={deploymentId} onChange={(event) => { setDeployment(event.target.value); setObservation(""); setDependency(""); }}><option value="">{t(($) => $.stage.code.chooseEvidence)}</option>
          {baseline.execution_facts.filter((f) => f.kind === "deployment" && f.result === "succeeded" && f.previous_artifact_digest !== null).map((f) => <option key={f.fact_id} value={f.fact_id}>{f.environment} · {f.operation_id}</option>)}</select>
        {deployment && <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.stage.code.rollbackTarget, { digest: deployment.previous_artifact_digest ?? "" })}</p>}</div>}
      {sourceReadable && intent.step === "rollback" && <div className="space-y-1"><label htmlFor={`${id}-metric`} className="block text-caption">{t(($) => $.stage.execution.metric)}</label>
        <select id={`${id}-metric`} className={selectClass} value={observationId} onChange={(event) => setObservation(event.target.value)}><option value="">{t(($) => $.stage.code.chooseEvidence)}</option>
          {metrics.map((item) => <option key={item.observation_id} value={item.observation_id}>{item.observed_at} · {item.observation_id}</option>)}</select>
        {observation && <><p className="text-caption whitespace-pre-wrap [overflow-wrap:anywhere]">{observation.summary}</p>
          <p className="text-caption">{t(($) => $.stage.execution.metricWindow, { start: observation.window_start, end: observation.window_end })}</p>
          <p className="text-caption">{t(($) => $.stage.execution.validUntil, { time: observation.valid_until })}</p></>}</div>}
      {sourceReadable && preceding && <div className="space-y-1"><label htmlFor={`${id}-dependency`} className="block text-caption">{t(($) => $.stage.code.precedingApproval)}</label>
        <select id={`${id}-dependency`} className={selectClass} value={dependency} onChange={(event) => setDependency(event.target.value)}><option value="">{t(($) => $.stage.code.chooseEvidence)}</option>
          {dependencies.map((d) => <option key={d.decision_id} value={d.decision_id}>{labels.codeStep(d.step)} · {d.actor_user_id} · {d.decision_id}</option>)}</select></div>}
      {intent.decision === "approve" && <div className="space-y-1"><label htmlFor={`${id}-expires`} className="block text-caption">{t(($) => $.stage.code.expiryUTC)}</label>
        <Input id={`${id}-expires`} type="datetime-local" step="1" min={new Date(now).toISOString().slice(0, 19)} max={limit ? new Date(limit).toISOString().slice(0, 19) : undefined} value={expires} onChange={(event) => setExpires(event.target.value.length === 16 ? `${event.target.value}:00` : event.target.value)} />
        {limit && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.maxExpiry, { time: limit })}</p>}</div>}
      <div className="space-y-1"><label htmlFor={`${id}-reason`} className="block text-caption">{t(($) => $.stage.code.reason)}</label><Textarea id={`${id}-reason`} rows={3} value={reason} required={intent.decision === "return"} aria-invalid={reasonOver || undefined} aria-describedby={reasonOver ? `${id}-reason-limit` : undefined} onChange={(event) => setReason(event.target.value)} />
        {reasonOver && <p id={`${id}-reason-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.decide.reasonTooLong)}</p>}</div>
      <label className="flex items-start gap-3 text-body"><Checkbox checked={reviewed} onCheckedChange={(value) => setReviewed(value === true)} /><span>{reviewLabel}</span></label>
    </fieldset>
    <Button type="submit" className={actionClass} disabled={!ready}>{intent.decision === "approve" ? t(($) => $.stage.code.recordApproval) : t(($) => $.stage.code.recordReturn)}</Button>
  </StageFormFrame>;
}

function CodeControlForm({ scope, issue, view, locked, feedback, submit, record, intent }: Props & { record: CollaborationStageCodeCandidate; intent: Extract<StageCodeFormIntent, { action: "revoke" | "advance" }> }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const now = useStageClock();
  const [baseline] = useState(record);
  const [originalRevisions] = useState(() => revisions(view));
  const [reason, setReason] = useState("");
  const state = deriveCollaborationStageCodeState(issue, view, record, scope.userId, now);
  const stale = originalRevisions !== revisions(view) || JSON.stringify(candidateRef(baseline)) !== JSON.stringify(candidateRef(record));
  const canAdvance = intent.action === "advance" && state.advances.some((move) => move.stage === intent.targetStage && JSON.stringify(move.decisionIds) === JSON.stringify(intent.decisionIds));
  const ready = !locked && !stale && (intent.action === "revoke" ? state.revocable.includes(intent.decisionId) && reason.trim().length > 0 && [...reason.trim()].length <= 2000 : canAdvance);
  const title = intent.action === "revoke" ? t(($) => $.stage.code.revoke) : t(($) => $.stage.code.advance, { stage: labels.stage(intent.targetStage) });
  return <StageFormFrame title={title} issue={issue} feedback={feedback} onSubmit={() => {
    if (!ready) return;
    submit(intent.action === "revoke" ? { action: "revoke_decision", record, decisionId: intent.decisionId, reason }
      : { action: "advance_stage", record, target_stage: intent.targetStage, candidate: candidateRef(baseline), decision_ids: intent.decisionIds });
  }}>
    <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.stage.code.version, { version: baseline.candidate.version })} · {baseline.candidate.content_digest}</p>
    <p className="text-caption text-muted-foreground">{intent.action === "revoke" ? t(($) => $.stage.code.revokeHint) : t(($) => $.stage.code.advanceHint)}</p>
    {stale && <p role="alert" className="text-caption">{t(($) => $.stage.scope.changed)}</p>}
    {intent.action === "revoke" ? <div className="space-y-1"><label htmlFor={`${id}-reason`} className="block text-caption">{t(($) => $.stage.code.reason)}</label>
      <Textarea id={`${id}-reason`} value={reason} disabled={locked} onChange={(event) => setReason(event.target.value)} /></div>
      : intent.decisionIds.map((decisionId) => <p key={decisionId} className="text-caption [overflow-wrap:anywhere]">{decisionId}</p>)}
    <Button type="submit" className={actionClass} disabled={!ready}>{intent.action === "revoke" ? t(($) => $.stage.code.confirmRevoke) : t(($) => $.stage.code.confirmAdvance)}</Button>
  </StageFormFrame>;
}
