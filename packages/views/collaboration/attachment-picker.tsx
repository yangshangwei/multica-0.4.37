import { useEffect, useId, useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationDeliveryKey, collaborationDraftAttachmentsOptions, CollaborationAttachmentError, executeCollaborationUpload,
  stageCollaborationUpload, type CollaborationDeliveryForm, type CollaborationScope, type CollaborationUploadDraft } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationAttachmentFailure({ error }: { error: unknown }) {
  const { t } = useT("collaboration");
  if (!(error instanceof CollaborationAttachmentError)) return <CollaborationQueryState error={error} />;
  const message = error.code === "attachment_too_large" ? t(($) => $.attachments.tooLarge)
    : error.code === "attachment_file_changed" ? t(($) => $.attachments.changed)
    : error.code === "attachment_limit" ? t(($) => $.attachments.limit)
    : error.code === "preview_too_large" ? t(($) => $.attachments.previewTooLarge) : t(($) => $.attachments.error);
  return <p role="alert" className="text-body text-muted-foreground">{message}</p>;
}

export function CollaborationAttachmentPicker({ scope, form, update, uploads, disabled = false, onBusyChange, onChange }: {
  scope: CollaborationScope; form: CollaborationDeliveryForm;
  update: (fields: Partial<Pick<CollaborationDeliveryForm, "selectedAttachmentIds">>) => void;
  uploads: CollaborationUploadDraft[]; disabled?: boolean; onBusyChange?: (busy: boolean) => void; onChange?: () => void;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const inputId = useId();
  const originals = useRef(new Map<string, File>());
  const controller = useRef<AbortController | null>(null);
  const query = useQuery({ ...collaborationDraftAttachmentsOptions(client, scope, form), enabled: form.selectedAttachmentIds.length > 0 || uploads.length > 0 });
  const upload = useMutation({ mutationKey: [...collaborationDeliveryKey(scope, form.scope.context), "upload", form.draftId], retry: false, networkMode: "always",
    mutationFn: async (selection: { files: File[] } | { draft: CollaborationUploadDraft; file?: File }) => {
      const attempt = new AbortController();
      controller.current = attempt;
      try {
        if ("draft" in selection) {
          await executeCollaborationUpload(client, scope, selection.draft, selection.file, undefined, attempt.signal);
          originals.current.delete(selection.draft.requestId);
          return;
        }
        if (selection.files.length + (query.data?.attachments.length ?? 0) + uploads.length > 32) throw new CollaborationAttachmentError("attachment_limit");
        const prepared: CollaborationUploadDraft[] = [];
        for (const file of selection.files) {
          const draft = await stageCollaborationUpload(client, scope, form, crypto.randomUUID(), file, undefined, attempt.signal);
          attempt.signal.throwIfAborted();
          originals.current.set(draft.requestId, file);
          prepared.push(draft);
        }
        for (const draft of prepared) {
          await executeCollaborationUpload(client, scope, draft, originals.current.get(draft.requestId), undefined, attempt.signal);
          originals.current.delete(draft.requestId);
        }
      } finally {
        if (controller.current === attempt) controller.current = null;
      }
    },
    onSuccess: () => { onChange?.(); },
  });
  useEffect(() => { onBusyChange?.(upload.isPending); return () => { onBusyChange?.(false); }; }, [onBusyChange, upload.isPending]);
  useEffect(() => { const files = originals.current; return () => { controller.current?.abort(); files.clear(); }; }, []);
  const locked = disabled || upload.isPending;
  return <fieldset className="min-w-0 space-y-3">
    <legend className="mb-2 text-body font-medium">{t(($) => $.attachments.title)}</legend>
    <p className="text-caption text-muted-foreground">{t(($) => $.attachments.help)}</p>
    <label htmlFor={inputId} className="block text-caption font-medium">{t(($) => $.attachments.add)}</label>
    <Input id={inputId} type="file" multiple disabled={locked || uploads.length > 0} className="h-auto min-h-11 min-w-0 py-2 text-caption"
      onChange={(event) => { const files = Array.from(event.target.files ?? []); event.target.value = ""; if (files.length) upload.mutate({ files }); }} />
    {query.isFetching && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.attachments.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && <ul className="space-y-2">{query.data.attachments.map((file) => <li key={file.id} className="min-w-0">
      <label className="flex min-h-11 items-start gap-3 py-1"><Checkbox disabled={locked}
        checked={form.selectedAttachmentIds.includes(file.id)} onCheckedChange={(checked) => {
          update({ selectedAttachmentIds: checked === true ? [...new Set([...form.selectedAttachmentIds, file.id])] : form.selectedAttachmentIds.filter((id) => id !== file.id) }); onChange?.();
        }} /><span className="sr-only">{t(($) => $.attachments.selected, { name: file.name })}</span><span aria-hidden="true" className="min-w-0 text-body [overflow-wrap:anywhere]">{file.name}<span className="block text-caption text-muted-foreground">{t(($) => $.attachments.size, { size: file.size_bytes.toLocaleString() })}</span></span></label>
    </li>)}</ul>}
    {uploads.map((draft) => <div key={draft.requestId} className="min-w-0 space-y-2 text-caption">
      <p role="status" className="[overflow-wrap:anywhere]">{t(($) => $.attachments.unknown, { name: draft.file.name })}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={locked} onClick={() => upload.mutate({ draft })}>{t(($) => $.attachments.recover)}</Button>
        {originals.current.has(draft.requestId) && <Button type="button" variant="outline" disabled={locked} onClick={() => upload.mutate({ draft, file: originals.current.get(draft.requestId) })}>{t(($) => $.attachments.retry)}</Button>}</div>
      {!originals.current.has(draft.requestId) && <label className="block space-y-2">{t(($) => $.attachments.reselect)}
        <Input type="file" disabled={locked} aria-label={t(($) => $.attachments.reselect)} className="h-auto min-h-11 py-2 text-caption" onChange={(event) => {
          const file = event.target.files?.[0]; event.target.value = ""; if (file) { originals.current.set(draft.requestId, file); upload.mutate({ draft, file }); }
        }} />
      </label>}
    </div>)}
    {upload.isPending && <div className="flex flex-wrap items-center gap-3"><p role="status" className="text-caption text-muted-foreground">{t(($) => $.attachments.uploading)}</p>
      <Button type="button" variant="outline" onClick={() => controller.current?.abort()}>{t(($) => $.attachments.cancelUpload)}</Button></div>}
    {upload.error?.name === "AbortError" ? <p role="status" className="text-caption text-muted-foreground">{t(($) => $.attachments.uploadCancelled)}</p>
      : upload.error && <CollaborationAttachmentFailure error={upload.error} />}
  </fieldset>;
}
