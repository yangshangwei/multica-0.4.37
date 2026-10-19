import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { collaborationAccessEpoch, collaborationKeys, clearCollaborationAccess, clearCollaborationProjection, isCollaborationScopeCurrent, type CollaborationScope } from "../collaboration/access";
import { collaborationIssueOptions } from "../collaboration/queries";
import { invalidateCollaborationSourceViews, isCollaborationInvalidation } from "../collaboration/invalidation";
import { nonnegativeVersion, positiveVersion } from "../collaboration/version";
import type { WSEventType } from "../types/events";
import { useOptionalWS } from "./provider";

const issueIdSchema = z.uuid().transform((id) => id.toLowerCase());
const changeSchema = z.strictObject({ issue_id: issueIdSchema, event_seq: positiveVersion, collaboration_revision: positiveVersion, authority_epoch: positiveVersion });
const issueHintSchema = z.strictObject({ issue_id: issueIdSchema });
const accessSchema = z.union([changeSchema, issueHintSchema]);
const snapshotSchema = z.object({ realtime: z.object({ event_seq: nonnegativeVersion }).optional() });
const sourceSchema = z.object({ source_workspace: z.object({ id: issueIdSchema }) });
const subscriptionSchema = z.object({ scope: z.literal("issue"), id: issueIdSchema, error: z.string().optional() });

interface CollaborationRealtimeTransport {
  subscribe: (event: WSEventType, callback: (payload: unknown) => void) => () => void;
  onAuthenticated: (callback: () => void) => () => void;
  subscribeScope: (scope: "account" | "issue", id: string) => () => void;
  revalidateScope: (scope: "account" | "issue", id: string) => void;
}

/** One account workbench owns subscriptions; Query remains the state authority.
 * Every event is an invalidation. Neither reconnect nor delivery calls a write.
 */
