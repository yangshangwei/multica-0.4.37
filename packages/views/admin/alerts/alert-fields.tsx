"use client";
import { useState } from "react";
import { adminAlertResolutions, useAdminUsers, type AdminAlertAction, type AdminAlertChange, type AdminScope } from "@multica/core/admin";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

function AssigneePicker({ scope }: { scope: AdminScope }) {
  const { t } = useT("admin");
  const [search, setSearch] = useState("");
  const [cursor, setCursor] = useState<string | undefined>();
  const [selected, setSelected] = useState({ id: scope.userId, name: scope.userId });
  const query = useAdminUsers(scope, { role: "super_admin", status: "active", q: search || undefined, cursor, limit: 50 });
  const users = query.data?.items ?? [];
  return <div className="space-y-3"><label className="block space-y-1 text-caption">{t($ => $.alerts.ownerSearch)}<Input value={search} onChange={event => { setSearch(event.target.value); setCursor(undefined); }} /></label><label className="block space-y-1 text-caption">{t($ => $.alerts.assignee)}<select name="assignee" value={selected.id} onChange={event => setSelected({ id: event.target.value, name: users.find(user => user.id === event.target.value)?.name ?? event.target.value })} className="h-10 w-full rounded-md border border-input bg-background px-3 text-body"><option value="">{t($ => $.alerts.unassigned)}</option>{selected.id && !users.some(user => user.id === selected.id) && <option value={selected.id}>{selected.name}</option>}{users.map(user => <option key={user.id} value={user.id}>{user.name}</option>)}</select></label><p className="text-caption text-muted-foreground">{t($ => $.alerts.ownerHint)}</p>
    {query.isError && <p role="alert">{t($ => $.observability.error)}</p>}{query.data?.dataQuality === "partial" && <p role="status">{t($ => $.observability.quality.partial)}</p>}
    <div className="flex gap-2"><Button type="button" variant="ghost" disabled={!cursor} onClick={() => setCursor(undefined)}>{t($ => $.observability.first)}</Button><Button type="button" variant="outline" disabled={!query.data?.nextCursor} onClick={() => setCursor(query.data?.nextCursor ?? undefined)}>{t($ => $.observability.next)}</Button></div>
  </div>;
}
export function AdminAlertFields({ scope, action, requiresResolution, locked, restored }: { scope: AdminScope; action: AdminAlertAction; requiresResolution: boolean; locked: boolean; restored?: AdminAlertChange }) {
  const { t } = useT("admin");
  const [resolution, setResolution] = useState(restored?.resolutionCode ?? "handled");
  if (action === "acknowledge") return null;
  if (action === "assign") return locked ? <label className="space-y-1 text-caption">{t($ => $.alerts.assignee)}<Input name="assignee" readOnly value={restored?.assigneeId ?? ""} /></label> : <AssigneePicker scope={scope} />;
  if (!requiresResolution) return null;
  return <><label className="space-y-1 text-caption">{t($ => $.alerts.resolution)}<select name="resolution" value={resolution} disabled={locked} onChange={event => { const value = adminAlertResolutions.find(value => value === event.target.value); if (value) setResolution(value); }} className="h-10 w-full rounded-md border border-input bg-background px-3 text-body">{adminAlertResolutions.map(value => <option key={value} value={value}>{t($ => $.alerts.resolutions[value])}</option>)}</select></label>{resolution === "retry_succeeded" && <label className="space-y-1 text-caption">{t($ => $.alerts.relatedTask)}<Input name="relatedTask" required readOnly={locked} defaultValue={restored?.relatedTaskId ?? ""} /></label>}</>;
}
