"use client";

import { useEffect, useState, type ReactNode } from "react";
import { ArrowRight, Loader2, Search } from "lucide-react";
import type {
  McpServerTemplate,
  WorkspaceMcpServer,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@multica/ui/components/ui/tabs";
import { useT } from "../i18n";
import { mcpTransportLabel } from "../common/mcp-transport";
import { filterMcpTemplates, isUsableMcpTemplate, matchesMcpTemplate, mcpTemplateIdentity, mcpTemplateTransport, type McpCatalogSource } from "./mcp-catalog";
import { McpTemplateIcon } from "../common/mcp-template-icon";
import { useMcpServerTemplates } from "../settings/hooks/use-mcp-server-templates";
import { McpSetupDialog, type McpAgentContext } from "./mcp-setup-dialog";

export type McpCustomPreset = { name: string; config: Record<string, unknown> };

function usableTemplates(templates: McpServerTemplate[] | undefined) {
  return (templates ?? []).filter(isUsableMcpTemplate);
}

export function McpLibraryCatalog({
  workspaceId,
  servers,
  loaded,
  canManage,
  onCustom,
  children,
  customAction,
  presentation = "settings",
}: {
  workspaceId: string;
  servers: readonly WorkspaceMcpServer[] | undefined;
  loaded: boolean;
  canManage: boolean;
  onCustom: (preset: McpCustomPreset) => void;
  children: ReactNode;
  customAction?: ReactNode;
  presentation?: "page" | "settings";
}) {
  const { t } = useT("settings");
  const [choice, setChoice] = useState<"workspace" | "market" | null>(null);
  useEffect(() => {
    if (choice === null && loaded && servers)
      setChoice(servers.length === 0 ? "market" : "workspace");
  }, [choice, loaded, servers]);
  const view =
    choice ?? (loaded && servers?.length === 0 ? "market" : "workspace");
  const templates = useMcpServerTemplates(workspaceId, { poll: view === "market" });
  const { refetch } = templates;
  useEffect(() => {
    if (view === "market") void refetch();
  }, [view, refetch]);
  const panelClass =
    presentation === "page"
      ? "min-h-0 min-w-0 overflow-y-auto px-4 pb-12 pt-4 @2xl/mcp-library:px-6"
      : undefined;
  return (
    <Tabs
      value={view}
      onValueChange={(value) => setChoice(value as "workspace" | "market")}
      className={cn(
        "@container/mcp-library min-w-0 pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11 pointer-coarse:[&_[data-slot=input]]:min-h-11 pointer-coarse:[&_[role=tab]]:min-h-11 pointer-coarse:[&_[data-slot=tabs-list]]:h-auto",
        presentation === "page" ? "min-h-0 flex-1 gap-0" : "gap-5",
      )}
    >
      <div className={cn(
        "flex shrink-0 flex-wrap items-center justify-between gap-3",
        presentation === "page" && "mx-4 mb-1 @2xl/mcp-library:mx-6",
      )}>
        <TabsList
          aria-label={t(($) => $.mcp.title)}
          variant="line"
          className="max-w-full items-stretch data-[orientation=horizontal]:h-auto"
        >
          <TabsTrigger
            value="workspace"
            aria-label={t(($) => $.mcp.market.workspace)}
            className="min-h-10 flex-none px-2"
            onClick={() => setChoice("workspace")}
          >
            {t(($) => $.mcp.market.workspace)}
            {loaded && servers !== undefined ? (
              <span className="text-caption tabular-nums text-muted-foreground">{servers.length}</span>
            ) : null}
          </TabsTrigger>
          <TabsTrigger
            value="market"
            aria-label={t(($) => $.mcp.market.title)}
            className="min-h-10 flex-none px-2"
            onClick={() => setChoice("market")}
          >
            {t(($) => $.mcp.market.title)}
            {templates.data !== undefined ? (
              <span className="text-caption tabular-nums text-muted-foreground">
                {usableTemplates(templates.data).length}
              </span>
            ) : null}
          </TabsTrigger>
        </TabsList>
        {customAction}
      </div>
      <TabsContent value="workspace" className={panelClass}>
        {children}
      </TabsContent>
      <TabsContent value="market" className={panelClass}>
        <McpTemplateCatalogContent
          templates={templates}
          workspaceId={workspaceId}
          servers={servers}
          canManage={canManage}
          onCustom={onCustom}
        />
      </TabsContent>
    </Tabs>
  );
}

type McpTemplateCatalogProps = {
  workspaceId: string;
  servers: readonly WorkspaceMcpServer[] | undefined;
  canManage: boolean;
  onCustom: (preset: McpCustomPreset) => void;
  agentContext?: McpAgentContext;
};

export function McpTemplateCatalog(props: McpTemplateCatalogProps) {
  const templates = useMcpServerTemplates(props.workspaceId, { poll: true });
  return <McpTemplateCatalogContent {...props} templates={templates} />;
}

function McpTemplateCatalogContent({
  workspaceId,
  servers,
  canManage,
  onCustom,
  agentContext,
  templates,
}: McpTemplateCatalogProps & {
  templates: ReturnType<typeof useMcpServerTemplates>;
}) {
  const { t } = useT("settings");
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [source, setSource] = useState<McpCatalogSource>("all");
  const [selected, setSelected] = useState<McpServerTemplate | null>(null);
  const usable = usableTemplates(templates.data);
  const sourced = filterMcpTemplates(usable, source, "all", "");
  const activeCategory = category === "all" || sourced.some((template) => template.category === category)
    ? category : "all";
  const matches = filterMcpTemplates(usable, source, activeCategory, search);
  const sources: { value: McpCatalogSource; label: string }[] = [
    { value: "all", label: t(($) => $.mcp.market.source_all) },
    { value: "deployment", label: t(($) => $.mcp.market.source_deployment) },
    { value: "builtin", label: t(($) => $.mcp.market.source_builtin) },
  ];
  const latest = selected ? usable.find((template) => mcpTemplateIdentity(template) === mcpTemplateIdentity(selected)) : undefined;
  // Offer only categories the loaded catalog uses: a deployment can withdraw
  // every recipe in one, and its filter would only lead to an empty result.
  const categories = [
    { value: "all", label: t(($) => $.mcp.market.all) },
    { value: "browser", label: t(($) => $.mcp.market.browser) },
    { value: "reasoning", label: t(($) => $.mcp.market.reasoning) },
    { value: "coding", label: t(($) => $.mcp.market.coding) },
    { value: "database", label: t(($) => $.mcp.market.database) },
    { value: "documentation", label: t(($) => $.mcp.market.documentation_category) },
  ].filter(
    (item) =>
      item.value === "all" ||
      sourced.some((template) => template.category === item.value),
  );
  return (
    <div className="@container/mcp-market space-y-4" data-testid="mcp-market">
      <div className="relative max-w-xl">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          type="search"
          className="pl-9"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          aria-label={t(($) => $.mcp.market.search)}
          placeholder={t(($) => $.mcp.market.search)}
        />
      </div>
      <div role="group" className="flex flex-wrap gap-2" aria-label={t(($) => $.mcp.market.source_filter)}>
        {sources.map((item) => (
          <Button
            key={item.value}
            size="sm"
            variant={source === item.value ? "secondary" : "ghost"}
            className={source === item.value ? "font-semibold" : "font-normal"}
            aria-label={item.label}
            aria-pressed={source === item.value}
            onClick={() => {
              setSource(item.value);
              if (!filterMcpTemplates(usable, item.value, "all", "").some((template) => template.category === category)) setCategory("all");
              if (item.value === "deployment") void templates.refetch();
            }}
          >
            {item.label}
            {templates.data !== undefined ? (
              <span className="text-caption tabular-nums text-muted-foreground">
                {filterMcpTemplates(usable, item.value, "all", "").length}
              </span>
            ) : null}
          </Button>
        ))}
      </div>
      <div
        role="group"
        className="flex flex-wrap gap-2"
        aria-label={t(($) => $.mcp.market.all)}
      >
        {categories.map((item) => (
          <Button
            key={item.value}
            size="sm"
            variant={activeCategory === item.value ? "secondary" : "ghost"}
            className={activeCategory === item.value ? "font-semibold" : "font-normal"}
            aria-pressed={activeCategory === item.value}
            onClick={() => setCategory(item.value)}
          >
            {item.label}
          </Button>
        ))}
      </div>
      <p role="status" aria-atomic="true" className="text-caption text-muted-foreground">
        {templates.data !== undefined
          ? t(($) => $.mcp.market.results_count, { count: matches.length })
          : ""}
      </p>
      {templates.isPending ? (
        <p
          role="status"
          className="flex items-center gap-2 py-8 text-caption text-muted-foreground"
        >
          <Loader2
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />
          {t(($) => $.mcp.market.catalog_loading)}
        </p>
      ) : null}
      {templates.isError ? (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-lg border p-4"
        >
          <p className="text-caption">{templates.data === undefined ? t(($) => $.mcp.market.catalog_error) : t(($) => $.mcp.market.catalog_refresh_error)}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void templates.refetch()}
          >
            {t(($) => $.mcp.market.retry)}
          </Button>
        </div>
      ) : null}
      {!templates.isPending && matches.length === 0 && !templates.isError ? (
        <div className="py-10 text-center">
          <p className="text-body font-medium">
            {source === "deployment" && sourced.length === 0
              ? t(($) => $.mcp.market.deployment_empty)
              : t(($) => $.mcp.market.no_results)}
          </p>
          {search || activeCategory !== "all" ? (
            <Button
              variant="ghost"
              className="mt-2"
              onClick={() => {
                setSearch("");
                setCategory("all");
              }}
            >
              {t(($) => $.mcp.market.clear_filters)}
            </Button>
          ) : source !== "deployment" ? (
            <p className="mt-2 text-caption text-muted-foreground">
              {t(($) => $.mcp.market.catalog_empty)}
            </p>
          ) : null}
        </div>
      ) : null}
      <div className="grid grid-cols-1 gap-3 @2xl/mcp-market:grid-cols-2 @5xl/mcp-market:grid-cols-3">
        {matches.map((template) => {
          const related =
            servers?.filter((server) => matchesMcpTemplate(server, template)) ??
            [];
          const categoryLabel = categories.find(
            (item) => item.value === template.category,
          )?.label;
          return (
            <div
              key={mcpTemplateIdentity(template)}
              className="min-w-0 overflow-hidden rounded-lg border bg-card"
            >
              <button
                type="button"
                aria-label={t(($) => $.mcp.market.view, {
                  name: template.title || template.key,
                })}
                className="flex h-full w-full flex-col items-start gap-3 rounded-lg p-4 text-left transition-colors hover:bg-accent/40 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-foreground"
                onClick={() => setSelected(template)}
              >
                <div className="flex w-full min-w-0 items-center gap-3">
                  <McpTemplateIcon templateKey={template.key} source={template.source} category={template.category} />
                  <h3 className="min-w-0 text-title-sm font-medium [overflow-wrap:anywhere]">
                    {template.title || template.key}
                  </h3>
                </div>
                <p className="line-clamp-2 min-h-[2lh] text-body text-muted-foreground [overflow-wrap:anywhere]" title={template.description}>
                  {template.description}
                </p>
                <p className="text-caption text-muted-foreground">
                  {template.source === "deployment" ? t(($) => $.mcp.market.source_deployment) : t(($) => $.mcp.market.source_builtin)}
                  {" · "}{mcpTransportLabel(mcpTemplateTransport(template))}
                </p>
                {categoryLabel ? (
                  <p className="text-caption text-muted-foreground">{categoryLabel}</p>
                ) : null}
                {related.length > 0 ? (
                  <p className="break-all text-caption text-muted-foreground">
                    {t(($) => $.mcp.market.related)}:{" "}
                    {related.map((server) => server.name).join(", ")}
                  </p>
                ) : null}
                <span className="mt-auto inline-flex items-center gap-1.5 pt-1 text-body font-medium">
                  {t(($) => $.mcp.market.view_action)}
                  <ArrowRight className="size-3.5" aria-hidden="true" />
                </span>
              </button>
            </div>
          );
        })}
      </div>
      {selected ? (
        <McpSetupDialog
          workspaceId={workspaceId}
          template={selected}
          available={latest !== undefined && latest.version === selected.version}
          latestTemplate={latest}
          catalogUnavailable={templates.isError}
          onRefresh={() => templates.refetch()}
          onReload={async () => {
            const result = await templates.refetch();
            if (result.isError) return false;
            const next = usableTemplates(result.data).find((template) => mcpTemplateIdentity(template) === mcpTemplateIdentity(selected));
            if (!next) return false;
            setSelected(next);
            return true;
          }}
          servers={servers}
          canManage={canManage}
          agentContext={agentContext}
          onClose={() => setSelected(null)}
          onCustom={(preset) => {
            setSelected(null);
            onCustom(preset);
          }}
        />
      ) : null}
    </div>
  );
}
