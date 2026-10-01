"use client";
import type { ReactNode } from "react";
import type { AdminInstallationAxis } from "@multica/core/admin";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { formatAdminTime } from "../executions/list-controls";
export function InstallationAxis({ axis }: {
  axis: AdminInstallationAxis;
}) {
  const { t } = useT("admin");
  return <div className="space-y-1">
    <Badge variant="outline">{t($ => $.installations.states[axis.state])}</Badge>
    <p className="text-caption text-muted-foreground">{t($ => $.installations.freshness[axis.freshness])}{axis.observedAt && <> · <time dateTime={axis.observedAt}>{formatAdminTime(axis.observedAt)}</time>
    </>}</p>
  </div>;
}
export function InstallationTabs() {
  const { t } = useT("admin");
  const nav = useNavigation();
  return <nav aria-label={t($ => $.installations.title)} className="flex flex-wrap gap-5 border-b border-surface-border pb-3">
    <AppLink href="/admin/installations" aria-current={nav.pathname === "/admin/installations" ? "page" : undefined} className="text-body aria-[current=page]:font-semibold">{t($ => $.installations.title)}</AppLink>
    <AppLink href="/admin/installations/unassociated" aria-current={nav.pathname.endsWith("/unassociated") ? "page" : undefined} className="text-body aria-[current=page]:font-semibold">{t($ => $.installations.legacy)}</AppLink>
  </nav>;
}
export function InstallationReadState({ pending, error, empty, legacy = false, retry, children }: {
  pending: boolean;
  error: boolean;
  empty: boolean;
  legacy?: boolean;
  retry: () => void;
  children: ReactNode;
}) {
  const { t } = useT("admin");
  if (error)
    return <div role="alert" className="space-y-3 py-8">
      <p className="text-body">{t($ => $.installations.unavailable)}</p>
      <Button variant="outline" onClick={retry}>{t($ => $.installations.retry)}</Button>
    </div>;
  if (pending)
    return <p role="status" className="py-8 text-body text-muted-foreground">{t($ => $.installations.loading)}</p>;
  if (empty)
    return <p className="py-8 text-body text-muted-foreground">{legacy ? t($ => $.installations.legacy_empty) : t($ => $.installations.empty)}</p>;
  return children;
}
export function InstallationPagination({ cursor, asOf, refresh }: {
  cursor: string | null;
  asOf: string;
  refresh: () => void;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  function move(next: string | null) {
    const p = new URLSearchParams(nav.searchParams);
    p.delete("cursor");
    if (next)
      p.set("cursor", next);
    nav.push(`${nav.pathname}${p.size ? `?${p}` : ""}`);
  }
  return <div className="flex flex-wrap items-center justify-between gap-3">
    <p className="text-caption text-muted-foreground">{t($ => $.installations.updated)} <time dateTime={asOf}>{formatAdminTime(asOf)}</time>
    </p>
    <div className="flex flex-wrap gap-2">
      <Button variant="ghost" onClick={() => nav.searchParams.has("cursor") ? move(null) : refresh()}>{t($ => $.installations.refresh)}</Button>
      <Button variant="outline" disabled={!nav.searchParams.has("cursor")} onClick={() => move(null)}>{t($ => $.installations.first)}</Button>
      <Button variant="outline" disabled={!cursor} onClick={() => move(cursor)}>{t($ => $.installations.next)}</Button>
    </div>
  </div>;
}
