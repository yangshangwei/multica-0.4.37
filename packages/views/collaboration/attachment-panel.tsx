import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationAttachmentsOptions, collaborationKeys, executeCollaborationAttachmentDelivery,
  loadCollaborationAttachmentDownload, loadCollaborationAttachmentPreview, prepareCollaborationAttachmentDelivery,
  useCollaborationDeliveryForm, type CollaborationAttachment, type CollaborationDeliveryRequest, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { useT } from "../i18n";
import { RichContent } from "../rich-content";
import { HtmlPreviewBody } from "../editor/html-preview-body";
import { getPreviewKind } from "../editor/utils/preview";
import { CollaborationAttachmentFailure, CollaborationAttachmentPicker } from "./attachment-picker";
import { CollaborationQueryState } from "./query-state";

async function downloadNativeFile(url: string) {
  const bridge = (window as unknown as { desktopAPI?: { downloadURL?: (url: string) => Promise<void> | void } }).desktopAPI;
  if (bridge?.downloadURL) { await bridge.downloadURL(url); return; }
  const anchor = document.createElement("a");
  anchor.href = url; anchor.download = ""; anchor.rel = "noopener noreferrer"; anchor.referrerPolicy = "no-referrer";
  document.body.appendChild(anchor); anchor.click(); anchor.remove();
}

export function CollaborationAttachmentList({ scope, issueId, files }: { scope: CollaborationScope; issueId: string; files: CollaborationAttachment[] }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [preview, setPreview] = useState<CollaborationAttachment | null>(null);
  const download = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issueId), "attachment-download"], retry: false, networkMode: "always",
    mutationFn: async (file: CollaborationAttachment) => {
      const capability = await loadCollaborationAttachmentDownload(client, scope, file);
      await downloadNativeFile(capability.url);
    } });
  return <>
    <ul className="space-y-3">{files.map((file) => <li key={file.id} className="flex min-w-0 flex-wrap items-center justify-between gap-3 rounded-md border border-border p-3">
      <div className="min-w-0 flex-1 [overflow-wrap:anywhere]"><p className="text-body font-medium">{file.name}</p><p className="text-caption text-muted-foreground">{t(($) => $.attachments.size, { size: file.size_bytes.toLocaleString() })}</p></div>
      <div className="flex flex-wrap gap-2">{getPreviewKind(file.content_type, file.name) && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" aria-label={t(($) => $.attachments.preview, { name: file.name })} onClick={() => setPreview(file)}>{t(($) => $.attachments.previewAction)}</Button>}
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" aria-label={t(($) => $.attachments.download, { name: file.name })} disabled={download.isPending} onClick={() => download.mutate(file)}>{t(($) => $.attachments.downloadAction)}</Button></div>
    </li>)}</ul>
    {download.error && <CollaborationAttachmentFailure error={download.error} />}
    <Dialog open={preview !== null} onOpenChange={(open) => { if (!open) setPreview(null); }}>
      {preview && <DialogContent className="flex max-h-[90dvh] flex-col overflow-y-auto sm:max-w-4xl" closeLabel={t(($) => $.attachments.close)}>
        <DialogTitle className="pr-8 [overflow-wrap:anywhere]">{preview.name}</DialogTitle>
        <DialogDescription>{t(($) => $.attachments.size, { size: preview.size_bytes.toLocaleString() })}</DialogDescription>
        <AttachmentPreview key={preview.id} scope={scope} issueId={issueId} file={preview} />
      </DialogContent>}
    </Dialog>
  </>;
}

