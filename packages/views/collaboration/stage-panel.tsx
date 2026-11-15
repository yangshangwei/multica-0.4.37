import { useId, useRef, useState, type ReactNode } from "react";
import {
  COLLABORATION_STAGE_LIMITS, COLLABORATION_STAGE_ROLES, isCollaborationStageSendable, deriveCollaborationStageState, dismissCollaborationStageIntent, prepareCollaborationStageAction,
  prepareCollaborationStagePolicyRevision, useCollaborationStageIntents, useCollaborationStageOperation, useCollaborationStagePolicy, useCollaborationStagePolicyAuthority, useCollaborationStageView,
  type CollaborationIssue, type CollaborationScope, type CollaborationStageActionInput, type CollaborationStageCandidate, type CollaborationStageCandidateRef,
  type CollaborationStageIntent, type CollaborationStageStoredIntent, type CollaborationStageOutcome, type CollaborationStagePolicy, type CollaborationStageState, type CollaborationStageView,
} from "@multica/core/collaboration";
import { paths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { Dialog, DialogContent } from "@multica/ui/components/ui/dialog";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useLocale, useT } from "../i18n";
import { AppLink } from "../navigation";
import { CollaborationQueryState } from "./query-state";
import { StageDecisionForm, StageProposalForm, StageRolesForm, StageScopeForm } from "./stage-forms";
import { useCollaborationStageLabels, type StageFeedback } from "./stage-labels";
import { StageProposals } from "./stage-proposals";
import { StageCodePanel } from "./stage-code-panel";
import { StageCodeForms, type StageCodeFormIntent } from "./stage-code-forms";
import { StageBlockers } from "./stage-blockers";

const actionClass = "h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11";
const factsClass = "flex min-w-0 flex-wrap gap-x-8 gap-y-3 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:text-body [&_dd]:[overflow-wrap:anywhere]";

type StageForm =
  | StageCodeFormIntent
  | { kind: "roles" }
  | { kind: "scope" }
  | { kind: "proposal"; mode: "create" }
  | { kind: "proposal"; mode: "revise" | "modify"; proposalId: string; base: CollaborationStageCandidate; stale: boolean }
  | { kind: "decide"; proposalId: string; candidate: CollaborationStageCandidateRef; decision: "accept" | "reject"; afterModification: boolean };

function Feedback({ value }: { value: StageFeedback | null }) {
  if (!value) return null;
  return <div role={value.tone === "failed" ? "alert" : "status"} className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
    <p className={value.tone === "failed" ? "text-body text-destructive" : "text-body"}>{value.text}</p>
    {value.detail && <p className="text-caption text-muted-foreground">{value.detail}</p>}
    {value.blockers && <StageBlockers blockers={value.blockers} truncated={value.blockersTruncated} />}
  </div>;
}

/**
 * Stored intents whose result is not known. Checking is a GET; resending the
 * same key and body and discarding the local record are separate, explicit
 * choices of the human. Nothing here ever sends a request on its own.
 */
function PendingStageRequests({ intents, error, busy, checked, onCheck, onResend, onDiscard, readOnly = false, sendDisabled = false }: {
  intents: CollaborationStageStoredIntent[]; error: boolean; busy: boolean; checked: string[]; readOnly?: boolean; sendDisabled?: boolean;
  onCheck: (intent: CollaborationStageStoredIntent) => void; onResend: (intent: CollaborationStageStoredIntent) => void; onDiscard: (intent: CollaborationStageStoredIntent) => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  if (!error && intents.length === 0) return null;
  const discardable = intents.some((intent) => checked.includes(intent.requestId));
  return <div className="min-w-0 space-y-3 rounded-md bg-muted/30 p-3" data-stage-pending>
    {error && <p role="alert" className="text-body text-destructive">{t(($) => $.stage.pending.storageError)}</p>}
    {intents.length > 0 && <><h4 className="text-body font-medium">{t(($) => $.stage.pending.title)}</h4>
      <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.pending.hint)}</p>
      <ul className="min-w-0 space-y-3">{intents.map((intent) => <li key={intent.requestId} className="min-w-0 space-y-2">
        <p className="text-body [overflow-wrap:anywhere]">{labels.pendingAction(intent.action)}</p>
        <div className="flex min-w-0 flex-wrap gap-2">
          <Button type="button" variant="outline" size="sm" className={actionClass} disabled={busy} data-stage-recovery onClick={() => onCheck(intent)}>{t(($) => $.stage.pending.check)}</Button>
          {!readOnly && isCollaborationStageSendable(intent) && <Button type="button" variant="ghost" size="sm" className={actionClass} disabled={busy || sendDisabled} onClick={() => onResend(intent)}>{t(($) => $.stage.pending.resend)}</Button>}
          {checked.includes(intent.requestId) && <Button type="button" variant="ghost" size="sm" className={actionClass} disabled={busy} aria-describedby={`${id}-discard`} onClick={() => onDiscard(intent)}>{t(($) => $.stage.pending.discard)}</Button>}
        </div>
      </li>)}</ul>
      {discardable && <p id={`${id}-discard`} className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.pending.discardHint)}</p>}
    </>}
  </div>;
}

