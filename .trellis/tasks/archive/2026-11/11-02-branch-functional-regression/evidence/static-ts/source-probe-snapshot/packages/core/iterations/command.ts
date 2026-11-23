import { refreshIterationSettingsDependents } from "./realtime";
import {
  notifyPendingIterations,
  pendingIterationRevision,
  subscribePendingIterations,
} from "./pending-signal";
import {
  hasIterationReadAccess,
  clearIterationReadAccess,
  iterationAccessEpoch,
} from "./access";
import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { IterationDraftSchema } from "../api/iteration-schemas";
import { useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, isIterationAccessDenied, errorCode } from "../api";
import { useAuthStore } from "../auth";
import { defaultStorage } from "../platform/storage";
import type {
  IterationCreateInput,
  IterationWriteInput,
} from "../api/iteration-schemas";
export type IterationCommand =
  | { kind: "operation"; body: IterationWriteInput }
  | { kind: "create"; body: IterationCreateInput }
  | {
      kind: "edit";
      id: string;
      body: Parameters<typeof api.updateIteration>[2];
    }
  | { kind: "enable"; body: Parameters<typeof api.enableIterations>[1] };
const commandSchema = z.discriminatedUnion("kind", [
  z.object({
    kind: z.literal("operation"),
    body: z.object({
      request_id: z.string().uuid(),
      draft: IterationDraftSchema,
      preview_hash: z.string().min(1),
    }),
  }),
  z.object({
    kind: z.literal("create"),
    body: z.object({
      request_id: z.string().uuid(),
      name: z.string(),
      description: z.string().nullable(),
      coordinator_user_id: z.string().uuid().nullable(),
      start_date: z.iso.date(),
      end_date: z.iso.date(),
      confirmed_timezone: z.string(),
    }),
  }),
  z.object({
    kind: z.literal("edit"),
    id: z.string().uuid(),
    body: z.object({
      request_id: z.string().uuid(),
      expected_revision: z.number().int().positive(),
      reason: z.string(),
      fields: z.object({
        name: z.string().optional(),
        description: z.string().nullable().optional(),
        coordinator_user_id: z.string().uuid().nullable().optional(),
        start_date: z.iso.date().optional(),
        end_date: z.iso.date().optional(),
      }),
    }),
  }),
  z.object({
    kind: z.literal("enable"),
    body: z.object({
      request_id: z.string().uuid(),
      expected_revision: z.number().int().positive(),
      confirmed_timezone: z.string(),
    }),
  }),
]);
function readCommand(prefix: string): IterationCommand | null {
  for (const key of defaultStorage.keys?.() ?? []) {
    if (!key.startsWith(prefix)) continue;
    try {
      const command = parseWithFallback<IterationCommand | null>(
        JSON.parse(defaultStorage.getItem(key) ?? "null"),
        commandSchema,
        null,
        { endpoint: "iteration command", redact: true },
      );
      if (command) return command;
    } catch {
      /* Invalid local data is never submitted. */
    }
  }
  return null;
}
export function definitelyRejected(error: unknown): boolean {
  return (
    isIterationAccessDenied(error) ||
    (error instanceof ApiError &&
      error.status === 404 &&
      errorCode(error) === "iteration_not_found") ||
    (error instanceof ApiError &&
      [400, 401, 403, 409, 413, 422, 428].includes(error.status))
  );
}
export async function sendIterationCommand(
  wsId: string,
  command: IterationCommand,
) {
  switch (command.kind) {
    case "create":
      return api.createIteration(wsId, command.body);
    case "edit":
      return api.updateIteration(wsId, command.id, command.body);
    case "enable":
      return api.enableIterations(wsId, command.body);
    case "operation":
      return api.applyIterationOperation(wsId, command.body);
  }
}
export function useIterationCommand(wsId: string, scope: string) {
  useSyncExternalStore(
    subscribePendingIterations,
    pendingIterationRevision,
    pendingIterationRevision,
  );
  const actor = useAuthStore((s) => s.user?.id ?? "");
  const client = useQueryClient();
  const prefix = `multica_iteration_command:${encodeURIComponent(api.getBaseUrl?.() ?? "")}:${actor}:${wsId}:${scope}:`;
  const [state, setState] = useState(() => ({
    prefix,
    command: readCommand(prefix),
  }));
  const accessAllowed = useSyncExternalStore(
    (notify) => client.getQueryCache().subscribe(notify),
    () => hasIterationReadAccess(client, wsId),
    () => hasIterationReadAccess(client, wsId),
  );
  const saved = state.prefix === prefix ? state.command : readCommand(prefix);
  const pending =
    accessAllowed &&
    saved &&
    defaultStorage.getItem(prefix + saved.body.request_id) !== null
      ? saved
      : null;
  const inFlight = useRef<{ prefix: string; command: IterationCommand } | null>(
    null,
  );
  const setPending = (command: IterationCommand | null) =>
    setState({ prefix, command });
  const mutation = useMutation({
    mutationKey: ["iterations", wsId, "command", scope],
    mutationFn: async ({
      command,
      recover = false,
    }: {
      command: IterationCommand;
      recover?: boolean;
    }) => {
      if (!hasIterationReadAccess(client, wsId))
        throw new ApiError("Iteration access revoked", 403, "Forbidden");
      const accessEpoch = iterationAccessEpoch(client, wsId);
      const session = api.getSessionScope();
      const requestActor = useAuthStore.getState().user?.id ?? "";
      const assertCurrentContext = () => {
        if (
          !requestActor ||
          requestActor !== actor ||
          (useAuthStore.getState().user?.id ?? "") !== requestActor ||
          api.getSessionScope() !== session ||
          !hasIterationReadAccess(client, wsId) ||
          iterationAccessEpoch(client, wsId) !== accessEpoch
        )
          throw new Error("Iteration command context changed");
      };
      assertCurrentContext();
      const exact =
        (inFlight.current?.prefix === prefix
          ? inFlight.current.command
          : null) ??
        pending ??
        command;
      const key = prefix + exact.body.request_id;
      const serialized = JSON.stringify(exact);
      defaultStorage.setItem(key, serialized);
      if (defaultStorage.getItem(key) !== serialized)
        throw new Error("Unable to preserve the original iteration request");
      notifyPendingIterations();
      inFlight.current = { prefix, command: exact };
      setPending(exact);
      try {
        let result;
        if (recover) {
          try {
            result = await api.getIterationOperation(
              wsId,
              exact.body.request_id,
            );
          } catch (error) {
            if (
              isIterationAccessDenied(error) ||
              !(error instanceof ApiError) ||
              error.status !== 404
            )
              throw error;
          }
        }
        // A delayed lookup must never replay an old actor's intent using a new
        // credential, or resume a command after workspace access changed.
        assertCurrentContext();
        result ??= await sendIterationCommand(wsId, exact);
        assertCurrentContext();
        defaultStorage.removeItem(key);
        notifyPendingIterations();
        setPending(null);
        await client.invalidateQueries({ queryKey: ["iterations", wsId] });
        assertCurrentContext();
        await refreshIterationSettingsDependents(client, wsId);
        assertCurrentContext();
        await client.invalidateQueries({ queryKey: ["issues", wsId] });
        assertCurrentContext();
        return result;
      } catch (error) {
        // Old failures cannot erase a replacement session's data or drafts.
        assertCurrentContext();
        if (definitelyRejected(error)) {
          defaultStorage.removeItem(key);
          notifyPendingIterations();
          setPending(null);
          if (isIterationAccessDenied(error))
            clearIterationReadAccess(client, wsId);
          else {
            await client.invalidateQueries({ queryKey: ["iterations", wsId] });
            assertCurrentContext();
            await refreshIterationSettingsDependents(client, wsId);
          }
        }
        throw error;
      } finally {
        inFlight.current = null;
      }
    },
  });
  return { ...mutation, pending };
}

export interface PendingIterationCommand {
  scope: string;
  command: IterationCommand;
}
export function usePendingIterationCommands(
  wsId: string,
): PendingIterationCommand[] {
  const actor = useAuthStore((state) => state.user?.id ?? "");
  const revision = useSyncExternalStore(
    subscribePendingIterations,
    pendingIterationRevision,
    () => -1,
  );
  const prefix = `multica_iteration_command:${encodeURIComponent(api.getBaseUrl?.() ?? "")}:${actor}:${wsId}:`;
  return useMemo(() => {
    if (revision < 0) return [];
    const entries: PendingIterationCommand[] = [];
    for (const key of defaultStorage.keys?.() ?? []) {
      if (!key.startsWith(prefix)) continue;
      const scope = key.slice(prefix.length, key.lastIndexOf(":"));
      const command = readCommand(key);
      if (command && key === `${prefix}${scope}:${command.body.request_id}`)
        entries.push({ scope, command });
    }
    return entries;
  }, [prefix, revision]);
}
