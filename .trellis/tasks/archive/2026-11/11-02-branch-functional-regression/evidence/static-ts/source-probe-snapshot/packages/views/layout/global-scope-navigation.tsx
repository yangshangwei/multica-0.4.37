"use client";

import type { ReactNode } from "react";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

interface GlobalScopeNavigationProps {
  activeScope: "workspace" | "account" | "none";
  onWorkspace(): void;
  onCollaboration(): void;
  children?: ReactNode;
  className?: string;
}

/** Scope switches resume a location; page links continue to open destinations. */
export function GlobalScopeNavigation({ activeScope, onWorkspace, onCollaboration, children, className }: GlobalScopeNavigationProps) {
  const { t } = useT("layout");
  const { t: collaboration } = useT("collaboration");
  const buttonClass = "min-h-9 shrink-0 rounded-md px-3 text-body text-muted-foreground hover:bg-accent hover:text-foreground aria-[current=page]:bg-accent aria-[current=page]:font-semibold aria-[current=page]:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11";
  return <nav aria-label={t(($) => $.global_navigation.label)} className={cn("flex min-w-0 items-center gap-1", className)}>
    <button type="button" aria-current={activeScope === "workspace" ? "page" : undefined} className={buttonClass}
      onClick={() => { if (activeScope !== "workspace") onWorkspace(); }}>{t(($) => $.sidebar.workspaces_label)}</button>
    <button type="button" aria-current={activeScope === "account" ? "page" : undefined} className={buttonClass}
      onClick={() => { if (activeScope !== "account") onCollaboration(); }}>{collaboration(($) => $.page.title)}</button>
    {children && <div className="ml-2 flex min-w-0 items-center gap-1">{children}</div>}
  </nav>;
}