export function CollaborationStagePanel({ scope, issue, readUnavailable = false }: { scope: CollaborationScope; issue: CollaborationIssue; readUnavailable?: boolean }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const query = useCollaborationStageView(scope, issue);
  const state = deriveCollaborationStageState(query.data, issue);
  const view = query.data?.state === "available" && state.availability === "available" ? query.data.value : null;
  const stored = useCollaborationStageIntents(scope, { kind: "issue", issueId: issue.id });
  const operation = useCollaborationStageOperation(scope, issue.id);
  const { mutateAsync } = operation;
  const [feedback, setFeedback] = useState<StageFeedback | null>(null);
  const [checked, setChecked] = useState<string[]>([]);
  const [form, setForm] = useState<StageForm | null>(null);
  const [codeCandidateId, setCodeCandidateId] = useState<string | null>(null);
  const section = useRef<HTMLElement>(null);
  const dialog = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLElement | null>(null);
  const running = useRef(false);
  const busy = operation.isPending;
  // An unconfirmed request blocks new stage writes for this issue: its outcome decides what the next one may observe.
  const locked = busy || stored.intents.length > 0 || stored.error || query.isError || readUnavailable;

  async function run(intent: CollaborationStageStoredIntent, mode: "send" | "recover"): Promise<CollaborationStageOutcome | null> {
    if (running.current) return null;
    running.current = true;
    setFeedback(null);
    try {
      const outcome = await mutateAsync({ intent, mode });
      setFeedback(labels.outcome(intent.action, outcome, mode === "recover"));
      // Only a request whose check settled nothing in this mount may be discarded locally.
      setChecked((value) => outcome.kind === "unknown" && mode === "recover" ? [...new Set([...value, intent.requestId])] : value.filter((item) => item !== intent.requestId));
      return outcome;
    } catch (error) {
      setFeedback(labels.failure(error));
      return null;
    } finally { running.current = false; }
  }

  function open(next: StageForm, opener: HTMLElement | null) { trigger.current = opener; setFeedback(null); operation.reset(); setForm(next); }
  function close() { setForm(null); }
  async function submit(input: CollaborationStageActionInput) {
    if (!view || locked) return;
    let intent: CollaborationStageIntent;
    try { intent = prepareCollaborationStageAction(scope, issue, view, input); }
    catch (error) { setFeedback(labels.failure(error)); return; }
    const current = form;
    const outcome = await run(intent, "send");
    if (outcome?.kind !== "confirmed") return;
    // Modify-accept is two explicit requests: the saved version is shown again and accepted separately.
    if (current?.kind === "proposal" && current.mode === "modify" && outcome.confirmation.action === "revise_proposal") {
      const saved = outcome.confirmation;
      setForm({ kind: "decide", proposalId: saved.proposal_id, candidate: { candidate_id: saved.candidate_id, version: saved.version, content_digest: saved.content_digest }, decision: "accept", afterModification: true });
      return;
    }
    if (outcome.confirmation.action === "create_code_candidate") setCodeCandidateId(outcome.confirmation.candidate_id);
    close();
  }
  function discard(intent: CollaborationStageStoredIntent) {
    if (busy) return;
    try { dismissCollaborationStageIntent(scope, intent); setChecked((value) => value.filter((item) => item !== intent.requestId)); setFeedback(null); }
    catch { setFeedback({ text: t(($) => $.stage.outcome.discardFailed), tone: "failed" }); }
  }

  const pending = <PendingStageRequests intents={stored.intents} error={stored.error} busy={busy} checked={checked} sendDisabled={query.isError || readUnavailable}
    onCheck={(intent) => void run(intent, "recover")} onResend={(intent) => void run(intent, "send")} onDiscard={discard} />;
  const live = <>
    {busy && <p role="status" className="text-body">{t(($) => $.stage.outcome.sending)}</p>}
    <Feedback value={feedback} />
    {pending}
  </>;
  const accountName = (userId: string) => {
    const known = [...(view?.roles ? COLLABORATION_STAGE_ROLES.map((role) => view.roles![role]) : []), issue.creator, issue.handler]
      .find((account) => account?.user_id.toLowerCase() === userId.toLowerCase());
    return known?.name.trim() ? known.name : t(($) => $.stage.proposals.unknownAccount);
  };
  const formProps = { issue, locked, feedback: live };
  const formNode = !form || !view ? null
    : form.kind === "code" ? <StageCodeForms key={JSON.stringify(form)} {...formProps} intent={form} scope={scope} view={view} submit={(input) => void submit(input)} />
    : form.kind === "roles" ? <StageRolesForm {...formProps} scope={scope} view={view} submit={(input) => void submit(input)} />
    : form.kind === "scope" ? <StageScopeForm {...formProps} view={view} submit={(input) => void submit(input)} />
    : form.kind === "proposal" ? <StageProposalForm key={form.mode === "create" ? "create" : `${form.mode}:${form.base.candidate_id}`} {...formProps} scope={scope} mode={form.mode}
      proposalId={form.mode === "create" ? undefined : form.proposalId} base={form.mode === "create" ? undefined : form.base} stale={form.mode === "create" ? undefined : form.stale} submit={(input) => void submit(input)} />
    : <StageDecisionForm key={`${form.decision}:${form.candidate.candidate_id}`} {...formProps} scope={scope} state={state} proposalId={form.proposalId} candidate={form.candidate}
      decision={form.decision} afterModification={form.afterModification} submit={(input) => void submit(input)} />;
  const formOpen = formNode !== null;

  return <section ref={section} id="stage" tabIndex={-1} aria-labelledby={`${id}-title`} className="min-w-0 space-y-5 outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <h3 id={`${id}-title`} className="text-title-sm font-semibold">{t(($) => $.stage.title)}</h3>
    {query.error && !query.isFetching && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {state.availability === "loading" && !query.error && <p role="status" className="text-body text-muted-foreground">{t(($) => $.stage.loading)}</p>}
    {state.availability === "unsupported" && <p className="text-body text-muted-foreground">{t(($) => $.stage.unsupported)}</p>}
    {state.availability === "unavailable" && <div role="status" className="min-w-0 space-y-3"><p className="text-body text-muted-foreground">
      {query.data?.state === "unavailable" && query.data.reason === "schema" ? t(($) => $.stage.schemaUnavailable) : t(($) => $.stage.unavailable)}</p>
      <Button type="button" variant="outline" className={actionClass} disabled={query.isFetching} onClick={() => void query.refetch()}>{t(($) => $.stage.refresh)}</Button></div>}
    {!formOpen && live}
    {view && state.managed !== true && <UnmanagedStage scope={scope} issue={issue} view={view} state={state} accountName={accountName} run={run} busy={busy} writeUnavailable={query.isError || readUnavailable} />}
    {view && state.managed === true && <>
      {!state.known && <p role="alert" className="text-body">{t(($) => $.stage.unknownState)}</p>}
      {state.known && !state.inSync && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.refreshing)}</p>}
      <dl className={factsClass}>
        <div><dt>{t(($) => $.stage.summary.policy)}</dt><dd>{t(($) => $.stage.summary.policyEnabled, { revision: view.policy.policy_revision })}</dd></div>
        <div><dt>{t(($) => $.stage.summary.currentStage)}</dt><dd>{labels.stage(state.stage)}</dd></div>
        <div><dt>{t(($) => $.stage.summary.businessOwner)}</dt><dd>{view.roles === null ? t(($) => $.stage.summary.notConfigured)
          : view.roles.business_owner?.name.trim() || t(($) => $.participants.accountUnavailable)}</dd></div>
        <div><dt>{t(($) => $.stage.summary.handler)}</dt><dd>{issue.handler.name}</dd></div>
      </dl>
      {state.missing.length > 0 && <div className="min-w-0 space-y-2 rounded-md bg-muted/30 p-3">
        <p className="text-body font-medium">{t(($) => $.stage.missing.title)}</p>
        <ul className="list-disc space-y-1 pl-5 text-body">{state.missing.map((item) => <li key={item}>{item === "roles" ? t(($) => $.stage.missing.roles) : t(($) => $.stage.missing.scope)}</li>)}</ul>
        <p className="text-caption text-muted-foreground">{t(($) => $.stage.missing.hint)}</p>
      </div>}
      {stored.intents.length > 0 && !formOpen && <p className="text-caption text-muted-foreground">{t(($) => $.stage.pending.locked)}</p>}
      <StageRoles view={view} action={state.can.configureRoles ? <Button type="button" variant="outline" className={actionClass} disabled={locked} onClick={(event) => open({ kind: "roles" }, event.currentTarget)}>
        {view.roles ? t(($) => $.stage.roles.change) : t(($) => $.stage.roles.configure)}</Button> : null} />
      <StageChildren issue={issue} view={view} action={state.can.confirmScope ? <Button type="button" variant="outline" className={actionClass} disabled={locked} onClick={(event) => open({ kind: "scope" }, event.currentTarget)}>
        {t(($) => $.stage.scope.confirm)}</Button> : null} />
      <StageProposals scope={scope} issue={issue} state={state} accountName={accountName} locked={locked}
        onCreate={(opener) => open({ kind: "proposal", mode: "create" }, opener)}
        onIntent={(intent, opener) => open(intent.kind === "decide" ? { ...intent, afterModification: false } : { kind: "proposal", mode: intent.kind, proposalId: intent.proposalId, base: intent.base, stale: intent.stale }, opener)} />
      <StageCodePanel scope={scope} issue={issue} view={view} locked={locked} selectedId={codeCandidateId} onSelect={setCodeCandidateId} onOpen={open} accountName={accountName} />
      <StagePolicy scope={scope} issue={issue} projectId={view.policy.project_id} run={run} busy={busy} writeUnavailable={query.isError || readUnavailable} />
    </>}
    <Dialog open={formOpen} onOpenChange={(next) => { if (!next && !busy) close(); }}>
      <DialogContent ref={dialog} showCloseButton={!busy} closeLabel={t(($) => $.creation.close)}
        initialFocus={() => dialog.current?.querySelector<HTMLElement>("[data-stage-heading]") ?? false}
        finalFocus={() => trigger.current?.isConnected && !trigger.current.matches(':disabled, [aria-disabled="true"]') ? trigger.current : section.current?.querySelector<HTMLElement>("[data-stage-recovery]") ?? section.current}
        className="max-h-[90dvh] overflow-y-auto overscroll-contain sm:max-w-2xl pointer-coarse:[&_[data-slot=dialog-close]]:min-h-11 pointer-coarse:[&_[data-slot=dialog-close]]:min-w-11">
        {formNode}
      </DialogContent>
    </Dialog>
  </section>;
}

