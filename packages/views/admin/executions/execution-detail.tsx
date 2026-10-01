"use client";
import { useAdminExecution } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { AdminListState, formatAdminTime } from "./list-controls";
export function AdminExecutionDetailPage({ id }: {
  id: string;
}) {
  const { t } = useT("admin");
  const query = useAdminExecution(id);
  const task = query.data;
  const timeline = task ? [[t($ => $.executions.created), task.createdAt], [t($ => $.executions.dispatched), task.dispatchedAt], [t($ => $.executions.started), task.startedAt], [t($ => $.executions.completed), task.completedAt]] : [];
  const identities = task ? [[t($ => $.executions.workspace), task.workspaceId], [t($ => $.executions.agent), task.agentId], [t($ => $.executions.runtime), task.runtimeId], [t($ => $.executions.user), task.accountableUserId], [t($ => $.executions.submitted), task.submittedInstallationId], [t($ => $.executions.target), task.executionInstallationId], [t($ => $.executions.provider), task.provider], [t($ => $.executions.model), task.model], [t($ => $.executions.failure), task.failureCode]] : [];
  const relations = task ? [[t($ => $.executions.parent), task.parentTaskId], [t($ => $.executions.retry_of), task.retryOfTaskId], [t($ => $.executions.rerun_of), task.rerunOfTaskId]] : [];
  return <section className="space-y-7">
    <AppLink className="text-body text-muted-foreground underline underline-offset-4" href="/admin/tasks">{t($ => $.executions.back)}</AppLink>
    <AdminListState loading={query.isPending} error={query.isError} empty={false} retry={() => void query.refetch()}>{task && <>
      <header className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-title font-semibold">{t($ => $.executions.details)}</h1>
          <Button variant="outline" onClick={() => void query.refetch()}>{t($ => $.executions.refresh)}</Button>
        </div>
        <p className="break-all text-body text-muted-foreground">{task.id}</p>
        <p className="text-body">{t($ => $.executions.statuses[task.status])} · {t($ => $.executions.sources[task.source])} · {t($ => $.executions.attempt)} {task.attempt}</p>
      </header>
      <div className="grid gap-7 lg:grid-cols-2">
        <dl className="grid gap-4 sm:grid-cols-2">{timeline.map(([label, value]) => <div key={label} className="space-y-1">
          <dt className="text-caption text-muted-foreground">{label}</dt>
          <dd className="text-body">{value ? <time dateTime={value}>{formatAdminTime(value)}</time> : t($ => $.executions.unknown)}</dd>
        </div>)}</dl>
        <section className="space-y-3">
          <h2 className="text-body-lg font-semibold">{t($ => $.executions.usage)}</h2>{task.usage ? <dl className="grid grid-cols-2 gap-3">{[[t($ => $.executions.input_tokens), task.usage.inputTokens], [t($ => $.executions.output_tokens), task.usage.outputTokens], [t($ => $.executions.cache_read), task.usage.cacheReadTokens], [t($ => $.executions.cache_write), task.usage.cacheWriteTokens]].map(([label, value]) => <div key={label}>
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className="text-body tabular-nums">{value?.toLocaleString()}</dd>
          </div>)}</dl> : <p className="text-body text-muted-foreground">{t($ => $.executions.usage_missing)}</p>}</section>
      </div>
      <dl className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{identities.map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
        <dt className="text-caption text-muted-foreground">{label}</dt>
        <dd className="break-all text-body">{value ?? t($ => $.executions.unknown)}</dd>
      </div>)}</dl>
      <section className="space-y-4">
        <h2 className="text-body-lg font-semibold">{t($ => $.executions.lineage)}</h2>
        <dl className="grid gap-4 sm:grid-cols-3">{relations.map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
          <dt className="text-caption text-muted-foreground">{label}</dt>
          <dd className="break-all text-body">{value ? <AppLink href={`/admin/tasks/${value}`} className="underline underline-offset-4">{value}</AppLink> : t($ => $.executions.unknown)}</dd>
        </div>)}</dl>
      </section>
      <section className="space-y-3">
        <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.executions.content_restricted)}</p>{task.contentAccess && task.contentUrl && <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={task.contentUrl}>{t($ => $.executions.open_content)}</AppLink>}</section>
    </>}</AdminListState>
  </section>;
}
