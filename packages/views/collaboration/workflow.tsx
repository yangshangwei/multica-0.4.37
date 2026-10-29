import { useId, useRef, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationFieldOptions, collaborationPropertyValueMatchesDefinition, collaborationKeys, collaborationRecipientOptions, executeCollaborationAction, parseCollaborationActionDraft, prepareCollaborationAction, useCollaborationPendingDrafts, useCollaborationStatusOptions,
  type CollaborationAction, type CollaborationActionIntent, type CollaborationDraft, type CollaborationIssue, type CollaborationScope, type CollaborationStatusAction } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationParticipants } from "./participants";
import type { IssuePropertyValue } from "@multica/core/types";
import { recoverFocus } from "./focus";
import { CollaborationIterationInput, CollaborationLabelInput, CollaborationPropertyInputs, CollaborationSourceInputs, type CollaborationSourceSelection } from "./field-inputs";

type ActionName = CollaborationAction["action"];
const successful = new Set(["participant_added", "already_participant", "transferred", "already_handler", "participant_left", "participant_removed", "information_requested", "information_returned", "acceptance_submitted", "status_changed", "status_unchanged", "completed", "cancelled", "reopened", "issue_edited", "iteration_changed", "unchanged"]);

export function CollaborationWorkflow({ scope, issue, initialAction, initialStatus }: { scope: CollaborationScope; issue: CollaborationIssue; initialAction?: "transfer" | "change_status" | "edit"; initialStatus?: string }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const [selected, setSelected] = useState<ActionName | null>(initialAction ?? null);
  const [restored, setRestored] = useState<CollaborationAction | null>(null);
  const submitting = useRef(false);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const section = useRef<HTMLElement>(null);
  const dismiss = () => { setSelected(null); recoverFocus(() => trigger.current, () => section.current); };
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const apply = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "human-action"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationAction(client, scope, draft, recovery),
    onSuccess: (result, { draft, recovery }) => {
      if (successful.has(result.code) && (result.status === undefined || result.status < 400)) { dismiss(); return; }
      if (recovery) {
        const original = parseCollaborationActionDraft(draft);
        if (original.action === "edit" || original.action === "edit_source_structure" || original.action === "change_iteration") {
          setRestored(original); setSelected(original.action);
        }
      }
    } });
  const actions: [ActionName, boolean][] = [["invite", issue.capabilities.invite], ["transfer", issue.capabilities.transfer],
    ["request_information", issue.capabilities.request_information], ["return_information", issue.capabilities.return_information], ["submit_acceptance", issue.capabilities.submit_acceptance],
    ["change_status", issue.capabilities.change_status], ["complete", issue.capabilities.complete], ["cancel", issue.capabilities.cancel], ["reopen", issue.capabilities.reopen],
    ["leave", issue.capabilities.leave], ["edit", issue.capabilities.edit],
    ["edit_source_structure", issue.capabilities.edit_source_structure && issue.source_structure !== undefined],
    ["change_iteration", issue.capabilities.edit_source_structure && issue.source_structure !== undefined]];
  const locked = apply.isPending || drafts.actions.length > 0 || drafts.error;
  return <section ref={section} tabIndex={-1} aria-label={t(($) => $.workflow.title)} className="space-y-4 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <div className="flex flex-wrap gap-2">{actions.filter(([, available]) => available === true && issue.write_context).map(([action]) =>
      <Button key={action} variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={(event) => { trigger.current = event.currentTarget; apply.reset(); setRestored(null); setSelected(action); }}>{labels.action(action)}</Button>)}</div>
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
    {selected && issue.write_context && actions.some(([action, available]) => action === selected && available === true) && <WorkflowForm key={`${selected}:${restored?.requestId ?? "new"}`} scope={scope} issue={issue} action={selected} restoredAction={restored?.action === selected ? restored : undefined} initialStatus={selected === "change_status" ? initialStatus : undefined} locked={locked} close={dismiss} submit={async (intent, baseline) => {
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

function WorkflowForm({ scope, issue, action, initialStatus, restoredAction, locked, submit, close }: { scope: CollaborationScope; issue: CollaborationIssue; action: ActionName; initialStatus?: string; restoredAction?: CollaborationAction; locked: boolean; submit: (intent: CollaborationActionIntent, baseline: CollaborationIssue) => Promise<void>; close: () => void }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const fieldId = useId();
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
  const [lookup, setLookup] = useState("");
  const [lookupRequest, setLookupRequest] = useState("");
  const [note, setNote] = useState("");
  const [status, setStatus] = useState(initialStatus ?? "");
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
  const sourceOptions = issue.capabilities.edit_source_structure === true ? fields.data?.source_options : undefined;
  const propertyDefinitions = [...(fields.data?.properties ?? []), ...(baseline.property_definitions ?? []).filter((item) => !fields.data?.properties.some((option) => option.id === item.id)).map((item) => ({ ...item, archived: true }))];
  const propertyValues: Record<string, IssuePropertyValue> = { ...baseline.properties };
  for (const [id, value] of Object.entries(properties)) { if (value === null) delete propertyValues[id]; else propertyValues[id] = value; }
  const invalidProperties = Object.entries(properties).some(([id, value]) => value !== null && !fields.data?.properties.some((definition) => definition.id === id && collaborationPropertyValueMatchesDefinition(value, definition)));
  const invalidLabels = labelIds !== undefined && fields.data !== undefined && labelIds.some((id) => !fields.data!.labels.some((label) => label.id === id));
  const staleBaseline = baseline.collaboration_revision !== issue.collaboration_revision || baseline.lifecycle_epoch !== issue.lifecycle_epoch ||
    baseline.write_context?.participant_epoch !== issue.write_context?.participant_epoch || baseline.write_context?.account_epoch !== issue.write_context?.account_epoch ||
    needsSource && baseline.source_structure?.issue_revision !== issue.source_structure?.issue_revision ||
    action === "change_iteration" && sourceOptions?.iterations.some((item) => item.id === iterationId && item.revision !== iterationRevision) === true;
  const needsRecipient = action === "invite" || action === "transfer" || action === "request_information";
  const needsStatus = ["change_status", "complete", "cancel", "reopen", "request_information", "return_information", "submit_acceptance"].includes(action);
  const options = useCollaborationStatusOptions(scope, issue.id, (needsStatus ? action : "change_status") as CollaborationStatusAction, needsStatus);
  const resolve = useQuery(collaborationRecipientOptions(client, scope, issue.id, lookupRequest));
  const recipient = resolve.isSuccess && !resolve.isFetching ? resolve.data : null;
  const chosenStatus = status || options.data?.default_status || "";
  const editReady = action !== "edit" || (title.trim() !== "" && ["none", "low", "medium", "high", "urgent"].includes(priority) && baseline.start_date !== undefined && baseline.due_date !== undefined && !invalidProperties && !invalidLabels);
  const sourceReady = !needsSource || sourceOptions !== undefined && !fields.isFetching && !fields.isPlaceholderData && (sourceSelection.stage === null || Number.isInteger(sourceSelection.stage) && sourceSelection.stage >= 1 && sourceSelection.stage <= 2147483647);
  const needsCompletedConfirmation = baseline.status_category === "done" && iterationId !== null && iterationId !== baseline.source_structure?.iteration?.id;
  const iterationReady = action !== "change_iteration" || sourceOptions?.iterations_enabled === true && (!iterationId || iterationRevision !== undefined && sourceOptions.iterations.some((item) => item.id === iterationId)) && (!baseline.source_structure?.iteration || baseline.source_structure.iteration.id === iterationId || iterationReason.trim() !== "") && (!needsCompletedConfirmation || allowCompleted);
  const ready = !locked && (!needsRecipient || recipient !== null) && (!needsStatus || (options.isSuccess && !options.isFetching && options.data.options.some((item) => item.key === chosenStatus))) && editReady && sourceReady && iterationReady;
  function intent(): CollaborationActionIntent {
    switch (action) {
      case "invite": return { action, targetUserId: recipient!.recipient.user_id };
      case "transfer": return { action, targetUserId: recipient!.recipient.user_id, note };
      case "request_information": return { action, targetUserId: recipient!.recipient.user_id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "return_information": return { action, informationRequestId: baseline.current_information_request!.id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "submit_acceptance": return { action, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "change_status": case "complete": case "cancel": case "reopen": return { action, status: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "leave": return { action };
      case "edit": return { action, title, description, startDate: startDate || null, dueDate: dueDate || null, priority: priority as "none" | "low" | "medium" | "high" | "urgent", ...(fields.data && labelIds ? { labelIds, properties } : {}) };
      case "edit_source_structure": return { action, ...sourceSelection };
      case "change_iteration": return { action, iterationId, expectedIssueRevision: baseline.source_structure!.issue_revision, ...(iterationId ? { expectedIterationRevision: iterationRevision } : {}), iterationReason, ...(needsCompletedConfirmation ? { allowCompleted } : {}) };
      default: throw new Error("Unsupported collaboration action");
    }
  }
  return <form onSubmit={(event) => { if (event.target !== event.currentTarget) return; event.preventDefault(); if (ready) void submit(intent(), baseline); }} className="min-w-0 space-y-4 rounded-lg border border-border p-4">
    <h3 className="text-body font-semibold">{labels.action(action)}</h3>
    <fieldset disabled={locked} className="min-w-0 space-y-4">
      {needsRecipient && <div className="space-y-2"><label htmlFor={`${fieldId}-recipient`} className="text-body">{t(($) => $.workflow.recipient)}</label>
        <Input id={`${fieldId}-recipient`} value={lookup} placeholder={t(($) => $.workflow.recipientHint)} onChange={(event) => { setLookup(event.target.value); setLookupRequest(""); }} className="pointer-coarse:min-h-11" />
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={resolve.isFetching || !lookup.trim()} onClick={() => { if (lookupRequest === lookup.trim()) void resolve.refetch(); else setLookupRequest(lookup.trim()); }}>{t(($) => $.workflow.resolve)}</Button>
        {recipient && <p role="status" className="text-body [overflow-wrap:anywhere]">{t(($) => $.workflow.confirmRecipient, { name: recipient.recipient.name })}</p>}
        {resolve.error && <CollaborationQueryState error={resolve.error} />}
      </div>}
      {needsStatus && <div className="space-y-2"><label htmlFor={`${fieldId}-status`} className="text-body">{t(($) => $.detail.status)}</label><select id={`${fieldId}-status`} value={chosenStatus} onChange={(event) => setStatus(event.target.value)} className="block min-h-11 max-w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        {chosenStatus && options.data && !options.data.options.some((item) => item.key === chosenStatus) && <option value={chosenStatus}>{t(($) => $.fields.unavailableSelection, { value: chosenStatus })}</option>}
        {(options.data?.options ?? []).map((option) => <option key={option.key} value={option.key}>{option.name}</option>)}</select>{options.error && <CollaborationQueryState error={options.error} />}</div>}
      {["transfer", "request_information", "return_information", "submit_acceptance"].includes(action) && <div className="space-y-2"><label htmlFor={`${fieldId}-note`} className="text-body">{t(($) => $.workflow.note)}</label><Textarea id={`${fieldId}-note`} value={note} onChange={(event) => setNote(event.target.value)} /></div>}
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
      {action === "edit_source_structure" && sourceOptions && <CollaborationSourceInputs options={sourceOptions} current={baseline.source_structure} value={sourceSelection} onChange={setSourceSelection} parentSearch={parentSearch} onParentSearch={setParentSearch} disabled={locked} />}
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
      <Button type="button" variant="outline" disabled={locked} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" onClick={() => { setBaseline(issue); if (iterationId) setIterationRevision(sourceOptions?.iterations.find((item) => item.id === iterationId)?.revision); }}>{t(($) => $.fields.useCurrentVersion)}</Button></div>}
    <div className="flex flex-wrap gap-2"><Button disabled={!ready || (action === "return_information" && !issue.current_information_request)} type="submit" className="pointer-coarse:min-h-11">{t(($) => $.workflow.confirm)}</Button><Button disabled={locked} type="button" variant="ghost" onClick={close} className="pointer-coarse:min-h-11">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}
