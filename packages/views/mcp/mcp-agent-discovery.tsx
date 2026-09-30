"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
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
  const [assigningServerId, setAssigningServerId] = useState<string | null>(null);
  const busy = assigningServerId !== null;
  const [announcement, setAnnouncement] = useState("");
  const pending = useRef(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [succeeded, setSucceeded] = useState<Set<string>>(new Set());
  const [preset, setPreset] = useState<McpCustomPreset | null>(null);
  const servers = library.data ?? [];
  const assignExisting = async (serverId: string, name: string) => {
    if (pending.current || context.unsupported || !assignments.data) return;
    pending.current = true;
    setAssigningServerId(serverId);
    setAnnouncement(
      t(($) => $.mcp.market.assigning_to, { name, agent: context.agent.name }),
    );
    try {
      const result = await assign.mutateAsync({
        serverId,
        agentIds: [context.agent.id],
      });
      if (result.succeeded.includes(context.agent.id)) {
        setAnnouncement(
          t(($) => $.mcp.market.assigned_to, { name, agent: context.agent.name }),
        );
        setSucceeded((previous) => new Set([...previous, serverId]));
        setErrors((previous) => {
          const next = { ...previous };
          delete next[serverId];
          return next;
        });
      } else {
        setAnnouncement("");
        setErrors((previous) => ({
          ...previous,
          [serverId]:
            result.failed[0]?.message ||
            t(($) => $.mcp.market.assignment_failed),
        }));
      }
    } catch (error) {
      setAnnouncement("");
      setErrors((previous) => ({
        ...previous,
        [serverId]:
          error instanceof Error && error.message
            ? error.message
            : t(($) => $.mcp.market.assignment_failed),
      }));
    } finally {
      pending.current = false;
      setAssigningServerId(null);
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
        className="flex max-h-[88vh] flex-col overflow-hidden sm:max-w-2xl pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11 pointer-coarse:[&_[data-slot=input]]:min-h-11 pointer-coarse:[&_[role=tab]]:min-h-11 pointer-coarse:[&_[data-slot=tabs-list]]:h-auto pointer-coarse:[&_[data-slot=dialog-close]]:min-h-11 pointer-coarse:[&_[data-slot=dialog-close]]:min-w-11"
        showCloseButton={!busy}
      >
        <DialogHeader className="pr-12">
          <DialogTitle>{t(($) => $.mcp.market.agent_title)}</DialogTitle>
          <DialogDescription className="[overflow-wrap:anywhere]">
            {t(($) => $.mcp.market.assign_to, { name: context.agent.name })}.{" "}
            {t(($) => $.mcp.market.existing_first)}
          </DialogDescription>
        </DialogHeader>
        <p role="status" aria-atomic="true" className="sr-only">
          {announcement}
        </p>
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
              const isAssigning = assigningServerId === server.id;
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
                      onClick={() => void assignExisting(server.id, server.name)}
                      aria-label={
                        isAssigned || isAssigning
                          ? undefined
                          : `${t(($) => $.mcp.market.reuse, { name: server.name })}: ${t(($) => $.mcp.market.assign_to, { name: context.agent.name })}`
                      }
                    >
                      {isAssigning ? (
                        <Loader2
                          className="size-3.5 animate-spin motion-reduce:animate-none"
                          aria-hidden="true"
                        />
                      ) : null}
                      {isAssigning
                        ? t(($) => $.mcp.market.assigning)
                        : isAssigned
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
