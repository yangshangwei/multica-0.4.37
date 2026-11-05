import { useEffect, useId, useRef, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationFieldOptions, collaborationPropertyValueMatchesDefinition, collaborationKeys, executeCollaborationAction, isCollaborationDenied, parseCollaborationActionDraft, prepareCollaborationAction, useCollaborationPendingDrafts, useCollaborationStatusOptions,
  deriveCollaborationDetailState, useCollaborationDeliveryForm, collaborationCommentWithinByteLimit, collaborationDraftAttachmentsOptions, collaborationAttachmentsOptions,
  type CollaborationAction, type CollaborationActionIntent, type CollaborationAttachment, type CollaborationComment, type CollaborationDraft, type CollaborationIssue, type CollaborationRecipientAccount, type CollaborationScope, type CollaborationStatusAction } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { RichContent } from "../rich-content";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationAccountPicker } from "./account-picker";
import { CollaborationParticipants } from "./participants";
import { CollaborationAttachmentPicker } from "./attachment-picker";
import { CollaborationSubmissionRecord } from "./delivery-panel";
import type { IssuePropertyValue } from "@multica/core/types";
import { recoverFocus, useOpenedForm } from "./focus";
import { CollaborationIterationInput, CollaborationLabelInput, CollaborationPropertyInputs, CollaborationSourceInputs, type CollaborationSourceSelection } from "./field-inputs";

type ActionName = CollaborationAction["action"];
const successful = new Set(["participant_added", "already_participant", "transferred", "already_handler", "participant_left", "participant_removed", "information_requested", "information_returned", "acceptance_submitted", "acceptance_returned", "ai_policy_changed", "ai_policy_unchanged", "status_changed", "status_unchanged", "completed", "cancelled", "reopened", "issue_edited", "iteration_changed", "unchanged"]);

