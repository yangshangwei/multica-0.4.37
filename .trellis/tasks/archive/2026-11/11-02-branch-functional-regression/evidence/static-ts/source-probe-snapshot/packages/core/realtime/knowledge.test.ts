// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "../collaboration/access";
import { collaborationKnowledgeKeys } from "../collaboration/knowledge-queries";
import { knowledgeDetailFixture, knowledgeIds } from "../collaboration/knowledge-test-fixtures";
import { bindKnowledgeRealtime } from "./knowledge";
import type { WSEventType } from "../types/events";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).reverse().forEach((dispose) => dispose()); vi.restoreAllMocks(); });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { gcTime: Infinity } } });
  cleanups.push(() => client.clear());
  const scope = collaborationScope(knowledgeIds.actor), key = collaborationKnowledgeKeys.candidate(scope, knowledgeIds.candidate);
  const handlers = new Map<WSEventType, (value: unknown) => void>();
  let authenticated = () => {}, disconnected = () => {};
  const transport = { subscribe: vi.fn((event: WSEventType, callback: (value: unknown) => void) => { handlers.set(event, callback); return () => { handlers.delete(event); }; }),
    onAuthenticated: vi.fn((callback: () => void) => { authenticated = callback; return () => {}; }),
    onDisconnected: vi.fn((callback: () => void) => { disconnected = callback; return () => {}; }) };
  cleanups.push(bindKnowledgeRealtime(client, scope, transport));
  return { client, scope, key, transport, handlers, set: () => client.setQueryData(key, knowledgeDetailFixture),
    emit: (event: WSEventType, value: unknown) => handlers.get(event)?.(value), reconnect: () => authenticated(), disconnect: () => disconnected() };
}

it("clears target-only knowledge with one responder and no source-room subscription", () => {
  const f = setup(); const second = bindKnowledgeRealtime(f.client, f.scope, f.transport); cleanups.push(second);
  expect(f.transport.subscribe.mock.calls.filter(([name]) => name === "collaboration:knowledge_changed")).toHaveLength(1);
  f.set(); f.emit("collaboration:knowledge_changed", { target_kind: "workspace", target_id: knowledgeIds.workspace, knowledge_revision: "4" });
  expect(f.client.getQueryData(f.key)).toBeUndefined();
  second(); f.set(); f.emit("collaboration:knowledge_changed", { target_kind: "workspace", target_id: knowledgeIds.workspace, knowledge_revision: "2" });
  expect(f.client.getQueryData(f.key)).toBeUndefined();
});

it("ignores private-field target frames and retains the old empty-only workbench contract", () => {
  const f = setup(); f.set();
  f.emit("collaboration:knowledge_changed", { target_kind: "workspace", target_id: knowledgeIds.workspace, knowledge_revision: "4", source_id: knowledgeIds.issue });
  expect(f.client.getQueryData(f.key)).toBeDefined();
  f.emit("collaboration:workbench_changed", { target_kind: "workspace", target_id: knowledgeIds.workspace, knowledge_revision: "4" });
  expect(f.client.getQueryData(f.key)).toBeDefined();
  f.emit("collaboration:workbench_changed", {}); expect(f.client.getQueryData(f.key)).toBeUndefined();
});

it("immediately clears on disconnect and every authorization hint before GET recovery", () => {
  const f = setup();
  for (const action of [() => f.disconnect(), () => f.reconnect(),
    () => f.emit("collaboration:access_changed", { issue_id: knowledgeIds.issue }),
    () => f.emit("collaboration:source_access_changed", { workspace_id: knowledgeIds.workspace }),
    () => f.emit("collaboration:changed", { issue_id: knowledgeIds.issue, event_seq: "1", collaboration_revision: "1", authority_epoch: "1" })]) {
    f.set(); action(); expect(f.client.getQueryData(f.key)).toBeUndefined();
  }
});

it("does not let an obsolete socket callback affect a new server/viewer scope", () => {
  const f = setup(); f.set();
  setApiInstance(new ApiClient("https://new.example"));
  f.emit("collaboration:knowledge_changed", { target_kind: "workspace", target_id: knowledgeIds.workspace, knowledge_revision: "4" });
  expect(f.client.getQueryData(f.key)).toBeDefined();
});
