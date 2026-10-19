"use client";

import type { ReactNode } from "react";
import { isCollaborationDenied, useCollaborationIssue, useCollaborationScope } from "@multica/core/collaboration";
import type { SourceCollaborationAction } from "@multica/core/issues";
import { useCollaborationRealtime } from "@multica/core/realtime";
import { accountPath } from "@multica/core/paths";
import type { Issue } from "@multica/core/types";
import { Avatar, AvatarFallback, AvatarImage } from "@multica/ui/components/ui/avatar";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { CollaborationCreationEntry } from "./creation-form";
import { CollaborationDetail } from "./collaboration-detail";
import { CollaborationQueryState } from "./query-state";
import { CollaborationWorkflow } from "./workflow";

export function SourceCollaborationCreation({ workspaceId }: { workspaceId: string }) {
  const scope = useCollaborationScope();
  return scope ? <CollaborationCreationEntry scope={scope} sourceWorkspaceId={workspaceId} /> : null;
}

export function SourceCollaborationAccount({ account, showName = true }: {
  account: NonNullable<Issue["collaboration"]>["handler"];
  showName?: boolean;
}) {
  const { t } = useT("collaboration");
  return <span className="inline-flex min-w-0 max-w-full items-center gap-1.5" title={account.name}>
    <Avatar className="size-5 shrink-0" aria-hidden><AvatarImage src={account.avatar_url || undefined} /><AvatarFallback>{account.name.slice(0, 1)}</AvatarFallback></Avatar>
    <span className={showName ? "truncate text-caption" : "sr-only"}>{account.name}</span>
    {account.external === true && <span className="shrink-0 text-micro text-muted-foreground">{t(($) => $.detail.external)}</span>}
  </span>;
}

export function SourceCollaborationDetail({ issueId, commentId, leading }: { issueId: string; commentId?: string; leading?: ReactNode }) {
  const scope = useCollaborationScope();
  const { t } = useT("collaboration");
  useCollaborationRealtime(scope, issueId);
  return <section className="flex min-h-0 min-w-0 flex-1 flex-col">
    <header className="flex shrink-0 flex-wrap items-center gap-3 px-4 py-3">{leading}
      <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={accountPath({ view: "participating", issueId, commentId: commentId ?? null })}>{t(($) => $.source.open)}</AppLink>
    </header>
    <div className="min-h-0 min-w-0 flex-1 overflow-y-auto px-4 pb-8" data-tab-scroll-root="account-detail">
      {scope ? <CollaborationDetail key={`${scope.apiScope}:${scope.userId}:${issueId}`} scope={scope} destination={{ view: "participating", issueId, commentId: commentId ?? null }} /> : <CollaborationQueryState denied error={null} />}
    </div>
  </section>;
}

export function SourceCollaborationActionDialog({ action, close }: { action: SourceCollaborationAction | null; close: () => void }) {
  const scope = useCollaborationScope();
  const { t } = useT("collaboration");
  return <Dialog open={action !== null} onOpenChange={(open) => { if (!open) close(); }}>
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl" closeLabel={t(($) => $.creation.close)}>
      <DialogTitle>{t(($) => $.workflow.title)}</DialogTitle>
      <DialogDescription>{action?.action === "lifecycle" ? t(($) => $.source.lifecycle) : t(($) => $.source.workflow)}</DialogDescription>
      {scope && action && <SourceAction key={`${scope.apiScope}:${scope.userId}:${action.issueId}:${action.action}:${action.status ?? ""}`} scope={scope} action={action} />}
    </DialogContent>
  </Dialog>;
}

function SourceAction({ scope, action }: { scope: NonNullable<ReturnType<typeof useCollaborationScope>>; action: SourceCollaborationAction }) {
  const { t } = useT("collaboration");
  const query = useCollaborationIssue(scope, action.issueId);
  useCollaborationRealtime(scope, action.issueId);
  if (query.isPending) return <p role="status">{t(($) => $.page.loading)}</p>;
  if (!query.data || isCollaborationDenied(query.error)) return <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />;
  const issue = query.data;
  const allowed = action.action === "lifecycle" ? issue.capabilities.complete || issue.capabilities.cancel || issue.capabilities.reopen : issue.capabilities[action.action === "transfer" ? "transfer" : action.action === "change_status" ? "change_status" : "edit"];
  if (allowed !== true || !issue.write_context) return <p role="alert" className="text-body">{t(($) => $.source.notAllowed)}</p>;
  return <>
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    <CollaborationWorkflow scope={scope} issue={issue} initialAction={action.action === "lifecycle" ? undefined : action.action} initialStatus={action.status} />
  </>;
}
