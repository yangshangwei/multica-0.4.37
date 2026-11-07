"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Compass, Loader2, Plus, Server } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { useCurrentWorkspace, useWorkspacePaths } from "@multica/core/paths";
import { cn } from "@multica/ui/lib/utils";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions, workspaceMcpServersOptions } from "@multica/core/workspace/queries";
import {
  useCreateWorkspaceMcpServer,
  useDeleteWorkspaceMcpServer,
  useUpdateWorkspaceMcpServer,
} from "@multica/core/workspace/mutations";
import type { WorkspaceMcpServer } from "@multica/core/types";
import { McpServerDialog } from "../../agents/components/tabs/mcp-server-dialog";
import type { ManagedMcpServer } from "../../agents/components/tabs/mcp-config-model";
import { McpServerRow } from "../../common/mcp-server-row";
import { McpTemplateIcon } from "../../common/mcp-template-icon";
import { useT } from "../../i18n";
import {
  CollectionPageHeader,
  CollectionPageHeaderAction,
} from "../../layout/collection-page";
import { useNavigation } from "../../navigation";
import { McpSetupDialog } from "../../mcp/mcp-setup-dialog";
import { SettingsCard, SettingsSection, SettingsTab } from "./settings-layout";
import { getSettingsQueryState, SettingsQueryFeedback } from "./settings-query-feedback";

/**
 * The workspace MCP server library (GH #6062).
 *
 * Two things shape this screen and are worth stating up front:
 *
 *  - A server added here is given to NO agent. It is a library entry, exactly
 *    like a workspace skill: explicit assignment is a separate step, available
 *    here or on the agent's MCP tab. Per-agent toggles stay on that tab.
 *    Nothing here reaches an agent implicitly.
 *  - The stored configuration is WRITE-ONLY. The API returns names and
 *    transports, never urls / commands / headers / env, so there is no
 *    "current value" to prefill and replacing a server means supplying its
 *    complete configuration again. Renaming stays separate and never touches
 *    the write-only entry.
 */
export function McpTab({ presentation = "settings" }: {
  presentation?: "page" | "settings";
}) {
  const workspace = useCurrentWorkspace();
  const wsId = workspace?.id ?? "";
  return <McpWorkspaceTab key={wsId} wsId={wsId} presentation={presentation} />;
}

function McpWorkspaceLayout({ presentation, action, children }: {
  presentation: "page" | "settings";
  action: ReactNode;
  children: ReactNode;
}) {
  const { t } = useT("settings");
  if (presentation === "settings") {
    return (
      <SettingsTab title={t(($) => $.mcp.title)} description={t(($) => $.mcp.description)}>
        {children}
      </SettingsTab>
    );
  }
  return (
    <div className="@container/mcp-library flex min-h-0 min-w-0 flex-1 flex-col pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11">
      <CollectionPageHeader
        icon={Server}
        title={t(($) => $.mcp.title)}
        description={t(($) => $.mcp.description)}
        actions={action}
      />
      {children}
    </div>
  );
}

