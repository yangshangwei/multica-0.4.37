"use client";

import { adminApiScope, useAdminAccess, useAdminCollaborationIssue, useAdminCollaborationIssues,
  type AdminCollaborationIssue, type AdminScope } from "@multica/core/admin";
import { ApiError, errorCode } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { adminTouchLinkClass } from "../executions/list-controls";
import { AdminCollaborationControls } from "./collaboration-controls";
import { AdminCollaborationMemoryControls } from "./collaboration-memory-controls";

const buttonClass = "h-auto min-h-11 whitespace-normal text-left";

export function AdminCollaborationEntry({ href }: { href: string }) {
  const { t } = useT("admin");
  const access = useAdminAccess();
  if (access.status !== "ready" || access.identity?.role !== "super_admin") return null;
  return <AppLink href={href} className={`inline-flex items-center text-body underline underline-offset-4 ${adminTouchLinkClass}`}>
    {t($ => $.collaboration.entry)}
  </AppLink>;
}

export function AdminCollaborationPage({ issueId, hrefForIssue }: { issueId?: string | null; hrefForIssue: (id: string | null) => string }) {
  const { t } = useT("admin");
  const access = useAdminAccess();
  if (access.status === "loading" || access.status === "signed_out") return <p role="status" className="text-body text-muted-foreground">{t($ => $.state.loading)}</p>;
  if (access.status !== "ready" || !access.identity || access.identity.role !== "super_admin") {
    return <section role="alert" className="min-w-0 space-y-4">
      <h1 className="text-title font-semibold">{t($ => $.collaboration.title)}</h1>
      <p className="max-w-prose text-body text-muted-foreground">{access.status === "unavailable" ? t($ => $.state.unavailable_description)
        : access.status === "unsupported" ? t($ => $.state.unsupported_description) : t($ => $.collaboration.accessRequired)}</p>
      <Button variant="outline" className={buttonClass} onClick={access.retry}>{t($ => $.state.retry)}</Button>
    </section>;
  }
  const scope: AdminScope = { apiScope: adminApiScope(), userId: access.identity.userId, organizationId: access.identity.organizationId };
  const identity = JSON.stringify([scope.apiScope, scope.userId, scope.organizationId, issueId]);
  return <div className="min-w-0 space-y-6 [overflow-wrap:anywhere]" key={identity}>
    {issueId ? <AdminCollaborationDetail scope={scope} issueId={issueId} hrefForIssue={hrefForIssue} />
      : <AdminCollaborationList scope={scope} hrefForIssue={hrefForIssue} />}
  </div>;
}

function IssueMetadata({ issue, identifiers = false }: { issue: AdminCollaborationIssue; identifiers?: boolean }) {
  const { t } = useT("admin");
  const fields = [
    { label: t($ => $.collaboration.source), value: issue.sourceName || issue.sourceWorkspaceId },
    { label: t($ => $.collaboration.creator), value: `${issue.creator.name || issue.creator.userId} · ${t($ => $.collaboration.disabled)}` },
    { label: t($ => $.collaboration.handler), value: `${issue.handler.name || issue.handler.userId} · ${issue.handler.disabled ? t($ => $.collaboration.disabled) : t($ => $.collaboration.enabled)}` },
    { label: t($ => $.collaboration.status), value: issue.status },
    { label: t($ => $.collaboration.queue), value: t($ => $.collaboration.queues[issue.queueState]) },
    { label: t($ => $.collaboration.unsettledRuns), value: issue.unsettledRuns },
  ];
  return <div className="min-w-0 space-y-4">
    <dl className="grid min-w-0 gap-x-8 gap-y-3 text-body sm:grid-cols-2 lg:grid-cols-3">
      {fields.map(field => <div key={field.label} className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{field.label}</dt><dd>{field.value}</dd></div>)}
    </dl>
    {identifiers && <details className="min-w-0 text-caption">
      <summary className="min-h-11 cursor-pointer py-3 text-muted-foreground">{t($ => $.collaboration.identifiers)}</summary>
      <dl className="grid min-w-0 gap-3 py-2 sm:grid-cols-2">
        {[{ label: t($ => $.collaboration.issueId), value: issue.issueId }, { label: t($ => $.collaboration.sourceId), value: issue.sourceWorkspaceId },
          { label: t($ => $.collaboration.creator), value: issue.creator.userId }, { label: t($ => $.collaboration.handler), value: issue.handler.userId },
          { label: t($ => $.collaboration.revision), value: issue.collaborationRevision }].map(field => <div key={field.label} className="min-w-0">
          <dt className="text-muted-foreground">{field.label}</dt><dd>{field.value}</dd>
        </div>)}
      </dl>
    </details>}
  </div>;
}