type StageRun = (intent: CollaborationStageStoredIntent, mode: "send" | "recover") => Promise<CollaborationStageOutcome | null>;

/** Without an enabled policy nothing is offered; stored roles and children are history only. */
function UnmanagedStage({ scope, issue, view, state, accountName, run, busy, writeUnavailable }: {
  scope: CollaborationScope; issue: CollaborationIssue; view: CollaborationStageView; state: CollaborationStageState; accountName: (userId: string) => string; run: StageRun; busy: boolean; writeUnavailable: boolean;
}) {
  const { t } = useT("collaboration");
  const [open, setOpen] = useState(false);
  // A root that was never configured has no stage history worth a disclosure.
  const hasHistory = view.roles !== null || view.scope !== null || view.children.items.some((child) => child.origin !== "none");
  return <>
    <p className="text-body text-muted-foreground">{view.policy.project_id === null ? t(($) => $.stage.notManaged.noProject)
      : state.policyState === "disabled" ? t(($) => $.stage.notManaged.disabled) : state.policyState === "none" ? t(($) => $.stage.notManaged.none) : t(($) => $.stage.notManaged.unknown)}</p>
    {hasHistory && <Collapsible open={open} onOpenChange={setOpen} className="min-w-0 space-y-4">
      <CollapsibleTrigger render={<Button type="button" variant="ghost" className={actionClass} />}>{t(($) => $.stage.history)}</CollapsibleTrigger>
      <CollapsibleContent className="min-w-0 space-y-5"><StageRoles view={view} action={null} /><StageChildren issue={issue} view={view} action={null} />
        {/* Read-only: `state.can` is false for every action on an unmanaged root. */}
        {open && <StageProposals scope={scope} issue={issue} state={state} accountName={accountName} locked onCreate={() => undefined} onIntent={() => undefined} />}</CollapsibleContent>
    </Collapsible>}
    <StagePolicy scope={scope} issue={issue} projectId={view.policy.project_id} run={run} busy={busy} writeUnavailable={writeUnavailable} />
  </>;
}

