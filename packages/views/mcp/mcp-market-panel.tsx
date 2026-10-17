"use client";

import { useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions, workspaceMcpServersOptions } from "@multica/core/workspace/queries";
import { useCreateWorkspaceMcpServer } from "@multica/core/workspace/mutations";
import { toast } from "sonner";
import { McpServerDialog } from "../agents/components/tabs/mcp-server-dialog";
import { useT } from "../i18n";
import { getSettingsQueryState, SettingsQueryFeedback } from "../settings/components/settings-query-feedback";
import { McpTemplateCatalog, type McpCustomPreset } from "./mcp-market";

export function McpMarketPanel({ workspaceId, active = true }: {
  workspaceId: string;
  active?: boolean;
}) {
  return <WorkspaceMcpMarket key={workspaceId} workspaceId={workspaceId} active={active} />;
}

function WorkspaceMcpMarket({ workspaceId, active }: {
  workspaceId: string;
  active: boolean;
}) {
  const { t } = useT("settings");
  const user = useAuthStore((state) => state.user);
  const members = useQuery({ ...memberListOptions(workspaceId), enabled: active && !!workspaceId });
  const memberState = getSettingsQueryState(members);
  const member = memberState.hasData
    ? members.data?.find((candidate) => candidate.user_id === user?.id)
    : undefined;
  const inventory = useQuery({
    ...workspaceMcpServersOptions(workspaceId),
    enabled: active && !!workspaceId && !!member,
  });
  const inventoryState = getSettingsQueryState(inventory);
  const servers = member && inventoryState.hasData ? inventory.data : undefined;
  const canManage = servers !== undefined && (member?.role === "owner" || member?.role === "admin");
  const create = useCreateWorkspaceMcpServer(workspaceId);
  const [preset, setPreset] = useState<McpCustomPreset | null>(null);
  const customFinalFocus = useRef<() => HTMLElement | null>(() => null);
  const existingNames = useMemo(() => new Set(servers?.map((server) => server.name)), [servers]);

  const saveCustom = async (name: string, config: Record<string, unknown>) => {
    if (!canManage) throw new Error(t(($) => $.query_feedback.access_denied));
    try {
      await create.mutateAsync({ name, config });
      toast.success(t(($) => $.mcp.added_toast));
    } catch (error) {
      toast.error(error instanceof Error && error.message ? error.message : t(($) => $.mcp.save_failed_toast));
      throw error;
    } finally {
      create.reset();
    }
  };

  return (
    <div className="min-w-0 space-y-4">
      <SettingsQueryFeedback
        query={members}
        loadingMessage={t(($) => $.query_feedback.permissions_loading)}
        errorMessage={t(($) => $.query_feedback.permissions_load_failed)}
        refreshMessage={t(($) => $.query_feedback.permissions_refresh_failed)}
      />
      {member ? (
        <SettingsQueryFeedback
          query={inventory}
          loadingMessage={t(($) => $.mcp.market.workspace_loading)}
          errorMessage={t(($) => $.mcp.market.workspace_error)}
          refreshMessage={t(($) => $.mcp.market.workspace_refresh_error)}
        />
      ) : memberState.hasData ? (
        <p role="alert" className="text-body text-muted-foreground">{t(($) => $.query_feedback.access_denied)}</p>
      ) : null}
      <McpTemplateCatalog
        workspaceId={workspaceId}
        servers={servers}
        canManage={canManage}
        active={active}
        onCustom={(nextPreset, finalFocus) => {
          customFinalFocus.current = finalFocus;
          setPreset(nextPreset);
        }}
      />
      {preset && canManage ? (
        <McpServerDialog
          open
          active={active}
          finalFocus={() => customFinalFocus.current()}
          server={null}
          preset={preset}
          existingNames={existingNames}
          onOpenChange={(open) => { if (!open) setPreset(null); }}
          onSave={saveCustom}
        />
      ) : null}
    </div>
  );
}
