"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Globe, Loader2, Search, Sparkles } from "lucide-react";
import type {
  McpServerTemplate,
  WorkspaceMcpServer,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@multica/ui/components/ui/tabs";
import { useT } from "../i18n";
import { useMcpServerTemplates } from "../settings/hooks/use-mcp-server-templates";
import { McpSetupDialog, type McpAgentContext } from "./mcp-setup-dialog";

export type McpCustomPreset = { name: string; config: Record<string, unknown> };

export function McpLibraryCatalog({
  workspaceId,
  servers,
  loaded,
  canManage,
  onCustom,
  children,
  customAction,
}: {
  workspaceId: string;
  servers: readonly WorkspaceMcpServer[] | undefined;
  loaded: boolean;
  canManage: boolean;
  onCustom: (preset: McpCustomPreset) => void;
  children: ReactNode;
  customAction?: ReactNode;
}) {
  const { t } = useT("settings");
  const [choice, setChoice] = useState<"workspace" | "market" | null>(null);
  useEffect(() => {
    if (choice === null && loaded && servers)
      setChoice(servers.length === 0 ? "market" : "workspace");
  }, [choice, loaded, servers]);
  const view =
    choice ?? (loaded && servers?.length === 0 ? "market" : "workspace");
  return (
    <Tabs
      value={view}
      onValueChange={(value) => setChoice(value as "workspace" | "market")}
      className="gap-5"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <TabsList aria-label={t(($) => $.mcp.title)} variant="line">
          <TabsTrigger value="workspace" onClick={() => setChoice("workspace")}>
            {t(($) => $.mcp.market.workspace)}
          </TabsTrigger>
          <TabsTrigger value="market" onClick={() => setChoice("market")}>
            {t(($) => $.mcp.market.title)}
          </TabsTrigger>
        </TabsList>
        {customAction}
      </div>
      <TabsContent value="workspace">{children}</TabsContent>
      <TabsContent value="market">
        <McpTemplateCatalog
          workspaceId={workspaceId}
          servers={servers}
          canManage={canManage}
          onCustom={onCustom}
        />
      </TabsContent>
    </Tabs>
  );
}

export function McpTemplateCatalog({
  workspaceId,
  servers,
  canManage,
  onCustom,
  agentContext,
}: {
  workspaceId: string;
  servers: readonly WorkspaceMcpServer[] | undefined;
  canManage: boolean;
  onCustom: (preset: McpCustomPreset) => void;
  agentContext?: McpAgentContext;
}) {
  const { t } = useT("settings");
  const templates = useMcpServerTemplates(workspaceId);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [selected, setSelected] = useState<McpServerTemplate | null>(null);
  const usable = (templates.data ?? []).filter(
    (template) => template.key && Object.keys(template.config ?? {}).length > 0,
  );
  const matches = usable.filter(
    (template) =>
      (category === "all" || template.category === category) &&
      `${template.key} ${template.title} ${template.description}`
        .toLocaleLowerCase()
        .includes(search.trim().toLocaleLowerCase()),
  );
  const categories = [
    { value: "all", label: t(($) => $.mcp.market.all) },
    { value: "browser", label: t(($) => $.mcp.market.browser) },
    { value: "reasoning", label: t(($) => $.mcp.market.reasoning) },
  ];
  return (
    <div className="space-y-4" data-testid="mcp-market">
      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground"
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
      <div
        className="flex flex-wrap gap-2"
        aria-label={t(($) => $.mcp.market.all)}
      >
        {categories.map((item) => (
          <Button
            key={item.value}
            size="sm"
            variant={category === item.value ? "secondary" : "ghost"}
            aria-pressed={category === item.value}
            onClick={() => setCategory(item.value)}
          >
            {item.label}
          </Button>
        ))}
      </div>
      {templates.isPending ? (
        <p
          role="status"
          className="flex items-center gap-2 py-8 text-caption text-muted-foreground"
        >
          <Loader2
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />
          {t(($) => $.mcp.builtin_loading)}
        </p>
      ) : null}
      {templates.isError ? (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-lg border p-4"
        >
          <p className="text-caption">{t(($) => $.mcp.builtin_error)}</p>
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
            {t(($) => $.mcp.market.no_results)}
          </p>
          {search || category !== "all" ? (
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
          ) : (
            <p className="mt-2 text-caption text-muted-foreground">
              {t(($) => $.mcp.builtin_empty)}
            </p>
          )}
        </div>
      ) : null}
      <div className="grid gap-3 sm:grid-cols-2">
        {matches.map((template) => {
          const related =
            servers?.filter((server) => server.template_key === template.key) ??
            [];
          const Icon = template.category === "reasoning" ? Sparkles : Globe;
          return (
            <div
              key={template.key}
              className="overflow-hidden rounded-xl border bg-surface-raised/40"
            >
              <button
                type="button"
                aria-label={t(($) => $.mcp.market.view, {
                  name: template.title || template.key,
                })}
                className="flex h-full min-h-36 w-full flex-col items-start gap-3 p-4 text-left transition-colors hover:bg-muted/40 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
                onClick={() => setSelected(template)}
              >
                <Icon
                  className="size-5 text-muted-foreground"
                  aria-hidden="true"
                />
                <div className="min-w-0">
                  <h3 className="break-words text-body font-medium">
                    {template.title || template.key}
                  </h3>
                  <p className="mt-1 text-caption leading-5 text-muted-foreground">
                    {template.description}
                  </p>
                </div>
                {related.length > 0 ? (
                  <p className="mt-auto break-all text-caption text-muted-foreground">
                    {t(($) => $.mcp.market.related)}:{" "}
                    {related.map((server) => server.name).join(", ")}
                  </p>
                ) : null}
              </button>
            </div>
          );
        })}
      </div>
      {selected ? (
        <McpSetupDialog
          workspaceId={workspaceId}
          template={selected}
          available={usable.some(
            (template) =>
              template.key === selected.key &&
              template.version === selected.version,
          )}
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