/** Role holders are accounts the server names. The business owner is shown apart from the current handler. */
function StageRoles({ view, action }: { view: CollaborationStageView; action: ReactNode }) {
  const { t } = useT("collaboration");
  const id = useId();
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-3">
    <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
      <h4 id={`${id}-title`} className="text-body font-semibold">{t(($) => $.stage.roles.title)}
        {view.roles && <span className="ml-2 text-caption font-normal text-muted-foreground">{t(($) => $.stage.roles.revision, { revision: view.roles.role_revision })}</span>}</h4>
      {action}
    </div>
    {view.roles === null ? <p className="text-body text-muted-foreground">{t(($) => $.stage.roles.empty)}</p>
      : <dl className={factsClass}>{COLLABORATION_STAGE_ROLES.map((role) => <div key={role}><dt>{t(($) => $.stage.roles.names[role])}</dt>
        {/* A holder whose account no longer resolves is shown as unavailable, never as a valid signer. */}
        <dd>{view.roles![role]?.name.trim() || t(($) => $.participants.accountUnavailable)}{view.roles![role]?.external === true && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.detail.external)}</span>}</dd></div>)}</dl>}
  </section>;
}

/** Children as the server projects them: where each came from, whether its approval is current, and whether AI execution is gated. */
function StageChildren({ issue, view, action }: { issue: CollaborationIssue; view: CollaborationStageView; action: ReactNode }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const slug = issue.ordinary_children?.source_workspace_slug ?? null;
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-3">
    <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
      <div className="min-w-0 space-y-1"><h4 id={`${id}-title`} className="text-body font-semibold">{t(($) => $.stage.scope.title)}</h4>
        <p className="text-caption text-muted-foreground">{t(($) => $.stage.scope.state)}: {labels.scopeState(view.scope_state)}</p></div>
      {action}
    </div>
    {view.children.items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.stage.scope.empty)}</p>}
    <ul className="min-w-0 space-y-2">{view.children.items.map((child) => {
      const reference = <><span className="text-caption text-muted-foreground">{child.identifier} </span><span className="text-body">{child.title}</span></>;
      return <li key={child.issue_id} className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
        {child.can_open === true && slug ? <AppLink href={paths.workspace(slug).issueDetail(child.issue_id)} aria-label={`${child.identifier} ${child.title}`}
          className="inline-flex min-h-8 min-w-0 flex-wrap items-baseline gap-x-2 rounded-md underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">{reference}</AppLink>
          : <div className="flex min-w-0 flex-wrap items-baseline gap-x-2">{reference}</div>}
        <p className="text-caption text-muted-foreground">{[labels.origin(child.origin),
          child.origin !== "none" ? child.approval_current === true ? t(($) => $.stage.scope.approvalCurrent) : t(($) => $.stage.scope.approvalNotCurrent) : null,
          child.execution_gated === true ? t(($) => $.stage.scope.gated) : null,
          child.attached !== true ? t(($) => $.stage.scope.detached) : null].filter(Boolean).join(" · ")}</p>
        {child.depth !== null && child.depth > 1 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.scope.depth, { depth: child.depth })}</p>}
        <p className="text-caption text-muted-foreground">{labels.codeState(child.code_state)}</p>
        {child.code !== null && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{child.code.pull_request.provider} · {child.code.pull_request.pull_request_id} · {child.code.pull_request.head_sha}</p>}
      </li>;
    })}</ul>
    {view.children.truncated === true && <p className="text-caption text-muted-foreground">{t(($) => $.stage.scope.truncated)}</p>}
  </section>;
}

