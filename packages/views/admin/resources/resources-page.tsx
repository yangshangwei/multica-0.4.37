"use client";
import { useRef, useState } from "react";
import { adminApiScope, useAdminAccess, useAdminResources, type AdminResource, type AdminResourceKind, type AdminResourceResult, type AdminScope } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";
import { ObservationHeader } from "../observability/common";
import styles from "../admin-visual.module.css";
import { ResourceEditor } from "./resource-editor";
import { resourceErrorKey } from "./resource-feedback";

type Selection = { action: "publish" | "withdraw"; target?: AdminResource };
export function AdminResourcesPage() {
  const { t } = useT("admin");
  const { identity, status } = useAdminAccess();
  const nav = useNavigation();
  const kind = nav.searchParams.get("kind") === "mcp" ? "mcp" : "skill";
  const scope = { apiScope: adminApiScope(), userId: identity?.userId ?? "", organizationId: identity?.organizationId ?? null };
  return <section className="space-y-6">
    <ObservationHeader title={t($ => $.resources.title)} description={t($ => $.resources.description)} />
    {status === "ready" && identity && <ResourceCatalog key={JSON.stringify([scope.apiScope, scope.userId, scope.organizationId, identity.role, kind])} scope={scope} kind={kind} administrator={identity.role === "super_admin"} />}
  </section>;
}
function ResourceCatalog({ scope, kind, administrator }: { scope: AdminScope; kind: AdminResourceKind; administrator: boolean }) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminResources(scope, kind);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<AdminResourceResult | null>(null);
  const newButton = useRef<HTMLButtonElement>(null);
  const data = query.data;
  const orderedItems = [...(data?.items ?? [])].sort((a, b) => Number(b.source === "managed") - Number(a.source === "managed"));
  const currentTarget = selection?.target && data?.items.find(item => item.source === selection.target?.source && item.key === selection.target?.key);
  // A withdrawal confirms the selected revision; polling must never upgrade its CAS.
  const withdrawalChanged = selection?.action === "withdraw" && !!selection.target &&
    (!currentTarget || currentTarget.version !== selection.target.version || currentTarget.state !== selection.target.state);
  const canWrite = administrator && data?.enabled === true && data?.canPublish === true && !query.isError;
  function choose(next: Selection) { if (busy || !canWrite) return; setSelection(next); setResult(null); }
  function close() { setSelection(null); newButton.current?.focus(); }
  function success(receipt: AdminResourceResult) { setResult(receipt); close(); void query.refetch(); }
  function changeKind(value: unknown) {
    if (busy || value !== "skill" && value !== "mcp") return;
    const next = new URLSearchParams(nav.searchParams); next.set("kind", value);
    nav.push(`${nav.pathname}?${next}${nav.hash}`);
  }
  return <Tabs value={kind} onValueChange={changeKind} className="space-y-5">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <TabsList aria-label={t($ => $.resources.tabs)} className={styles.resourceTabs}>
        <TabsTrigger value="skill" disabled={busy}>{t($ => $.resources.skill)}</TabsTrigger>
        <TabsTrigger value="mcp" disabled={busy}>{t($ => $.resources.mcp)}</TabsTrigger>
      </TabsList>
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" disabled={busy || query.isFetching} onClick={() => void query.refetch()}>{t($ => $.resources.refresh)}</Button>
        {canWrite && <Button ref={newButton} type="button" disabled={busy} onClick={() => choose({ action: "publish" })}>{t($ => $.resources.new)}</Button>}
      </div>
    </div>
    <TabsContent value={kind} aria-label={kind === "skill" ? t($ => $.resources.skill) : t($ => $.resources.mcp)} className="space-y-5">
      {query.isPending && <p role="status" className="text-body text-muted-foreground">{t($ => $.resources.loading)}</p>}
      {query.isError && <p role="alert" className="text-body">{resourceErrorKey(query.error) === "unsupported" ? t($ => $.resources.unsupported) : t($ => $.resources.unavailable)}</p>}
      {data?.enabled === false && <p role="status" className="text-body text-muted-foreground">{t($ => $.resources.disabled)}</p>}
      {data && (!administrator || data.enabled === true && data.canPublish === false) && <p role="status" className="text-body text-muted-foreground">{t($ => $.resources.readonly)}</p>}
      {result && <div role="status" className="space-y-1"><p className="text-body">{result.resource.state === "withdrawn" ? t($ => $.resources.withdrawnSuccess) : t($ => $.resources.publishedSuccess)}</p><p className="break-all text-caption text-muted-foreground">{t($ => $.resources.operation)}: {result.operationId}</p></div>}
      {data && <div className={selection ? styles.resourceLayout : undefined}>
        <ul className={styles.resourceList} aria-label={kind === "skill" ? t($ => $.resources.skill) : t($ => $.resources.mcp)}>
          {data.items.length === 0 && <li className="py-8 text-body text-muted-foreground">{t($ => $.resources.empty)}</li>}
          {orderedItems.map(resource => <li key={`${resource.source}:${resource.key}`} className={styles.resourceCard}>
            <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1 space-y-1"><h2 className="text-body-lg font-semibold [overflow-wrap:anywhere]">{resource.name || resource.key}</h2>{resource.name && resource.name !== resource.key && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{resource.key}</p>}</div>
              <div className="flex flex-wrap gap-2 text-caption"><span className={cn(styles.statusBadge, styles.statusNeutral)}>{t($ => $.resources[resource.source])}</span><span className={cn(styles.statusBadge, resource.state === "published" ? styles.statusSuccess : styles.statusNeutral)}>{t($ => $.resources[resource.state])}</span></div>
            </div>
            {resource.description && <p className="text-body text-muted-foreground [overflow-wrap:anywhere]">{resource.description}</p>}
            <dl className="grid min-w-0 gap-3 text-caption sm:grid-cols-2">
              <div className="min-w-0"><dt className="text-muted-foreground">{t($ => $.resources.version)}</dt><dd className="mt-1 [overflow-wrap:anywhere]">{resource.version || t($ => $.resources.unknown)}</dd></div>
              {(resource.source === "managed" || resource.updatedAt) && <div><dt className="text-muted-foreground">{t($ => $.resources.updated)}</dt><dd className="mt-1">{resource.updatedAt ? <time dateTime={resource.updatedAt}>{formatAdminTime(resource.updatedAt, nav.searchParams.get("timezone") ?? "UTC")}</time> : t($ => $.resources.unknown)}</dd></div>}
            </dl>
            {canWrite && resource.source === "managed" && (resource.state === "published" || resource.state === "withdrawn") ? <div className="flex flex-wrap gap-2" role="group" aria-label={`${t($ => $.resources.actions)}: ${resource.key}`}>
              <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => choose({ action: "publish", target: resource })}>{resource.state === "withdrawn" ? t($ => $.resources.republish) : t($ => $.resources.update)}</Button>
              {resource.state === "published" && <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => choose({ action: "withdraw", target: resource })}>{t($ => $.resources.withdraw)}</Button>}
            </div> : resource.source !== "managed" && <p className="text-caption text-muted-foreground">{t($ => $.resources.immutable)}</p>}
          </li>)}
        </ul>
        {selection && administrator && <ResourceEditor key={`${selection.action}:${selection.target?.key ?? "new"}:${selection.action === "withdraw" ? selection.target?.version : ""}`} scope={scope} kind={kind} action={selection.action} target={selection.action === "withdraw" ? selection.target : currentTarget ?? selection.target} targetChanged={withdrawalChanged} limits={data.limits} disabled={!canWrite} onClose={close} onSuccess={success} onBusy={setBusy} />}
      </div>}
    </TabsContent>
  </Tabs>;
}
