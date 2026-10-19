import { useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationNotificationsOptions, useUpdateCollaborationNotification, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath, type AccountView } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger, PopoverTitle } from "@multica/ui/components/ui/popover";
import { AppLink, resolveClickIntent } from "../navigation";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationNotifications({ scope, view }: { scope: CollaborationScope; view: AccountView }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const navigating = useRef(false);
  const query = useInfiniteQuery({ ...collaborationNotificationsOptions(client, scope), enabled: scope.userId !== "" });
  const update = useUpdateCollaborationNotification(scope);
  const items = [...new Map((query.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()];
  const notificationLabel = (kind: string) => {
    switch (kind) {
      case "ai_completed": return t(($) => $.notifications.aiCompleted);
      case "ai_failed": return t(($) => $.notifications.aiFailed);
      case "ai_stopped": return t(($) => $.notifications.aiStopped);
      case "assigned": return t(($) => $.creation.assignedSelf);
      case "removed": return t(($) => $.notifications.removed);
      case "created": case "participant_added": case "participant_removed": case "participant_left":
      case "transferred": case "information_requested": case "information_returned": case "acceptance_submitted":
      case "comment_created": case "comment_edited": case "comment_deleted": case "issue_edited":
      case "status_changed": case "iteration_changed": case "completed": case "cancelled": case "reopened":
        return t(($) => $.activity.kinds[kind]);
      default: return t(($) => $.activity.kinds.other);
    }
  };
  return <div className="ml-auto min-w-0"><Popover open={open} onOpenChange={(next) => { if (next) navigating.current = false; setOpen(next); }}>
    <PopoverTrigger render={<Button variant="ghost" className="pointer-coarse:min-h-11" />}>{t(($) => $.page.notifications)}{query.data && <span className="ml-2 tabular-nums">{t(($) => $.page.unread, { count: query.data.pages[0]?.unread_count ?? 0 })}</span>}</PopoverTrigger>
    <PopoverContent align="end" finalFocus={() => !navigating.current} className="max-h-[70vh] w-96 max-w-[calc(100vw-2rem)] min-w-0 overflow-y-auto p-4">
      <PopoverTitle>{t(($) => $.page.notifications)}</PopoverTitle>
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {(query.error || update.error) && <CollaborationQueryState error={query.error ?? update.error} retry={() => void query.refetch()} />}
      {query.isSuccess && items.length === 0 && <p className="py-6 text-body text-muted-foreground">{t(($) => $.page.emptyNotifications)}</p>}
      <ul className="space-y-4">{items.map((item) => <li key={item.id} className="space-y-2 [overflow-wrap:anywhere]">
        {item.kind === "removed" ? <p className="text-body text-muted-foreground">{notificationLabel(item.kind)}</p> : item.can_open === true ? <>
          <AppLink href={accountPath({ view: item.run_id ? "participating" : view, issueId: item.issue_id, runId: item.run_id, commentId: item.source_kind === "comment" ? item.source_id : null, anchor: item.source_kind === "body" ? "description" : undefined })}
            className="block text-body font-medium underline-offset-4 hover:underline" onClick={(event) => { navigating.current = resolveClickIntent(event) === "push" && !event.shiftKey; setOpen(false); }}>{item.issue_title}</AppLink>
          <p className="text-caption text-muted-foreground">{item.is_mention ? t(($) => $.notifications.mention) : notificationLabel(item.kind)}</p>
        </> : <p className="text-body text-muted-foreground">{t(($) => $.page.notificationUnavailable)}</p>}
        <div className="flex flex-wrap gap-2">{item.read_at === null && <Button size="sm" variant="outline" disabled={update.isPending} className="pointer-coarse:min-h-11" onClick={() => update.mutate({ id: item.id, action: "read" })}>{t(($) => $.page.read)}</Button>}
          <Button size="sm" variant="ghost" disabled={update.isPending} className="pointer-coarse:min-h-11" onClick={() => update.mutate({ id: item.id, action: "archive" })}>{t(($) => $.page.archive)}</Button>
        </div>
      </li>)}</ul>
      {query.hasNextPage && <Button variant="ghost" disabled={query.isFetchingNextPage} className="mt-4 pointer-coarse:min-h-11" onClick={() => void query.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
    </PopoverContent></Popover>
  </div>;
}
