"use client";

import { useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { useSidebarPreferences, type SidebarGroup as SidebarGroupKey } from "@multica/core/navigation";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { SidebarGroup, SidebarGroupContent, SidebarGroupLabel } from "@multica/ui/components/ui/sidebar";
import { currentPath, useNavigation } from "../navigation";

export function useNavigationGroupDisclosure(group: SidebarGroupKey, active: boolean) {
  const navigation = useNavigation();
  const path = currentPath(navigation);
  const preferred = useSidebarPreferences((state) => state.groups[group]);
  const setPreference = useSidebarPreferences((state) => state.setGroupOpen);
  const [manual, setManual] = useState<{ path: string; open: boolean } | null>(null);
  if (manual !== null && manual.path !== path) setManual(null);
  const open = manual?.path === path ? manual.open : active || preferred;
  const onOpenChange = (next: boolean) => { setManual({ path, open: next }); setPreference(group, next); };
  return { open, onOpenChange };
}

export function NavigationGroup({ group, label, active, children }: { group: SidebarGroupKey; label: string; active: boolean; children: ReactNode }) {
  const disclosure = useNavigationGroupDisclosure(group, active);
  return <Collapsible {...disclosure}>
    <SidebarGroup><SidebarGroupLabel render={<CollapsibleTrigger />} className="group/trigger cursor-pointer gap-2 text-left hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground pointer-coarse:min-h-11">
      <span className="min-w-0 flex-1 truncate">{label}</span><ChevronRight aria-hidden className="size-3 transition-transform duration-150 motion-reduce:transition-none data-[panel-open]:rotate-90 group-data-[panel-open]/trigger:rotate-90" />
    </SidebarGroupLabel><CollapsibleContent><SidebarGroupContent>{children}</SidebarGroupContent></CollapsibleContent></SidebarGroup>
  </Collapsible>;
}
