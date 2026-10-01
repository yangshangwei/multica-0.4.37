"use client";

import type { ReactNode } from "react";
import { useAdminAccess, type AdminIdentity } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { ShieldCheck, ShieldX, ServerOff, Loader2 } from "lucide-react";
import { useT } from "../i18n";
import { AppLink, useNavigation } from "../navigation";
import { useLogout } from "../auth/use-logout";

function ScopeDetails({ identity }: { identity: AdminIdentity }) {
  const { t } = useT("admin");
  return (
    <dl className="grid min-w-0 gap-5 sm:grid-cols-2">
      <div className="min-w-0 space-y-1">
        <dt className="text-caption text-muted-foreground">{t(($) => $.shell.organization)}</dt>
        <dd className="break-all text-body">{identity.organizationId}</dd>
      </div>
      <div className="min-w-0 space-y-1">
        <dt className="text-caption text-muted-foreground">{t(($) => $.shell.role)}</dt>
        <dd className="break-all text-body">{identity.role}</dd>
      </div>
    </dl>
  );
}

export function AdminShell({ children }: { children?: ReactNode }) {
  const { t } = useT("admin");
  const { status, identity, retry } = useAdminAccess();
  const logout = useLogout();
  const { pathname } = useNavigation();
  const showAccess = pathname === "/admin" || pathname === "/admin/";
  const destinations = [
    { href: "/admin", label: t(($) => $.observability.title) },
    { href: "/admin/users", label: t(($) => $.shell.accounts) },
    { href: "/admin/installations", label: t(($) => $.shell.installations) },
    { href: "/admin/tasks", label: t(($) => $.shell.executions) },
    { href: "/admin/alerts", label: t(($) => $.shell.monitoring) },
    { href: "/admin/settings", label: t(($) => $.shell.system) },
  ];
  const accountSection = pathname.startsWith("/admin/users") || pathname.startsWith("/admin/workspaces");
  const monitoringSection = pathname.startsWith("/admin/alerts") || pathname.startsWith("/admin/health");
  const systemSection = ["/admin/settings", "/admin/administrators", "/admin/audit"].some(path => pathname.startsWith(path));
  const sectionLinks = accountSection ? [
    { href: "/admin/users", label: t(($) => $.workspaces.accounts) }, { href: "/admin/workspaces", label: t(($) => $.workspaces.title) },
  ] : monitoringSection ? [
    { href: "/admin/alerts", label: t(($) => $.alerts.title) }, { href: "/admin/health", label: t(($) => $.health.title) },
  ] : systemSection ? [
    { href: "/admin/administrators", label: t(($) => $.shell.administrators) }, { href: "/admin/audit", label: t(($) => $.audit.title) }, { href: "/admin/settings", label: t(($) => $.settings.title) },
  ] : [];

  return (
    <div className="flex h-full min-h-0 flex-col bg-background text-foreground">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-surface-border px-5 py-4 sm:px-8">
        <AppLink href="/admin" className="flex min-w-0 items-center gap-2 text-body-lg font-semibold">
          <ShieldCheck className="size-5 shrink-0" aria-hidden="true" />
          {t(($) => $.shell.title)}
        </AppLink>
        <div className="flex flex-wrap items-center gap-3">
          <AppLink href="/login" className="inline-flex min-h-11 items-center text-body text-muted-foreground hover:text-foreground focus-visible:outline-ring">
            {t(($) => $.shell.return_to_app)}
          </AppLink>
          <Button variant="ghost" className="min-h-11" onClick={logout}>{t(($) => $.shell.logout)}</Button>
        </div>
      </header>
      {status === "ready" && identity && (
        <nav aria-label={t(($) => $.shell.navigation)} className="flex shrink-0 gap-1 overflow-x-auto whitespace-nowrap px-5 pt-3 sm:px-8">
          {destinations.map(({ href, label }) => {
            const active = href === "/admin" ? showAccess : href === "/admin/users" ? accountSection : href === "/admin/alerts" ? monitoringSection : href === "/admin/settings" ? systemSection : pathname.startsWith(href) || (href === "/admin/tasks" && pathname.startsWith("/admin/issues"));
            return (
              <AppLink key={href} href={href} aria-current={active ? "page" : undefined}
                className={`inline-flex min-h-11 shrink-0 items-center rounded-md px-3 py-2 text-body hover:bg-accent focus-visible:outline-ring ${active ? "bg-accent font-semibold text-foreground" : "text-muted-foreground"}`}>
                {label}
              </AppLink>
            );
          })}
        </nav>
      )}
      {status === "ready" && identity && sectionLinks.length > 0 && <nav aria-label={t(($) => $.shell.section_navigation)} className="flex shrink-0 gap-4 overflow-x-auto whitespace-nowrap px-8 pt-2 sm:px-11">
        {sectionLinks.map(link => <AppLink key={link.href} href={link.href} aria-current={pathname.startsWith(link.href) ? "page" : undefined} className={`inline-flex min-h-11 shrink-0 items-center text-caption hover:underline focus-visible:outline-ring ${pathname.startsWith(link.href) ? "font-semibold text-foreground" : "text-muted-foreground"}`}>{link.label}</AppLink>)}
      </nav>}
      <main className="min-h-0 flex-1 overflow-y-auto px-5 py-8 sm:px-8">
        {status === "loading" || status === "signed_out" ? (
          <p role="status" className="flex items-center gap-2 text-body text-muted-foreground">
            <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
            {t(($) => $.state.loading)}
          </p>
        ) : status !== "ready" || !identity ? (
          <section role="alert" className="mx-auto flex max-w-xl flex-col items-start gap-4 py-8">
            {status === "denied" ? <ShieldX className="size-6 text-muted-foreground" aria-hidden="true" /> : <ServerOff className="size-6 text-muted-foreground" aria-hidden="true" />}
            <h1 className="text-title font-semibold">{status === "denied" ? t(($) => $.state.denied_title) : status === "unsupported" ? t(($) => $.state.unsupported_title) : t(($) => $.state.unavailable_title)}</h1>
            <p className="text-body text-muted-foreground">{status === "denied" ? t(($) => $.state.denied_description) : status === "unsupported" ? t(($) => $.state.unsupported_description) : t(($) => $.state.unavailable_description)}</p>
            <Button variant="outline" onClick={retry}>{t(($) => $.state.retry)}</Button>
          </section>
        ) : (
          <div className="mx-auto max-w-7xl space-y-8">
            {children}
            {showAccess && <details className="space-y-4 text-body">
              <summary className="cursor-pointer text-muted-foreground">{t(($) => $.access.title)}</summary>
              <ScopeDetails identity={identity} />
              <p className="text-caption text-muted-foreground">{t(($) => $.access.scope_notice)}</p>
            </details>}
          </div>
        )}
      </main>
    </div>
  );
}