function McpWorkspaceTab({ wsId, presentation }: {
  wsId: string;
  presentation: "page" | "settings";
}) {
  const { t } = useT("settings");
  const paths = useWorkspacePaths();
  const { push } = useNavigation();
  const user = useAuthStore((s) => s.user);
  const membersQuery = useQuery(memberListOptions(wsId));
  const memberState = getSettingsQueryState(membersQuery);
  const currentMember = memberState.hasData
    ? membersQuery.data?.find((member) => member.user_id === user?.id)
    : undefined;
  const serversQuery = useQuery({
    ...workspaceMcpServersOptions(wsId),
    enabled: !!wsId && !!currentMember,
  });
  const serversState = getSettingsQueryState(serversQuery);
  const canManage = serversState.hasData &&
    (currentMember?.role === "owner" || currentMember?.role === "admin");
  const createServer = useCreateWorkspaceMcpServer(wsId);
  const updateServer = useUpdateWorkspaceMcpServer(wsId);
  const deleteServer = useDeleteWorkspaceMcpServer(wsId);

  const servers = useMemo(() => serversQuery.data ?? [], [serversQuery.data]);
  const existingNames = useMemo(
    () => new Set(servers.map((server) => server.name)),
    [servers],
  );

  const [assigningServer, setAssigningServer] =
    useState<WorkspaceMcpServer | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingServer, setEditingServer] = useState<WorkspaceMcpServer | null>(
    null,
  );
  const [renamingServer, setRenamingServer] =
    useState<WorkspaceMcpServer | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const [renameError, setRenameError] = useState("");
  const [renamePending, setRenamePending] = useState(false);
  const [deletingServer, setDeletingServer] =
    useState<WorkspaceMcpServer | null>(null);

  // The dialog is shared with the agent MCP tab, which hands it the saved
  // entry to prefill. Here there is nothing to prefill — an edit always
  // starts from an empty form and REPLACES the entry. The transport still
  // comes from the safe summary so the form opens on the right one.
  const dialogServer: ManagedMcpServer | null = useMemo(
    () =>
      editingServer
        ? {
            name: editingServer.name,
            config: {},
            container: "mcpServers",
            transport: editingServer.transport,
            // The library has no per-agent toggle; this field only feeds the
            // dialog's shape.
            enabled: true,
          }
        : null,
    [editingServer],
  );

  const handleSaveServer = async (
    name: string,
    config: Record<string, unknown>,
  ) => {
    if (!canManage) throw new Error(t(($) => $.query_feedback.access_denied));
    try {
      if (editingServer) {
        await updateServer.mutateAsync({ serverId: editingServer.id, config });
      } else {
        await createServer.mutateAsync({ name, config });
      }
      toast.success(
        editingServer
          ? t(($) => $.mcp.updated_toast)
          : t(($) => $.mcp.added_toast),
      );
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(($) => $.mcp.save_failed_toast),
      );
      throw error;
    } finally {
      if (!editingServer) createServer.reset();
    }
  };

  const startRename = (server: WorkspaceMcpServer) => {
    if (renamePending) return;
    setRenamingServer(server);
    setRenameDraft(server.name);
    setRenameError("");
  };

  const cancelRename = () => {
    if (renamePending) return;
    setRenamingServer(null);
    setRenameDraft("");
    setRenameError("");
  };

  const handleRename = async () => {
    if (!canManage || !renamingServer || renamePending) return;
    const name = renameDraft.trim();
    if (name === "") {
      setRenameError(t(($) => $.mcp.rename_required));
      return;
    }
    if (!/^[A-Za-z0-9_-]+$/.test(name)) {
      setRenameError(t(($) => $.mcp.rename_invalid));
      return;
    }
    if (name !== renamingServer.name && existingNames.has(name)) {
      setRenameError(t(($) => $.mcp.rename_duplicate));
      return;
    }
    if (name === renamingServer.name) {
      cancelRename();
      return;
    }

    setRenamePending(true);
    try {
      await updateServer.mutateAsync({ serverId: renamingServer.id, name });
      toast.success(t(($) => $.mcp.renamed_toast));
      setRenamingServer(null);
      setRenameDraft("");
      setRenameError("");
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : t(($) => $.mcp.rename_failed_toast);
      setRenameError(message);
      toast.error(message);
    } finally {
      setRenamePending(false);
    }
  };

  const handleDelete = async () => {
    if (!canManage || !deletingServer || deleteServer.isPending) return;
    try {
      await deleteServer.mutateAsync(deletingServer.id);
      toast.success(t(($) => $.mcp.removed_toast));
      setDeletingServer(null);
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(($) => $.mcp.remove_failed_toast),
      );
    }
  };

  const openCustomEditor = () => {
    cancelRename();
    setEditingServer(null);
    setEditorOpen(true);
  };
  const customAction = !canManage ? null : presentation === "page" ? (
    <CollectionPageHeaderAction
      icon={Plus}
      label={t(($) => $.mcp.add_server)}
      disabled={renamePending}
      onClick={openCustomEditor}
    />
  ) : (
    <Button size="sm" variant="outline" disabled={renamePending} onClick={openCustomEditor}>
      <Plus aria-hidden className="size-4" />
      {t(($) => $.mcp.add_server)}
    </Button>
  );

  const marketAction = presentation === "page" ? (
    <CollectionPageHeaderAction
      icon={Compass}
      label={t(($) => $.mcp.browse_market)}
      disabled={renamePending}
      onClick={() => push(paths.marketplaceTab("mcp"))}
    />
  ) : (
    <Button size="sm" variant="outline" disabled={renamePending} onClick={() => push(paths.marketplaceTab("mcp"))}>
      <Compass aria-hidden className="size-4" />
      {t(($) => $.mcp.browse_market)}
    </Button>
  );
  const actions = <>{marketAction}{customAction}</>;

  const feedback = <>
    <SettingsQueryFeedback
      query={membersQuery}
      loadingMessage={t(($) => $.query_feedback.permissions_loading)}
      errorMessage={t(($) => $.query_feedback.permissions_load_failed)}
      refreshMessage={t(($) => $.query_feedback.permissions_refresh_failed)}
    />
    {currentMember ? (
      <SettingsQueryFeedback
        query={serversQuery}
        loadingMessage={t(($) => $.mcp.market.workspace_loading)}
        errorMessage={t(($) => $.mcp.market.workspace_error)}
        refreshMessage={t(($) => $.mcp.market.workspace_refresh_error)}
      />
    ) : memberState.hasData ? (
      <p role="alert" className="text-body text-muted-foreground">{t(($) => $.query_feedback.access_denied)}</p>
    ) : null}
  </>;

  if (!currentMember || !serversState.hasData) {
    return (
      <McpWorkspaceLayout presentation={presentation} action={marketAction}>
        <div className={presentation === "page" ? "px-4 py-4" : "space-y-5"}>
          {presentation === "settings" ? marketAction : null}
          {feedback}
        </div>
      </McpWorkspaceLayout>
    );
  }

  return (
    <McpWorkspaceLayout presentation={presentation} action={actions}>
      <div className={cn(
        "min-w-0 space-y-5 pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11 pointer-coarse:[&_[data-slot=input]]:min-h-11",
        presentation === "page" && "min-h-0 flex-1 overflow-y-auto px-4 pb-12 pt-4 @2xl/mcp-library:px-6",
      )}>
        {presentation === "settings" ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
        {feedback}
        <SettingsSection
          title={t(($) => $.mcp.servers_title)}
          description={t(($) => $.mcp.write_only_note)}
        >
          <SettingsCard>
            {servers.length === 0 ? (
              <div className="px-4 py-8 text-center">
                <Server className="mx-auto h-5 w-5 text-muted-foreground" />
                <p className="mt-3 text-body font-medium">
                  {t(($) => $.mcp.empty_title)}
                </p>
                <p className="mx-auto mt-1 max-w-md text-caption leading-5 text-muted-foreground">
                  {t(($) => $.mcp.empty_description)}
                </p>
              </div>
            ) : (
              <ul className="divide-y divide-surface-border">
                {servers.map((server) => (
                  <McpServerRow
                    key={server.id}
                    name={server.name}
                    transport={server.transport}
                    icon={server.template_key ? <McpTemplateIcon templateKey={server.template_key} source={server.template_source} /> : undefined}
                    status={
                      server.enabled === false ? (
                        <Badge variant="secondary">
                          {t(($) => $.mcp.disabled_badge)}
                        </Badge>
                      ) : undefined
                    }
                    details={
                      <span className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
                        {server.template_key ? (
                          <Badge
                            variant="outline"
                            className="max-w-full whitespace-normal break-all"
                          >
                            {server.template_source === "deployment"
                              ? t(($) => $.mcp.market.source_deployment)
                              : server.template_source !== undefined && server.template_source !== "builtin"
                                ? t(($) => $.mcp.market.source_unknown)
                                : t(($) => $.mcp.market.source_builtin)}
                            {" · "}
                            {t(($) => $.mcp.market.source, {
                              name: server.template_key,
                            })}
                          </Badge>
                        ) : null}
                        {canManage ? (
                          <Button
                            size="sm"
                            variant="ghost"
                            disabled={renamePending}
                            onClick={() => {
                              cancelRename();
                              setAssigningServer(server);
                            }}
                            aria-label={t(($) => $.mcp.market.assign_server, {
                              name: server.name,
                            })}
                          >
                            {t(($) => $.mcp.market.assign)}
                          </Button>
                        ) : null}
                      </span>
                    }
                    canManage={canManage}
                    actionsDisabled={renamePending}
                    rename={
                      renamingServer?.id === server.id
                        ? {
                            draft: renameDraft,
                            error: renameError,
                            pending: renamePending,
                            onChange: (value) => {
                              setRenameDraft(value);
                              setRenameError("");
                            },
                            onCancel: cancelRename,
                            onSubmit: () => void handleRename(),
                          }
                        : undefined
                    }
                    labels={{
                      rename: t(($) => $.mcp.rename_action),
                      renameAria: t(($) => $.mcp.rename_server),
                      renameSave: t(($) => $.mcp.rename_save),
                      renameCancel: t(($) => $.mcp.rename_cancel),
                      configure: t(($) => $.mcp.replace_config),
                      configureAria: t(($) => $.mcp.replace_config),
                      remove: t(($) => $.mcp.remove_action),
                      removeAria: t(($) => $.mcp.remove_server),
                    }}
                    onRenameStart={() => startRename(server)}
                    onConfigure={() => {
                      cancelRename();
                      setEditingServer(server);
                      setEditorOpen(true);
                    }}
                    onRemove={() => {
                      cancelRename();
                      setDeletingServer(server);
                    }}
                  />
                ))}
              </ul>
            )}
          </SettingsCard>
          {!canManage ? (
            <p className="px-0.5 text-caption text-muted-foreground">
              {t(($) => $.mcp.admin_only_note)}
            </p>
          ) : null}
        </SettingsSection>
      </div>

      {assigningServer && canManage ? (
        <McpSetupDialog
          key={`${wsId}-${assigningServer.id}`}
          workspaceId={wsId}
          initialServer={assigningServer}
          servers={serversQuery.data}
          canManage
          onClose={() => setAssigningServer(null)}
        />
      ) : null}
      <McpServerDialog
        open={editorOpen && canManage}
        server={dialogServer}
        existingNames={existingNames}
        replacementMode={editingServer !== null}
        onOpenChange={setEditorOpen}
        onSave={handleSaveServer}
      />

      <AlertDialog
        open={deletingServer !== null && canManage}
        onOpenChange={(open) => {
          if (!open && !deleteServer.isPending) setDeletingServer(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.mcp.delete_title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.mcp.delete_description, {
                name: deletingServer?.name ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteServer.isPending}>
              {t(($) => $.mcp.cancel)}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void handleDelete();
              }}
              disabled={deleteServer.isPending}
            >
              {deleteServer.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : null}
              {t(($) => $.mcp.delete_confirm)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </McpWorkspaceLayout>
  );
}
