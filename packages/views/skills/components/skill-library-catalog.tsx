"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronUp, Search } from "lucide-react";
import { useSkillsViewStore, type SkillMarketSource } from "@multica/core/skills/stores";
import type { SkillSummary } from "@multica/core/types";
import { skillTemplateListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { useSkillCategoryLabels } from "../hooks/use-skill-category-labels";
import { filterSkillMarketItems, getSkillMarketCategories } from "../lib/skill-market";
import { getSkillTemplateDiscoveryItems } from "../lib/skill-template-discovery";
import { SkillTemplateCard } from "./skill-template-card";

interface SkillLibraryCatalogProps {
  workspaceId: string;
  skills: readonly SkillSummary[] | undefined;
  skillsError: boolean;
  children: ReactNode;
  onPreview: (templateName: string, trigger: HTMLButtonElement) => void;
  onCreate: (trigger: HTMLButtonElement) => void;
}

// Key only the local browsing state. Workspace preferences rehydrate in core.
export function SkillLibraryCatalog(props: SkillLibraryCatalogProps) {
  return <SkillLibraryCatalogContent key={props.workspaceId} {...props} />;
}

const GRID_CLASS = "grid grid-cols-1 gap-3 @2xl/catalog:grid-cols-2 @5xl/catalog:grid-cols-3 @7xl/catalog:grid-cols-4";
const SHELF_CARD_CLASS = ["", "hidden @2xl/catalog:block", "hidden @5xl/catalog:block", "hidden @7xl/catalog:block"];

function SkillLibraryCatalogContent({ workspaceId, skills, skillsError, children, onPreview, onCreate }: SkillLibraryCatalogProps) {
  const { t } = useT("skills");
  const categoryLabels = useSkillCategoryLabels();
  const libraryView = useSkillsViewStore((state) => state.libraryView);
  const setLibraryView = useSkillsViewStore((state) => state.setLibraryView);
  const source = useSkillsViewStore((state) => state.marketSource);
  const setSource = useSkillsViewStore((state) => state.setMarketSource);
  const category = useSkillsViewStore((state) => state.marketCategory);
  const setCategory = useSkillsViewStore((state) => state.setMarketCategory);
  const collapsed = useSkillsViewStore((state) => state.templatesCollapsed);
  const setCollapsed = useSkillsViewStore((state) => state.setTemplatesCollapsed);
  const [search, setSearch] = useState("");
  const workspaceLoaded = skills !== undefined && !skillsError;
  const view = libraryView ?? (workspaceLoaded && skills.length === 0 ? "market" : "workspace");
  const catalog = useQuery(skillTemplateListOptions(workspaceId, { poll: view === "market" }));
  const hasData = catalog.data !== undefined;
  const items = useMemo(() => getSkillTemplateDiscoveryItems(catalog.data ?? [], t), [catalog.data, t]);
  const deploymentItems = items.filter((item) => item.source === "deployment");
  const categories = useMemo(() => getSkillMarketCategories(items, source), [items, source]);
  const filteredItems = filterSkillMarketItems(items, { source, category, search });
  const sourceCounts = { all: items.length, deployment: deploymentItems.length, builtin: items.length - deploymentItems.length };
  const sourceLabels = {
    all: t(($) => $.market.source_all),
    deployment: t(($) => $.market.source_deployment),
    builtin: t(($) => $.market.source_builtin),
  };

  useEffect(() => {
    if (libraryView === null && workspaceLoaded) setLibraryView(skills.length === 0 ? "market" : "workspace");
  }, [libraryView, workspaceLoaded, skills, setLibraryView]);

  useEffect(() => {
    if (hasData && !catalog.isError && category !== null && !categories.includes(category)) setCategory(null);
  }, [hasData, catalog.isError, category, categories, setCategory]);

  function selectSource(next: SkillMarketSource) {
    setSource(next);
    if (hasData && category !== null && !getSkillMarketCategories(items, next).includes(category)) setCategory(null);
    if (next === "deployment") void catalog.refetch({ cancelRefetch: false });
  }

  function browseSource(next: "deployment" | "builtin") {
    selectSource(next);
    setLibraryView("market");
  }

  const status = catalog.isError ? (
    <div role="alert" className="flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
      <span>{hasData ? t(($) => $.create.template.refresh_failed) : t(($) => $.create.template.load_failed)}</span>
      <Button variant="ghost" size="sm" onClick={() => void catalog.refetch()}>{t(($) => $.create.template.retry)}</Button>
    </div>
  ) : hasData && catalog.isFetching ? (
    <p role="status" className="text-caption text-muted-foreground">{t(($) => $.create.template.refreshing)}</p>
  ) : null;

  function renderCard(item: (typeof items)[number]) {
    return <SkillTemplateCard item={item} skills={skills} skillsError={skillsError} onPreview={onPreview} />;
  }

  return (
    <Tabs value={view} onValueChange={(next) => { if (next === "workspace" || next === "market") setLibraryView(next); }} className="min-h-0 min-w-0 flex-1 gap-0 @container/catalog">
      <TabsList variant="line" aria-label={t(($) => $.market.views_label)} className="mx-4 mb-1 max-w-[calc(100%-2rem)] shrink-0 items-stretch group-data-horizontal/tabs:h-auto @2xl/catalog:mx-6">
        <TabsTrigger value="workspace" onClick={() => setLibraryView("workspace")} aria-label={t(($) => $.market.workspace)} className="min-h-10 flex-none px-2 @2xl/catalog:px-3">
          {t(($) => $.market.workspace)}
          {workspaceLoaded && <span className="text-caption tabular-nums text-muted-foreground">{skills.length}</span>}
        </TabsTrigger>
        <TabsTrigger value="market" onClick={() => setLibraryView("market")} aria-label={t(($) => $.market.title)} className="min-h-10 flex-none px-2 @2xl/catalog:px-3">
          {t(($) => $.market.title)}
          {hasData && <span className="text-caption tabular-nums text-muted-foreground">{items.length}</span>}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="workspace" keepMounted className="flex min-h-0 min-w-0 flex-col data-hidden:hidden">
        {view === "workspace" && <section aria-label={t(($) => $.market.source_deployment)} className="shrink-0 px-4 py-3 @2xl/catalog:px-6">
          {hasData && deploymentItems.length === 0 ? (
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <p className="text-caption text-muted-foreground">{t(($) => $.market.deployment_empty)}</p>
              <Button size="sm" variant="ghost" onClick={() => browseSource("builtin")}>{t(($) => $.market.browse_builtin)}</Button>
            </div>
          ) : (
            <Collapsible open={!collapsed} onOpenChange={(open) => setCollapsed(!open)}>
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="min-w-0 text-title font-medium">{t(($) => $.market.source_deployment)}</h2>
                {hasData && <span className="text-caption text-muted-foreground">{t(($) => $.market.template_count, { count: deploymentItems.length })}</span>}
                <div className="ml-auto flex items-center gap-1">
                  <Button variant="ghost" size="sm" onClick={() => browseSource("deployment")}>{t(($) => $.market.browse_all)}</Button>
                  <CollapsibleTrigger render={<Button variant="ghost" size="icon-sm" />} aria-label={collapsed ? t(($) => $.market.expand) : t(($) => $.market.collapse)}>
                    {collapsed ? <ChevronDown aria-hidden className="size-4" /> : <ChevronUp aria-hidden className="size-4" />}
                  </CollapsibleTrigger>
                </div>
              </div>
              <CollapsibleContent>
                {hasData ? (
                  <ul className={cn(GRID_CLASS, "pt-3")}>
                    {deploymentItems.slice(0, 4).map((item, index) => <li key={item.template.name} className={cn("min-w-0", SHELF_CARD_CLASS[index])}>{renderCard(item)}</li>)}
                  </ul>
                ) : !catalog.isError && <CatalogSkeleton label={t(($) => $.create.template.loading)} compact />}
              </CollapsibleContent>
            </Collapsible>
          )}
          {status && <div className="pt-2">{status}</div>}
        </section>}
        {children}
      </TabsContent>

      <TabsContent value="market" className="min-h-0 min-w-0 overflow-y-auto px-4 pb-12 pt-4 @2xl/catalog:px-6">
        <div className="space-y-5">
          <div className="space-y-1">
            <h2 className="text-title font-medium">{workspaceLoaded && skills.length === 0 ? t(($) => $.market.start_title) : t(($) => $.market.browse_title)}</h2>
            <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.market.description)}</p>
          </div>
          <div className="space-y-3">
            <div className="relative max-w-xl">
              <Search aria-hidden className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input value={search} onChange={(event) => setSearch(event.target.value)} aria-label={t(($) => $.market.search)} placeholder={t(($) => $.market.search_placeholder)} className="pl-9" />
            </div>
            <Tabs value={source} onValueChange={(next) => { if (next === "all" || next === "deployment" || next === "builtin") selectSource(next); }}>
              <TabsList aria-label={t(($) => $.market.sources_label)} className="max-w-full flex-wrap justify-start gap-1 bg-transparent p-0 group-data-horizontal/tabs:h-auto">
                {(["all", "deployment", "builtin"] as const).map((value) => (
                  <TabsTrigger key={value} value={value} aria-label={sourceLabels[value]} className="min-h-9 flex-none whitespace-normal px-3 data-active:border-border data-active:bg-muted data-active:font-semibold dark:data-active:bg-muted">
                    {sourceLabels[value]}
                    {hasData && <span className="text-caption tabular-nums text-muted-foreground">{sourceCounts[value]}</span>}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            {categories.length > 1 && (
              <div role="group" aria-label={t(($) => $.categories.section_title)} className="flex flex-wrap gap-1.5">
                <Button size="sm" variant={category === null ? "secondary" : "ghost"} aria-pressed={category === null} onClick={() => setCategory(null)}>{t(($) => $.categories.all)}</Button>
                {categories.map((value) => (
                  <Button key={value} size="sm" variant={category === value ? "secondary" : "ghost"} aria-pressed={category === value} className={category === value ? "font-semibold" : "text-muted-foreground"} onClick={() => setCategory(value)}>{categoryLabels[value]}</Button>
                ))}
              </div>
            )}
          </div>
          {status}
          {!hasData ? (
            !catalog.isError && <CatalogSkeleton label={t(($) => $.create.template.loading)} />
          ) : items.length === 0 ? (
            <div className="space-y-3 py-8">
              <h3 className="text-title-sm font-medium">{t(($) => $.market.empty_title)}</h3>
              <p className="text-body text-muted-foreground">{t(($) => $.market.empty_description)}</p>
              <Button onClick={(event) => onCreate(event.currentTarget)}>{t(($) => $.page.new_skill)}</Button>
            </div>
          ) : sourceCounts[source] === 0 ? (
            <div className="space-y-3 py-5">
              <p className="text-body text-muted-foreground">{source === "deployment" ? t(($) => $.market.deployment_empty) : t(($) => $.market.builtin_empty)}</p>
              <Button variant="outline" size="sm" onClick={() => selectSource(source === "deployment" ? "builtin" : "all")}>{source === "deployment" ? t(($) => $.market.browse_builtin) : t(($) => $.market.browse_all)}</Button>
            </div>
          ) : (
            <>
              {source === "all" && deploymentItems.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.market.deployment_empty)}</p>}
              <p role="status" className="text-caption text-muted-foreground">{t(($) => $.market.result_count, { count: filteredItems.length })}</p>
              {filteredItems.length === 0 ? (
                <div className="space-y-3 py-5">
                  <p className="text-body text-muted-foreground">{search.trim() ? t(($) => $.market.search_empty, { query: search.trim() }) : t(($) => $.market.filter_empty)}</p>
                  <Button variant="outline" size="sm" onClick={() => { setSearch(""); setCategory(null); }}>{t(($) => $.market.clear_filters)}</Button>
                </div>
              ) : (
                <ul className={GRID_CLASS}>{filteredItems.map((item) => <li key={item.template.name} className="min-w-0">{renderCard(item)}</li>)}</ul>
              )}
            </>
          )}
        </div>
      </TabsContent>
    </Tabs>
  );
}

function CatalogSkeleton({ label, compact = false }: { label: string; compact?: boolean }) {
  return (
    <div role="status" aria-label={label} className={cn(GRID_CLASS, compact && "pt-3")}>
      <span className="sr-only">{label}</span>
      {Array.from({ length: compact ? 4 : 6 }, (_, index) => (
        <div key={index} className={cn("space-y-3 rounded-lg border p-4", compact && SHELF_CARD_CLASS[index])}>
          <Skeleton className="size-10 rounded-lg" />
          <Skeleton className="h-5 w-3/4" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      ))}
    </div>
  );
}
