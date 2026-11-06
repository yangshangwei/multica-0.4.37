import { useEffect } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { isCollaborationScopeCurrent, type CollaborationScope } from "../collaboration/access";
import { expireKnowledgeProjections, knowledgeScopeKey } from "../collaboration/knowledge-cache";
import { CollaborationKnowledgeHintSchema } from "../collaboration/knowledge-schema";
import { isCollaborationInvalidation } from "../collaboration/invalidation";
import { CollaborationAccessHintSchema, CollaborationChangeSchema, CollaborationSourceAccessHintSchema } from "../collaboration/realtime-schema";
import type { WSEventType } from "../types/events";
import { useOptionalWS } from "./provider";

interface KnowledgeRealtimeTransport {
  subscribe: (event: WSEventType, callback: (payload: unknown) => void) => () => void;
  onAuthenticated: (callback: () => void) => () => void;
  onDisconnected: (callback: () => void) => () => void;
}
const bindings = new WeakMap<QueryClient, Map<string, Map<KnowledgeRealtimeTransport["subscribe"], { count: number; dispose: () => void }>>>();

/** One responder per connection/viewer, including target-only pages with no source room subscription. */
export function bindKnowledgeRealtime(client: QueryClient, scope: CollaborationScope, transport: KnowledgeRealtimeTransport): () => void {
  let clients = bindings.get(client);
  if (!clients) { clients = new Map(); bindings.set(client, clients); }
  const key = JSON.stringify(knowledgeScopeKey(scope));
  let transports = clients.get(key);
  if (!transports) { transports = new Map(); clients.set(key, transports); }
  let binding = transports.get(transport.subscribe);
  if (!binding) {
    let active = true;
    // A replacement connection owns disconnect/teardown erasure even while old consumers drain.
    const isCurrentTransport = () => Array.from(transports.keys()).at(-1) === transport.subscribe;
    const clear = (refetch = true) => { if (active && isCollaborationScopeCurrent(scope)) expireKnowledgeProjections(client, scope, refetch); };
    const unsubscribe = [
      transport.subscribe("collaboration:knowledge_changed", (data) => { if (CollaborationKnowledgeHintSchema.safeParse(data).success) clear(); }),
      transport.subscribe("collaboration:changed", (data) => { if (CollaborationChangeSchema.safeParse(data).success) clear(); }),
      transport.subscribe("collaboration:access_changed", (data) => { if (CollaborationAccessHintSchema.safeParse(data).success) clear(); }),
      transport.subscribe("collaboration:source_access_changed", (data) => { if (CollaborationSourceAccessHintSchema.safeParse(data).success) clear(); }),
      transport.subscribe("collaboration:workbench_changed", (data) => { if (isCollaborationInvalidation(data)) clear(); }),
      transport.onAuthenticated(() => clear()),
      transport.onDisconnected(() => { if (active && isCurrentTransport()) expireKnowledgeProjections(client, scope, false); }),
    ];
    binding = { count: 0, dispose: () => { active = false; for (const dispose of unsubscribe) dispose(); if (isCurrentTransport()) expireKnowledgeProjections(client, scope, false); } };
    transports.set(transport.subscribe, binding);
  }
  binding.count++;
  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    if (--binding.count === 0) { binding.dispose(); transports.delete(transport.subscribe); if (!transports.size) clients.delete(key); }
  };
}

export function useKnowledgeRealtime(scope: CollaborationScope): void {
  const client = useQueryClient();
  const ws = useOptionalWS();
  const subscribe = ws?.subscribe, onAuthenticated = ws?.onAuthenticated, onDisconnected = ws?.onDisconnected;
  useEffect(() => {
    if (!scope.userId || !subscribe || !onAuthenticated || !onDisconnected) return;
    return bindKnowledgeRealtime(client, scope, { subscribe, onAuthenticated, onDisconnected });
  }, [client, scope, subscribe, onAuthenticated, onDisconnected]);
}