/**
 * P1-P3. The policy belongs to the source project and is read with the source
 * workspace named explicitly. Enabling is offered only while the server states
 * `activation_available === true`; the temporary 503 is "not available yet".
 */
function StagePolicy({ scope, issue, projectId, run, busy, writeUnavailable = false }: { scope: CollaborationScope; issue: CollaborationIssue; projectId: string | null; run: StageRun; busy: boolean; writeUnavailable?: boolean }) {
  const { t } = useT("collaboration");
  const [open, setOpen] = useState(false);
  if (projectId === null) return null;
  return <Collapsible open={open} onOpenChange={setOpen} className="min-w-0 space-y-4">
    <CollapsibleTrigger render={<Button type="button" variant="ghost" className={actionClass} />}>{t(($) => $.stage.policy.open)}</CollapsibleTrigger>
    <CollapsibleContent className="min-w-0 space-y-4">{open && <StagePolicyContent scope={scope} issue={issue} projectId={projectId} run={run} busy={busy} writeUnavailable={writeUnavailable} />}</CollapsibleContent>
  </Collapsible>;
}

function StagePolicyContent({ scope, issue, projectId, run, busy, writeUnavailable }: { scope: CollaborationScope; issue: CollaborationIssue; projectId: string; run: StageRun; busy: boolean; writeUnavailable: boolean }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const labels = useCollaborationStageLabels();
  const id = useId();
  const workspaceId = issue.source_workspace.id;
  const query = useCollaborationStagePolicy(scope, issue.id, workspaceId, projectId, true);
  const authority = useCollaborationStagePolicyAuthority(scope, workspaceId);
  const admin = authority.data?.role === "owner" || authority.data?.role === "admin";
  const canManage = admin && authority.isSuccess && !authority.isFetching;
  const stored = useCollaborationStageIntents(scope, { kind: "project", workspaceId, projectId });
  const [reason, setReason] = useState("");
  const [touched, setTouched] = useState(false);
  const [checked, setChecked] = useState<string[]>([]);
  const [failure, setFailure] = useState<StageFeedback | null>(null);
  const policy: CollaborationStagePolicy | null = query.data?.state === "available" ? query.data.value : null;
  const sendDisabled = busy || stored.error || query.isError || writeUnavailable || !canManage || policy === null;
  const locked = sendDisabled || stored.intents.length > 0;
  const length = [...reason.trim()].length;
  const reasonValid = length >= 1 && length <= COLLABORATION_STAGE_LIMITS.policyReasonCharacters;
  async function revise(enabled: boolean) {
    setTouched(true);
    if (!policy || locked || !reasonValid) return;
    let intent: CollaborationStageIntent;
    try { intent = prepareCollaborationStagePolicyRevision(scope, workspaceId, policy, { enabled, reason }, canManage ? authority.data ?? null : null); setFailure(null); }
    catch (error) { setFailure(labels.failure(error)); return; }
    const outcome = await run(intent, "send");
    if (outcome?.kind === "confirmed") { setReason(""); setTouched(false); }
    // "Not available yet" changes what may be offered: read the server's current statement again.
    if (outcome?.kind === "not_written") void query.refetch();
  }
  async function check(intent: CollaborationStageStoredIntent) {
    const outcome = await run(intent, "recover");
    setChecked((value) => outcome?.kind === "unknown" ? [...new Set([...value, intent.requestId])] : value.filter((item) => item !== intent.requestId));
  }
  // Enabling is offered only while the server states it can be written, exactly.
  const change = policy === null || !admin ? null : policy.enabled === true ? "disable" as const : policy.activation_available === true ? "enable" as const : null;
  return <section aria-label={t(($) => $.stage.policy.title)} className="min-w-0 space-y-4">
    <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.policy.hint)}</p>
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {authority.error && <CollaborationQueryState error={authority.error} retry={() => void authority.refetch()} />}
    {!query.data && !query.error && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.policy.loading)}</p>}
    {query.data && query.data.state !== "available" && <div role="status" className="min-w-0 space-y-3"><p className="text-body text-muted-foreground">{
      query.data.state === "denied" ? t(($) => $.stage.policy.denied) : query.data.state === "not_found" ? t(($) => $.stage.policy.notFound)
        : query.data.state === "unsupported" ? t(($) => $.stage.policy.unsupported) : query.data.reason === "schema" ? t(($) => $.stage.schemaUnavailable) : t(($) => $.stage.policy.unavailable)}</p>
      {query.data.state === "unavailable" && <Button type="button" variant="outline" className={actionClass} disabled={query.isFetching} onClick={() => void query.refetch()}>{t(($) => $.stage.refresh)}</Button>}</div>}
    <PendingStageRequests intents={stored.intents} error={stored.error} busy={busy} checked={checked} sendDisabled={sendDisabled} onCheck={(intent) => void check(intent)} onResend={(intent) => void run(intent, "send")}
      onDiscard={(intent) => { try { dismissCollaborationStageIntent(scope, intent); setChecked((value) => value.filter((item) => item !== intent.requestId)); setFailure(null); }
        catch { setFailure({ text: t(($) => $.stage.outcome.discardFailed), tone: "failed" }); } }} />
    {failure && <p role={failure.tone === "failed" ? "alert" : "status"} className={failure.tone === "failed" ? "text-body text-destructive" : "text-body"}>{failure.text}</p>}
    {policy && <>
      <dl className={factsClass}>
        <div><dt>{t(($) => $.stage.policy.state)}</dt><dd>{policy.enabled === true ? t(($) => $.stage.policy.enabled) : policy.policy_revision === "0" ? t(($) => $.stage.policy.neverSet) : t(($) => $.stage.policy.disabled)}</dd></div>
        <div><dt>{t(($) => $.stage.policy.revision)}</dt><dd>{policy.policy_revision}</dd></div>
        {policy.created_at && <div><dt>{t(($) => $.stage.policy.changedAt)}</dt><dd><time dateTime={policy.created_at}>{new Date(policy.created_at).toLocaleString(locale)}</time></dd></div>}
        {policy.reason.trim() && <div><dt>{t(($) => $.stage.policy.lastReason)}</dt><dd className="whitespace-pre-wrap">{policy.reason}</dd></div>}
      </dl>
      {policy.enabled !== true && policy.activation_available !== true && <p role="status" className="text-body text-muted-foreground">{t(($) => $.stage.policy.activationUnavailable)}</p>}
      {change !== null && <form className="min-w-0 space-y-3" aria-label={t(($) => $.stage.policy.title)} onSubmit={(event) => { event.preventDefault(); void revise(change === "enable"); }}>
        <p className="text-caption text-muted-foreground">{t(($) => $.stage.policy.adminOnly)}</p>
        <fieldset disabled={locked} className="min-w-0 space-y-3">
          <div className="space-y-1"><label htmlFor={`${id}-reason`} className="block text-body">{t(($) => $.stage.policy.reason)}</label>
            <Textarea id={`${id}-reason`} value={reason} rows={2} aria-invalid={touched && !reasonValid || undefined} aria-describedby={touched && !reasonValid ? `${id}-reason-error` : undefined} onChange={(event) => setReason(event.target.value)} />
            {touched && !reasonValid && <p id={`${id}-reason-error`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.policy.reasonRequired)}</p>}
          </div>
          <Button type="submit" variant={change === "enable" ? "default" : "outline"} className={actionClass}>{change === "enable" ? t(($) => $.stage.policy.enable) : t(($) => $.stage.policy.disable)}</Button>
        </fieldset>
      </form>}
    </>}
  </section>;
}