function AdminCollaborationList({ scope, hrefForIssue }: { scope: AdminScope; hrefForIssue: (id: string | null) => string }) {
  const { t } = useT("admin");
  const query = useAdminCollaborationIssues(scope);
  const items = query.isError ? [] : query.data?.pages.flatMap(page => page.items) ?? [];
  return <>
    <header className="flex min-w-0 flex-wrap items-start justify-between gap-4">
      <div className="min-w-0 space-y-2"><h1 className="text-title-lg font-semibold">{t($ => $.collaboration.title)}</h1>
        <p className="max-w-prose text-body text-muted-foreground">{t($ => $.collaboration.description)}</p></div>
      <Button variant="outline" className={buttonClass} disabled={query.isFetching} onClick={() => { void query.refetch(); }}>{t($ => $.collaboration.refreshList)}</Button>
    </header>
    {query.isError ? <p role="alert" className="text-body">{t($ => $.state.unavailable_description)}</p>
      : query.isPending ? <p role="status" className="text-body text-muted-foreground">{t($ => $.collaboration.loading)}</p>
      : !items.length ? <p className="text-body text-muted-foreground">{t($ => $.collaboration.empty)}</p>
      : <ul className="min-w-0 space-y-5">
        {items.map(issue => <li key={issue.issueId} className="min-w-0 space-y-3 rounded-md border border-surface-border p-4 sm:p-5">
          <AppLink href={hrefForIssue(issue.issueId)} className={`inline-flex min-w-0 items-center text-body-lg font-semibold underline underline-offset-4 ${adminTouchLinkClass}`}>{issue.identifier}</AppLink>
          <IssueMetadata issue={issue} />
        </li>)}
      </ul>}
    {query.hasNextPage && !query.isError && <Button variant="outline" className={buttonClass} disabled={query.isFetching} onClick={() => { void query.fetchNextPage(); }}>{t($ => $.collaboration.loadMore)}</Button>}
  </>;
}

function AdminCollaborationDetail({ scope, issueId, hrefForIssue }: { scope: AdminScope; issueId: string; hrefForIssue: (id: string | null) => string }) {
  const { t } = useT("admin");
  const query = useAdminCollaborationIssue(scope, issueId);
  const detail = query.isError ? null : query.data ?? null;
  const restored = query.error instanceof ApiError && errorCode(query.error) === "creator_not_disabled";
  return <>
    <AppLink href={hrefForIssue(null)} className={`inline-flex items-center text-body underline underline-offset-4 ${adminTouchLinkClass}`}>{t($ => $.collaboration.backToList)}</AppLink>
    <header className="flex min-w-0 flex-wrap items-center justify-between gap-4">
      <h1 className="min-w-0 text-title-lg font-semibold">{detail?.identifier ?? t($ => $.collaboration.title)}</h1>
      <Button variant="outline" className={buttonClass} disabled={query.isFetching} onClick={() => { void query.refetch(); }}>{t($ => $.collaboration.refresh)}</Button>
    </header>
    {detail ? <IssueMetadata issue={detail} identifiers />
      : query.isError ? <p role="alert" className="max-w-prose text-body text-muted-foreground">{restored ? t($ => $.collaboration.creatorRestored) : t($ => $.collaboration.metadataUnavailable)}</p>
      : <p role="status" className="text-body text-muted-foreground">{t($ => $.collaboration.loading)}</p>}
    <AdminCollaborationControls scope={scope} issueId={issueId} detail={detail} refreshing={query.isFetching} />
    <AdminCollaborationMemoryControls scope={scope} issueId={issueId} detail={detail} refreshing={query.isFetching} />
  </>;
}
