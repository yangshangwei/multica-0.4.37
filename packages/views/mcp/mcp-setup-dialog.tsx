"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Check, ExternalLink, Loader2 } from "lucide-react";
import type {
  Agent,
  McpServerTemplate,
  WorkspaceMcpServer,
} from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
import {
  useAssignWorkspaceMcpServer,
  useCreateWorkspaceMcpServerFromTemplate,
} from "@multica/core/workspace/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../i18n";
import type { McpCustomPreset } from "./mcp-market";

export type McpAgentContext = {
  agent: Pick<Agent, "id" | "name">;
  managedNames: ReadonlySet<string>;
  runtimeNames: ReadonlySet<string>;
  unsupported: boolean;
};

export function McpConflictNotice({
  name,
  context,
}: {
  name: string;
  context?: McpAgentContext;
}) {
  const { t } = useT("settings");
  if (!context) return null;
  return (
    <>
      {context.unsupported ? (
        <p role="status" className="text-caption text-muted-foreground">
          {t(($) => $.mcp.market.runtime_unsupported)}
        </p>
      ) : null}
      {context.managedNames.has(name) ? (
        <p role="status" className="text-caption text-muted-foreground">
          {t(($) => $.mcp.market.same_name_agent)}
        </p>
      ) : context.runtimeNames.has(name) ? (
        <p role="status" className="text-caption text-muted-foreground">
          {t(($) => $.mcp.market.same_name_runtime)}
        </p>
      ) : null}
    </>
  );
}

