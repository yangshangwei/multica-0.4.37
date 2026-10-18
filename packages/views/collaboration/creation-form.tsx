import { useEffect, useId, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCreationOptions, collaborationCreationRecipientOptions, collaborationDeliveryKey, collaborationDeliveryRevision,
  CollaborationCreationRejectedError,
  executeCollaborationCreation, isCollaborationScopeCurrent, prepareCollaborationCreation, readCollaborationDeliveryRecords,
  subscribeCollaborationDelivery, useCollaborationDeliveryForm, type CollaborationCreationDraft, type CollaborationCreationOptions,
  type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@multica/ui/components/ui/dialog";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { CollaborationAttachmentPicker } from "./attachment-picker";

export function CollaborationCreationEntry({ scope, sourceWorkspaceId }: { scope: CollaborationScope; sourceWorkspaceId?: string }) {
  const { t } = useT("collaboration");
  const navigation = useNavigation();
  const [open, setOpen] = useState(false);
  return <Dialog open={open} onOpenChange={setOpen}>
    <DialogTrigger render={<Button className="pointer-coarse:min-h-11" />}>{t(($) => $.creation.entry)}</DialogTrigger>
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl" closeLabel={t(($) => $.creation.close)}>
      <DialogTitle className="pr-8">{t(($) => $.creation.title)}</DialogTitle>
      <DialogDescription>{t(($) => $.creation.help)}</DialogDescription>
      {open && <CollaborationCreationPanel scope={scope} sourceWorkspaceId={sourceWorkspaceId} onCreated={(issue) => {
        setOpen(false); navigation.push(accountPath({ view: "participating", issueId: issue.id }));
      }} />}
    </DialogContent>
  </Dialog>;
}

export function CollaborationCreationPanel({ scope, sourceWorkspaceId, onCreated }: { scope: CollaborationScope; sourceWorkspaceId?: string; onCreated: (issue: CollaborationIssue) => void }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const selectId = useId();
  useSyncExternalStore(subscribeCollaborationDelivery, collaborationDeliveryRevision, () => 0);
  let draftError = false;
  let records: ReturnType<typeof readCollaborationDeliveryRecords> = [];
  try { records = readCollaborationDeliveryRecords(scope); } catch { draftError = true; }
  const previousSource = records.find((record) => record.scope.context.kind === "creation")?.scope.context.sourceWorkspaceId;
  const [selection, setSelection] = useState(sourceWorkspaceId ?? previousSource ?? "");
  const sources = useQuery(collaborationCreationOptions(client, scope));
  const sourceId = selection || (sources.data?.sources.length === 1 ? sources.data.sources[0]!.id : "");
  const recoveries = records.filter((record): record is CollaborationCreationDraft => record.kind === "creation" && record.input === null);
  const recovery = useMutation({ mutationKey: [...collaborationDeliveryKey(scope, { kind: "creation", sourceWorkspaceId: sourceId }), "recover"], retry: false,
    mutationFn: (draft: CollaborationCreationDraft) => executeCollaborationCreation(client, scope, draft, true),
    onSuccess: (issue) => { if (isCollaborationScopeCurrent(scope)) onCreated(issue); } });
  return <div className="min-w-0 space-y-5">
    {sources.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
    {sources.error && <CollaborationQueryState error={sources.error} retry={() => void sources.refetch()} />}
    {draftError && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {recoveries.map((draft) => <div key={draft.requestId} className="space-y-2"><p className="text-caption text-muted-foreground">{t(($) => $.creation.recoveryOnly)}</p>
      <Button type="button" variant="outline" disabled={recovery.isPending} onClick={() => recovery.mutate(draft)}>{t(($) => $.creation.recover)}</Button></div>)}
    {recovery.error && <CollaborationQueryState error={recovery.error} />}
    {sources.data && <>
      <div className="space-y-2"><label htmlFor={selectId} className="block text-body font-medium">{t(($) => $.creation.source)}</label>
        <select id={selectId} value={sourceId} onChange={(event) => setSelection(event.target.value)} className="min-h-11 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">{t(($) => $.creation.chooseSource)}</option>{sources.data.sources.map((source) => <option key={source.id} value={source.id}>{source.name}</option>)}
        </select>
      </div>
      {!sources.data.creation_enabled && <p role="status" className="text-body text-muted-foreground">{t(($) => $.creation.disabled)}</p>}
      {sources.data.sources.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.creation.noSources)}</p>}
      {!draftError && sources.data.sources.some((source) => source.id === sourceId) && <SelectedCreationSource key={`${scope.apiScope}:${scope.userId}:${sourceId}`} scope={scope} sourceId={sourceId} onCreated={onCreated} />}
    </>}
  </div>;
}

function SelectedCreationSource({ scope, sourceId, onCreated }: { scope: CollaborationScope; sourceId: string; onCreated: (issue: CollaborationIssue) => void }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery(collaborationCreationOptions(client, scope, sourceId));
  if (query.isPending) return <p role="status">{t(($) => $.page.loading)}</p>;
  if (query.error || !query.data?.source) return <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />;
  return <CreationForm scope={scope} options={query.data} onCreated={onCreated} />;
}

function CreationForm({ scope, options, onCreated }: { scope: CollaborationScope; options: CollaborationCreationOptions; onCreated: (issue: CollaborationIssue) => void }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const id = useId();
  const sourceId = options.source!.id;
  const context = { kind: "creation" as const, sourceWorkspaceId: sourceId };
  const draft = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId, context }, "creation");
  const { form } = draft;
  const [lookup, setLookup] = useState("");
  const [uploading, setUploading] = useState(false);
  const active = useRef(true);
  useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);
  const recipient = useQuery({ ...collaborationCreationRecipientOptions(client, scope, sourceId, lookup), enabled: lookup !== "" && lookup === form.recipientQuery.trim() && options.creation_enabled });
  const create = useMutation({ mutationKey: [...collaborationDeliveryKey(scope, context), "create"], retry: false, networkMode: "always",
    mutationFn: ({ value, recovery }: { value: CollaborationCreationDraft; recovery: boolean }) => executeCollaborationCreation(client, scope, value, recovery),
    onSuccess: (issue) => { if (active.current && isCollaborationScopeCurrent(scope)) onCreated(issue); } });
  const locked = create.isPending || draft.creations.length > 0 || Boolean(draft.error) || uploading || draft.uploads.length > 0;
  const editable = !locked && options.creation_enabled;
  const recipientUnconfirmed = form.recipientQuery.trim() !== "" && form.recipient === null;
  return <form className="min-w-0 space-y-5" onSubmit={(event) => {
    event.preventDefault(); if (!editable || !form.title.trim() || recipientUnconfirmed) return;
    create.mutate({ value: prepareCollaborationCreation(form, crypto.randomUUID()), recovery: false });
  }}>
    <div className="space-y-2"><label htmlFor={`${id}-title`} className="block text-body font-medium">{t(($) => $.creation.titleLabel)}</label>
      <Input id={`${id}-title`} value={form.title} maxLength={1000} disabled={!editable} onChange={(event) => draft.update({ title: event.target.value })} required className="min-h-11" />
    </div>
    <div className="space-y-2"><label htmlFor={`${id}-description`} className="block text-body font-medium">{t(($) => $.creation.description)}</label>
      <Textarea id={`${id}-description`} value={form.content} maxLength={1_000_000} disabled={!editable} onChange={(event) => draft.update({ content: event.target.value })} className="min-h-28" />
    </div>
    <fieldset className="min-w-0 space-y-3"><legend className="mb-2 text-body font-medium">{t(($) => $.creation.recipient)}</legend>
      <div className="flex flex-wrap items-center gap-3"><p role="status" className="text-body [overflow-wrap:anywhere]">{form.recipient ? t(($) => $.creation.assignedTo, { name: form.recipient.name }) : t(($) => $.creation.assignedSelf)}</p>
        <Button type="button" variant="outline" disabled={!editable} onClick={() => { draft.update({ recipient: null, recipientQuery: "" }); setLookup(""); }}>{t(($) => $.creation.self)}</Button></div>
      <label htmlFor={`${id}-recipient`} className="block text-caption font-medium">{t(($) => $.creation.lookupLabel)}</label>
      <div className="flex flex-wrap gap-2"><Input id={`${id}-recipient`} value={form.recipientQuery} disabled={!editable} className="min-h-11 min-w-0 flex-1" onChange={(event) => { draft.update({ recipientQuery: event.target.value, recipient: null }); setLookup(""); }} />
        <Button type="button" variant="outline" disabled={!editable || !form.recipientQuery.trim() || recipient.isFetching} onClick={() => { setLookup(form.recipientQuery.trim()); if (lookup === form.recipientQuery.trim()) void recipient.refetch(); }}>{t(($) => $.creation.lookup)}</Button></div>
      {recipient.error && lookup && <CollaborationQueryState error={recipient.error} retry={() => void recipient.refetch()} />}
      {recipient.data && lookup === form.recipientQuery.trim() && <div className="flex flex-wrap items-center gap-3 text-body [overflow-wrap:anywhere]"><span>{recipient.data.recipient.name}</span>
        <Button type="button" variant="outline" disabled={!editable} onClick={() => draft.update({ recipient: recipient.data!.recipient })}>{t(($) => $.creation.useRecipient)}</Button></div>}
      {recipientUnconfirmed && <p className="text-caption text-muted-foreground">{t(($) => $.creation.confirmRecipient)}</p>}
    </fieldset>
    <CollaborationAttachmentPicker scope={scope} form={form} update={draft.update} uploads={draft.uploads} disabled={create.isPending || draft.creations.length > 0 || Boolean(draft.error) || !options.creation_enabled} onBusyChange={setUploading} />
    {Boolean(draft.error) && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {create.error instanceof CollaborationCreationRejectedError && <p role="status" className="text-body text-muted-foreground">{t(($) => $.creation.rejected)}</p>}
    {create.error && !(create.error instanceof CollaborationCreationRejectedError) && <p role="alert" className="text-body text-muted-foreground">{t(($) => $.creation.error)}</p>}
    {create.isPending && <p role="status">{t(($) => $.creation.submitting)}</p>}
    {draft.creations.map((value) => <div key={value.requestId} className="space-y-3"><p role="status" className="text-body text-muted-foreground">{t(($) => $.creation.unknown)}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={create.isPending} onClick={() => create.mutate({ value, recovery: true })}>{t(($) => $.creation.recover)}</Button>
        <Button type="button" variant="outline" disabled={create.isPending || !options.creation_enabled} onClick={() => create.mutate({ value, recovery: false })}>{t(($) => $.creation.retry)}</Button></div>
    </div>)}
    <div className="flex flex-wrap items-center justify-between gap-3"><p className="text-caption text-muted-foreground">{t(($) => $.creation.saved)}</p>
      <Button type="submit" disabled={!editable || !form.title.trim() || recipientUnconfirmed} className="pointer-coarse:min-h-11">{t(($) => $.creation.submit)}</Button></div>
  </form>;
}
