"use client";
import type { FormEvent } from "react";
import { useAdminUnassociatedRuntimes } from "@multica/core/admin";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { formatAdminTime } from "../executions/list-controls";
import { InstallationTabs, InstallationReadState, InstallationPagination } from "./installation-common";
export function AdminUnassociatedRuntimesPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminUnassociatedRuntimes(nav.searchParams);
  const data = query.data;
  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = String(new FormData(event.currentTarget).get("workspace_id") ?? "").trim();
    const params = new URLSearchParams();
    if (value)
      params.set("workspace_id", value);
    nav.push(`${nav.pathname}${params.size ? `?${params}` : ""}`);
  }
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.installations.legacy)}</h1>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.legacy_description)}</p>
    </header>
    <InstallationTabs />
    <form key={nav.searchParams.toString()} onSubmit={filter} className="flex flex-wrap items-end gap-3">
      <label className="min-w-64 flex-1 space-y-1.5 text-caption">{t($ => $.installations.workspace)}<Input name="workspace_id" defaultValue={nav.searchParams.get("workspace_id") ?? ""} />
      </label>
      <Button type="submit">{t($ => $.installations.apply)}</Button>
      <Button type="button" variant="ghost" onClick={() => nav.push(nav.pathname)}>{t($ => $.installations.reset)}</Button>
    </form>
    <InstallationReadState pending={query.isPending} error={query.isError} empty={!data?.items.length} legacy retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>{t($ => $.installations.workspace)}</TableHead>
              <TableHead>{t($ => $.installations.provider)}</TableHead>
              <TableHead>{t($ => $.installations.runtime_status)}</TableHead>
              <TableHead>{t($ => $.installations.last_seen)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell>{item.id}</TableCell>
            <TableCell>{item.workspaceId}</TableCell>
            <TableCell>{item.provider}</TableCell>
            <TableCell>{item.status}</TableCell>
            <TableCell>{item.lastSeenAt ? formatAdminTime(item.lastSeenAt) : t($ => $.installations.unknown)}</TableCell>
          </TableRow>)}</TableBody>
        </Table>
      </div>
    </InstallationReadState>{data && !query.isError && <InstallationPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}</section>;
}
