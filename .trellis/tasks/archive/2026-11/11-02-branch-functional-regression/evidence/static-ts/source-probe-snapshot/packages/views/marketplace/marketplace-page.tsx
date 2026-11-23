"use client";

import { useState } from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { PageHeader, PAGE_GUTTER } from "../layout/page-header";
import { McpMarketPanel } from "../mcp";
import { currentPath, useNavigation } from "../navigation";
import { SkillMarketPanel } from "../skills";

export function MarketplacePage() {
  const workspaceId = useWorkspaceId();
  return <WorkspaceMarketplace key={workspaceId} workspaceId={workspaceId} />;
}

function WorkspaceMarketplace({ workspaceId }: { workspaceId: string }) {
  const { t } = useT("layout");
  const navigation = useNavigation();
  const type = navigation.searchParams.get("type") === "skills" ? "skills" : "mcp";
  const [visited, setVisited] = useState({ mcp: type === "mcp", skills: type === "skills" });
  // Remember entry from both tab clicks and browser history. The URL remains
  // the selection source; retained panels own their unfinished draft sessions.
  if (!visited[type]) setVisited({ ...visited, [type]: true });

  function selectType(value: unknown) {
    if ((value !== "mcp" && value !== "skills") || value === type) return;
    const searchParams = new URLSearchParams(navigation.searchParams);
    searchParams.set("type", value);
    navigation.push(currentPath({ ...navigation, searchParams }));
  }

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <PageHeader>
        <h1 className="truncate text-body font-medium">{t(($) => $.nav.marketplace)}</h1>
      </PageHeader>
      <Tabs value={type} onValueChange={selectType} className="min-h-0 min-w-0 flex-1 gap-0">
        <div className={cn("shrink-0 border-b py-2", PAGE_GUTTER)}>
          <TabsList variant="line" activateOnFocus={false} aria-label={t(($) => $.marketplace.type_label)}>
            <TabsTrigger value="mcp">{t(($) => $.marketplace.mcp)}</TabsTrigger>
            <TabsTrigger value="skills">{t(($) => $.marketplace.skills)}</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="mcp" keepMounted hidden={type !== "mcp"} inert={type !== "mcp"} className={cn("min-h-0 min-w-0 overflow-y-auto py-4", PAGE_GUTTER)}>
          {visited.mcp && <McpMarketPanel workspaceId={workspaceId} active={type === "mcp"} />}
        </TabsContent>
        <TabsContent value="skills" keepMounted hidden={type !== "skills"} inert={type !== "skills"} className={cn("min-h-0 min-w-0 overflow-y-auto py-4", PAGE_GUTTER)}>
          {visited.skills && <SkillMarketPanel workspaceId={workspaceId} active={type === "skills"} />}
        </TabsContent>
      </Tabs>
    </div>
  );
}
