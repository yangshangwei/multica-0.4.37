import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationKeys, collaborationParticipantsOptions, prepareCollaborationParticipantRemoval,
  type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { AlertDialog, AlertDialogContent, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction, AlertDialogTrigger } from "@multica/ui/components/ui/alert-dialog";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationParticipants({ scope, issue, locked, submit }: { scope: CollaborationScope; issue: CollaborationIssue; locked: boolean; submit: (draft: CollaborationDraft) => Promise<unknown> }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const contentId = useId();
  const [open, setOpen] = useState(false);
  const [selection, setSelection] = useState<{ name: string; handler: boolean; draft: CollaborationDraft } | null>(null);
  const query = useQuery({ ...collaborationParticipantsOptions(client, scope, issue.id), enabled: open });
  const canRemove = issue.capabilities.remove_participants === true && issue.write_context != null && issue.creator.user_id === scope.userId;
  const current = query.data?.collaboration_revision === issue.collaboration_revision;
  return <div className="space-y-3">
    <Button variant="ghost" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" aria-expanded={open} aria-controls={contentId} onClick={() => setOpen(!open)}>{t(($) => $.participants.title)}</Button>
    {open && <div id={contentId} className="space-y-3">
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      {query.data && !current && <p role="status" className="text-body">{t(($) => $.execution.staleRequest)}</p>}
      {query.data && <Button variant="ghost" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" disabled={query.isFetching || locked} onClick={() => void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id) })}>{t(($) => $.page.refresh)}</Button>}
      {query.isSuccess && query.data.participants.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.participants.empty)}</p>}
      <AlertDialog open={selection !== null && canRemove} onOpenChange={(value) => { if (!value) setSelection(null); }}>
        <ul aria-label={t(($) => $.participants.title)} className="space-y-3">{query.data?.participants.map((participant) => <li key={participant.user_id} className="flex min-w-0 flex-wrap items-center justify-between gap-3">
          <div className="min-w-0 space-y-1"><p className="text-body [overflow-wrap:anywhere]">{participant.name}</p><p className="text-caption text-muted-foreground">
            {participant.user_id === issue.creator.user_id && <span className="mr-3">{t(($) => $.participants.creator)}</span>}
            {participant.user_id === issue.handler.user_id && <span className="mr-3">{t(($) => $.participants.handler)}</span>}
            {participant.external && <span>{t(($) => $.detail.external)}</span>}
            {participant.account_active !== true && <span className="ml-3">{t(($) => $.participants.accountUnavailable)}</span>}
          </p></div>
          {canRemove && participant.user_id !== scope.userId && <AlertDialogTrigger render={<Button variant="outline" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" />} disabled={locked || query.isFetching || !current} onClick={() => setSelection({ name: participant.name, handler: participant.user_id === issue.handler.user_id,
            draft: prepareCollaborationParticipantRemoval(scope, issue, crypto.randomUUID(), query.data!, participant.user_id) })}>{t(($) => $.participants.remove, { name: participant.name })}</AlertDialogTrigger>}
        </li>)}</ul>
        {selection && <AlertDialogContent className="max-h-[90dvh] overflow-y-auto [overflow-wrap:anywhere]">
          <AlertDialogTitle>{t(($) => $.participants.removeTitle, { name: selection.name })}</AlertDialogTitle>
          <AlertDialogDescription>{t(($) => $.participants.effect)}{selection.handler && <> {t(($) => $.participants.handlerEffect)}</>}</AlertDialogDescription>
          <AlertDialogFooter><AlertDialogCancel disabled={locked}>{t(($) => $.workflow.dismiss)}</AlertDialogCancel>
            <AlertDialogAction disabled={locked || !canRemove} variant="destructive" onClick={() => { void submit(selection.draft).finally(() => setSelection(null)); }}>{t(($) => $.participants.confirm)}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>}
      </AlertDialog>
    </div>}
  </div>;
}
