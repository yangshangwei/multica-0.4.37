import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  COLLABORATION_STAGE_LIMITS, COLLABORATION_STAGE_PRIORITIES, COLLABORATION_STAGE_ROLES, collaborationCommentsOptions, collaborationParticipantsOptions,
  collaborationStageProposalActions, collaborationStageRoleConflicts, collaborationStageScopeSelection, collaborationTextByteLength, useCollaborationStageProposal,
  type CollaborationIssue, type CollaborationScope, type CollaborationStageActionInput, type CollaborationStageCandidate, type CollaborationStageCandidateRef,
  type CollaborationStageProposalContentInput, type CollaborationStageProposalSourceInput, type CollaborationStageRole, type CollaborationStageRolesInput,
  type CollaborationStageState, type CollaborationStageView, type CollaborationStageScopeCode,
} from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { collaborationCommentAuthor } from "./comment-author";
import { CollaborationQueryState } from "./query-state";
import { useCollaborationStageLabels } from "./stage-labels";
import { StageProposalContent } from "./stage-proposals";

const selectClass = "block min-h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11";
const actionClass = "h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11";
const characters = (value: string) => [...value].length;

export type StageSubmit = (input: CollaborationStageActionInput) => void;
interface FormProps { issue: CollaborationIssue; locked: boolean; feedback: ReactNode; submit: StageSubmit }

/** Shared dialog chrome: a focusable title, the issue it acts on, and the live result of the last attempt. */
export function StageFormFrame({ title, issue, feedback, children, onSubmit, label }: { title: string; issue: CollaborationIssue; feedback: ReactNode; children: ReactNode; onSubmit: () => void; label?: string }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus({ preventScroll: true }); }, [title]);
  // Validation is reported next to each field; native bubbles would hide which rule failed.
  return <form noValidate aria-label={label ?? title} className="min-w-0 space-y-4" onSubmit={(event) => { if (event.target !== event.currentTarget) return; event.preventDefault(); onSubmit(); }}>
    <DialogTitle ref={heading} tabIndex={-1} data-stage-heading className="pr-10 outline-none focus-visible:ring-2 focus-visible:ring-ring [overflow-wrap:anywhere]">{title}</DialogTitle>
    <DialogDescription className="[overflow-wrap:anywhere]">{issue.title}</DialogDescription>
    {feedback}
    {children}
  </form>;
}

