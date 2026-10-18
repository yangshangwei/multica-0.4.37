import { useId, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationKeys, collaborationRecipientOptions, executeCollaborationAction, prepareCollaborationAction, useCollaborationPendingDrafts, useCollaborationStatusOptions,
  type CollaborationAction, type CollaborationActionIntent, type CollaborationDraft, type CollaborationIssue, type CollaborationScope, type CollaborationStatusAction } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";

type ActionName = CollaborationAction["action"];
const successful = new Set(["participant_added", "already_participant", "transferred", "already_handler", "participant_left", "participant_removed", "information_requested", "information_returned", "acceptance_submitted", "status_changed", "status_unchanged", "completed", "cancelled", "reopened", "issue_edited", "unchanged"]);

export function CollaborationWorkflow({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const [selected, setSelected] = useState<ActionName | null>(null);
  const submitting = useRef(false);
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const apply = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "human-action"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationAction(client, scope, draft, recovery),
    onSuccess: (result) => { if (successful.has(result.code) && (result.status === undefined || result.status < 400)) setSelected(null); } });
  const actions: [ActionName, boolean][] = [["invite", issue.capabilities.invite], ["transfer", issue.capabilities.transfer],
    ["request_information", issue.capabilities.request_information], ["return_information", issue.capabilities.return_information], ["submit_acceptance", issue.capabilities.submit_acceptance],
    ["change_status", issue.capabilities.change_status], ["complete", issue.capabilities.complete], ["cancel", issue.capabilities.cancel], ["reopen", issue.capabilities.reopen],
    ["leave", issue.capabilities.leave], ["edit", issue.capabilities.edit]];
  const locked = apply.isPending || drafts.actions.length > 0 || drafts.error;
  return <section aria-label={t(($) => $.workflow.title)} className="space-y-4">
    <div className="flex flex-wrap gap-2">{actions.filter(([, available]) => available === true && issue.write_context).map(([action]) =>
      <Button key={action} variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => { apply.reset(); setSelected(action); }}>{labels.action(action)}</Button>)}</div>
    {drafts.error && <p role="alert" className="text-body">{t(($) => $.workflow.storageError)}</p>}
    {apply.isPending && <p role="status" className="text-body">{t(($) => $.workflow.pending)}</p>}
    {apply.error && <CollaborationQueryState error={apply.error} />}
    {apply.data && <p role="status" className="text-body">{successful.has(apply.data.code) && (apply.data.status === undefined || apply.data.status < 400) ? t(($) => $.workflow.success) : t(($) => $.workflow.refused)}</p>}
    {!apply.isPending && drafts.actions.length > 0 && <div className="space-y-3"><p role="status" className="text-body">{t(($) => $.workflow.unknown)}</p>{drafts.actions.map((draft) =>
      <Button key={draft.requestId} variant="outline" disabled={apply.isPending} className="pointer-coarse:min-h-11" onClick={() => apply.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button>)}</div>}
    {selected && actions.some(([action, available]) => action === selected && available === true) && <WorkflowForm key={selected} scope={scope} issue={issue} action={selected} locked={locked} close={() => setSelected(null)} submit={async (intent, baseline) => {
      if (submitting.current) return;
      submitting.current = true;
      try {
        const draft = prepareCollaborationAction(scope, baseline, crypto.randomUUID(), intent);
        const result = await apply.mutateAsync({ draft, recovery: false });
        if (successful.has(result.code) && (result.status === undefined || result.status < 400)) setSelected(null);
      } catch { /* The original saved intent is the recovery boundary. */ }
      finally { submitting.current = false; }
    }} />}
  </section>;
}

