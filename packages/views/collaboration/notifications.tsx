import { useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationNotificationsOptions, useUpdateCollaborationNotification, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath, type AccountView } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger, PopoverTitle } from "@multica/ui/components/ui/popover";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationNotifications({ scope, view }: { scope: CollaborationScope; view: AccountView }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const query = useInfiniteQuery({ ...collaborationNotificationsOptions(client, scope), enabled: scope.userId !== "" });
  const update = useUpdateCollaborationNotification(scope);
  const items = [...new Map((query.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()];
  return <div className="ml-auto min-w-0"><Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger render={<Button variant="ghost" className="pointer-coarse:min-h-11" />}>{t(($) => $.page.notifications)}{query.data && <span className="ml-2 tabular-nums">{t(($) => $.page.unread, { count: query.data.pages[0]?.unread_count ?? 0 })}</span>}</PopoverTrigger>
    <PopoverContent align="end" className="max-h-[70vh] w-96 max-w-[calc(100vw-2rem)] min-w-0 overflow-y-auto p-4">
      <PopoverTitle>{t(($) => $.page.notifications)}</PopoverTitle>
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {(query.error || update.error) && <CollaborationQueryState error={query.error ?? update.error} retry={() => void query.refetch()} />}
      {query.isSuccess && items.length === 0 && <p className="py-6 text-body text-muted-foreground">{t(($) => $.page.emptyNotifications)}</p>}
      <ul className="space-y-4">{items.map((item) => <li key={item.id} className="space-y-2 [overflow-wrap:anywhere]">
        {item.can_open === true ? <AppLink href={accountPath({ view, issueId: item.issue_id, commentId: item.source_kind === "comment" ? item.source_id : null, anchor: item.source_kind === "body" ? "description" : undefined })}
          className="block text-body font-medium underline-offset-4 hover:underline" onClick={() => setOpen(false)}>{item.issue_title}</AppLink> : <p className="text-body text-muted-foreground">{t(($) => $.page.notificationUnavailable)}</p>}
        <div className="flex flex-wrap gap-2">{item.read_at === null && <Button size="sm" variant="outline" disabled={update.isPending} className="pointer-coarse:min-h-11" onClick={() => update.mutate({ id: item.id, action: "read" })}>{t(($) => $.page.read)}</Button>}
          <Button size="sm" variant="ghost" disabled={update.isPending} className="pointer-coarse:min-h-11" onClick={() => update.mutate({ id: item.id, action: "archive" })}>{t(($) => $.page.archive)}</Button>
        </div>
      </li>)}</ul>
      {query.hasNextPage && <Button variant="ghost" disabled={query.isFetchingNextPage} className="mt-4 pointer-coarse:min-h-11" onClick={() => void query.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
    </PopoverContent></Popover>
  </div>;
}