/** A1. Six explicit holders chosen among current participants; titles grant nothing. */
export function StageRolesForm({ scope, issue, view, locked, feedback, submit }: FormProps & { scope: CollaborationScope; view: CollaborationStageView }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const id = useId();
  const participants = useQuery(collaborationParticipantsOptions(client, scope, issue.id));
  const [roles, setRoles] = useState<Record<CollaborationStageRole, string>>(() => Object.fromEntries(COLLABORATION_STAGE_ROLES.map((role) => [role, view.roles?.[role]?.user_id ?? ""])) as Record<CollaborationStageRole, string>);
  const [touched, setTouched] = useState(false);
  const people = participants.data?.participants ?? [];
  const eligible = new Set(people.filter((person) => person.account_active === true).map((person) => person.user_id.toLowerCase()));
  const input = Object.fromEntries(COLLABORATION_STAGE_ROLES.map((role) => [`${role}_user_id`, roles[role]])) as CollaborationStageRolesInput;
  const conflicts = collaborationStageRoleConflicts(input);
  const complete = COLLABORATION_STAGE_ROLES.every((role) => roles[role] !== "");
  const allEligible = COLLABORATION_STAGE_ROLES.every((role) => roles[role] === "" || eligible.has(roles[role].toLowerCase()));
  const ready = !locked && participants.isSuccess && complete && allEligible && conflicts.length === 0;
  const invalid = (role: CollaborationStageRole) => touched && roles[role] === "" || roles[role] !== "" && participants.isSuccess && !eligible.has(roles[role].toLowerCase()) ||
    conflicts.includes("self_review") && (role === "implementer" || role === "code_reviewer") || conflicts.includes("qa_release") && (role === "qa_approver" || role === "release_approver");
  return <StageFormFrame title={view.roles ? t(($) => $.stage.roles.change) : t(($) => $.stage.roles.configure)} issue={issue} feedback={feedback}
    onSubmit={() => { setTouched(true); if (ready) submit({ action: "configure_roles", roles: input }); }}>
    <p id={`${id}-hint`} className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.roles.formHint)}</p>
    {participants.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {participants.error && <CollaborationQueryState error={participants.error} retry={() => void participants.refetch()} />}
    <fieldset disabled={locked} className="min-w-0 space-y-3">
      {COLLABORATION_STAGE_ROLES.map((role) => {
        const current = roles[role];
        const known = people.find((person) => person.user_id.toLowerCase() === current.toLowerCase());
        const holder = view.roles?.[role];
        const previous = holder?.user_id === current && holder.name.trim() ? holder.name : t(($) => $.participants.accountUnavailable);
        return <div key={role} className="space-y-1">
          <label htmlFor={`${id}-${role}`} className="block text-body">{t(($) => $.stage.roles.names[role])}</label>
          <select id={`${id}-${role}`} value={current} aria-invalid={invalid(role) || undefined} className={selectClass}
            aria-describedby={[`${id}-hint`, conflicts.includes("self_review") && (role === "implementer" || role === "code_reviewer") ? `${id}-self-review` : "",
              conflicts.includes("qa_release") && (role === "qa_approver" || role === "release_approver") ? `${id}-qa-release` : ""].filter(Boolean).join(" ")}
            onChange={(event) => setRoles((value) => ({ ...value, [role]: event.target.value }))}>
            <option value="">{t(($) => $.stage.roles.choose)}</option>
            {current !== "" && participants.isSuccess && !known && <option value={current}>{t(($) => $.stage.roles.notParticipant, { name: previous })}</option>}
            {people.map((person) => <option key={person.user_id} value={person.user_id} disabled={person.account_active !== true}>
              {person.account_active === true ? person.name : t(($) => $.stage.roles.inactive, { name: person.name })}</option>)}
          </select>
        </div>;
      })}
    </fieldset>
    {conflicts.includes("self_review") && <p id={`${id}-self-review`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.roles.selfReview)}</p>}
    {conflicts.includes("qa_release") && <p id={`${id}-qa-release`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.roles.qaRelease)}</p>}
    {touched && !complete && <p role="alert" className="text-caption text-destructive">{t(($) => $.stage.roles.required)}</p>}
    {participants.isSuccess && !allEligible && <p role="alert" className="text-caption text-destructive">{t(($) => $.stage.refusals.stage_role_target_unavailable)}</p>}
    <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.roles.effect)}</p>
    <Button type="submit" disabled={locked || !participants.isSuccess} className={actionClass}>{t(($) => $.stage.roles.save)}</Button>
  </StageFormFrame>;
}