function WorkflowForm({ scope, issue, action, locked, submit, close }: { scope: CollaborationScope; issue: CollaborationIssue; action: ActionName; locked: boolean; submit: (intent: CollaborationActionIntent, baseline: CollaborationIssue) => Promise<void>; close: () => void }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const [baseline] = useState(issue);
  const [lookup, setLookup] = useState("");
  const [lookupRequest, setLookupRequest] = useState("");
  const [note, setNote] = useState("");
  const [status, setStatus] = useState("");
  const [title, setTitle] = useState(issue.title);
  const [description, setDescription] = useState(issue.description);
  const [startDate, setStartDate] = useState(issue.start_date ?? "");
  const [dueDate, setDueDate] = useState(issue.due_date ?? "");
  const [priority, setPriority] = useState(issue.priority);
  const needsRecipient = action === "invite" || action === "transfer" || action === "request_information";
  const needsStatus = ["change_status", "complete", "cancel", "reopen", "request_information", "return_information", "submit_acceptance"].includes(action);
  const options = useCollaborationStatusOptions(scope, issue.id, (needsStatus ? action : "change_status") as CollaborationStatusAction, needsStatus);
  const resolve = useQuery(collaborationRecipientOptions(client, scope, issue.id, lookupRequest));
  const recipient = resolve.isSuccess && !resolve.isFetching ? resolve.data : null;
  const chosenStatus = status || options.data?.default_status || "";
  const editReady = action !== "edit" || (title.trim() !== "" && ["none", "low", "medium", "high", "urgent"].includes(priority) && issue.start_date !== undefined && issue.due_date !== undefined);
  const ready = !locked && (!needsRecipient || recipient !== null) && (!needsStatus || (options.isSuccess && !options.isFetching && options.data.options.some((item) => item.key === chosenStatus))) && editReady;
  function intent(): CollaborationActionIntent {
    switch (action) {
      case "invite": return { action, targetUserId: recipient!.recipient.user_id };
      case "transfer": return { action, targetUserId: recipient!.recipient.user_id, note };
      case "request_information": return { action, targetUserId: recipient!.recipient.user_id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "return_information": return { action, informationRequestId: baseline.current_information_request!.id, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "submit_acceptance": return { action, note, expectedStatus: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "change_status": case "complete": case "cancel": case "reopen": return { action, status: chosenStatus, statusOptionsVersion: options.data!.status_options_version };
      case "leave": return { action };
      case "edit": return { action, title, description, startDate: startDate || null, dueDate: dueDate || null, priority: priority as "none" | "low" | "medium" | "high" | "urgent" };
      default: throw new Error("Unsupported collaboration action");
    }
  }
  return <form onSubmit={(event) => { event.preventDefault(); if (ready) void submit(intent(), baseline); }} className="min-w-0 space-y-4 rounded-lg border border-border p-4">
    <h3 className="text-body font-semibold">{labels.action(action)}</h3>
    <fieldset disabled={locked} className="min-w-0 space-y-4">
      {needsRecipient && <div className="space-y-2"><label htmlFor={`${fieldId}-recipient`} className="text-body">{t(($) => $.workflow.recipient)}</label>
        <Input id={`${fieldId}-recipient`} value={lookup} placeholder={t(($) => $.workflow.recipientHint)} onChange={(event) => { setLookup(event.target.value); setLookupRequest(""); }} className="pointer-coarse:min-h-11" />
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={resolve.isFetching || !lookup.trim()} onClick={() => { if (lookupRequest === lookup.trim()) void resolve.refetch(); else setLookupRequest(lookup.trim()); }}>{t(($) => $.workflow.resolve)}</Button>
        {recipient && <p role="status" className="text-body [overflow-wrap:anywhere]">{t(($) => $.workflow.confirmRecipient, { name: recipient.recipient.name })}</p>}
        {resolve.error && <CollaborationQueryState error={resolve.error} />}
      </div>}
      {needsStatus && <div className="space-y-2"><label htmlFor={`${fieldId}-status`} className="text-body">{t(($) => $.detail.status)}</label><select id={`${fieldId}-status`} value={chosenStatus} onChange={(event) => setStatus(event.target.value)} className="block min-h-11 max-w-full rounded-md border border-input bg-background px-3 text-body focus-visible:ring-2 focus-visible:ring-ring">
        {(options.data?.options ?? []).map((option) => <option key={option.key} value={option.key}>{option.name}</option>)}</select>{options.error && <CollaborationQueryState error={options.error} />}</div>}
      {["transfer", "request_information", "return_information", "submit_acceptance"].includes(action) && <div className="space-y-2"><label htmlFor={`${fieldId}-note`} className="text-body">{t(($) => $.workflow.note)}</label><Textarea id={`${fieldId}-note`} value={note} onChange={(event) => setNote(event.target.value)} /></div>}
      {action === "edit" && <>
        <div className="space-y-2"><label htmlFor={`${fieldId}-title`}>{t(($) => $.workflow.titleLabel)}</label><Input id={`${fieldId}-title`} value={title} onChange={(event) => setTitle(event.target.value)} /></div>
        <div className="space-y-2"><label htmlFor={`${fieldId}-body`}>{t(($) => $.workflow.descriptionLabel)}</label><Textarea id={`${fieldId}-body`} value={description} onChange={(event) => setDescription(event.target.value)} /></div>
        <div className="flex flex-wrap gap-4"><div className="space-y-2"><label htmlFor={`${fieldId}-start`}>{t(($) => $.workflow.startDate)}</label><Input type="date" id={`${fieldId}-start`} value={startDate} onChange={(event) => setStartDate(event.target.value)} /></div><div className="space-y-2"><label htmlFor={`${fieldId}-due`}>{t(($) => $.workflow.dueDate)}</label><Input type="date" id={`${fieldId}-due`} value={dueDate} onChange={(event) => setDueDate(event.target.value)} /></div></div>
        <div className="space-y-2"><label htmlFor={`${fieldId}-priority`}>{t(($) => $.workflow.priority)}</label><select id={`${fieldId}-priority`} value={priority} onChange={(event) => setPriority(event.target.value)} className="block min-h-11 rounded-md border border-input bg-background px-3">
          {(["none", "low", "medium", "high", "urgent"] as const).map((value) => <option key={value} value={value}>{t(($) => $.priorities[value])}</option>)}
        </select></div>
      </>}
    </fieldset>
    <div className="flex flex-wrap gap-2"><Button disabled={!ready || (action === "return_information" && !issue.current_information_request)} type="submit" className="pointer-coarse:min-h-11">{t(($) => $.workflow.confirm)}</Button><Button disabled={locked} type="button" variant="ghost" onClick={close} className="pointer-coarse:min-h-11">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}
