"use client";
import type { FormEvent } from "react";
import { useAdminInstallations } from "@multica/core/admin";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { InstallationAxis, InstallationTabs, InstallationReadState, InstallationPagination } from "./installation-common";
export function AdminInstallationsPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminInstallations(nav.searchParams);
  const data = query.data;
  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const params = new URLSearchParams();
    for (const [key, value] of new FormData(event.currentTarget)) {
      if (typeof value === "string" && value.trim())
        params.set(key, value.trim());
    }
    nav.push(`${nav.pathname}${params.size ? `?${params}` : ""}`);
  }
  const selectClass = "h-9 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-ring";
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.installations.title)}</h1>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.description)}</p>
    </header>
    <InstallationTabs />
    <form key={nav.searchParams.toString()} onSubmit={filter} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <label className="space-y-1.5 text-caption">{t($ => $.installations.search)}<Input name="q" maxLength={128} defaultValue={nav.searchParams.get("q") ?? ""} />
      </label>
      <label className="space-y-1.5 text-caption">{t($ => $.installations.lifecycle)}<select name="lifecycle" className={selectClass} defaultValue={nav.searchParams.get("lifecycle") ?? ""}>
        <option value="">{t($ => $.installations.all)}</option>
        <option value="active">{t($ => $.installations.active)}</option>
        <option value="retired">{t($ => $.installations.retired)}</option>
      </select>
      </label>
      <label className="space-y-1.5 text-caption">{t($ => $.installations.client)}<select name="client_state" className={selectClass} defaultValue={nav.searchParams.get("client_state") ?? ""}>
        <option value="">{t($ => $.installations.all)}</option>
        <option value="active">{t($ => $.installations.states.active)}</option>
        <option value="inactive">{t($ => $.installations.states.inactive)}</option>
        <option value="unknown">{t($ => $.installations.states.unknown)}</option>
      </select>
      </label>
      <label className="space-y-1.5 text-caption">{t($ => $.installations.version)}<Input name="version" defaultValue={nav.searchParams.get("version") ?? ""} />
      </label>
      <label className="space-y-1.5 text-caption">{t($ => $.installations.os)}<Input name="os" defaultValue={nav.searchParams.get("os") ?? ""} />
      </label>
      <label className="space-y-1.5 text-caption">{t($ => $.installations.group)}<Input name="group" defaultValue={nav.searchParams.get("group") ?? ""} />
      </label>
      <div className="flex gap-2">
        <Button type="submit">{t($ => $.installations.apply)}</Button>
        <Button type="button" variant="ghost" onClick={() => nav.push(nav.pathname)}>{t($ => $.installations.reset)}</Button>
      </div>
    </form>
    {data?.dataQuality === "unavailable" && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.dependency_unavailable)}</p>}{data?.dataQuality === "partial" && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.truncated)}</p>}
    <InstallationReadState pending={query.isPending} error={query.isError} empty={!data?.items.length} retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t($ => $.installations.name)}</TableHead>
              <TableHead>{t($ => $.installations.client)}</TableHead>
              <TableHead>{t($ => $.installations.daemon)}</TableHead>
              <TableHead>{t($ => $.installations.readiness)}</TableHead>
              <TableHead>{t($ => $.installations.runtime_count)}</TableHead>
              <TableHead>{t($ => $.installations.version)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell className="min-w-64 max-w-64 whitespace-normal">
              <AppLink href={`/admin/installations/${item.id}`} className="block break-all text-body underline underline-offset-4">{item.displayName || item.id}</AppLink>
              <p className="text-caption text-muted-foreground">{item.os ?? t($ => $.installations.unknown)}</p>
            </TableCell>
            <TableCell>
              <InstallationAxis axis={item.clientActivity} />
            </TableCell>
            <TableCell>
              <InstallationAxis axis={item.daemonReachability} />
            </TableCell>
            <TableCell>
              <InstallationAxis axis={item.executionReadiness} />
            </TableCell>
            <TableCell>{item.runtimeCount}</TableCell>
            <TableCell>{item.desktopVersion ?? t($ => $.installations.unknown)}</TableCell>
          </TableRow>)}</TableBody>
        </Table>
      </div>
    </InstallationReadState>
    {query.isError && data && <p className="text-caption text-muted-foreground">{t($ => $.installations.last_seen)}: <time dateTime={data.asOf}>{data.asOf}</time>
    </p>}{data && !query.isError && <InstallationPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}<p className="max-w-3xl text-caption text-muted-foreground">{t($ => $.installations.readiness_notice)}</p>
  </section>;
}