/** A5. Pin the visible scope and explicit code choices until this form closes. */
export function StageScopeForm({ issue, view, locked, feedback, submit }: FormProps & { view: CollaborationStageView }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const id = useId();
  const [baseline] = useState(view);
  const selection = collaborationStageScopeSelection(baseline);
  const [selected, setSelected] = useState<string[]>(selection.preselected);
  const [codes, setCodes] = useState<Record<string, CollaborationStageScopeCode | undefined>>(() => Object.fromEntries(
    selection.candidates.filter((child) => child.approval_current === true).map((child) => [child.issue_id,
      child.code_state === "current" ? child.code : child.code_state === "none" ? null : undefined])));
  const [reason, setReason] = useState("");
  const entries = selection.entries(selected, codes);
  const latest = collaborationStageScopeSelection(view).entries(selected, codes);
  const stale = baseline.lifecycle_epoch !== view.lifecycle_epoch || baseline.role_revision !== view.role_revision ||
    baseline.policy.policy_revision !== view.policy.policy_revision || baseline.scope_revision !== view.scope_revision ||
    entries !== null && JSON.stringify(entries) !== JSON.stringify(latest);
  const reasonOver = characters(reason.trim()) > COLLABORATION_STAGE_LIMITS.decisionReasonCharacters;
  const ready = !locked && !stale && entries !== null && !reasonOver;
  return <StageFormFrame title={t(($) => $.stage.scope.confirm)} issue={issue} feedback={feedback}
    onSubmit={() => { if (ready && entries !== null) submit({ action: "confirm_scope", children: entries, reason }); }}>
    <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.scope.formHint)}</p>
    {stale && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.stage.scope.changed)}</p>}
    {!selection.complete && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.scope.truncated)}</p>}
    {selection.candidates.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.stage.scope.emptyHint)}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-2">
      {selection.candidates.map((child) => <div key={child.issue_id} className="min-w-0 space-y-2">
        <label className="flex min-w-0 cursor-pointer items-start gap-3 py-1 text-body pointer-coarse:min-h-11">
        <Checkbox checked={selected.includes(child.issue_id)}
          onCheckedChange={(checked) => setSelected((value) => checked ? [...new Set([...value, child.issue_id])] : value.filter((item) => item !== child.issue_id))} />
        <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{`${child.identifier} ${child.title}`}</span>
        </label>
        <p className="text-caption text-muted-foreground">{labels.origin(child.origin)}</p>
        {child.depth !== null && child.depth > 1 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.scope.depth, { depth: child.depth })}</p>}
        {selected.includes(child.issue_id) && <div className="min-w-0 space-y-1 pl-7">
          <label htmlFor={`${id}-code-${child.issue_id}`} className="block text-caption">{t(($) => $.stage.scope.code.label, { identifier: child.identifier })}</label>
          <select id={`${id}-code-${child.issue_id}`} className={selectClass}
            value={codes[child.issue_id] === undefined ? "" : codes[child.issue_id] === null ? "none" : JSON.stringify(codes[child.issue_id]?.pull_request)}
            onChange={(event) => {
              const value = event.target.value;
              const pr = child.pull_requests.find((item) => JSON.stringify(item) === value);
              setCodes((current) => ({ ...current, [child.issue_id]: value === "none" ? null : pr ? { pull_request: pr } : undefined }));
            }}>
            <option value="">{t(($) => $.stage.scope.code.choose)}</option>
            <option value="none">{t(($) => $.stage.scope.code.noVersion)}</option>
            {child.pull_requests.map((pr) => <option key={`${pr.provider}:${pr.pull_request_id}`} value={JSON.stringify(pr)}>{pr.provider} · {pr.pull_request_id} · {pr.head_sha}</option>)}
          </select>
          {codes[child.issue_id] && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{codes[child.issue_id]?.pull_request.head_sha}</p>}
        </div>}
      </div>)}
    </fieldset>
    <div className="space-y-1"><label htmlFor={`${id}-reason`} className="block text-body">{t(($) => $.stage.scope.reason)}</label>
      <Textarea id={`${id}-reason`} value={reason} disabled={locked} rows={2} aria-invalid={reasonOver || undefined} aria-describedby={reasonOver ? `${id}-reason-limit` : undefined} onChange={(event) => setReason(event.target.value)} />
      {reasonOver && <p id={`${id}-reason-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.decide.reasonTooLong)}</p>}
    </div>
    <Button type="submit" disabled={!ready} className={actionClass}>{selected.length === 0 ? t(($) => $.stage.scope.submitNone) : t(($) => $.stage.scope.submit, { count: selected.length })}</Button>
  </StageFormFrame>;
}

type ChildDraft = { key: number; title: string; description: string; priority: typeof COLLABORATION_STAGE_PRIORITIES[number] };
const knownPriority = (value: string): ChildDraft["priority"] => (COLLABORATION_STAGE_PRIORITIES as readonly string[]).includes(value) ? value as ChildDraft["priority"] : "none";

/**
 * A2 / A3. Content is written by a human; an agent comment or run may only be
 * named as the source, and the server states who authored it. `modify` is
 * step 1 of modify-accept: it saves a visible new version and decides nothing.
 */
export function StageProposalForm({ scope, issue, mode, proposalId, base, stale, locked: parentLocked, feedback, submit }: FormProps & {
  scope: CollaborationScope; mode: "create" | "revise" | "modify"; proposalId?: string; base?: CollaborationStageCandidate; stale?: boolean;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const client = useQueryClient();
  const id = useId();
  const requiredRead = useCollaborationStageProposal(scope, issue.id, mode === "create" ? null : proposalId ?? null);
  const locked = parentLocked || mode !== "create" && (requiredRead.isError || requiredRead.data?.state !== "available");
  const nextKey = useRef(base?.content.children.length ?? 1);
  const [summary, setSummary] = useState(base?.content.summary ?? "");
  const [children, setChildren] = useState<ChildDraft[]>(() => base ? base.content.children.map((child, index) => ({ key: index, title: child.title, description: child.description, priority: knownPriority(child.priority) }))
    : [{ key: 0, title: "", description: "", priority: "none" }]);
  const [kind, setKind] = useState<"manual" | "comment" | "run">("manual");
  const [commentId, setCommentId] = useState(""), [runId, setRunId] = useState("");
  const [touched, setTouched] = useState(false);
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issue.id), enabled: kind === "comment" });
  const usable = comments.data?.pages.flatMap((page) => page.items).filter((comment) => comment.deleted_at === null) ?? [];
  const comment = usable.find((item) => item.id === commentId) ?? null;
  const runs = issue.execution?.runs ?? [];
  const source: CollaborationStageProposalSourceInput | null = kind === "manual" ? { kind } : kind === "comment"
    ? comment ? { kind, comment_id: comment.id, comment_revision: comment.comment_revision } : null
    : runs.some((run) => run.id === runId) ? { kind, run_id: runId } : null;
  const summaryOver = collaborationTextByteLength(summary) > COLLABORATION_STAGE_LIMITS.summaryBytes;
  const titleOver = (child: ChildDraft) => characters(child.title.trim()) > COLLABORATION_STAGE_LIMITS.childTitleCharacters;
  const descriptionOver = (child: ChildDraft) => collaborationTextByteLength(child.description) > COLLABORATION_STAGE_LIMITS.childDescriptionBytes;
  const content: CollaborationStageProposalContentInput = { summary, children: children.map((child) => ({ title: child.title.trim(), description: child.description, priority: child.priority })) };
  const valid = !summaryOver && children.length >= 1 && children.every((child) => child.title.trim() !== "" && !titleOver(child) && !descriptionOver(child));
  // Identical content is refused by the server unless the newest version is from an earlier lifecycle.
  const unchanged = base !== undefined && stale !== true && JSON.stringify(content) === JSON.stringify({ summary: base.content.summary,
    children: base.content.children.map((child) => ({ title: child.title, description: child.description, priority: knownPriority(child.priority) })) });
  const ready = !locked && valid && source !== null && !unchanged;
  const title = mode === "create" ? t(($) => $.stage.proposals.form.createTitle) : mode === "modify" ? t(($) => $.stage.proposals.form.modifyTitle) : t(($) => $.stage.proposals.form.reviseTitle);
  function send() {
    setTouched(true);
    if (!ready || source === null) return;
    if (mode === "create") submit({ action: "create_proposal", content, source });
    else if (proposalId && base) submit({ action: "revise_proposal", proposalId, expectedCandidate: { candidate_id: base.candidate_id, version: base.version, content_digest: base.content_digest }, content, source });
  }
  const update = (key: number, change: Partial<ChildDraft>) => setChildren((value) => value.map((child) => child.key === key ? { ...child, ...change } : child));
  return <StageFormFrame title={title} issue={issue} feedback={feedback} onSubmit={send}>
    {requiredRead.error && <CollaborationQueryState error={requiredRead.error} retry={() => void requiredRead.refetch()} />}
    {mode === "modify" && <p className="max-w-prose text-body">{t(($) => $.stage.proposals.form.modifyHint)}</p>}
    {base && <p className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.version, { version: base.version })}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-4">
      <div className="space-y-1"><label htmlFor={`${id}-summary`} className="block text-body">{t(($) => $.stage.proposals.form.summary)}</label>
        <Textarea id={`${id}-summary`} value={summary} rows={3} aria-invalid={summaryOver || undefined} aria-describedby={summaryOver ? `${id}-summary-limit` : undefined} onChange={(event) => setSummary(event.target.value)} />
        {summaryOver && <p id={`${id}-summary-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.form.summaryTooLong)}</p>}
      </div>
      <fieldset className="min-w-0 space-y-4"><legend className="mb-2 text-body font-medium">{t(($) => $.stage.proposals.form.childrenLegend)}</legend>
        {children.map((child, index) => {
          const missingTitle = touched && child.title.trim() === "";
          return <fieldset key={child.key} className="min-w-0 space-y-2 rounded-md bg-muted/30 p-3">
            <legend className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.form.child, { index: index + 1 })}</legend>
            <div className="space-y-1"><label htmlFor={`${id}-title-${child.key}`} className="block text-caption">{t(($) => $.stage.proposals.form.childTitle)}</label>
              <Input id={`${id}-title-${child.key}`} value={child.title} required aria-invalid={missingTitle || titleOver(child) || undefined}
                aria-describedby={titleOver(child) ? `${id}-title-limit-${child.key}` : undefined} onChange={(event) => update(child.key, { title: event.target.value })} />
              {titleOver(child) && <p id={`${id}-title-limit-${child.key}`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.form.titleTooLong)}</p>}
            </div>
            <div className="space-y-1"><label htmlFor={`${id}-description-${child.key}`} className="block text-caption">{t(($) => $.stage.proposals.form.childDescription)}</label>
              <Textarea id={`${id}-description-${child.key}`} value={child.description} rows={2} aria-invalid={descriptionOver(child) || undefined}
                aria-describedby={descriptionOver(child) ? `${id}-description-limit-${child.key}` : undefined} onChange={(event) => update(child.key, { description: event.target.value })} />
              {descriptionOver(child) && <p id={`${id}-description-limit-${child.key}`} role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.form.descriptionTooLong)}</p>}
            </div>
            <div className="flex min-w-0 flex-wrap items-end justify-between gap-3">
              <div className="min-w-40 space-y-1"><label htmlFor={`${id}-priority-${child.key}`} className="block text-caption">{t(($) => $.stage.proposals.form.childPriority)}</label>
                <select id={`${id}-priority-${child.key}`} value={child.priority} className={selectClass} onChange={(event) => update(child.key, { priority: knownPriority(event.target.value) })}>
                  {COLLABORATION_STAGE_PRIORITIES.map((priority) => <option key={priority} value={priority}>{labels.priority(priority)}</option>)}
                </select>
              </div>
              {children.length > 1 && <Button type="button" variant="ghost" size="sm" className={actionClass} onClick={() => setChildren((value) => value.filter((item) => item.key !== child.key))}>{t(($) => $.stage.proposals.form.removeChild, { index: index + 1 })}</Button>}
            </div>
          </fieldset>;
        })}
        {children.length < COLLABORATION_STAGE_LIMITS.children && <Button type="button" variant="outline" size="sm" className={actionClass}
          onClick={() => { nextKey.current += 1; setChildren((value) => [...value, { key: nextKey.current, title: "", description: "", priority: "none" }]); }}>{t(($) => $.stage.proposals.form.addChild)}</Button>}
      </fieldset>
      <div className="space-y-2"><label htmlFor={`${id}-source`} className="block text-body">{t(($) => $.stage.proposals.form.sourceKind)}</label>
        <select id={`${id}-source`} value={kind} className={selectClass} onChange={(event) => setKind(event.target.value === "comment" ? "comment" : event.target.value === "run" ? "run" : "manual")}>
          <option value="manual">{t(($) => $.stage.proposals.sources.manual)}</option>
          <option value="comment">{t(($) => $.stage.proposals.sources.comment)}</option>
          <option value="run">{t(($) => $.stage.proposals.sources.run)}</option>
        </select>
        {kind === "comment" && <>
          {comments.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
          {comments.error && <CollaborationQueryState error={comments.error} retry={() => void comments.refetch()} />}
          {comments.isSuccess && usable.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.form.noComments)}</p>}
          {usable.length > 0 && <select aria-label={t(($) => $.stage.proposals.form.chooseComment)} value={commentId} className={selectClass} onChange={(event) => setCommentId(event.target.value)}>
            <option value="">{t(($) => $.stage.proposals.form.chooseComment)}</option>
            {usable.map((item) => <option key={item.id} value={item.id}>{collaborationCommentAuthor(item, { agent: t(($) => $.execution.agent), unknownAuthor: t(($) => $.detail.unknownAuthor) }).name} · {item.content.slice(0, 60)}</option>)}
          </select>}
          {comments.hasNextPage && <Button type="button" variant="ghost" size="sm" className={actionClass} disabled={comments.isFetchingNextPage} onClick={() => void comments.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
          {comment && <Button type="button" variant="outline" size="sm" className={actionClass} onClick={() => setSummary(comment.content)}>{t(($) => $.stage.proposals.form.useCommentText)}</Button>}
        </>}
        {kind === "run" && (runs.length === 0 ? <p className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.form.noRuns)}</p>
          : <select aria-label={t(($) => $.stage.proposals.form.chooseRun)} value={runId} className={selectClass} onChange={(event) => setRunId(event.target.value)}>
            <option value="">{t(($) => $.stage.proposals.form.chooseRun)}</option>
            {runs.map((run) => <option key={run.id} value={run.id}>{t(($) => $.execution[run.step_kind])} · {run.id.slice(-8)}</option>)}
          </select>)}
      </div>
    </fieldset>
    {touched && !valid && <p role="alert" className="text-caption text-destructive">{t(($) => $.stage.proposals.form.invalid)}</p>}
    {unchanged && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.form.unchanged)}</p>}
    {mode === "create" && <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.proposals.form.createEffect)}</p>}
    <Button type="submit" disabled={locked || unchanged || source === null} className={actionClass}>{mode === "create" ? t(($) => $.stage.proposals.form.submitCreate) : t(($) => $.stage.proposals.form.submitRevise)}</Button>
  </StageFormFrame>;
}

