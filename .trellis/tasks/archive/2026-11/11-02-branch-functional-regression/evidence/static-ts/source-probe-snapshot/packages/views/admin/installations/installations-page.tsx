"use client";
import type { FormEvent } from "react";
import { adminDetailHref, normalizeAdminFilters, useAdminInstallations } from "@multica/core/admin";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { adminSelectClass, useAdminFilterFeedback } from "../filter-feedback";
import { AdminFilterSummary } from "../filter-summary";
import { adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { InstallationAxis, InstallationTabs, InstallationReadState, InstallationPagination } from "./installation-common";
import { InstallationIdentifier } from "./installation-identity";
import styles from "../admin-visual.module.css";
export function AdminInstallationsPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminInstallations(nav.searchParams);
  const data = query.data;
  const feedback = useAdminFilterFeedback(nav.searchParams);
  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const params = normalizeAdminFilters(new FormData(event.currentTarget));
    if (!feedback.validate(params, event.currentTarget)) return;
    if (nav.searchParams.has("timezone")) params.set("timezone", nav.searchParams.get("timezone")!);
    nav.push(`${nav.pathname}${params.size ? `?${params}` : ""}`);
  }
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.installations.title)}</h1>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.description)}</p>
    </header>
    <InstallationTabs />
    <form key={nav.searchParams.toString()} noValidate onSubmit={filter} onReset={() => { feedback.reset(); nav.push(nav.pathname); }} className="space-y-3">
      <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label className="space-y-1.5 text-caption">{t($ => $.installations.search)}<Input name="q" maxLength={128} defaultValue={nav.searchParams.get("q") ?? ""} {...feedback.fieldProps("q")} />{feedback.message("q")}</label>
        <label className="space-y-1.5 text-caption">{t($ => $.installations.lifecycle)}<select name="lifecycle" className={adminSelectClass} defaultValue={nav.searchParams.get("lifecycle") ?? ""}>
          <option value="">{t($ => $.installations.all)}</option>
          <option value="active">{t($ => $.installations.active)}</option>
          <option value="retired">{t($ => $.installations.retired)}</option>
        </select></label>
        <div className="flex gap-2">
          <Button type="submit">{t($ => $.installations.apply)}</Button>
          <Button type="reset" variant="ghost">{t($ => $.installations.reset)}</Button>
        </div>
      </div>
      <details open={["client_state", "version", "os", "group"].some(key => nav.searchParams.has(key)) || undefined}>
        <summary className="w-fit cursor-pointer text-caption text-muted-foreground">{t($ => $.installations.moreFilters)}</summary>
        <div className="grid gap-3 pt-3 sm:grid-cols-2 lg:grid-cols-4">
          <label className="space-y-1.5 text-caption">{t($ => $.installations.client)}<select name="client_state" className={adminSelectClass} defaultValue={nav.searchParams.get("client_state") ?? ""}>
            <option value="">{t($ => $.installations.all)}</option>
            <option value="active">{t($ => $.installations.states.active)}</option>
            <option value="inactive">{t($ => $.installations.states.inactive)}</option>
            <option value="unknown">{t($ => $.installations.states.unknown)}</option>
          </select></label>
          <label className="space-y-1.5 text-caption">{t($ => $.installations.version)}<Input name="version" maxLength={128} defaultValue={nav.searchParams.get("version") ?? ""} {...feedback.fieldProps("version")} />{feedback.message("version")}</label>
          <label className="space-y-1.5 text-caption">{t($ => $.installations.os)}<Input name="os" maxLength={128} defaultValue={nav.searchParams.get("os") ?? ""} {...feedback.fieldProps("os")} />{feedback.message("os")}</label>
          <label className="space-y-1.5 text-caption">{t($ => $.installations.group)}<Input name="group" maxLength={128} defaultValue={nav.searchParams.get("group") ?? ""} {...feedback.fieldProps("group")} />{feedback.message("group")}</label>
        </div>
      </details>
      <AdminFilterSummary params={nav.searchParams} fields={[
        { name: "q", label: t($ => $.installations.search) },
        { name: "lifecycle", label: t($ => $.installations.lifecycle), options: [
          { value: "active", label: t($ => $.installations.active) },
          { value: "retired", label: t($ => $.installations.retired) },
        ] },
        { name: "client_state", label: t($ => $.installations.client), options: [
          { value: "active", label: t($ => $.installations.states.active) },
          { value: "inactive", label: t($ => $.installations.states.inactive) },
          { value: "unknown", label: t($ => $.installations.states.unknown) },
        ] },
        { name: "version", label: t($ => $.installations.version) },
        { name: "os", label: t($ => $.installations.os) },
        { name: "group", label: t($ => $.installations.group) },
        { name: "timezone", label: t($ => $.executions.timezone) },
      ]} />
    </form>
    {data?.dataQuality === "unavailable" && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.dependency_unavailable)}</p>}{data?.dataQuality === "partial" && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.truncated)}</p>}
    <InstallationReadState pending={query.isPending} error={query.isError} empty={!data?.items.length} retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table role="table" className="block w-full lg:table">
          <TableHeader className="sr-only lg:not-sr-only lg:table-header-group">
            <TableRow>
              <TableHead>{t($ => $.installations.name)}</TableHead>
              <TableHead>{t($ => $.installations.client)}</TableHead>
              <TableHead>{t($ => $.installations.daemon)}</TableHead>
              <TableHead>{t($ => $.installations.readiness)}</TableHead>
              <TableHead>{t($ => $.installations.runtime_count)}</TableHead>
              <TableHead>{t($ => $.installations.version)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody role="rowgroup" className="block lg:table-row-group">{data?.items.map(item => {
            const hasName = Boolean(item.displayName?.trim());
            const shortId = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(item.id) ? `${item.id.slice(0, 8)}...${item.id.slice(-4)}` : item.id;
            return <TableRow role="row" key={item.id} className={`${styles.compactRow} grid grid-cols-2 gap-x-4 gap-y-3 py-4 lg:table-row lg:py-0`}>
            <TableCell className="col-span-2 block min-w-0 whitespace-normal lg:table-cell lg:w-64">
              <AppLink href={adminDetailHref(`/admin/installations/${item.id}`, nav.pathname, nav.searchParams)} title={item.id} aria-label={hasName ? undefined : item.id} className={`block break-words text-body font-medium underline underline-offset-4 ${adminTouchLinkClass}`}>{hasName ? item.displayName : shortId}</AppLink>
              <p className="text-caption text-muted-foreground">{item.os ?? t($ => $.installations.unknown)}</p>
              <InstallationIdentifier id={item.id} />
            </TableCell>
            <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.client)}</span>
              <InstallationAxis axis={item.clientActivity} />
            </TableCell>
            <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.daemon)}</span>
              <InstallationAxis axis={item.daemonReachability} />
            </TableCell>
            <TableCell className="col-span-2 block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.readiness)}</span>
              <InstallationAxis axis={item.executionReadiness} />
            </TableCell>
            <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.runtime_count)}</span>{item.runtimeCount}</TableCell>
            <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.version)}</span>{item.desktopVersion ?? t($ => $.installations.unknown)}</TableCell>
          </TableRow>;
          })}</TableBody>
        </Table>
      </div>
    </InstallationReadState>
    {query.isError && data && <p className="text-caption text-muted-foreground">{t($ => $.installations.last_seen)}: <time dateTime={data.asOf}>{formatAdminTime(data.asOf, nav.searchParams.get("timezone") ?? "UTC")}</time>
    </p>}{data && !query.isError && <InstallationPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}<p className="max-w-3xl text-caption text-muted-foreground">{t($ => $.installations.readiness_notice)}</p>
  </section>;
}