export function CollaborationStageRecovery({ scope, issueId }: { scope: CollaborationScope; issueId: string }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const stored = useCollaborationStageIntents(scope, { kind: "issue", issueId });
  const operation = useCollaborationStageOperation(scope, issueId);
  const [checked, setChecked] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<StageFeedback | null>(null);
  if (!stored.error && stored.intents.length === 0 && !feedback) return null;
  return <section aria-label={t(($) => $.stage.pending.recoveryOnly)} className="min-w-0 space-y-3">
    <p className="text-caption text-muted-foreground">{t(($) => $.stage.pending.recoveryOnlyHint)}</p>
    <Feedback value={feedback} />
    <PendingStageRequests intents={stored.intents} error={stored.error} busy={operation.isPending} checked={checked} readOnly
      onResend={() => undefined} onCheck={(intent) => { void operation.mutateAsync({ intent, mode: "recover" }).then((outcome) => {
        setFeedback(labels.outcome(intent.action, outcome, true));
        if (outcome.kind === "unknown") setChecked((keys) => [...new Set([...keys, intent.requestId])]);
      }).catch((error: unknown) => setFeedback(labels.failure(error))); }}
      onDiscard={(intent) => { try { dismissCollaborationStageIntent(scope, intent); } catch (error) { setFeedback(labels.failure(error)); } }} />
  </section>;
}