/**
 * A4. The decision is pinned to one exact CandidateRef. It is offered only
 * while that reference is still the newest version the server projects; the
 * form never follows a newer version on its own.
 */
export function StageDecisionForm({ scope, issue, state, proposalId, candidate, decision, afterModification, locked, feedback, submit }: FormProps & {
  scope: CollaborationScope; state: CollaborationStageState; proposalId: string; candidate: CollaborationStageCandidateRef; decision: "accept" | "reject"; afterModification: boolean;
}) {
  const { t } = useT("collaboration");
  const id = useId();
  const query = useCollaborationStageProposal(scope, issue.id, proposalId);
  const detail = query.data?.state === "available" ? query.data.value : null;
  const actions = detail ? collaborationStageProposalActions(state, detail, issue) : null;
  const pinned = detail?.versions.find((version) => version.candidate_id === candidate.candidate_id && version.version === candidate.version && version.content_digest === candidate.content_digest) ?? null;
  const newest = actions !== null && actions.newest.candidate_id === candidate.candidate_id && actions.newest.version === candidate.version && actions.newest.content_digest === candidate.content_digest;
  const [reviewed, setReviewed] = useState(false);
  const [reason, setReason] = useState("");
  const [touched, setTouched] = useState(false);
  const trimmed = reason.trim();
  const reasonOver = characters(trimmed) > COLLABORATION_STAGE_LIMITS.decisionReasonCharacters;
  const reasonMissing = decision === "reject" && trimmed === "";
  const ready = !locked && !query.isError && pinned !== null && newest && actions?.canDecide === true && !reasonOver && !reasonMissing && (decision === "reject" || reviewed);
  const title = decision === "reject" ? t(($) => $.stage.proposals.decide.rejectTitle, { version: candidate.version })
    : afterModification ? t(($) => $.stage.proposals.decide.acceptStepTitle) : t(($) => $.stage.proposals.decide.acceptTitle, { version: candidate.version });
  return <StageFormFrame title={title} issue={issue} feedback={feedback}
    onSubmit={() => { setTouched(true); if (ready) submit({ action: "decide_proposal", proposalId, candidate, decision, reason: trimmed }); }}>
    <p className="max-w-prose text-body">{t(($) => $.stage.proposals.decide.exact, { version: candidate.version })}</p>
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && query.data.state !== "available" && <p role="alert" className="text-body">{query.data.state === "not_found" ? t(($) => $.stage.proposals.notFound) : t(($) => $.stage.proposals.unavailable)}</p>}
    {!query.error && !query.data && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {detail && pinned === null && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.decide.waiting)}</p>}
    {pinned && <div className="min-w-0 space-y-3 rounded-md bg-muted/30 p-3"><p className="text-caption text-muted-foreground">{t(($) => $.stage.proposals.version, { version: pinned.version })}</p>
      <StageProposalContent content={pinned.content} /></div>}
    {detail && pinned !== null && !newest && <p role="alert" className="text-body">{t(($) => $.stage.proposals.decide.newerVersion)}</p>}
    {pinned && newest && actions?.canDecide !== true && <p role="status" className="text-caption text-muted-foreground">{actions?.stale ? t(($) => $.stage.proposals.stale) : !state.inSync ? t(($) => $.stage.refreshing) : t(($) => $.stage.outcome.prepare.unavailable)}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-3">
      {decision === "accept" && pinned && <>
        <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.stage.proposals.decide.acceptEffect, { count: pinned.content.children.length })}</p>
        <label className="flex cursor-pointer items-start gap-3 text-body pointer-coarse:min-h-11"><Checkbox checked={reviewed} onCheckedChange={(checked) => setReviewed(checked === true)} />{t(($) => $.stage.proposals.decide.reviewed, { version: candidate.version })}</label>
      </>}
      <div className="space-y-1"><label htmlFor={`${id}-reason`} className="block text-body">{decision === "reject" ? t(($) => $.stage.proposals.decide.reason) : t(($) => $.stage.proposals.decide.reasonOptional)}</label>
        <Textarea id={`${id}-reason`} value={reason} rows={2} required={decision === "reject"} aria-invalid={reasonOver || touched && reasonMissing || undefined}
          aria-describedby={reasonOver || touched && reasonMissing ? `${id}-reason-error` : undefined} onChange={(event) => setReason(event.target.value)} />
        {(reasonOver || touched && reasonMissing) && <p id={`${id}-reason-error`} role="alert" className="text-caption text-destructive">{reasonOver ? t(($) => $.stage.proposals.decide.reasonTooLong) : t(($) => $.stage.proposals.decide.reasonRequired)}</p>}
      </div>
    </fieldset>
    <Button type="submit" variant={decision === "reject" ? "outline" : "default"} disabled={locked || query.isError || pinned === null || !newest || actions?.canDecide !== true || decision === "accept" && !reviewed} className={actionClass}>
      {decision === "reject" ? t(($) => $.stage.proposals.decide.submitReject, { version: candidate.version }) : t(($) => $.stage.proposals.decide.submitAccept, { version: candidate.version })}</Button>
  </StageFormFrame>;
}