function AttachmentPreview({ scope, issueId, file }: { scope: CollaborationScope; issueId: string; file: CollaborationAttachment }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const kind = getPreviewKind(file.content_type, file.name);
  const text = kind === "html" || kind === "markdown" || kind === "text";
  const query = useQuery({ queryKey: [...collaborationKeys.issue(scope, issueId), "attachment-preview", file.id, file.version],
    staleTime: 0, gcTime: 0, retry: false, refetchOnWindowFocus: false,
    queryFn: async ({ signal }) => {
      const blob = await loadCollaborationAttachmentPreview(client, scope, file, signal);
      const content = text ? await blob.text() : null;
      signal.throwIfAborted();
      return { content, blob: text ? null : blob };
    } });
  const [objectUrl, setObjectUrl] = useState("");
  useEffect(() => {
    if (!query.data?.blob) return;
    const url = URL.createObjectURL(query.data.blob);
    setObjectUrl(url);
    return () => { URL.revokeObjectURL(url); };
  }, [query.data?.blob]);
  if (query.isPending) return <p role="status">{t(($) => $.attachments.previewLoading)}</p>;
  if (query.error) return <CollaborationAttachmentFailure error={query.error} />;
  const content = query.data?.content ?? "";
  switch (kind) {
    case "html": return <HtmlPreviewBody source={{ kind: "inline", html: content }} title={file.name} className="h-[60dvh] w-full" />;
    case "markdown": return <div className="min-h-0 overflow-auto"><RichContent content={content} resourcePolicy="restricted" /></div>;
    case "text": return <pre className="min-h-0 max-w-full overflow-auto whitespace-pre-wrap rounded-md bg-muted p-4 text-body [overflow-wrap:anywhere]">{content}</pre>;
    case "image": return objectUrl ? <img src={objectUrl} alt={file.name} className="max-h-[65dvh] max-w-full object-contain" /> : null;
    case "pdf": return objectUrl ? <iframe src={objectUrl} title={file.name} sandbox="" referrerPolicy="no-referrer" className="h-[65dvh] w-full" /> : null;
    case "audio": return objectUrl ? <audio controls src={objectUrl} aria-label={file.name} className="w-full" /> : null;
    case "video": return objectUrl ? <video controls src={objectUrl} aria-label={file.name} className="max-h-[65dvh] max-w-full" /> : null;
    default: return <p>{t(($) => $.attachments.previewUnavailable)}</p>;
  }
}

export function CollaborationAttachmentPanel({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery(collaborationAttachmentsOptions(client, scope, issue.id));
  return <section aria-label={t(($) => $.attachments.title)} className="min-w-0 space-y-4">
    <h3 className="text-body font-semibold">{t(($) => $.attachments.title)}</h3>
    <p className="text-caption text-muted-foreground">{t(($) => $.attachments.retained)}</p>
    {query.isPending && <p role="status">{t(($) => $.attachments.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && (query.data.attachments.length ? <CollaborationAttachmentList scope={scope} issueId={issue.id} files={query.data.attachments} />
      : <p className="text-body text-muted-foreground">{t(($) => $.attachments.empty)}</p>)}
    {issue.capabilities.comment === true && <AttachmentDeliveryForm scope={scope} issue={issue} />}
  </section>;
}

function AttachmentDeliveryForm({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const draft = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "attachment");
  const [uploading, setUploading] = useState(false);
  const send = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "attachment-delivery"], retry: false, networkMode: "always",
    mutationFn: ({ value, recovery }: { value: CollaborationDeliveryRequest; recovery: boolean }) => executeCollaborationAttachmentDelivery(client, scope, value, recovery),
    onSuccess: () => { draft.reset(); } });
  const locked = send.isPending || uploading || draft.uploads.length > 0 || draft.deliveries.length > 0 || Boolean(draft.error);
  return <details className="space-y-4"><summary className="cursor-pointer py-2 text-body font-medium">{t(($) => $.attachments.add)}</summary>
    <CollaborationAttachmentPicker scope={scope} form={draft.form} update={draft.update} uploads={draft.uploads} onBusyChange={setUploading} disabled={send.isPending || draft.deliveries.length > 0 || Boolean(draft.error)} />
    <Button type="button" disabled={locked || draft.form.selectedAttachmentIds.length === 0} onClick={() => send.mutate({ value: prepareCollaborationAttachmentDelivery(draft.form, crypto.randomUUID()), recovery: false })}>{t(($) => $.attachments.deliver)}</Button>
    {send.isPending && <p role="status">{t(($) => $.attachments.delivering)}</p>}
    {send.error && <CollaborationAttachmentFailure error={send.error} />}
    {Boolean(draft.error) && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {draft.deliveries.map((value) => <div key={value.requestId} className="space-y-2"><p role="status">{t(($) => $.attachments.deliveryUnknown)}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={send.isPending} onClick={() => send.mutate({ value, recovery: true })}>{t(($) => $.attachments.recoverDelivery)}</Button>
        <Button type="button" variant="outline" disabled={send.isPending} onClick={() => send.mutate({ value, recovery: false })}>{t(($) => $.attachments.retryDelivery)}</Button></div></div>)}
  </details>;
}

export function CollaborationCommentAttachments({ scope, issueId, attachmentIds }: { scope: CollaborationScope; issueId: string; attachmentIds: string[] }) {
  const client = useQueryClient();
  const query = useQuery({ ...collaborationAttachmentsOptions(client, scope, issueId), enabled: attachmentIds.length > 0 });
  if (attachmentIds.length === 0 || !query.data) return null;
  return <CollaborationAttachmentList scope={scope} issueId={issueId} files={query.data.attachments.filter((file) => attachmentIds.includes(file.id))} />;
}
