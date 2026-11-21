"use client";

import type { ReactNode } from "react";
import { AppSidebar } from "@multica/views/layout";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "@multica/ui/components/ui/sidebar";
import { FeedbackModalHost } from "@multica/views/modals/registry";
import { useWebSidebarNavigation } from "./scope-shell";

/** Account chrome previews workspace shortcuts without mounting workspace resources. */
export function WebAccountShell({ children }: { children: ReactNode }) {
  const sidebar = useWebSidebarNavigation();
  return <SidebarProvider hasExternalTrigger className="h-full min-h-0 bg-app-shell">
    <AppSidebar {...sidebar} />
    <FeedbackModalHost />
    <SidebarInset className="relative min-h-0 min-w-0 overflow-hidden">
      <div className="flex min-h-12 shrink-0 items-center px-4 xl:hidden">
        <SidebarTrigger className="pointer-coarse:min-h-11 pointer-coarse:min-w-11" />
      </div>
      <div className="min-h-0 min-w-0 flex-1">{children}</div>
    </SidebarInset>
  </SidebarProvider>;
}