export function CollaborationWorkflow({ scope, issue, initialAction, initialStatus, resultFromComment, onResultPrepared, onDiscuss }: {
  scope: CollaborationScope; issue: CollaborationIssue; initialAction?: "transfer" | "change_status" | "edit"; initialStatus?: string;
  resultFromComment?: CollaborationComment | null; onResultPrepared?: () => void; onDiscuss?: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const state = deriveCollaborationDetailState(issue, scope.userId);
  const [selection, setSelection] = useState<{ action: ActionName } | null>(() => initialAction ? { action: initialAction } : null);
  const selected = selection?.action;
  const [restored, setRestored] = useState<CollaborationAction | null>(null);
  const submitting = useRef(false);
  const trigger = useRef<HTMLElement | null>(null);
  const section = useRef<HTMLElement>(null);
  const dismiss = () => { setSelection(null); recoverFocus(() => {
    if (trigger.current?.isConnected) trigger.current.scrollIntoView({ block: "nearest" });
    return trigger.current;
  }, () => section.current); };
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const apply = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "human-action"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationAction(client, scope, draft, recovery),
    onSuccess: (result, { draft, recovery }) => {
      if (successful.has(result.code) && (result.status === undefined || result.status < 400)) { dismiss(); return; }
      if (recovery) {
        const original = parseCollaborationActionDraft(draft);
        if (original.action !== "remove") {
          setRestored(original); setSelection({ action: original.action });
        }
      }
    } });
  const actions: [ActionName, boolean][] = [["invite", issue.capabilities.invite], ["transfer", issue.capabilities.transfer],
    ["request_information", issue.capabilities.request_information], ["return_information", issue.capabilities.return_information], ["submit_acceptance", issue.capabilities.submit_acceptance],
    ["return_acceptance", issue.capabilities.return_acceptance === true], ["change_ai_policy", issue.capabilities.change_ai_policy === true],
    ["change_status", issue.capabilities.change_status], ["complete", issue.capabilities.complete && (state.canAccept || state.canSelfComplete)], ["cancel", issue.capabilities.cancel], ["reopen", issue.capabilities.reopen],
    ["leave", issue.capabilities.leave], ["edit", issue.capabilities.edit],
    ["edit_source_structure", issue.capabilities.edit_source_structure && issue.source_structure !== undefined],
    ["change_iteration", issue.capabilities.edit_source_structure && issue.source_structure !== undefined]];
  const allowed = new Set([state.primary, ...state.secondary]);
  const available = actions.filter(([action, permitted]) => permitted === true && issue.write_context && allowed.has(action)).map(([action]) => action);
  const primary = state.primary !== "discuss" && state.primary !== null && available.includes(state.primary) ? state.primary : null;
  const secondary = state.secondary.filter((action): action is ActionName => action !== "discuss" && available.includes(action)).slice(0, 2);
  const more = available.filter((action) => action !== primary && !secondary.includes(action));
  const locked = apply.isPending || drafts.actions.length > 0 || drafts.error;
  const startingWork = selected === "change_status" && state.primary === "change_status" && ["backlog", "todo"].includes(issue.status_category) && initialAction === undefined;
  const preparedSelection = useRef<CollaborationComment | null>(null);
  const resetApply = apply.reset;
  useEffect(() => {
    if (preparedSelection.current === resultFromComment) return;
    preparedSelection.current = resultFromComment ?? null;
    if (!resultFromComment || resultFromComment.issue_id !== issue.id || locked) return;
    const action = state.canSelfComplete ? "complete" : "submit_acceptance";
    if (available.includes(action)) {
      trigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      resetApply(); setRestored(null); setSelection({ action });
    }
  }, [resultFromComment, issue.id, locked, state.canSelfComplete, available, resetApply]);
  const actionLabel = (action: ActionName, isPrimary = false) => action === "complete" ? state.canAccept ? t(($) => $.delivery.accept) : t(($) => $.delivery.selfComplete)
    : action === "change_status" && isPrimary && ["backlog", "todo"].includes(issue.status_category) ? t(($) => $.delivery.start) : labels.action(action);
  function select(action: ActionName, button: HTMLButtonElement) { trigger.current = button; apply.reset(); setRestored(null); setSelection({ action }); }
  return <section ref={section} tabIndex={-1} aria-label={t(($) => $.workflow.title)} className="space-y-4 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <div className="flex min-w-0 flex-wrap items-start gap-2">
      {primary && <Button className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={locked} onClick={(event) => select(primary, event.currentTarget)}>{actionLabel(primary, true)}</Button>}
      {state.primary === "discuss" && onDiscuss && <Button className="pointer-coarse:min-h-11" onClick={onDiscuss}>{t(($) => $.detail.comments)}</Button>}
      {secondary.map((action) => <Button key={action} variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={locked} onClick={(event) => select(action, event.currentTarget)}>{actionLabel(action)}</Button>)}
      {more.length > 0 && <details className="min-w-0 open:basis-full"><summary className="cursor-pointer rounded-md px-3 py-1.5 text-body font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">{t(($) => $.delivery.more)}</summary>
        <div className="flex min-w-0 flex-wrap gap-2 pt-3">{more.map((action) => <Button key={action} variant="ghost" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={locked} onClick={(event) => select(action, event.currentTarget)}>{actionLabel(action)}</Button>)}</div>
      </details>}
    </div>
    <CollaborationParticipants key={issue.id} scope={scope} issue={issue} locked={locked} submit={async (draft) => {
      if (submitting.current) return;
      submitting.current = true;
      try { await apply.mutateAsync({ draft, recovery: false }); } catch { /* Original-key recovery remains available above the form. */ }
      finally { submitting.current = false; }
    }} />
    {drafts.error && <p role="alert" className="text-body">{t(($) => $.workflow.storageError)}</p>}
    {apply.isPending && <p role="status" className="text-body">{t(($) => $.workflow.pending)}</p>}
    {apply.error && <CollaborationQueryState error={apply.error} />}
    {apply.data && <p role="status" className="text-body">{successful.has(apply.data.code) && (apply.data.status === undefined || apply.data.status < 400) ? t(($) => $.workflow.success) : t(($) => $.workflow.refused)}</p>}
    {!apply.isPending && drafts.actions.length > 0 && <div className="space-y-3"><p role="status" className="text-body">{t(($) => $.workflow.unknown)}</p>{drafts.actions.map((draft) =>
      <Button key={draft.requestId} variant="outline" disabled={apply.isPending} className="pointer-coarse:min-h-11" onClick={() => apply.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button>)}</div>}
    {selected && issue.write_context && available.includes(selected) && <WorkflowForm key={`${selected}:${restored?.requestId ?? "new"}`} opening={selection} scope={scope} issue={issue} action={selected} restoredAction={restored?.action === selected ? restored : undefined} initialStatus={selected === "change_status" ? initialStatus : undefined} startWork={startingWork} resultFromComment={resultFromComment} onResultPrepared={onResultPrepared} locked={locked} close={dismiss} submit={async (intent, baseline) => {
      if (submitting.current) return;
      submitting.current = true;
      try {
        const draft = prepareCollaborationAction(scope, baseline, crypto.randomUUID(), intent);
        const result = await apply.mutateAsync({ draft, recovery: false });
        if (successful.has(result.code) && (result.status === undefined || result.status < 400)) dismiss();
      } catch { /* The original saved intent is the recovery boundary. */ }
      finally { submitting.current = false; }
    }} />}
  </section>;
}

