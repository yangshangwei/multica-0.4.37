import { useEffect, useId, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { ChevronRight, Maximize2, Minimize2, XIcon } from "lucide-react";
import { keepPreviousData, useIsMutating, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCreationOptions, collaborationDeliveryKey, collaborationDeliveryRevision, collaborationKeys,
  CollaborationCreationRejectedError,
  CollaborationCreationFieldsSchema, collaborationPropertyValueMatchesDefinition,
  executeCollaborationCreation, isCollaborationDenied, isCollaborationScopeCurrent, prepareCollaborationCreation, readCollaborationDeliveryRecords,
  subscribeCollaborationDelivery, useCollaborationDeliveryForm, type CollaborationCreationDraft, type CollaborationCreationOptions,
  type CollaborationCreationFields, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import type { IssuePropertyValue } from "@multica/core/types";
import { accountPath } from "@multica/core/paths";
import { isImeComposing } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@multica/ui/components/ui/dialog";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { CollaborationAccountPicker } from "./account-picker";
import { CollaborationAttachmentPicker } from "./attachment-picker";
import { CollaborationIterationInput, CollaborationLabelInput, CollaborationPropertyInputs, CollaborationSourceInputs, collaborationSelectClass } from "./field-inputs";
import { useCollaborationLabels } from "./labels";
import { collaborationFocusWithinClass } from "./form-styles";
import { recoverFocus } from "./focus";
import { CollaborationCreationDescriptionAssist } from "./creation-description-assist";

export function CollaborationCreationEntry({ scope, sourceWorkspaceId }: { scope: CollaborationScope; sourceWorkspaceId?: string }) {
  const { t } = useT("collaboration");
  const navigation = useNavigation();
  const [open, setOpen] = useState(false);
  const [size, setSize] = useState<"default" | "assist" | "expanded">("default");
  const explicitlySized = useRef(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLDivElement>(null);
  const expanded = size === "expanded";
  const resizeLabel = expanded ? t(($) => $.creation.collapse) : t(($) => $.creation.expand);
  const headerActions = <div className="flex shrink-0 items-center gap-1">
    <Button type="button" variant="ghost" size="icon-sm" className="max-sm:size-11 pointer-coarse:size-11"
      aria-label={resizeLabel} title={resizeLabel} aria-pressed={expanded}
      onClick={() => { explicitlySized.current = true; setSize(expanded ? "default" : "expanded"); }}>
      {expanded ? <Minimize2 aria-hidden className="size-4" /> : <Maximize2 aria-hidden className="size-4" />}
    </Button>
    <DialogClose render={<Button variant="ghost" size="icon-sm" className="max-sm:size-11 pointer-coarse:size-11" />}
      aria-label={t(($) => $.creation.close)} title={t(($) => $.creation.close)}><XIcon aria-hidden className="size-4" /></DialogClose>
  </div>;
  return <Dialog open={open} onOpenChange={(next) => {
    if (next) { setSize("default"); explicitlySized.current = false; }
    setOpen(next);
  }}>
    <DialogTrigger ref={trigger} render={<Button className="pointer-coarse:min-h-11 focus-visible:ring-faint-foreground focus-visible:border-faint-foreground" />}>{t(($) => $.creation.entry)}</DialogTrigger>
    <DialogContent ref={dialog} showCloseButton={false}
      initialFocus={() => {
        if (document.activeElement === trigger.current) dialog.current?.focus({ preventScroll: true });
        // Base UI otherwise queues an unconditional focus frame. Recover only
        // while focus is still lost, so an intervening user interaction wins.
        recoverFocus(() => {
          const popup = dialog.current;
          if (!popup?.hasAttribute("data-open")) return null;
          return popup.querySelector<HTMLElement>('[data-creation-title]:not(:disabled)')
            ?? popup.querySelector<HTMLElement>('[data-creation-source-empty]:not(:disabled)') ?? popup;
        });
        return false;
      }}
      className={`flex w-[calc(100vw-1.5rem)] max-w-[calc(100vw-1.5rem)] flex-col gap-0 overflow-hidden bg-background p-0 motion-reduce:animate-none ${expanded ? "h-[calc(100dvh-1.5rem)] max-h-[calc(100dvh-1.5rem)] sm:max-w-4xl" : `${size === "assist" ? "h-[min(600px,90dvh)]" : "h-[min(34rem,90dvh)]"} max-h-[90dvh] sm:max-w-2xl`} ${collaborationFocusWithinClass}`} closeLabel={t(($) => $.creation.close)}>
      <DialogTitle className="sr-only">{t(($) => $.creation.title)}</DialogTitle>
      <DialogDescription className="sr-only">{t(($) => $.creation.help)}</DialogDescription>
      {open && <CollaborationCreationPanel scope={scope} sourceWorkspaceId={sourceWorkspaceId} constrained headerActions={headerActions}
        onNeedsSpace={() => { if (!explicitlySized.current) setSize((current) => current === "default" ? "assist" : current); }} onCancel={() => setOpen(false)} onCreated={(issue) => {
        setOpen(false); navigation.push(accountPath({ view: "participating", issueId: issue.id }));
      }} />}
    </DialogContent>
  </Dialog>;
}

export function CollaborationCreationPanel({ scope, sourceWorkspaceId, onCreated, constrained = false, onCancel, headerActions, onNeedsSpace }: {
  scope: CollaborationScope; sourceWorkspaceId?: string; onCreated: (issue: CollaborationIssue) => void; constrained?: boolean; onCancel?: () => void; headerActions?: ReactNode; onNeedsSpace?: () => void;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const selectId = useId();
  const sourceSelect = useRef<HTMLSelectElement>(null);
  useSyncExternalStore(subscribeCollaborationDelivery, collaborationDeliveryRevision, () => 0);
  let draftError = false;
  let records: ReturnType<typeof readCollaborationDeliveryRecords> = [];
  try { records = readCollaborationDeliveryRecords(scope); } catch { draftError = true; }
  const previousSource = records.find((record) => record.scope.context.kind === "creation")?.scope.context.sourceWorkspaceId;
  const pendingSource = records.find((record) => record.scope.context.kind === "creation" && (record.kind === "upload" || record.kind === "creation" && record.input !== null))?.scope.context.sourceWorkspaceId;
  const [selection, setSelection] = useState(pendingSource ?? sourceWorkspaceId ?? previousSource ?? "");
  const sources = useQuery(collaborationCreationOptions(client, scope));
  const sourceId = selection || (sources.data?.sources.length === 1 ? sources.data.sources[0]!.id : "");
  useEffect(() => {
    if (constrained && !sourceId) recoverFocus(() => sourceSelect.current);
  }, [constrained, sourceId, sources.data]);
  const creationKey = [...collaborationKeys.scope(scope), "creation"];
  // Core retires records before onSuccess. Keep the owning form mounted through
  // that interval, as well as while a contentful request awaits confirmation.
  const processing = useIsMutating({ mutationKey: creationKey, predicate: (mutation) => {
    const key = mutation.options.mutationKey;
    return key?.length === creationKey.length + 2 && (key[key.length - 1] === "create" || key[key.length - 1] === "recover");
  } }) > 0;
  const sourceLocked = processing || Boolean(pendingSource);
  const recoveries = records.filter((record): record is CollaborationCreationDraft => record.kind === "creation" && record.input === null);
  const recovery = useMutation({ mutationKey: [...collaborationDeliveryKey(scope, { kind: "creation", sourceWorkspaceId: sourceId }), "recover"], retry: false,
    mutationFn: (draft: CollaborationCreationDraft) => executeCollaborationCreation(client, scope, draft, true),
    onSuccess: (issue) => { if (isCollaborationScopeCurrent(scope)) onCreated(issue); } });
  const notices = <>
    {sources.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
    {sources.error && <CollaborationQueryState error={sources.error} retry={() => void sources.refetch()} />}
    {draftError && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {recoveries.map((draft) => <div key={draft.requestId} className="space-y-2"><p className="text-caption text-muted-foreground">{t(($) => $.creation.recoveryOnly)}</p>
      <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={recovery.isPending} onClick={() => recovery.mutate(draft)}>{t(($) => $.creation.recover)}</Button></div>)}
    {recovery.error && <CollaborationQueryState error={recovery.error} />}
    {sources.data && <>
      {!sources.data.creation_enabled && <p role="status" className="text-body text-muted-foreground">{t(($) => $.creation.disabled)}</p>}
      {sources.data.sources.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.creation.noSources)}</p>}
    </>}
  </>;
  const availableSource = !draftError && sources.data?.sources.some((source) => source.id === sourceId);
  return <div className={`min-w-0 ${constrained ? "flex min-h-0 flex-1 flex-col" : "space-y-5"} ${collaborationFocusWithinClass}`}>
    {!constrained && notices}
    <div className={constrained ? "flex shrink-0 items-start gap-2 px-5 pb-3 pt-3" : undefined}>
      <div className={constrained ? "flex min-h-8 min-w-0 flex-1 flex-wrap items-center gap-1.5 text-caption max-sm:min-h-11 pointer-coarse:min-h-11" : "space-y-2"}>
        {sources.data && <>
          <label htmlFor={selectId} className={constrained ? "sr-only" : "block text-body font-medium"}>{t(($) => $.creation.source)}</label>
          <select ref={sourceSelect} data-creation-source-empty={!sourceId ? "" : undefined} id={selectId} value={sourceId} disabled={sourceLocked} onChange={(event) => { if (!sourceLocked) setSelection(event.target.value); }}
            className={`min-w-0 rounded-md bg-background text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${constrained ? "min-h-8 min-w-32 max-w-full flex-1 basis-32 py-1 pr-1 text-caption sm:max-w-64 pointer-coarse:min-h-11" : "min-h-11 w-full border border-input px-3"}`}>
            <option value="">{t(($) => $.creation.chooseSource)}</option>{sources.data.sources.map((source) => <option key={source.id} value={source.id}>{source.name}</option>)}
          </select>
        </>}
        {constrained && <span className="flex min-w-0 items-center gap-1.5 font-medium"><ChevronRight aria-hidden className="size-3 shrink-0 text-faint-foreground" /><span>{t(($) => $.creation.title)}</span></span>}
      </div>
      {headerActions}
    </div>
    {availableSource ? <SelectedCreationSource key={`${scope.apiScope}:${scope.userId}:${sourceId}`} scope={scope} sourceId={sourceId} onCreated={onCreated}
      constrained={constrained} onCancel={onCancel} onNeedsSpace={onNeedsSpace} notices={constrained ? notices : null} />
      : constrained && <CreationContents constrained actions={onCancel && <CancelCreation onCancel={onCancel} />}>{notices}</CreationContents>}
  </div>;
}

function CreationContents({ constrained, children, actions }: { constrained: boolean; children: ReactNode; actions?: ReactNode }) {
  return <>
    <div data-collaboration-creation-body className={`min-w-0 space-y-4 ${constrained ? "min-h-0 flex-1 overflow-y-auto overscroll-contain scroll-py-1 px-5 pb-4 pt-1 [&_:is(input,textarea,select,button,summary)]:scroll-my-1" : ""}`}
      onFocusCapture={constrained ? (event) => {
        const body = event.currentTarget;
        const target = event.target;
        // React focus also bubbles from portals; only reveal actual body controls.
        if (!(target instanceof HTMLElement) || !body.contains(target) || !target.matches("input,textarea,select,button,summary")) return;
        const visible = body.getBoundingClientRect();
        const focused = target.getBoundingClientRect();
        if (body.clientHeight <= 0 || visible.height <= 0 || focused.height <= 0) return;
        if (focused.top < visible.top + 4 || focused.bottom > visible.bottom - 4) target.scrollIntoView({ block: "nearest" });
      } : undefined}>{children}</div>
    {actions && <div data-collaboration-creation-actions className={`flex flex-wrap items-center justify-between gap-2 ${constrained ? "shrink-0 border-t px-5 py-3" : ""}`}>{actions}</div>}
  </>;
}

function CancelCreation({ onCancel }: { onCancel: () => void }) {
  const { t } = useT("collaboration");
  return <Button type="button" variant="outline" className="pointer-coarse:min-h-11" onClick={onCancel}>{t(($) => $.creation.cancel)}</Button>;
}

function SelectedCreationSource({ scope, sourceId, onCreated, constrained, onCancel, notices, onNeedsSpace }: {
  scope: CollaborationScope; sourceId: string; onCreated: (issue: CollaborationIssue) => void; constrained: boolean; onCancel?: () => void; notices: ReactNode; onNeedsSpace?: () => void;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [parentSearch, setParentSearch] = useState("");
  const [parentQuery, setParentQuery] = useState("");
  useEffect(() => {
    const timer = window.setTimeout(() => setParentQuery(parentSearch.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [parentSearch]);
  const query = useQuery({ ...collaborationCreationOptions(client, scope, sourceId, parentQuery), placeholderData: keepPreviousData });
  // Reuse authorized Query data after a transient search failure, never after
  // source access loss, so retry keeps this form mounted with its input intact.
  const options = isCollaborationDenied(query.error) ? undefined : query.data
    ?? client.getQueryData(collaborationCreationOptions(client, scope, sourceId).queryKey);
  if (!options?.source) return <div className={constrained ? "flex min-h-0 flex-1 flex-col" : undefined}>
    <CreationContents constrained={constrained} actions={onCancel && <CancelCreation onCancel={onCancel} />}>{notices}
      {query.isPending && !options ? <p role="status">{t(($) => $.page.loading)}</p> : <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    </CreationContents>
  </div>;
  return <CreationForm scope={scope} options={options} onCreated={onCreated} parentSearch={parentSearch} onParentSearch={setParentSearch}
    optionsPending={parentSearch.trim() !== parentQuery || query.isFetching || query.isPlaceholderData} optionsError={query.error} onRetryOptions={() => void query.refetch()}
    sourceAccessCurrent={() => !isCollaborationDenied(client.getQueryState(collaborationCreationOptions(client, scope, sourceId, parentQuery).queryKey)?.error)}
    constrained={constrained} onCancel={onCancel} notices={notices} onNeedsSpace={onNeedsSpace} />;
}

function CreationForm({ scope, options, onCreated, parentSearch, onParentSearch, optionsPending, optionsError, onRetryOptions, sourceAccessCurrent, constrained, onCancel, notices, onNeedsSpace }: {
  scope: CollaborationScope; options: CollaborationCreationOptions; onCreated: (issue: CollaborationIssue) => void;
  parentSearch: string; onParentSearch: (search: string) => void; optionsPending: boolean; optionsError: unknown; onRetryOptions: () => void; sourceAccessCurrent: () => boolean;
  constrained: boolean; onCancel?: () => void; notices: ReactNode; onNeedsSpace?: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const id = useId();
  const titleInput = useRef<HTMLInputElement>(null);
  useEffect(() => { if (constrained) recoverFocus(() => titleInput.current); }, [constrained]);
  const sourceId = options.source!.id;
  const context = { kind: "creation" as const, sourceWorkspaceId: sourceId };
  const draft = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId, context }, "creation");
  const { form } = draft;
  const descriptionInput = useRef<HTMLTextAreaElement>(null);
  const [uploading, setUploading] = useState(false);
  const active = useRef(true);
  useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);
  const create = useMutation({ mutationKey: [...collaborationDeliveryKey(scope, context), "create"], retry: false, networkMode: "always",
    mutationFn: ({ value, recovery }: { value: CollaborationCreationDraft; recovery: boolean }) => executeCollaborationCreation(client, scope, value, recovery),
    onSuccess: (issue) => { if (active.current && isCollaborationScopeCurrent(scope)) onCreated(issue); } });
  const feedback = useRef<HTMLDivElement>(null);
  // Reveal a settled mutation outcome, without query refreshes or edits taking
  // over the user's scroll position or changing keyboard focus.
  useEffect(() => {
    if (!constrained || !create.isError || !active.current) return;
    const currentScope = { server: scope.server, apiScope: scope.apiScope, userId: scope.userId };
    if (isCollaborationScopeCurrent(currentScope)) feedback.current?.scrollIntoView({ block: "nearest" });
  }, [constrained, create.isError, create.error, scope.server, scope.apiScope, scope.userId]);
  const locked = create.isPending || draft.creations.length > 0 || Boolean(draft.error) || uploading || draft.uploads.length > 0;
  const editable = !locked && options.creation_enabled;
  const assistCurrent = () => {
    if (!active.current || !isCollaborationScopeCurrent(scope)) return false;
    if (isCollaborationDenied(optionsError) || !sourceAccessCurrent()) return false;
    try {
      const forms = readCollaborationDeliveryRecords(form.scope).filter((record) => record.kind === "form" && record.purpose === "creation");
      // A new empty draft is not persisted until its first edit. Nonempty
      // requests must still own a stored record, so retirement fences late AI.
      return forms.some((record) => record.draftId === form.draftId) || forms.length === 0 && !form.content && !form.title;
    } catch { return false; }
  };
  const assistBlocked = () => {
    if (!editable || !assistCurrent()) return true;
    try { return readCollaborationDeliveryRecords(form.scope).some((record) => record.draftId === form.draftId && (record.kind === "upload" || record.kind === "creation")); }
    catch { return true; }
  };
  const pendingRequest = draft.creations.find((value) => value.requestId === create.variables?.value.requestId);
  const initial = form.initialFields ?? {};
  const fields = options.fields;
  const updateFields = (value: Partial<CollaborationCreationFields>) => draft.update({ initialFields: { ...initial, ...value } });
  const updateProperty = (propertyId: string, value: IssuePropertyValue | undefined) => {
    const next = { ...initial.properties };
    if (value === undefined) delete next[propertyId]; else next[propertyId] = value;
    updateFields({ properties: next });
  };
  const properties: Record<string, IssuePropertyValue> = {};
  for (const [key, value] of Object.entries(initial.properties ?? {})) if (value !== null) properties[key] = value;
  const initialValid = CollaborationCreationFieldsSchema.safeParse(initial).success && (fields === undefined ? Object.keys(initial).length === 0 :
    (!initial.status || fields.statuses.some((item) => item.key === initial.status)) &&
    (initial.label_ids ?? []).every((id) => fields.labels.some((item) => item.id === id)) &&
    Object.entries(properties).every(([id, value]) => fields.properties.some((definition) => definition.id === id && collaborationPropertyValueMatchesDefinition(value, definition))) &&
    (!initial.current_iteration_id || fields.source_options.iterations_enabled && fields.source_options.iterations.some((item) => item.id === initial.current_iteration_id)));
  const iterationChanged = initial.current_iteration_id && fields?.source_options.iterations.some((item) => item.id === initial.current_iteration_id && item.revision !== initial.expected_iteration_revision);
  const initialSection = useRef<HTMLDetailsElement>(null);
  useEffect(() => { if (!initialValid && initialSection.current) initialSection.current.open = true; }, [initialValid, fields]);
  const configuredCount = [
    initial.status && initial.status !== fields?.default_status,
    initial.priority && initial.priority !== "none",
    initial.start_date, initial.due_date, initial.label_ids?.length,
    initial.project_id, initial.parent_issue_id, initial.stage, initial.current_iteration_id,
  ].filter(Boolean).length + Object.keys(properties).length;
  const submitHint = draft.uploads.length > 0 && !uploading ? t(($) => $.creation.uploadRecoveryHint)
    : editable && initialValid && !form.title.trim() ? t(($) => $.creation.titleRequired) : null;
  return <form className={`min-w-0 ${constrained ? "flex min-h-0 flex-1 flex-col" : "space-y-5"}`}
    onKeyDownCapture={(event) => { if (event.key === "Enter" && isImeComposing(event)) event.preventDefault(); }}
    onSubmit={(event) => {
    if (event.target !== event.currentTarget) return;
    event.preventDefault(); if (!editable || !form.title.trim() || !initialValid) return;
    create.mutate({ value: prepareCollaborationCreation(form, crypto.randomUUID()), recovery: false });
  }}>
    <CollaborationAttachmentPicker scope={scope} form={form} update={draft.update} uploads={draft.uploads}
      disabled={create.isPending || draft.creations.length > 0 || Boolean(draft.error) || !options.creation_enabled} onBusyChange={setUploading}
      renderComposer={({ trigger, details }) => <CreationContents constrained={constrained} actions={<>
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 max-sm:w-full">
          {trigger}
          <p className="text-caption text-muted-foreground">{t(($) => $.creation.saved)}</p>
        </div>
        <div className="flex min-w-0 flex-wrap items-center justify-end gap-1.5 max-sm:w-full">{onCancel && <CancelCreation onCancel={onCancel} />}
          <Button type="submit" disabled={!editable || !form.title.trim() || !initialValid} aria-describedby={submitHint ? `${id}-submit-hint` : undefined}
            aria-busy={create.isPending || undefined} className="max-w-full whitespace-normal text-caption pointer-coarse:min-h-11">{t(($) => $.creation.submit)}</Button>
        </div>
      </>}>
    {notices}
    <div className="space-y-1">
      <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 text-caption text-muted-foreground">
        <label htmlFor={`${id}-title`}>{t(($) => $.creation.titleLabel)}</label>
        {submitHint && <p id={`${id}-submit-hint`}>{submitHint}</p>}
      </div>
      <Input ref={titleInput} data-creation-title id={`${id}-title`} value={form.title} maxLength={1000} disabled={!editable} onChange={(event) => draft.update({ title: event.target.value })} required aria-describedby={submitHint ? `${id}-submit-hint` : undefined} placeholder={t(($) => $.creation.titlePlaceholder)} className="min-h-11 rounded-none border-0 border-b-2 border-transparent bg-transparent px-0 text-title-lg font-semibold shadow-none focus-visible:border-faint-foreground focus-visible:ring-0 md:text-title-lg dark:bg-transparent" />
    </div>
    <div className="space-y-1"><label htmlFor={`${id}-description`} className="block text-caption text-muted-foreground">{t(($) => $.creation.description)}</label>
      <Textarea ref={descriptionInput} id={`${id}-description`} value={form.content} maxLength={1_000_000} disabled={!editable} onChange={(event) => draft.update({ content: event.target.value })} placeholder={t(($) => $.creation.descriptionPlaceholder)} className="min-h-16 resize-none rounded-sm border-0 bg-transparent px-0 text-body shadow-none focus-visible:ring-2 focus-visible:ring-inset dark:bg-transparent" />
      <CollaborationCreationDescriptionAssist key={form.draftId} textareaRef={descriptionInput} sourceId={sourceId} value={form.content} title={form.title}
        uploading={uploading || draft.uploads.length > 0} submitting={create.isPending || draft.creations.length > 0}
        isBlocked={assistBlocked} isCurrent={assistCurrent} onChange={(content) => draft.update({ content })} onNeedsSpace={onNeedsSpace} />
    </div>
    <div className="grid min-w-0 gap-x-4 gap-y-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
      <p className="text-caption text-muted-foreground sm:col-span-2">{t(($) => $.creation.help)}</p>
      <fieldset className="min-w-0 space-y-1"><legend className="mb-1 text-caption text-muted-foreground">{t(($) => $.creation.recipient)}</legend>
        <CollaborationAccountPicker scope={scope} context={context} selected={form.recipient} allowSelf compact selfName={t(($) => $.creation.self)} label={t(($) => $.creation.recipient)} disabled={!editable}
          onSelect={(account) => draft.update({ recipient: account ? { ...account, external: account.external === true } : null, recipientQuery: "" })} />
        <p role="status" className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{form.recipient ? t(($) => $.creation.assignedTo, { name: form.recipient.name }) : t(($) => $.creation.assignedSelf)}</p>
      </fieldset>
      <div className="min-w-0 space-y-1"><label htmlFor={`${id}-policy`} className="block text-caption text-muted-foreground">{t(($) => $.policy.title)}</label>
        <select id={`${id}-policy`} value={form.aiPolicy ?? "assisted"} disabled={!editable}
          className="min-h-11 w-full min-w-0 rounded-md bg-muted/50 px-3 py-2 text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
          aria-describedby={`${id}-policy-help`} onChange={(event) => draft.update({ aiPolicy: event.target.value === "manual" ? "manual" : "assisted" })}>
          <option value="manual">{t(($) => $.policy.manual)}</option><option value="assisted">{t(($) => $.policy.assisted)}</option>
        </select>
        <p id={`${id}-policy-help`} className="text-caption text-muted-foreground">{t(($) => $.policy.creationHint)}</p>
      </div>
    </div>
    {details}
    {fields && <details ref={initialSection} className="min-w-0 space-y-4"><summary className="cursor-pointer py-2 text-body font-medium pointer-coarse:min-h-11">{t(($) => $.fields.initial)}{configuredCount > 0 && <span className="ml-2 text-caption font-normal text-muted-foreground">{t(($) => $.fields.configured, { count: configuredCount })}</span>}</summary>
      <fieldset disabled={!editable} className="min-w-0 space-y-5">
        <div className="grid min-w-0 gap-4 sm:grid-cols-2">
          <div className="space-y-2"><label htmlFor={`${id}-status`} className="text-body">{t(($) => $.detail.status)}</label>
            <select id={`${id}-status`} value={initial.status ?? fields.default_status} className={collaborationSelectClass} onChange={(event) => updateFields({ status: event.target.value })}>
              {initial.status && !fields.statuses.some((item) => item.key === initial.status) && <option value={initial.status}>{t(($) => $.fields.unavailableSelection, { value: initial.status })}</option>}
              {fields.statuses.map((status) => <option key={status.key} value={status.key}>{labels.status(status.key) === status.key ? status.name : labels.status(status.key)}</option>)}
            </select>
          </div>
          <div className="space-y-2"><label htmlFor={`${id}-priority`} className="text-body">{t(($) => $.workflow.priority)}</label>
            <select id={`${id}-priority`} value={initial.priority ?? "none"} className={collaborationSelectClass} onChange={(event) => updateFields({ priority: event.target.value as CollaborationCreationFields["priority"] })}>
              {(["none", "low", "medium", "high", "urgent"] as const).map((value) => <option key={value} value={value}>{t(($) => $.priorities[value])}</option>)}
            </select>
          </div>
          <div className="space-y-2"><label htmlFor={`${id}-start`} className="text-body">{t(($) => $.workflow.startDate)}</label><Input id={`${id}-start`} type="date" value={initial.start_date ?? ""} onChange={(event) => updateFields({ start_date: event.target.value || null })} className="pointer-coarse:min-h-11" /></div>
          <div className="space-y-2"><label htmlFor={`${id}-due`} className="text-body">{t(($) => $.workflow.dueDate)}</label><Input id={`${id}-due`} type="date" value={initial.due_date ?? ""} onChange={(event) => updateFields({ due_date: event.target.value || null })} className="pointer-coarse:min-h-11" /></div>
        </div>
        <CollaborationLabelInput labels={fields.labels} selectedIds={initial.label_ids ?? []} onChange={(label_ids) => updateFields({ label_ids })} disabled={!editable} />
        <CollaborationPropertyInputs scope={scope} sourceId={sourceId} definitions={fields.properties} values={properties} disabled={!editable}
          onChange={updateProperty} />
        {Object.keys(properties).filter((propertyId) => !fields.properties.some((definition) => definition.id === propertyId)).map((propertyId) => <div key={propertyId} className="flex min-w-0 flex-wrap items-center gap-2">
          <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.fields.unavailableSelection, { value: propertyId })}</p>
          <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={!editable} onClick={() => updateProperty(propertyId, undefined)}>{t(($) => $.fields.clear)}</Button>
        </div>)}
        <CollaborationSourceInputs options={fields.source_options} value={{ projectId: initial.project_id ?? null, parentIssueId: initial.parent_issue_id ?? null, stage: initial.stage ?? null }}
          onChange={(value) => updateFields({ project_id: value.projectId, parent_issue_id: value.parentIssueId, stage: value.stage })} parentSearch={parentSearch} onParentSearch={onParentSearch}
          parentsPending={optionsPending} parentsError={optionsError} onRetryParents={onRetryOptions} disabled={!editable} />
        {initial.parent_issue_id && !initial.project_id && <p className="text-caption text-muted-foreground">{t(($) => $.fields.parentProjectHint)}</p>}
        <CollaborationIterationInput options={fields.source_options} selectedId={initial.current_iteration_id ?? null} disabled={!editable || optionsPending} allowClearWhenDisabled
          onChange={(iterationId) => updateFields({ current_iteration_id: iterationId, expected_iteration_revision: fields.source_options.iterations.find((period) => period.id === iterationId)?.revision })} />
        {iterationChanged && <div className="space-y-2"><p role="status" className="text-caption">{t(($) => $.fields.changed)}</p><Button type="button" variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={!editable || optionsPending}
          onClick={() => updateFields({ expected_iteration_revision: fields.source_options.iterations.find((period) => period.id === initial.current_iteration_id)?.revision })}>{t(($) => $.fields.useCurrentVersion)}</Button></div>}
      </fieldset>
    </details>}
    {!initialValid && <div className="space-y-2"><p role="alert" className="text-body">{t(($) => $.fields.invalidSelection)}</p>
      {!fields && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={!editable} onClick={() => draft.update({ initialFields: {} })}>{t(($) => $.fields.clearInitial)}</Button>}</div>}
    {Boolean(draft.error) && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {(create.isPending || Boolean(create.error) || draft.creations.length > 0) && <div className="space-y-5">
    {create.error instanceof CollaborationCreationRejectedError && <div ref={feedback} role="status" className="space-y-2 text-body text-muted-foreground">
      <p>{t(($) => $.creation.rejected)}</p><p>{labels.creationRejection(create.error.code)}</p>
    </div>}
    {create.error && !(create.error instanceof CollaborationCreationRejectedError) && <div ref={pendingRequest ? undefined : feedback}>
      <p role="alert" className="text-body text-muted-foreground">{t(($) => $.creation.error)}</p>
    </div>}
    {create.isPending && <p role="status">{t(($) => $.creation.submitting)}</p>}
    {draft.creations.map((value) => <div key={value.requestId} className="space-y-3"><p role="status" className="text-body text-muted-foreground">{t(($) => $.creation.unknown)}</p>
      <div ref={value.requestId === pendingRequest?.requestId ? feedback : undefined} className="flex flex-wrap gap-2"><Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={create.isPending} onClick={() => create.mutate({ value, recovery: true })}>{t(($) => $.creation.recover)}</Button>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={create.isPending || !options.creation_enabled} onClick={() => create.mutate({ value, recovery: false })}>{t(($) => $.creation.retry)}</Button></div>
    </div>)}
    </div>}
    </CreationContents>} />
  </form>;
}
