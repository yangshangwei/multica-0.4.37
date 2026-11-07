"use client";
import { useState } from "react";
import { CopyIcon, CheckIcon } from "lucide-react";
import { adminDetailHref, adminReturnHref, useAdminExecution } from "@multica/core/admin";
import type { AdminExecution } from "@multica/core/admin";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { copyText } from "@multica/ui/lib/clipboard";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminListState, adminTouchLinkClass, formatAdminTime } from "./list-controls";
import { AdminExecutionControls } from "../operations/control-panel";
import styles from "../admin-visual.module.css";

function executionStatusClass(status: AdminExecution["status"]) {
  switch (status) {
    case "completed": return styles.statusSuccess;
    case "failed": return styles.statusDanger;
    case "deferred":
    case "waiting_local_directory": return styles.statusWarning;
    case "preparing":
    case "dispatched":
    case "running": return styles.statusInfo;
    default: return styles.statusNeutral;
  }
}

export function AdminExecutionDetailPage({ id }: {
  id: string;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminExecution(id);
  const task = query.data;
  const [copyResult, setCopyResult] = useState<{ id: string; success: boolean } | null>(null);
  const returnHref = adminReturnHref(nav.searchParams, "/admin/tasks");
  const listParams = new URL(returnHref, "https://admin.invalid").searchParams;
  const timezone = nav.searchParams.get("timezone") ?? listParams.get("timezone") ?? "UTC";
  if (nav.searchParams.has("timezone")) listParams.set("timezone", timezone);
  const copied = copyResult?.id === task?.id && copyResult?.success === true;
  async function copyId() {
    if (!task) return;
    setCopyResult({ id: task.id, success: await copyText(task.id) });
  }
  const timeline = task ? [[t($ => $.executions.created), task.createdAt], [t($ => $.executions.dispatched), task.dispatchedAt], [t($ => $.executions.started), task.startedAt], [t($ => $.executions.completed), task.completedAt]] : [];
  const identities = task ? [[t($ => $.executions.workspace), task.workspaceId], [t($ => $.executions.agent), task.agentId], [t($ => $.executions.runtime), task.runtimeId], [t($ => $.executions.user), task.accountableUserId], [t($ => $.executions.submitted), task.submittedInstallationId], [t($ => $.executions.target), task.executionInstallationId], [t($ => $.executions.provider), task.provider], [t($ => $.executions.model), task.model], [t($ => $.executions.failure), task.failureCode]] : [];
  const relations = task ? [[t($ => $.executions.parent), task.parentTaskId], [t($ => $.executions.retry_of), task.retryOfTaskId], [t($ => $.executions.rerun_of), task.rerunOfTaskId]] : [];
  return <section className="space-y-7">
    <AppLink className="inline-flex min-h-11 items-center text-body text-muted-foreground underline underline-offset-4" href={returnHref}>{t($ => $.executions.back)}</AppLink>
    <AdminListState loading={query.isPending} error={query.isError} empty={false} retry={() => void query.refetch()}>{task && <>
      <header className="space-y-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-title font-semibold">{t($ => $.executions.details)}</h1>
          <Button variant="outline" onClick={() => void query.refetch()}>{t($ => $.executions.refresh)}</Button>
        </div>
        <div className="space-y-4">
          <p className="max-w-3xl break-words text-title font-semibold">{task.contentAccess ? task.title ?? t($ => $.executions.restricted) : t($ => $.executions.restricted)}</p>
          <dl className="flex flex-wrap gap-x-8 gap-y-3">
            <div className="space-y-1">
              <dt className="text-caption text-muted-foreground">{t($ => $.executions.status)}</dt>
              <dd><Badge variant="outline" className={`${styles.statusBadge} ${executionStatusClass(task.status)}`}>{t($ => $.executions.statuses[task.status])}</Badge></dd>
            </div>
            <div className="space-y-1">
              <dt className="text-caption text-muted-foreground">{t($ => $.executions.source)}</dt>
              <dd className="text-body font-medium">{t($ => $.executions.sources[task.source])}</dd>
            </div>
            <div className="space-y-1">
              <dt className="text-caption text-muted-foreground">{t($ => $.executions.attempt)}</dt>
              <dd className="text-body font-medium tabular-nums">{task.attempt}</dd>
            </div>
          </dl>
        </div>
      </header>
      <AdminExecutionControls key={task.id} execution={task} onRefresh={() => query.refetch({ throwOnError: true })} />
      <div className="grid gap-7 lg:grid-cols-2">
        <section className={`${styles.detailSection} space-y-4`}>
          <h2 className="text-body-lg font-semibold">{t($ => $.executions.time_records)}</h2>
          <dl className="grid gap-4 sm:grid-cols-2">{timeline.map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className="break-words text-body">{value ? <time dateTime={value}>{formatAdminTime(value, timezone)}</time> : t($ => $.executions.unknown)}</dd>
          </div>)}</dl>
        </section>
        <section className={`${styles.detailSection} space-y-4`}>
          <h2 className="text-body-lg font-semibold">{t($ => $.executions.usage)}</h2>{task.usage ? <dl className="grid grid-cols-2 gap-4">{[[t($ => $.executions.input_tokens), task.usage.inputTokens], [t($ => $.executions.output_tokens), task.usage.outputTokens], [t($ => $.executions.cache_read), task.usage.cacheReadTokens], [t($ => $.executions.cache_write), task.usage.cacheWriteTokens]].map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className="break-words text-body-lg font-semibold tabular-nums">{value?.toLocaleString()}</dd>
          </div>)}</dl> : <p className="text-body text-muted-foreground">{t($ => $.executions.usage_missing)}</p>}</section>
      </div>
      <section className={`${styles.detailSection} space-y-4`}>
        <h2 className="text-body-lg font-semibold">{t($ => $.executions.lineage)}</h2>
        <dl className="grid gap-4 sm:grid-cols-3">{relations.map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
          <dt className="text-caption text-muted-foreground">{label}</dt>
          <dd className="break-all text-body">{value ? <AppLink href={adminDetailHref(`/admin/tasks/${value}`, "/admin/tasks", listParams)} className={`inline-block underline underline-offset-4 ${adminTouchLinkClass}`}>{value}</AppLink> : t($ => $.executions.unknown)}</dd>
        </div>)}</dl>
      </section>
      <details className={styles.detailSection}>
        <summary className="min-h-11 cursor-pointer content-center text-body-lg font-semibold">{t($ => $.executions.technical_details)}</summary>
        <div className="space-y-5 pt-4">
          <div className="space-y-2">
            <Button variant="outline" size="sm" onClick={() => void copyId()}>{copied ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}<span aria-live="polite">{copied ? t($ => $.executions.copied_id) : t($ => $.executions.copy_id)}</span></Button>
            {copyResult?.id === task.id && !copyResult.success && <p role="alert" className="text-caption text-destructive">{t($ => $.executions.copy_failed)}</p>}
            <dl className="space-y-1">
              <dt className="text-caption text-muted-foreground">{t($ => $.executions.id)}</dt>
              <dd className={`${styles.codeValue} break-all text-body select-all`}>{task.id}</dd>
            </dl>
          </div>
          <dl className={styles.metadataGrid}>{identities.map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className={`${styles.codeValue} break-all text-body select-all`}>{value ?? t($ => $.executions.unknown)}</dd>
          </div>)}</dl>
        </div>
      </details>
      <section className={`${styles.detailSection} space-y-3`}>
        <h2 className="text-body-lg font-semibold">{t($ => $.executions.original_content)}</h2>
        <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.executions.content_restricted)}</p>{task.contentAccess && task.contentUrl && <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={task.contentUrl}>{t($ => $.executions.open_content)}</AppLink>}</section>
    </>}</AdminListState>
  </section>;
}
