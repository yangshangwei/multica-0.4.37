import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentHistoryOptions, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { RichContent } from "../rich-content";
import { CollaborationQueryState } from "./query-state";

export function CollaborationCommentHistory({ scope, issueId, commentId }: { scope: CollaborationScope; issueId: string; commentId: string }) {
  const { t } = useT("collaboration");
  const [open, setOpen] = useState(false);
  const region = useId();
  const client = useQueryClient();
  const query = useQuery({ ...collaborationCommentHistoryOptions(client, scope, issueId, commentId), enabled: open });
  return <div className="min-w-0 space-y-3">
    <Button variant="ghost" size="sm" aria-expanded={open} aria-controls={region} onClick={() => setOpen(!open)}>{t(($) => $.detail.commentHistory)}</Button>
    {open && <div id={region} className="min-w-0 space-y-4 rounded-md border border-border p-4">
      {query.isPending && <p role="status" className="text-caption">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      {query.isSuccess && query.data.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.activity.empty)}</p>}
      {query.data?.items.map((item) => <div key={item.event_id} className="min-w-0 space-y-3">
        <p className="text-caption text-muted-foreground">{item.actor?.name ?? t(($) => $.detail.unknownAuthor)} · <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString()}</time></p>
        {item.before !== null && <div className="space-y-1"><p className="text-caption text-muted-foreground">{t(($) => $.activity.before)}</p><RichContent resourcePolicy="restricted" content={item.before} /></div>}
        <div className="space-y-1"><p className="text-caption text-muted-foreground">{t(($) => $.activity.after)}</p>{item.kind === "comment_deleted" ? <p className="text-body">{t(($) => $.detail.deleted)}</p> : <RichContent resourcePolicy="restricted" content={item.after} />}</div>
      </div>)}
    </div>}
  </div>;
}