export function bindCollaborationRealtime(client: QueryClient, scope: CollaborationScope, selectedIssueId: string | null, transport: CollaborationRealtimeTransport): () => void {
  let active = true;
  const current = () => active && isCollaborationScopeCurrent(scope);
  const selected = selectedIssueId?.toLowerCase() ?? null;
  let subscriptionDenied = false;
  const seen = new Map<string, bigint>();
  const fresh = (issueId: string, sequence?: string) => {
    if (sequence === undefined) return true;
    const snapshot = snapshotSchema.safeParse(client.getQueryData(collaborationKeys.issue(scope, issueId)));
    const baseline = BigInt(snapshot.success ? snapshot.data.realtime?.event_seq ?? "0" : "0");
    const previous = seen.get(issueId) ?? 0n;
    const next = BigInt(sequence);
    if (next <= baseline || next <= previous) return false;
    seen.set(issueId, next);
    return true;
  };
  const refreshAccount = () => {
    if (!current()) return;
    invalidateCollaborationSourceViews(client, scope);
    void client.invalidateQueries({ predicate: (query) => {
      const key = query.queryKey;
      return key[0] === "collaboration" && key[1] === scope.apiScope && key[2] === scope.userId && key[3] !== "issue";
    } });
  };
  const refreshIssue = (id: string) => {
    if (current()) {
      invalidateCollaborationSourceViews(client, scope, id);
      void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, id) });
    }
  };
  const recoverAccess = (id: string, sourceOnly = false) => {
    if (!current()) return;
    if (sourceOnly) clearCollaborationProjection(client, scope, id);
    else clearCollaborationAccess(client, scope, id);
    const epoch = collaborationAccessEpoch(client, scope, id);
    void client.fetchQuery(collaborationIssueOptions(client, scope, id)).then(() => {
      if (!current() || collaborationAccessEpoch(client, scope, id) !== epoch) return;
      refreshIssue(id);
      refreshAccount();
    }).catch(() => { /* The protected read owns denial; ordinary failures stay unavailable. */ });
    refreshAccount();
  };
  const unsubscribe = [
    transport.subscribe("collaboration:changed", (payload) => {
      if (!current()) return;
      const parsed = changeSchema.safeParse(payload);
      if (parsed.success && fresh(parsed.data.issue_id, parsed.data.event_seq)) refreshIssue(parsed.data.issue_id);
    }),
    transport.subscribe("collaboration:children_changed", (payload) => {
      if (!current()) return;
      const parsed = issueHintSchema.safeParse(payload);
      if (parsed.success) refreshIssue(parsed.data.issue_id);
    }),
    transport.subscribe("collaboration:access_changed", (payload) => {
      if (!current()) return;
      const parsed = accessSchema.safeParse(payload);
      if (parsed.success && fresh(parsed.data.issue_id, "event_seq" in parsed.data ? parsed.data.event_seq : undefined)) {
        if (parsed.data.issue_id === selected) subscriptionDenied = true;
        recoverAccess(parsed.data.issue_id);
      }
    }),
    transport.subscribe("collaboration:source_access_changed", (payload) => {
      if (!current()) return;
      const parsed = z.strictObject({ workspace_id: issueIdSchema }).safeParse(payload);
      if (!parsed.success) return;
      const ids = new Set<string>();
      for (const query of client.getQueryCache().findAll({ queryKey: collaborationKeys.scope(scope) })) {
        const key = query.queryKey;
        if (key[3] !== "issue" || key.length !== 5 || typeof key[4] !== "string") continue;
        const source = sourceSchema.safeParse(query.state.data);
        if (source.success && source.data.source_workspace.id === parsed.data.workspace_id) ids.add(key[4]);
      }
      // A first detail request may still be in flight without a source snapshot.
      if (selected && !sourceSchema.safeParse(client.getQueryData(collaborationKeys.issue(scope, selected))).success) ids.add(selected);
      for (const id of ids) recoverAccess(id, true);
      refreshAccount();
    }),
    transport.subscribe("collaboration:workbench_changed", (payload) => {
      if (!current() || !isCollaborationInvalidation(payload)) return;
      refreshAccount();
      if (selected && subscriptionDenied) transport.revalidateScope("issue", selected);
    }),
    transport.subscribe("subscribe_error", (payload) => {
      const parsed = subscriptionSchema.safeParse(payload);
      if (parsed.success && parsed.data.id === selected && parsed.data.error === "forbidden") {
        subscriptionDenied = true;
        recoverAccess(parsed.data.id);
      }
      // Old servers and transient lookup failures retain HTTP polling recovery.
    }),
    transport.subscribe("subscribe_ack", (payload) => {
      const parsed = subscriptionSchema.safeParse(payload);
      if (parsed.success && parsed.data.id === selected) {
        subscriptionDenied = false;
        refreshIssue(parsed.data.id);
      }
    }),
    transport.onAuthenticated(() => {
      if (!current()) return;
      invalidateCollaborationSourceViews(client, scope);
      // Explicit cancellation covers a first GET with no cached data as well as
      // background refetches. It does not fence or cancel a pending POST.
      void client.cancelQueries({ queryKey: collaborationKeys.scope(scope) }, { revert: false }).then(() => {
        if (current()) void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
      });
    }),
  ];
  if (selected) unsubscribe.push(transport.subscribeScope("issue", selected));
  return () => { active = false; for (const dispose of unsubscribe) dispose(); };
}

export function useCollaborationRealtime(scope: CollaborationScope, issueId: string | null): void {
  const client = useQueryClient();
  const transport = useOptionalWS();
  const subscribe = transport?.subscribe;
  const onAuthenticated = transport?.onAuthenticated;
  const subscribeScope = transport?.subscribeScope;
  const revalidateScope = transport?.revalidateScope;
  useEffect(() => {
    if (!scope.userId || !subscribe || !onAuthenticated || !subscribeScope || !revalidateScope) return;
    return bindCollaborationRealtime(client, scope, issueId, { subscribe, onAuthenticated, subscribeScope, revalidateScope });
  }, [client, scope, issueId, subscribe, onAuthenticated, subscribeScope, revalidateScope]);
}
