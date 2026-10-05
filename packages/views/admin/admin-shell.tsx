"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useAdminAccess, type AdminIdentity } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@multica/ui/components/ui/sheet";
import { useIsMobile } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import {
  ArrowLeft, Bell, ChevronRight, LayoutDashboard, ListTodo, Loader2,
  LogOut, Menu, Monitor, ServerOff, Settings, ShieldCheck, ShieldX, Users, X,
  type LucideIcon,
} from "lucide-react";
import { useT } from "../i18n";
import { AppLink, useNavigation } from "../navigation";
import { useLogout } from "../auth/use-logout";
import styles from "./admin-visual.module.css";

// Content controls keep their desktop density for fine pointers and reach 44px on narrow or
// coarse-pointer layouts. Summaries grow through padding and min-height rather than a display
// change, so they keep their disclosure marker.
const contentTouchClass = [
  "max-md:[&_[data-slot=button]]:min-h-11 max-md:[&_[data-slot=button]]:min-w-11 pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11",
  "max-md:[&_[data-slot=input]]:min-h-11 pointer-coarse:[&_[data-slot=input]]:min-h-11",
  "max-md:[&_select]:min-h-11 pointer-coarse:[&_select]:min-h-11",
  "max-md:[&_summary]:min-h-11 max-md:[&_summary]:py-3 pointer-coarse:[&_summary]:min-h-11 pointer-coarse:[&_summary]:py-3",
].join(" ");

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
  const isMobile = useIsMobile();
  const [menuOpen, setMenuOpen] = useState(false);
  const selectedLink = useRef<HTMLAnchorElement>(null);
  const ready = status === "ready" && !!identity;
  useEffect(() => {
    if (menuOpen && (!isMobile || !ready)) {
      setMenuOpen(false);
      if (!isMobile) selectedLink.current?.focus();
    }
  }, [isMobile, menuOpen, ready]);
  const showAccess = pathname === "/admin" || pathname === "/admin/";
  const matches = (href: string) => href === "/admin"
    ? showAccess : pathname === href || pathname.startsWith(`${href}/`);
  const destinations: {
    href: string;
    label: string;
    icon: LucideIcon;
    related?: string;
    children?: { href: string; label: string }[];
  }[] = [
    { href: "/admin", label: t(($) => $.observability.title), icon: LayoutDashboard },
    { href: "/admin/users", label: t(($) => $.shell.accounts), icon: Users, children: [
      { href: "/admin/users", label: t(($) => $.workspaces.accounts) },
      { href: "/admin/workspaces", label: t(($) => $.workspaces.title) },
    ] },
    { href: "/admin/installations", label: t(($) => $.shell.installations), icon: Monitor },
    { href: "/admin/tasks", label: t(($) => $.shell.executions), icon: ListTodo, related: "/admin/issues" },
    { href: "/admin/alerts", label: t(($) => $.shell.monitoring), icon: Bell, children: [
      { href: "/admin/alerts", label: t(($) => $.alerts.title) },
      { href: "/admin/health", label: t(($) => $.health.title) },
    ] },
    { href: "/admin/settings", label: t(($) => $.shell.system), icon: Settings, children: [
      { href: "/admin/administrators", label: t(($) => $.shell.administrators) },
      { href: "/admin/resources", label: t(($) => $.shell.resources) },
      { href: "/admin/audit", label: t(($) => $.audit.title) },
      { href: "/admin/settings", label: t(($) => $.settings.title) },
    ] },
  ];
  const current = destinations.find(item => matches(item.href) ||
    (item.related && matches(item.related)) || item.children?.some(child => matches(child.href)));
  const currentChild = current?.children?.find(item => matches(item.href));
  const closeMenu = () => setMenuOpen(false);
  const linkClass = "flex min-h-11 min-w-0 items-center gap-3 rounded-md px-3 py-2 text-body outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring md:min-h-10 pointer-coarse:min-h-11";
  const navigation = ready && (
    <nav aria-label={t(($) => $.shell.navigation)} className="min-h-0 flex-1 overflow-y-auto p-3">
      <ul className="space-y-1">
        {destinations.map(item => {
          const active = item === current;
          const Icon = item.icon;
          return (
            <li key={item.href}>
              <AppLink href={item.href} onClick={closeMenu}
                ref={active && !item.children ? selectedLink : undefined}
                aria-current={active && !item.children ? "page" : undefined}
                className={cn(linkClass, active
                  ? "bg-sidebar-accent font-semibold text-sidebar-accent-foreground hover:bg-sidebar-accent"
                  : "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground")}>
                <Icon className="size-4 shrink-0" aria-hidden="true" />
                <span className="min-w-0 [overflow-wrap:anywhere]">{item.label}</span>
              </AppLink>
              {active && item.children && (
                <ul aria-label={t(($) => $.shell.section_navigation)} className="my-2 ml-5 space-y-1 border-l border-sidebar-border pl-3">
                  {item.children.map(child => (
                    <li key={child.href}>
                      <AppLink href={child.href} onClick={closeMenu}
                        ref={child === currentChild ? selectedLink : undefined}
                        aria-current={child === currentChild ? "page" : undefined}
                        className={cn(linkClass, "text-label", child === currentChild
                          ? "font-semibold text-sidebar-foreground hover:bg-sidebar-accent"
                          : "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground")}>
                        <span className="min-w-0 [overflow-wrap:anywhere]">{child.label}</span>
                      </AppLink>
                    </li>
                  ))}
                </ul>
              )}
            </li>
          );
        })}
      </ul>
    </nav>
  );
  const footer = (
    <div className="mt-auto shrink-0 space-y-1 border-t border-sidebar-border p-3">
      <AppLink href="/login" onClick={closeMenu} className={cn(linkClass, "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground")}>
        <ArrowLeft className="size-4 shrink-0" aria-hidden="true" />{t(($) => $.shell.return_to_app)}
      </AppLink>
      <Button variant="ghost" className="min-h-11 w-full justify-start gap-3 px-3 text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground md:min-h-10 pointer-coarse:min-h-11" onClick={logout}>
        <LogOut className="size-4" aria-hidden="true" />{t(($) => $.shell.logout)}
      </Button>
    </div>
  );

  return (
    <div data-admin-page className={cn(styles.theme, styles.shell, "flex h-full min-h-0 text-foreground")}>
      {!isMobile && (
        <aside className={cn(styles.sidebar, "flex w-60 shrink-0 flex-col border-r border-sidebar-border text-sidebar-foreground")}>
          <AppLink href="/admin" className="flex min-h-16 shrink-0 items-center gap-2.5 px-6 py-4 text-body-lg font-semibold outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring">
            <ShieldCheck className="size-5 shrink-0" aria-hidden="true" />
            <span>{t(($) => $.shell.title)}</span>
          </AppLink>
          {navigation}
          {footer}
        </aside>
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className={cn(styles.topbar, "flex min-h-16 shrink-0 items-center gap-3 border-b border-surface-border px-4 sm:px-8")}>
          {isMobile && (
            <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
              <SheetTrigger render={<Button variant="ghost" size="icon" className="size-11 shrink-0" aria-label={t(($) => $.shell.open_navigation)} />}>
                <Menu className="size-5" aria-hidden="true" />
              </SheetTrigger>
              <SheetContent side="left" showCloseButton={false} initialFocus={selectedLink} aria-describedby={undefined}
                className={cn(styles.theme, styles.sidebar, "gap-0 text-sidebar-foreground data-[side=left]:w-72 data-[side=left]:max-w-full")}>
                <SheetHeader className="min-h-16 flex-row items-center justify-between gap-2 px-5">
                  <SheetTitle className="text-body-lg">{t(($) => $.shell.navigation)}</SheetTitle>
                  <SheetClose render={<Button variant="ghost" size="icon" className="size-11" aria-label={t(($) => $.shell.close_navigation)} />}>
                    <X className="size-4" aria-hidden="true" />
                  </SheetClose>
                </SheetHeader>
                {navigation}
                {footer}
              </SheetContent>
            </Sheet>
          )}
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 py-3 text-label">
            <span className={currentChild ? "text-muted-foreground" : "font-medium"}>{current?.label ?? t(($) => $.shell.title)}</span>
            {currentChild && <><ChevronRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" /><span className="font-medium">{currentChild.label}</span></>}
          </div>
        </header>
      <main className={cn(styles.pageCanvas, "min-h-0 flex-1 overflow-y-auto p-5 sm:p-8", contentTouchClass)}>
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
          <div className={cn(styles.pageContent, "min-w-0 space-y-6")}>
            {showAccess ? children : <div className={styles.pagePanel}>{children}</div>}
            {showAccess && <details className={cn(styles.access, "space-y-4 text-body")}>
              <summary className="cursor-pointer text-muted-foreground">{t(($) => $.access.title)}</summary>
              <ScopeDetails identity={identity} />
              <p className="text-caption text-muted-foreground">{t(($) => $.access.scope_notice)}</p>
            </details>}
          </div>
        )}
      </main>
      </div>
    </div>
  );
}
