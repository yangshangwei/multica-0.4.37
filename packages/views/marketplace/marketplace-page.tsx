"use client";

import { useState } from "react";
import { Store } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { Tabs, TabsContent } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { CollectionPageHeader } from "../layout/collection-page";
import { PAGE_GUTTER } from "../layout/page-header";
import { McpMarketPanel } from "../mcp";
import { currentPath, useNavigation } from "../navigation";
import { SkillMarketPanel } from "../skills";
import { AgentTemplatesMarketPanel } from "./agent-templates-market-panel";
import { MARKETPLACE_CATEGORIES, MarketplaceCompactExample, MarketplaceWelcome, type MarketplaceCategory } from "./marketplace-welcome";
import styles from "./marketplace-welcome.module.css";
import { SquadTemplatesMarketPanel } from "./squad-templates-market-panel";

const catalogs = {
  mcp: McpMarketPanel,
  skills: SkillMarketPanel,
  "squad-templates": SquadTemplatesMarketPanel,
};

export function MarketplacePage() {
  const workspaceId = useWorkspaceId();
  return <WorkspaceMarketplace key={workspaceId} workspaceId={workspaceId} />;
}

function WorkspaceMarketplace({ workspaceId }: { workspaceId: string }) {
  const { t } = useT("layout");
  const navigation = useNavigation();
  const selectedType = navigation.searchParams.get("type");
  const category = MARKETPLACE_CATEGORIES.find(({ value }) => value === selectedType) ?? MARKETPLACE_CATEGORIES[0];
  const type = category.value;
  const [visited, setVisited] = useState(() => new Set<MarketplaceCategory["value"]>([type]));
  // Remember entry from both tab clicks and browser history. The URL remains
  // the selection source; retained panels own their unfinished draft sessions.
  if (!visited.has(type)) setVisited(new Set([...visited, type]));

  function selectType(value: unknown) {
    const next = MARKETPLACE_CATEGORIES.find((category) => category.value === value);
    if (!next || next.value === type) return;
    const searchParams = new URLSearchParams(navigation.searchParams);
    searchParams.set("type", next.value);
    navigation.push(currentPath({ ...navigation, searchParams }));
  }

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1 flex-col", styles.marketplace)}>
      <CollectionPageHeader icon={Store} title={t(($) => $.nav.marketplace)} />
      <Tabs value={type} onValueChange={selectType} className="min-h-0 min-w-0 flex-1 gap-0">
        <MarketplaceWelcome category={category} />
        {MARKETPLACE_CATEGORIES.map((item) => {
          const active = item.value === type;
          // Agent templates own their scroller so the creation route can restore it.
          const Catalog = item.value === "agent-templates" ? null : catalogs[item.value];
          const example = <MarketplaceCompactExample category={item} active={active} />;

          return (
            <TabsContent
              key={item.value}
              value={item.value}
              keepMounted
              hidden={!active}
              inert={!active}
              className={Catalog ? cn("min-h-0 min-w-0 overflow-y-auto py-4", PAGE_GUTTER) : "flex min-h-0 min-w-0 flex-col overflow-hidden"}
            >
              {visited.has(item.value) && (Catalog ? (
                <>
                  {example}
                  <Catalog workspaceId={workspaceId} active={active} />
                </>
              ) : (
                <AgentTemplatesMarketPanel workspaceId={workspaceId} active={active} leading={example} />
              ))}
            </TabsContent>
          );
        })}
      </Tabs>
    </div>
  );
}