function WorkflowForm({ scope, issue, action, opening, initialStatus, restoredAction, locked, submit, close, startWork = false, resultFromComment, onResultPrepared }: {
  scope: CollaborationScope; issue: CollaborationIssue; action: ActionName; initialStatus?: string; restoredAction?: CollaborationAction; locked: boolean;
  opening: unknown;
  submit: (intent: CollaborationActionIntent, baseline: CollaborationIssue) => Promise<void>; close: () => void;
  startWork?: boolean; resultFromComment?: CollaborationComment | null; onResultPrepared?: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const heading = useOpenedForm<HTMLHeadingElement>({ opening, reveal: true });
  // A recovered rejection retains the original observations. Accepting a new
  // baseline is a separate, explicit action and never submits automatically.
  const [baseline, setBaseline] = useState<CollaborationIssue>(() => restoredAction ? { ...issue,
    collaboration_revision: restoredAction.expectedCollaborationRevision, lifecycle_epoch: restoredAction.expectedLifecycleEpoch,
    write_context: { participant_epoch: restoredAction.expectedParticipantEpoch, account_epoch: restoredAction.expectedAccountEpoch },
    source_structure: issue.source_structure && restoredAction.action === "change_iteration" ? { ...issue.source_structure, issue_revision: restoredAction.expectedIssueRevision } : issue.source_structure,
  } : issue);
  const restoredEdit = restoredAction?.action === "edit" ? restoredAction : null;
  const restoredStructure = restoredAction?.action === "edit_source_structure" ? restoredAction : null;
  const restoredIteration = restoredAction?.action === "change_iteration" ? restoredAction : null;
  const [submissionId, setSubmissionId] = useState(() => restoredAction?.action === "return_acceptance" ? restoredAction.submissionId
    : restoredAction?.action === "complete" ? restoredAction.submissionId ?? null : baseline.delivery?.current?.id ?? null);
  const [informationRequestId, setInformationRequestId] = useState(() => restoredAction?.action === "return_information" ? restoredAction.informationRequestId : baseline.current_information_request?.id ?? null);
  const [recipient, setRecipient] = useState<CollaborationRecipientAccount | null>(() => {
    if (restoredAction && "targetUserId" in restoredAction) return [baseline.handler, baseline.creator, baseline.delivery?.current?.author, baseline.current_information_request?.requester]
      .find((person) => person?.user_id === restoredAction.targetUserId) ?? null;
    return action === "return_acceptance" ? baseline.delivery?.current?.author ?? null : null;
  });
  const recipientWasChosen = useRef(restoredAction !== undefined && "targetUserId" in restoredAction);
  const [note, setNote] = useState(restoredAction && "note" in restoredAction ? restoredAction.note ?? "" : "");
  const [status, setStatus] = useState(initialStatus ?? "");
  const [aiPolicy, setAIPolicy] = useState(restoredAction?.action === "change_ai_policy" ? restoredAction.aiPolicy : baseline.ai_policy ?? "");
  const result = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId,
    context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "result");
  const [uploading, setUploading] = useState(false);
  const [referencesOpen, setReferencesOpen] = useState(false);
  const [preparationBlocked, setPreparationBlocked] = useState(false);
  const preparedComment = useRef<CollaborationComment | null>(null);
  const baselineState = deriveCollaborationDetailState(baseline, scope.userId);
  const deciding = action === "return_acceptance" || action === "complete" && submissionId !== null;
  const submitted = [baseline.delivery?.current, baseline.delivery?.latest].find((value) => value?.id === submissionId) ?? null;
  const needsResult = action === "submit_acceptance" || action === "complete" && submissionId === null && baselineState.canSelfComplete;
  const draftFiles = useQuery({ ...collaborationDraftAttachmentsOptions(client, scope, result.form), enabled: needsResult && result.form.selectedAttachmentIds.length > 0 });
  const references = useQuery({ ...collaborationAttachmentsOptions(client, scope, issue.id), enabled: needsResult && (referencesOpen || result.form.selectedAttachmentIds.length > 0) });
  const selectedFilesKnown = result.form.selectedAttachmentIds.every((id) => [...(draftFiles.data?.attachments ?? []), ...(references.data?.attachments ?? [])].some((file) => file.id === id));
  const resultLocked = locked || uploading || result.uploads.length > 0 || Boolean(result.error);
  useEffect(() => {
    if (preparedComment.current === resultFromComment) return;
    preparedComment.current = resultFromComment ?? null;
    if (!needsResult || !resultFromComment || resultFromComment.issue_id !== issue.id) return;
    setPreparationBlocked(resultLocked);
    if (!resultLocked) result.update({ content: resultFromComment.content, basis: "", sourceCommentId: resultFromComment.id, selectedAttachmentIds: resultFromComment.attachment_ids });
    onResultPrepared?.();
  }, [needsResult, resultFromComment, issue.id, resultLocked, result, onResultPrepared]);
  const [title, setTitle] = useState(restoredEdit?.title ?? issue.title);
  const [description, setDescription] = useState(restoredEdit?.description ?? issue.description);
  const [startDate, setStartDate] = useState((restoredEdit ? restoredEdit.startDate : issue.start_date) ?? "");
  const [dueDate, setDueDate] = useState((restoredEdit ? restoredEdit.dueDate : issue.due_date) ?? "");
  const [priority, setPriority] = useState(restoredEdit?.priority ?? issue.priority);
  const [labelIds, setLabelIds] = useState(restoredEdit?.labelIds ?? issue.labels?.map((label) => label.id));
  const [properties, setProperties] = useState<Record<string, IssuePropertyValue | null>>(restoredEdit?.properties ?? {});
  const [parentSearch, setParentSearch] = useState("");
  const [sourceSelection, setSourceSelection] = useState<CollaborationSourceSelection>(restoredStructure ? { projectId: restoredStructure.projectId, parentIssueId: restoredStructure.parentIssueId, stage: restoredStructure.stage }
    : { projectId: issue.source_structure?.project?.id ?? null, parentIssueId: issue.source_structure?.parent_issue?.id ?? null, stage: issue.source_structure?.stage ?? null });
  const [iterationId, setIterationId] = useState(restoredIteration ? restoredIteration.iterationId : issue.source_structure?.iteration?.id ?? null);
  const [iterationRevision, setIterationRevision] = useState(restoredIteration ? restoredIteration.expectedIterationRevision : issue.source_structure?.iteration?.revision);
  const [iterationReason, setIterationReason] = useState(restoredIteration?.iterationReason ?? "");
  const [allowCompleted, setAllowCompleted] = useState(restoredIteration?.allowCompleted ?? false);
  const needsSource = action === "edit_source_structure" || action === "change_iteration";
  const needsFields = needsSource || action === "edit" && baseline.labels !== undefined && baseline.properties !== undefined;
  const fields = useQuery({ ...collaborationFieldOptions(client, scope, issue, parentSearch), enabled: needsFields && issue.capabilities.edit === true, placeholderData: keepPreviousData });
  const sourceOptions = issue.capabilities.edit_source_structure === true ? fields.data?.source_options
    ?? (fields.error && !isCollaborationDenied(fields.error) ? client.getQueryData(collaborationFieldOptions(client, scope, issue).queryKey)?.source_options : undefined) : undefined;
  const propertyDefinitions = [...(fields.data?.properties ?? []), ...(baseline.property_definitions ?? []).filter((item) => !fields.data?.properties.some((option) => option.id === item.id)).map((item) => ({ ...item, archived: true }))];
  const propertyValues: Record<string, IssuePropertyValue> = { ...baseline.properties };
  for (const [id, value] of Object.entries(properties)) { if (value === null) delete propertyValues[id]; else propertyValues[id] = value; }
  const invalidProperties = Object.entries(properties).some(([id, value]) => value !== null && !fields.data?.properties.some((definition) => definition.id === id && collaborationPropertyValueMatchesDefinition(value, definition)));
  const invalidLabels = labelIds !== undefined && fields.data !== undefined && labelIds.some((id) => !fields.data!.labels.some((label) => label.id === id));
  const staleBaseline = baseline.collaboration_revision !== issue.collaboration_revision || baseline.lifecycle_epoch !== issue.lifecycle_epoch ||
    baseline.write_context?.participant_epoch !== issue.write_context?.participant_epoch || baseline.write_context?.account_epoch !== issue.write_context?.account_epoch ||
    needsSource && baseline.source_structure?.issue_revision !== issue.source_structure?.issue_revision ||
    action === "change_iteration" && sourceOptions?.iterations.some((item) => item.id === iterationId && item.revision !== iterationRevision) === true;
  const needsRecipient = action === "invite" || action === "transfer" || action === "request_information" || action === "return_acceptance";
  const needsStatus = ["change_status", "complete", "cancel", "reopen", "request_information", "return_information", "submit_acceptance", "return_acceptance"].includes(action);
  const options = useCollaborationStatusOptions(scope, issue.id, (needsStatus ? action : "change_status") as CollaborationStatusAction, needsStatus);
  const chosenStatus = status || (startWork ? options.data?.options.find((option) => option.category === "in_progress")?.key : options.data?.default_status) || "";
  const editReady = action !== "edit" || (title.trim() !== "" && ["none", "low", "medium", "high", "urgent"].includes(priority) && baseline.start_date !== undefined && baseline.due_date !== undefined && !invalidProperties && !invalidLabels);
  const sourceReady = !needsSource || sourceOptions !== undefined && !fields.error && !fields.isFetching && !fields.isPlaceholderData && (sourceSelection.stage === null || Number.isInteger(sourceSelection.stage) && sourceSelection.stage >= 1 && sourceSelection.stage <= 2147483647);
  // The answer goes to the open request the form observed, so show that one.
  const informationRequest = action === "return_information" && baseline.current_information_request?.id === informationRequestId ? baseline.current_information_request : null;
  const needsCompletedConfirmation = baseline.status_category === "done" && iterationId !== null && iterationId !== baseline.source_structure?.iteration?.id;
  const iterationReady = action !== "change_iteration" || sourceOptions?.iterations_enabled === true && (!iterationId || iterationRevision !== undefined && sourceOptions.iterations.some((item) => item.id === iterationId)) && (!baseline.source_structure?.iteration || baseline.source_structure.iteration.id === iterationId || iterationReason.trim() !== "") && (!needsCompletedConfirmation || allowCompleted);
  const resultTooLong = needsResult && !collaborationCommentWithinByteLimit(result.form.content);
  const basisTooLong = needsResult && !collaborationCommentWithinByteLimit(result.form.basis ?? "");
  const reasonTooLong = action === "return_acceptance" && !collaborationCommentWithinByteLimit(note);
  const resultReady = !needsResult || !resultLocked && selectedFilesKnown && result.form.content.trim() !== "" && (result.form.basis ?? "").trim() !== "" && !resultTooLong && !basisTooLong;
  const decisionReady = !deciding || submitted !== null && submissionId === issue.delivery?.current?.id && !staleBaseline &&
    (action === "return_acceptance" ? note.trim() !== "" && !reasonTooLong : baselineState.canAccept);
  const completionReady = action !== "complete" || (deciding ? baselineState.canAccept : baselineState.canSelfComplete);
  const informationReady = action !== "return_information" || informationRequest !== null && informationRequestId === issue.current_information_request?.id;
  const policyReady = action !== "change_ai_policy" || aiPolicy === "manual" || aiPolicy === "assisted";
  const ready = !locked && (!needsRecipient || recipient !== null) && (!needsStatus || (options.isSuccess && !options.isFetching && options.data.options.some((item) => item.key === chosenStatus))) &&
    editReady && sourceReady && iterationReady && resultReady && decisionReady && completionReady && informationReady && policyReady && (!needsResult || !staleBaseline);
  const formLabel = action === "complete" ? deciding ? t(($) => $.delivery.accept) : t(($) => $.delivery.selfComplete)
    : startWork ? t(($) => $.delivery.start) : labels.action(action);
  const evidence = () => ({ deliveryDraftId: result.form.draftId, attachmentIds: [...result.form.selectedAttachmentIds],
    ...(draftFiles.data?.attachments.some((file) => file.delivered_at === null && result.form.selectedAttachmentIds.includes(file.id)) ? { draftId: result.form.draftId } : {}),
    ...(result.form.sourceCommentId ? { sourceCommentId: result.form.sourceCommentId } : {}) });
  function intent(): CollaborationActionIntent {
    switch (action) {
      case "invite": return { action, targetUserId: recipient!.user_id };
      case "transfer": return { action, targetUserId: recipient!.user_id, note };
      case "request_information": return { action, targetUserId: recipient!.user_id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "return_information": return { action, informationRequestId: informationRequestId!, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "submit_acceptance": return { action, note: result.form.content, basis: result.form.basis ?? "", ...evidence(), expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "return_acceptance": return { action, submissionId: submissionId!, targetUserId: recipient!.user_id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "complete": return deciding ? { action, submissionId: submissionId!, note, status: chosenStatus, statusOptionsVersion: options.data!.status_options_version }
        : { action, note: result.form.content, basis: result.form.basis ?? "", ...evidence(), status: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "change_ai_policy": {
        if (aiPolicy !== "manual" && aiPolicy !== "assisted") throw new Error("Unsupported AI policy");
        return { action, aiPolicy };
      }
      case "change_status": case "cancel": case "reopen": return { action, status: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "leave": return { action };
      case "edit": return { action, title, description, startDate: startDate || null, dueDate: dueDate || null, priority: priority as "none" | "low" | "medium" | "high" | "urgent", ...(fields.data && labelIds ? { labelIds, properties } : {}) };
      case "edit_source_structure": return { action, ...sourceSelection };
      case "change_iteration": return { action, iterationId, expectedIssueRevision: baseline.source_structure!.issue_revision, ...(iterationId ? { expectedIterationRevision: iterationRevision } : {}), iterationReason, ...(needsCompletedConfirmation ? { allowCompleted } : {}) };
      default: throw new Error("Unsupported collaboration action");
    }
  }
  return <form aria-label={formLabel} onSubmit={(event) => { if (event.target !== event.currentTarget) return; event.preventDefault(); if (ready) void submit(intent(), baseline); }} className="min-w-0 space-y-4 rounded-lg border border-border p-4">
    <h3 ref={heading} tabIndex={-1} className="rounded-sm text-body font-semibold outline-none focus-visible:ring-2 focus-visible:ring-faint-foreground">{formLabel}</h3>
    <fieldset disabled={locked} className="min-w-0 space-y-4">
      {deciding && (submitted ? <CollaborationSubmissionRecord scope={scope} issueId={issue.id} submission={submitted} /> : <p className="text-body text-muted-foreground">{t(($) => $.delivery.unavailable)}</p>)}
      {informationRequest && <div id={`${fieldId}-request`} className="min-w-0 space-y-2 rounded-md bg-muted/30 p-3 [overflow-wrap:anywhere]">
        <p className="text-caption text-muted-foreground">{t(($) => $.workflow.requestedBy, { name: informationRequest.requester.name })}</p>
        {informationRequest.note.trim() ? <RichContent resourcePolicy="restricted" content={informationRequest.note} /> : <p className="text-body text-muted-foreground">{t(($) => $.workflow.requestNoNote)}</p>}
        {informationRequest.return_status_fallback === true && informationRequest.return_status && <p className="text-caption text-muted-foreground">{t(($) => $.workflow.returnStatusFallback,
          { status: options.data?.options.find((item) => item.key === informationRequest.return_status)?.name ?? informationRequest.return_status })}</p>}
      </div>}
      {needsRecipient && <div className="space-y-2"><label className="text-body">{t(($) => $.workflow.recipient)}</label>
        <CollaborationAccountPicker scope={scope} context={{ kind: "issue", issueId: issue.id }} selected={recipient} label={t(($) => $.workflow.recipient)} disabled={locked}
          onSelect={(person) => { recipientWasChosen.current = true; setRecipient(person); }} />
        {recipient && <p role="status" className="text-body [overflow-wrap:anywhere]">{t(($) => $.workflow.confirmRecipient, { name: recipient.name })}</p>}
      </div>}
      {needsStatus && <div className="space-y-2"><label htmlFor={`${fieldId}-status`} className="text-body">{t(($) => $.detail.status)}</label><select id={`${fieldId}-status`} value={chosenStatus} onChange={(event) => setStatus(event.target.value)} className="block min-h-11 max-w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        {chosenStatus && options.data && !options.data.options.some((item) => item.key === chosenStatus) && <option value={chosenStatus}>{t(($) => $.fields.unavailableSelection, { value: chosenStatus })}</option>}
        {(options.data?.options ?? []).map((option) => <option key={option.key} value={option.key}>{option.name}</option>)}</select>{options.error && <CollaborationQueryState error={options.error} />}</div>}
      {["transfer", "request_information", "return_information", "return_acceptance"].includes(action) && <div className="space-y-2"><label htmlFor={`${fieldId}-note`} className="text-body">{action === "return_acceptance" ? t(($) => $.delivery.revisionReason) : t(($) => $.workflow.note)}</label>
        <Textarea id={`${fieldId}-note`} required={action === "return_acceptance"} aria-invalid={reasonTooLong || undefined}
          aria-describedby={[informationRequest ? `${fieldId}-request` : "", reasonTooLong ? `${fieldId}-note-limit` : ""].filter(Boolean).join(" ") || undefined} value={note} onChange={(event) => setNote(event.target.value)} />
        {reasonTooLong && <p id={`${fieldId}-note-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.memory.tooLong, { max: 65_536 })}</p>}
      </div>}
      {needsResult && <>
        <p id={`${fieldId}-result-hint`} className="text-caption text-muted-foreground">{t(($) => $.delivery.hint)}</p>
        <div className="space-y-2"><label htmlFor={`${fieldId}-result`} className="text-body">{t(($) => $.delivery.result)}</label>
          <Textarea id={`${fieldId}-result`} value={result.form.content} required disabled={resultLocked} aria-describedby={`${fieldId}-result-hint${resultTooLong ? ` ${fieldId}-result-limit` : ""}`} aria-invalid={resultTooLong || undefined}
            onChange={(event) => result.update({ content: event.target.value })} className="min-h-28" />
          {resultTooLong && <p id={`${fieldId}-result-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.delivery.tooLong)}</p>}
        </div>
        <div className="space-y-2"><label htmlFor={`${fieldId}-basis`} className="text-body">{t(($) => $.delivery.basis)}</label>
          <Textarea id={`${fieldId}-basis`} value={result.form.basis ?? ""} required disabled={resultLocked} aria-invalid={basisTooLong || undefined} aria-describedby={basisTooLong ? `${fieldId}-basis-limit` : undefined}
            onChange={(event) => result.update({ basis: event.target.value })} className="min-h-24" />
          {basisTooLong && <p id={`${fieldId}-basis-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.delivery.tooLong)}</p>}
        </div>
        {preparationBlocked && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.delivery.preparationBlocked)}</p>}
        <CollaborationAttachmentPicker scope={scope} form={result.form} update={result.update} uploads={result.uploads} onBusyChange={setUploading}
          disabled={locked || Boolean(result.error)} />
        <details className="min-w-0 space-y-3" onToggle={(event) => setReferencesOpen(event.currentTarget.open)}>
          <summary className="cursor-pointer py-2 text-body font-medium pointer-coarse:min-h-11">{t(($) => $.delivery.reuseFiles)}</summary>
          {references.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.attachments.loading)}</p>}
          {references.error && <CollaborationQueryState error={references.error} retry={() => void references.refetch()} />}
          <ResultReferenceFiles files={references.data?.attachments ?? []} selected={result.form.selectedAttachmentIds} disabled={resultLocked}
            onChange={(selectedAttachmentIds) => result.update({ selectedAttachmentIds })} />
        </details>
        {!selectedFilesKnown && <div className="space-y-2"><p role="status" className="text-caption text-muted-foreground">{t(($) => $.delivery.filesUnavailable)}</p>
          <Button type="button" variant="outline" disabled={resultLocked} className="pointer-coarse:min-h-11" onClick={() => result.update({ selectedAttachmentIds: [] })}>{t(($) => $.delivery.clearFiles)}</Button></div>}
        {draftFiles.error && <CollaborationQueryState error={draftFiles.error} retry={() => void draftFiles.refetch()} />}
        {Boolean(result.error) && <p role="alert" className="text-body">{t(($) => $.workflow.storageError)}</p>}
      </>}
      {action === "change_ai_policy" && <div className="space-y-3">
        <label htmlFor={`${fieldId}-policy`} className="text-body">{t(($) => $.policy.title)}</label>
        <select id={`${fieldId}-policy`} value={aiPolicy} onChange={(event) => setAIPolicy(event.target.value === "manual" ? "manual" : "assisted")} className="block min-h-11 max-w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {!aiPolicy && <option value="">{t(($) => $.policy.unknown)}</option>}
          <option value="manual">{t(($) => $.policy.manual)}</option><option value="assisted">{t(($) => $.policy.assisted)}</option>
        </select>
        <p className="text-caption text-muted-foreground">{aiPolicy === "manual" ? t(($) => $.policy.manualHint) : t(($) => $.policy.assistedHint)}</p>
      </div>}
      {action === "edit" && <>
        <div className="space-y-2"><label htmlFor={`${fieldId}-title`} className="text-body">{t(($) => $.workflow.titleLabel)}</label><Input id={`${fieldId}-title`} value={title} onChange={(event) => setTitle(event.target.value)} className="pointer-coarse:min-h-11" /></div>
        <div className="space-y-2"><label htmlFor={`${fieldId}-body`} className="text-body">{t(($) => $.workflow.descriptionLabel)}</label><Textarea id={`${fieldId}-body`} value={description} onChange={(event) => setDescription(event.target.value)} /></div>
        <div className="flex flex-wrap gap-4"><div className="space-y-2"><label htmlFor={`${fieldId}-start`} className="text-body">{t(($) => $.workflow.startDate)}</label><Input type="date" id={`${fieldId}-start`} value={startDate} onChange={(event) => setStartDate(event.target.value)} className="pointer-coarse:min-h-11" /></div><div className="space-y-2"><label htmlFor={`${fieldId}-due`} className="text-body">{t(($) => $.workflow.dueDate)}</label><Input type="date" id={`${fieldId}-due`} value={dueDate} onChange={(event) => setDueDate(event.target.value)} className="pointer-coarse:min-h-11" /></div></div>
        <div className="space-y-2"><label htmlFor={`${fieldId}-priority`} className="text-body">{t(($) => $.workflow.priority)}</label><select id={`${fieldId}-priority`} value={priority} onChange={(event) => setPriority(event.target.value)} className="block min-h-11 max-w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {(["none", "low", "medium", "high", "urgent"] as const).map((value) => <option key={value} value={value}>{t(($) => $.priorities[value])}</option>)}
        </select></div>
        {fields.data && labelIds !== undefined && <>
          <CollaborationLabelInput labels={fields.data.labels} selectedIds={labelIds} onChange={setLabelIds} disabled={locked} />
          <CollaborationPropertyInputs scope={scope} issueId={issue.id} sourceId={issue.source_workspace.id} definitions={propertyDefinitions} values={propertyValues} disabled={locked}
            onChange={(id, value) => setProperties((current) => ({ ...current, [id]: value ?? null }))} />
        </>}
        {(invalidProperties || invalidLabels) && <p role="alert" className="text-body">{t(($) => $.fields.invalidSelection)}</p>}
      </>}
      {action === "edit_source_structure" && sourceOptions && <CollaborationSourceInputs options={sourceOptions} current={baseline.source_structure} value={sourceSelection} onChange={setSourceSelection} parentSearch={parentSearch} onParentSearch={setParentSearch}
        parentsPending={fields.isFetching || fields.isPlaceholderData} parentsError={fields.error} onRetryParents={() => void fields.refetch()} disabled={locked} />}
      {action === "change_iteration" && sourceOptions && <>
        <CollaborationIterationInput options={sourceOptions} selectedId={iterationId} current={baseline.source_structure?.iteration} disabled={locked || fields.isFetching} statusCategory={baseline.status_category}
          onChange={(id) => { setIterationId(id); setIterationRevision(sourceOptions.iterations.find((item) => item.id === id)?.revision); setAllowCompleted(false); }} />
        <div className="space-y-2"><label htmlFor={`${fieldId}-iteration-reason`} className="text-body">{t(($) => $.fields.iterationReason)}</label>
          <Textarea id={`${fieldId}-iteration-reason`} maxLength={2000} value={iterationReason} onChange={(event) => setIterationReason(event.target.value)} />
        </div>
        {needsCompletedConfirmation && <label htmlFor={`${fieldId}-allow-completed`} className="flex min-h-11 items-center gap-2 text-body"><Checkbox id={`${fieldId}-allow-completed`} checked={allowCompleted} onCheckedChange={(checked) => setAllowCompleted(checked === true)} />{t(($) => $.fields.allowCompleted)}</label>}
      </>}
      {needsFields && fields.error && <CollaborationQueryState error={fields.error} retry={() => void fields.refetch()} />}
      {needsSource && !fields.isPending && !fields.error && !sourceOptions && <p role="status">{t(($) => $.fields.optionsUnavailable)}</p>}
    </fieldset>
    {staleBaseline && <div className="space-y-2"><p role="status" className="text-body">{t(($) => $.fields.changed)}</p>
      <Button type="button" variant="outline" disabled={locked} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" onClick={() => {
        setBaseline(issue); setSubmissionId(issue.delivery?.current?.id ?? null); setInformationRequestId(issue.current_information_request?.id ?? null);
        if (action === "return_acceptance" && !recipientWasChosen.current) setRecipient(issue.delivery?.current?.author ?? null);
        if (iterationId) setIterationRevision(sourceOptions?.iterations.find((item) => item.id === iterationId)?.revision);
      }}>{t(($) => $.fields.useCurrentVersion)}</Button></div>}
    <div className="flex flex-wrap gap-2"><Button disabled={!ready} type="submit" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11">{needsResult || deciding || startWork ? formLabel : t(($) => $.workflow.confirm)}</Button><Button disabled={locked} type="button" variant="ghost" onClick={close} className="pointer-coarse:min-h-11">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}

function ResultReferenceFiles({ files, selected, disabled, onChange }: { files: CollaborationAttachment[]; selected: string[]; disabled: boolean; onChange: (ids: string[]) => void }) {
  return <div className="min-w-0 space-y-2">{files.map((file) => <label key={file.id} className="flex min-h-11 min-w-0 items-start gap-3 py-1 text-body [overflow-wrap:anywhere]">
    <Checkbox disabled={disabled || !selected.includes(file.id) && selected.length >= 32} checked={selected.includes(file.id)} onCheckedChange={(checked) => onChange(checked === true ? [...new Set([...selected, file.id])] : selected.filter((id) => id !== file.id))} />{file.name}
  </label>)}</div>;
}
