"use client";

import { useCallback, useRef } from "react";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { RoleTemplateCard } from "../agents/create/role-template-card";
import { useRoleTemplates } from "../agents/create/use-role-templates";
import { useT } from "../i18n";
import { PAGE_GUTTER } from "../layout/page-header";
import { useRestoredScrollRef, useRestoredViewState, useViewStateWriter } from "../platform/scroll-restoration";

export function AgentTemplatesMarketPanel({ workspaceId, active }: { workspaceId: string; active: boolean }) {
  return <AgentTemplatesMarketPanelContent key={workspaceId} workspaceId={workspaceId} active={active} />;
}

function AgentTemplatesMarketPanelContent({ workspaceId, active }: { workspaceId: string; active: boolean }) {
  const { t } = useT("agents");
  const paths = useWorkspacePaths();
  const { data, isPending, isError, refetch } = useRoleTemplates({ enabled: active && !!workspaceId });
  const templates = (data ?? []).filter((template) => template.key);
  const restoredTop = useRestoredViewState("agent-template-market-scroll");
  const restoreScroll = useRestoredScrollRef("agent-template-market");
  const writeViewState = useViewStateWriter();
  const scrollElement = useRef<HTMLElement | null>(null);
  const didRestore = useRef(false);
  const ready = !isPending && !isError;
  const attachScroll = useCallback((element: HTMLElement | null) => {
    scrollElement.current = element;
    if (!element || !ready || didRestore.current) return;
    didRestore.current = true;
    const top = restoredTop === undefined ? undefined : Number(restoredTop);
    if (top !== undefined && Number.isFinite(top) && top >= 0) element.scrollTop = top;
    else restoreScroll(element);
  }, [ready, restoredTop, restoreScroll]);
  const rememberScroll = () => {
    if (!active || !ready || !didRestore.current) return;
    writeViewState("agent-template-market-scroll", String(scrollElement.current?.scrollTop ?? 0));
  };

  return (
    <section
      ref={attachScroll}
      onScroll={rememberScroll}
      aria-label={t(($) => $.role_templates.market_title)}
      aria-busy={active && isPending}
      data-tab-scroll-root="agent-template-market"
      className={cn("min-h-0 min-w-0 flex-1 overflow-y-auto py-6", PAGE_GUTTER)}
    >
      <div className="mx-auto w-full max-w-5xl">
        <h2 className="text-balance text-title-lg font-semibold">{t(($) => $.role_templates.market_title)}</h2>
        <p className="mt-2 max-w-2xl text-pretty text-body text-muted-foreground">{t(($) => $.role_templates.market_description)}</p>
        {isError ? (
          <div role="alert" className="mt-8 flex flex-wrap items-center gap-3 text-body text-muted-foreground">
            <p>{t(($) => $.role_templates.load_failed)}</p>
            <Button type="button" variant="outline" onClick={() => void refetch()}>{t(($) => $.role_templates.retry)}</Button>
          </div>
        ) : isPending ? active ? (
          <div role="status" className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <span className="sr-only">{t(($) => $.role_templates.market_loading)}</span>
            {[0, 1, 2, 3, 4, 5].map((key) => <Skeleton key={key} className="h-40 rounded-xl" />)}
          </div>
        ) : null : templates.length === 0 ? (
          <p className="mt-8 text-body text-muted-foreground">{t(($) => $.role_templates.empty)}</p>
        ) : (
          <ul className="mt-6 grid gap-x-8 sm:grid-cols-2 lg:grid-cols-3">
            {templates.map((template) => (
              <RoleTemplateCard
                key={template.key}
                template={template}
                createHref={`${paths.newAgentTemplate()}?${new URLSearchParams({ template: template.key, source: "marketplace" })}`}
                createLabel={t(($) => $.role_templates.use_template)}
                onNavigate={rememberScroll}
              />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
