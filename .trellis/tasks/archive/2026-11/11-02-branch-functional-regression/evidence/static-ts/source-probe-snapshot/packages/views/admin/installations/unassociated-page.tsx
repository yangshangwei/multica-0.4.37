"use client";
import type { FormEvent } from "react";
import { normalizeAdminFilters, useAdminUnassociatedRuntimes } from "@multica/core/admin";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { formatAdminTime } from "../executions/list-controls";
import { InstallationTabs, InstallationReadState, InstallationPagination, InstallationRuntimeStatus } from "./installation-common";
import { useAdminFilterFeedback } from "../filter-feedback";
export function AdminUnassociatedRuntimesPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminUnassociatedRuntimes(nav.searchParams);
  const data = query.data;
  const feedback = useAdminFilterFeedback(nav.searchParams);
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const params = normalizeAdminFilters(new FormData(event.currentTarget));
    if (!feedback.validate(params, event.currentTarget)) return;
    if (nav.searchParams.has("timezone")) params.set("timezone", timezone);
    nav.push(`${nav.pathname}${params.size ? `?${params}` : ""}`);
  }
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.installations.legacy)}</h1>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.legacy_description)}</p>
    </header>
    <InstallationTabs />
    <form key={nav.searchParams.toString()} noValidate onSubmit={filter} onReset={() => { feedback.reset(); nav.push(nav.pathname); }} className="flex flex-wrap items-end gap-3">
      <label className="min-w-0 basis-64 flex-1 space-y-1.5 text-caption">{t($ => $.installations.workspace)}<Input name="workspace_id" defaultValue={nav.searchParams.get("workspace_id") ?? ""} {...feedback.fieldProps("workspace_id")} />{feedback.message("workspace_id")}
      </label>
      <Button type="submit">{t($ => $.installations.apply)}</Button>
      <Button type="reset" variant="ghost">{t($ => $.installations.reset)}</Button>
    </form>
    <InstallationReadState pending={query.isPending} error={query.isError} empty={!data?.items.length} legacy retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table role="table" className="block w-full lg:table">
          <TableHeader className="sr-only lg:not-sr-only lg:table-header-group">
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>{t($ => $.installations.workspace)}</TableHead>
              <TableHead>{t($ => $.installations.provider)}</TableHead>
              <TableHead>{t($ => $.installations.runtime_status)}</TableHead>
              <TableHead>{t($ => $.installations.last_seen)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody role="rowgroup" className="block lg:table-row-group">{data?.items.map(item => <TableRow role="row" key={item.id} className="grid grid-cols-2 gap-x-4 gap-y-3 py-4 lg:table-row lg:py-0">
            <TableCell className="col-span-2 row-start-2 block break-all whitespace-normal text-caption text-muted-foreground lg:table-cell">{item.id}</TableCell>
            <TableCell className="col-span-2 block break-all whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.workspace)}</span>{item.workspaceId}</TableCell>
            <TableCell className="row-start-1 block min-w-0 break-words whitespace-normal font-medium lg:table-cell">{item.provider}</TableCell>
            <TableCell className="row-start-1 block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.runtime_status)}</span><InstallationRuntimeStatus status={item.status} /></TableCell>
            <TableCell className="col-span-2 block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.last_seen)}</span>{item.lastSeenAt ? <time dateTime={item.lastSeenAt}>{formatAdminTime(item.lastSeenAt, timezone)}</time> : t($ => $.installations.unknown)}</TableCell>
          </TableRow>)}</TableBody>
        </Table>
      </div>
    </InstallationReadState>{data && !query.isError && <InstallationPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}</section>;
}
