import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentHistoryOptions, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "../i18n";
import { RichContent } from "../rich-content";
import { CollaborationQueryState } from "./query-state";
import { CollaborationCommentAttachments } from "./attachment-panel";

// Controlled by the comment row: its menu item and Edited marker open it. The
// query stays lazy, so a closed history costs no request.
export function CollaborationCommentHistory({ scope, issueId, commentId, open, onClose }: { scope: CollaborationScope; issueId: string; commentId: string; open: boolean; onClose: () => void }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const client = useQueryClient();
  const query = useQuery({ ...collaborationCommentHistoryOptions(client, scope, issueId, commentId), enabled: open });
  if (!open) return null;
  return <div role="region" aria-label={t(($) => $.detail.commentHistory)} className="min-w-0 space-y-4 rounded-md border border-border p-4">
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <p className="min-w-0 text-caption font-medium [overflow-wrap:anywhere]">{t(($) => $.detail.commentHistory)}</p>
      <Button type="button" variant="ghost" size="sm" className="h-auto min-h-7 whitespace-normal text-muted-foreground pointer-coarse:min-h-11" onClick={onClose}>{t(($) => $.commentActions.hideHistory)}</Button>
    </div>
    {query.isPending && <p role="status" className="text-caption">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.isSuccess && query.data.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.activity.empty)}</p>}
    {query.data?.items.map((item) => <div key={item.event_id} className="min-w-0 space-y-3">
      <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{item.actor?.name ?? t(($) => $.detail.unknownAuthor)} · <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString(locale)}</time></p>
      {item.before !== null && <div className="space-y-1"><p className="text-caption text-muted-foreground">{t(($) => $.activity.before)}</p><RichContent resourcePolicy="restricted" content={item.before} /><CollaborationCommentAttachments scope={scope} issueId={issueId} attachmentIds={item.before_attachment_ids ?? []} /></div>}
      <div className="space-y-1"><p className="text-caption text-muted-foreground">{t(($) => $.activity.after)}</p>{item.kind === "comment_deleted" ? <p className="text-body">{t(($) => $.detail.deleted)}</p> : <RichContent resourcePolicy="restricted" content={item.after} />}<CollaborationCommentAttachments scope={scope} issueId={issueId} attachmentIds={item.after_attachment_ids ?? []} /></div>
    </div>)}
  </div>;
}
