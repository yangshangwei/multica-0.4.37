"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useCurrentMember } from "@multica/core/permissions";
import {
  workspaceMcpServersOptions,
  agentMcpServersOptions,
} from "@multica/core/workspace/queries";
import {
  useAssignWorkspaceMcpServer,
  useCreateWorkspaceMcpServer,
} from "@multica/core/workspace/mutations";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@multica/ui/components/ui/tabs";
import { McpServerDialog } from "../agents/components/tabs/mcp-server-dialog";
import { useT } from "../i18n";
import { McpTemplateCatalog, type McpCustomPreset } from "./mcp-market";
import { McpConflictNotice, type McpAgentContext } from "./mcp-setup-dialog";

export function McpAgentDiscovery({
  workspaceId,
  context,
  onClose,
}: {
  workspaceId: string;
  context: McpAgentContext;
  onClose: () => void;
}) {
  const { t } = useT("settings");
  const member = useCurrentMember(workspaceId);
  const canManage = member.role === "owner" || member.role === "admin";
  const library = useQuery(workspaceMcpServersOptions(workspaceId));
  const assignments = useQuery(agentMcpServersOptions(context.agent.id));
  const assign = useAssignWorkspaceMcpServer(workspaceId);
  const create = useCreateWorkspaceMcpServer(workspaceId);
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [succeeded, setSucceeded] = useState<Set<string>>(new Set());
  const [preset, setPreset] = useState<McpCustomPreset | null>(null);
  const servers = library.data ?? [];
  const assignExisting = async (serverId: string) => {
    if (pending.current || context.unsupported || !assignments.data) return;
    pending.current = true;
    setBusy(true);
    try {
      const result = await assign.mutateAsync({
        serverId,
        agentIds: [context.agent.id],
      });
      if (result.succeeded.includes(context.agent.id)) {
        setSucceeded((previous) => new Set([...previous, serverId]));
        setErrors((previous) => {
          const next = { ...previous };
          delete next[serverId];
          return next;
        });
      } else
        setErrors((previous) => ({
          ...previous,
          [serverId]:
            result.failed[0]?.message ||
            t(($) => $.mcp.market.assignment_failed),
        }));
    } catch (error) {
      setErrors((previous) => ({
        ...previous,
        [serverId]:
          error instanceof Error && error.message
            ? error.message
            : t(($) => $.mcp.market.assignment_failed),
      }));
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending.current) onClose();
      }}
    >
      <DialogContent
        className="flex max-h-[88vh] flex-col overflow-hidden sm:max-w-2xl"
        showCloseButton={!busy}
      >
        <DialogHeader className="pr-8">
          <DialogTitle>{t(($) => $.mcp.market.agent_title)}</DialogTitle>
          <DialogDescription>
            {t(($) => $.mcp.market.assign_to, { name: context.agent.name })}.{" "}
            {t(($) => $.mcp.market.existing_first)}
          </DialogDescription>
        </DialogHeader>
        <Tabs
          defaultValue="workspace"
          className="min-h-0 gap-4 overflow-y-auto"
        >
          <TabsList variant="line">
            <TabsTrigger value="workspace">
              {t(($) => $.mcp.market.workspace)}
            </TabsTrigger>
            <TabsTrigger value="market">
              {t(($) => $.mcp.market.title)}
            </TabsTrigger>
          </TabsList>
          <TabsContent value="workspace" className="space-y-3">
            {library.isPending || assignments.isPending ? (
              <p
                role="status"
                className="py-6 text-caption text-muted-foreground"
              >
                {t(($) => $.mcp.builtin_loading)}
              </p>
            ) : null}
            {library.isError || assignments.isError ? (
              <div role="alert" className="space-y-2">
                <p className="text-caption">
                  {t(($) => $.mcp.market.workspace_error)}
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    void library.refetch();
                    void assignments.refetch();
                  }}
                >
                  {t(($) => $.mcp.market.retry)}
                </Button>
              </div>
            ) : null}
            {!library.isPending && !library.isError && servers.length === 0 ? (
              <p className="py-6 text-caption text-muted-foreground">
                {t(($) => $.mcp.market.no_existing)}
              </p>
            ) : null}
            {servers.map((server) => {
              const assigned = assignments.data?.find(
                (item) => item.id === server.id,
              );
              const isAssigned = !!assigned || succeeded.has(server.id);
              return (
                <div
                  key={server.id}
                  className="space-y-2 rounded-lg border p-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <span className="min-w-0 break-all text-body font-medium">
                      {server.name}
                    </span>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-auto min-h-8 max-w-full whitespace-normal break-all py-1 text-left"
                      disabled={
                        busy ||
                        isAssigned ||
                        context.unsupported ||
                        !assignments.data
                      }
                      onClick={() => void assignExisting(server.id)}
                      aria-label={
                        isAssigned
                          ? undefined
                          : `${t(($) => $.mcp.market.reuse, { name: server.name })}: ${t(($) => $.mcp.market.assign_to, { name: context.agent.name })}`
                      }
                    >
                      {isAssigned
                        ? assigned?.enabled === false
                          ? t(($) => $.mcp.market.already_disabled)
                          : t(($) => $.mcp.market.already_assigned)
                        : t(($) => $.mcp.market.assign_to, {
                            name: context.agent.name,
                          })}
                    </Button>
                  </div>
                  <McpConflictNotice name={server.name} context={context} />
                  {errors[server.id] ? (
                    <p role="alert" className="text-caption text-destructive">
                      {errors[server.id]}
                    </p>
                  ) : null}
                </div>
              );
            })}
          </TabsContent>
          <TabsContent value="market">
            <McpTemplateCatalog
              workspaceId={workspaceId}
              servers={library.data}
              canManage={canManage}
              onCustom={setPreset}
              agentContext={context}
            />
          </TabsContent>
        </Tabs>
        <Button
          className="shrink-0 self-end"
          variant="outline"
          disabled={busy}
          onClick={onClose}
        >
          {t(($) => $.mcp.market.done)}
        </Button>
        {preset ? (
          <McpServerDialog
            open
            server={null}
            preset={preset}
            existingNames={new Set(servers.map((server) => server.name))}
            onOpenChange={(open) => {
              if (!open) setPreset(null);
            }}
            onSave={async (name, config) => {
              await create.mutateAsync({ name, config });
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