export function McpSetupDialog({
  workspaceId,
  template,
  initialServer,
  available,
  servers,
  canManage,
  agentContext,
  onClose,
  onCustom,
}: {
  workspaceId: string;
  servers: readonly WorkspaceMcpServer[] | undefined;
  canManage: boolean;
  agentContext?: McpAgentContext;
  onClose: () => void;
} & (
  | {
      template: McpServerTemplate;
      available: boolean;
      onCustom: (preset: McpCustomPreset) => void;
      initialServer?: never;
    }
  | {
      initialServer: WorkspaceMcpServer;
      template?: never;
      available?: never;
      onCustom?: never;
    }
)) {
  const { t } = useT("settings");
  const create = useCreateWorkspaceMcpServerFromTemplate(workspaceId);
  const assign = useAssignWorkspaceMcpServer(workspaceId);
  const [name, setName] = useState(() => {
    if (initialServer) return initialServer.name;
    const names = new Set(servers?.map((server) => server.name));
    let candidate = template?.key ?? "";
    for (let suffix = 2; names.has(candidate); suffix++)
      candidate = `${template?.key}-${suffix}`;
    return candidate;
  });
  const [saved, setSaved] = useState<WorkspaceMcpServer | null>(
    initialServer ?? null,
  );
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [succeeded, setSucceeded] = useState<Set<string>>(new Set());
  const [failures, setFailures] = useState<
    { agentId: string; message: string }[]
  >([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const operation = useRef(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (saved) titleRef.current?.focus();
  }, [saved]);
  const id = useId();
  const agents = useQuery({
    ...agentListOptions(workspaceId),
    enabled: !!saved && !agentContext,
  });
  const malformedAgents =
    agents.data !== undefined && !Array.isArray(agents.data);
  const choices = agentContext
    ? [agentContext.agent]
    : (Array.isArray(agents.data) ? agents.data : []).filter(
        (agent) =>
          agent &&
          typeof agent.id === "string" &&
          typeof agent.name === "string" &&
          !agent.archived_at &&
          !agent.system_key,
      );
  const related =
    servers?.filter(
      (server) => template && server.template_key === template.key,
    ) ?? [];
  const recipe =
    typeof template?.version === "string" && template.version.length > 0;
  const docsUrl = template?.documentationUrl?.startsWith("https://")
    ? template.documentationUrl
    : null;
  const failedIds = new Set(failures.map((failure) => failure.agentId));
  const canAssign = canManage || !!agentContext;
  const pendingIds = [...selected].filter(
    (agentId) =>
      !succeeded.has(agentId) && choices.some((agent) => agent.id === agentId),
  );

  const save = async () => {
    if (operation.current || !canManage || !available || !servers || !template)
      return;
    const value = name.trim();
    const message = !value
      ? t(($) => $.mcp.rename_required)
      : !/^[A-Za-z0-9_-]+$/.test(value)
        ? t(($) => $.mcp.rename_invalid)
        : servers.some((server) => server.name === value)
          ? t(($) => $.mcp.rename_duplicate)
          : "";
    if (message) {
      setError(message);
      inputRef.current?.focus();
      return;
    }
    if (!recipe) {
      onCustom?.({ name: value, config: template.config });
      return;
    }
    operation.current = true;
    setBusy(true);
    setError("");
    try {
      const result = await create.mutateAsync({
        name: value,
        templateKey: template.key,
        templateVersion: template.version!,
      });
      if (!result.id) throw new Error(t(($) => $.mcp.market.save_failed));
      setSaved(result);
    } catch (cause) {
      setError(
        cause instanceof Error && cause.message
          ? cause.message
          : t(($) => $.mcp.market.save_failed),
      );
    } finally {
      operation.current = false;
      setBusy(false);
    }
  };
  const assignSelected = async (retry: boolean) => {
    if (!saved || operation.current || !canAssign || agentContext?.unsupported)
      return;
    const agentIds = pendingIds.filter((agentId) =>
      retry ? failedIds.has(agentId) : !failedIds.has(agentId),
    );
    if (!agentIds.length) return;
    operation.current = true;
    setBusy(true);
    setError("");
    try {
      const result = await assign.mutateAsync({ serverId: saved.id, agentIds });
      setSucceeded((previous) => new Set([...previous, ...result.succeeded]));
      setFailures((previous) => [
        ...previous.filter((failure) => !agentIds.includes(failure.agentId)),
        ...result.failed,
      ]);
    } catch (cause) {
      const message =
        cause instanceof Error && cause.message
          ? cause.message
          : t(($) => $.mcp.market.assignment_failed);
      setFailures(agentIds.map((agentId) => ({ agentId, message })));
    } finally {
      operation.current = false;
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !operation.current) onClose();
      }}
    >
      <DialogContent
        className="flex max-h-[88vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-xl pointer-coarse:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-w-11 pointer-coarse:[&_[data-slot=input]]:min-h-11 pointer-coarse:[&_[data-slot=dialog-close]]:min-h-11 pointer-coarse:[&_[data-slot=dialog-close]]:min-w-11"
        showCloseButton={!busy}
      >
        <DialogHeader className="shrink-0 px-5 pb-4 pt-5 pr-12">
          <DialogTitle
            ref={titleRef}
            tabIndex={-1}
            className="[overflow-wrap:anywhere] focus-visible:outline-2 focus-visible:outline-foreground focus-visible:outline-offset-2"
          >
            {saved
              ? t(($) => $.mcp.market.assign_title)
              : t(($) => $.mcp.market.setup_title, {
                  name: template?.title || template?.key || "",
                })}
          </DialogTitle>
          <DialogDescription className="[overflow-wrap:anywhere]">
            {saved
              ? t(($) => $.mcp.market.saved, { name: saved.name })
              : template?.description}
          </DialogDescription>
        </DialogHeader>
        <div
          className="min-h-0 space-y-5 overflow-y-auto px-5 pb-5"
          data-slot="mcp-setup-scroll-area"
        >
          {!saved ? (
            <>
              {template?.version ? (
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.mcp.market.version, {
                    version: template.version,
                  })}
                </p>
              ) : null}
              {(template?.requirements ?? []).length > 0 ? (
                <section className="space-y-2">
                  <h3 className="text-body font-medium">
                    {t(($) => $.mcp.market.requirements)}
                  </h3>
                  <ul className="list-disc space-y-1 pl-5 text-caption text-muted-foreground">
                    {(template?.requirements ?? []).map((requirement) => (
                      <li key={requirement}>{requirement}</li>
                    ))}
                  </ul>
                </section>
              ) : null}
              {docsUrl ? (
                <a
                  href={docsUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-caption underline underline-offset-4 pointer-coarse:min-h-11"
                >
                  {t(($) => $.mcp.market.documentation)}
                  <ExternalLink className="size-3" aria-hidden="true" />
                </a>
              ) : null}
              {related.length > 0 ? (
                <section className="space-y-2">
                  <h3 className="text-body font-medium">
                    {t(($) => $.mcp.market.related)}
                  </h3>
                  {related.map((server) => (
                    <div
                      key={server.id}
                      className="flex min-w-0 flex-wrap items-center justify-between gap-2 rounded-lg border p-3"
                    >
                      <span className="min-w-0 break-all text-caption">
                        {server.name}
                      </span>
                      {canManage || agentContext ? (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-auto min-h-8 max-w-full whitespace-normal break-all py-1 text-left"
                          disabled={busy}
                          onClick={() => {
                            if (!operation.current) setSaved(server);
                          }}
                        >
                          {t(($) => $.mcp.market.reuse, { name: server.name })}
                        </Button>
                      ) : null}
                    </div>
                  ))}
                </section>
              ) : null}
              {!available ? (
                <p role="alert" className="text-caption">
                  {t(($) => $.mcp.builtin_empty)}
                </p>
              ) : null}
              {!servers ? (
                <p role="alert" className="text-caption">
                  {t(($) => $.mcp.market.unknown_library)}
                </p>
              ) : null}
              {canManage ? (
                <form
                  id={`${id}-form`}
                  onSubmit={(event) => {
                    event.preventDefault();
                    void save();
                  }}
                  className="space-y-2"
                >
                  <Label htmlFor={`${id}-name`}>
                    {t(($) => $.mcp.market.configuration_name)}
                  </Label>
                  <Input
                    ref={inputRef}
                    id={`${id}-name`}
                    value={name}
                    onChange={(event) => {
                      setName(event.target.value);
                      setError("");
                    }}
                    aria-invalid={error ? true : undefined}
                    aria-required="true"
                    aria-describedby={`${id}-name-hint${error ? ` ${id}-error` : ""}`}
                    disabled={busy}
                    autoComplete="off"
                  />
                  <p
                    id={`${id}-name-hint`}
                    className="text-caption text-muted-foreground"
                  >
                    {t(($) => $.mcp.market.configuration_name_hint)}
                  </p>
                  <p className="text-caption leading-5 text-muted-foreground">
                    {recipe
                      ? t(($) => $.mcp.market.save_note)
                      : t(($) => $.mcp.market.legacy_note)}
                  </p>
                </form>
              ) : (
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.mcp.admin_only_note)}
                </p>
              )}
              <McpConflictNotice name={name.trim()} context={agentContext} />
            </>
          ) : (
            <>
              <p className="text-caption leading-5 text-muted-foreground">
                {t(($) => $.mcp.market.assignment_note)}
              </p>
              <McpConflictNotice name={saved.name} context={agentContext} />
              {!agentContext && agents.isPending ? (
                <p role="status" className="text-caption text-muted-foreground">
                  {t(($) => $.mcp.market.agents_loading)}
                </p>
              ) : null}
              {!agentContext && (agents.isError || malformedAgents) ? (
                <div role="alert" className="space-y-2">
                  <p className="text-caption">
                    {t(($) => $.mcp.market.agents_error)}
                  </p>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void agents.refetch()}
                  >
                    {t(($) => $.mcp.market.retry)}
                  </Button>
                </div>
              ) : null}
              {!agentContext &&
              !agents.isPending &&
              !agents.isError &&
              !malformedAgents &&
              choices.length === 0 ? (
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.mcp.market.agents_empty)}
                </p>
              ) : null}
              {failures.length > 0 ? (
                <p role="alert" className="text-caption">
                  {t(($) => $.mcp.market.partial_failure)}
                </p>
              ) : null}
              <div className="space-y-2">
                {choices.map((agent) => {
                  const failure = failures.find(
                    (item) => item.agentId === agent.id,
                  );
                  return (
                    <div key={agent.id} className="rounded-lg border p-3">
                      <div className="flex min-w-0 items-center gap-3">
                        <Checkbox
                          className="pointer-coarse:after:-inset-3.5"
                          id={`${id}-${agent.id}`}
                          checked={selected.has(agent.id)}
                          disabled={
                            !canAssign ||
                            busy ||
                            succeeded.has(agent.id) ||
                            agentContext?.unsupported
                          }
                          onCheckedChange={(checked) =>
                            setSelected((previous) => {
                              const next = new Set(previous);
                              if (checked) next.add(agent.id);
                              else next.delete(agent.id);
                              return next;
                            })
                          }
                        />
                        <Label
                          htmlFor={`${id}-${agent.id}`}
                          className="min-w-0 flex-1 text-body [overflow-wrap:anywhere] pointer-coarse:min-h-11"
                        >
                          {agent.name}
                        </Label>
                        {succeeded.has(agent.id) ? (
                          <span className="flex shrink-0 items-center gap-1 text-caption text-muted-foreground">
                            <Check className="size-3" aria-hidden="true" />
                            {t(($) => $.mcp.market.assigned)}
                          </span>
                        ) : null}
                      </div>
                      {failure ? (
                        <p
                          role="alert"
                          className="mt-2 break-words text-caption text-destructive"
                        >
                          {failure.message}
                        </p>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            </>
          )}
          {error ? (
            <p
              role="alert"
              id={`${id}-error`}
              className="text-caption text-destructive"
            >
              {error}
            </p>
          ) : null}
        </div>
        <DialogFooter
          className="mx-0 mb-0 shrink-0 border-t bg-muted/30 px-5 py-4 sm:flex-wrap [&_[data-slot=button]]:h-auto [&_[data-slot=button]]:min-h-8 [&_[data-slot=button]]:whitespace-normal [&_[data-slot=button]]:py-1.5"
          data-slot="mcp-setup-footer"
        >
          <Button variant="outline" disabled={busy} onClick={onClose}>
            {saved
              ? succeeded.size
                ? t(($) => $.mcp.market.done)
                : t(($) => $.mcp.market.skip)
              : t(($) => $.mcp.cancel)}
          </Button>
          {!saved && template && canManage ? (
            <Button
              type="submit"
              form={`${id}-form`}
              disabled={busy || !available || !servers}
            >
              {busy ? (
                <Loader2
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : null}
              {recipe
                ? t(($) => $.mcp.market.save_continue)
                : t(($) => $.mcp.market.custom)}
            </Button>
          ) : null}
          {saved && pendingIds.some((agentId) => failedIds.has(agentId)) ? (
            <Button
              disabled={!canAssign || busy || agentContext?.unsupported}
              onClick={() => void assignSelected(true)}
            >
              {t(($) => $.mcp.market.retry_failed)}
            </Button>
          ) : null}
          {saved && pendingIds.some((agentId) => !failedIds.has(agentId)) ? (
            <Button
              disabled={!canAssign || busy || agentContext?.unsupported}
              onClick={() => void assignSelected(false)}
            >
              {busy ? (
                <Loader2
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : null}
              {t(($) => $.mcp.market.assign_selected)}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
