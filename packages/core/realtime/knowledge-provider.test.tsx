// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { collaborationScope } from "../collaboration/access";
import { useCollaborationKnowledgeTarget } from "../collaboration/knowledge-queries";
import { expireKnowledgeProjections } from "../collaboration/knowledge-cache";
import { knowledgeIds, knowledgeTargetFixture } from "../collaboration/knowledge-test-fixtures";
import { WSProvider } from "./provider";

// Schema/cache matrices live in collaboration/knowledge-*.test.ts. This suite
// exercises the real hook -> context -> WSClient disconnect wiring only.
vi.mock("./use-realtime-sync", () => ({ useRealtimeSync: () => {} }));
class Socket {
  static OPEN = 1;
  static sockets: Socket[] = [];
  readyState = 1;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  send = vi.fn(); close = vi.fn();
  constructor(readonly url: string) { Socket.sockets.push(this); }
}
afterEach(() => { cleanup(); Socket.sockets = []; vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("clears a mounted target-only projection on a real WSClient close without waiting for polling or a source subscription", async () => {
  vi.stubGlobal("WebSocket", Socket);
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  vi.spyOn(api, "getCollaborationKnowledgeTarget").mockResolvedValue(knowledgeTargetFixture);
  const storage = { getItem: () => null, setItem: () => {}, removeItem: () => {} };
  const auth = createAuthStore({ api, storage, cookieAuth: true }); registerAuthStore(auth);
  auth.setState({ status: "authenticated", isLoading: false, user: { id: knowledgeIds.actor, name: "Reader", email: "reader@example.test", avatar_url: null,
    onboarded_at: null, onboarding_questionnaire: {}, starter_content_state: null, language: null, profile_description: "", timezone: null,
    created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z" } });
  const scope = collaborationScope(knowledgeIds.actor), client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function Panel() {
    const query = useCollaborationKnowledgeTarget(scope, knowledgeTargetFixture.target);
    const candidate = query.data?.value.items[0];
    return <div>{candidate?.availability === "available" ? candidate.summary : "Unavailable"}</div>;
  }
  render(<QueryClientProvider client={client}><WSProvider wsUrl="wss://server.example/ws" authStore={auth} storage={storage} cookieAuth><Panel /></WSProvider></QueryClientProvider>);
  try {
    await waitFor(() => expect(Socket.sockets).toHaveLength(1));
    act(() => Socket.sockets[0]!.onopen?.());
    await screen.findByText("PRIVATE SUMMARY");
    expect(Socket.sockets[0]!.send.mock.calls.some(([frame]) => JSON.parse(frame).type === "subscribe")).toBe(false);
    act(() => Socket.sockets[0]!.onclose?.());
    await waitFor(() => expect(screen.queryByText("PRIVATE SUMMARY")).toBeNull());
    expect(screen.getByText("Unavailable")).toBeTruthy();
    expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain("PRIVATE");
  } finally { cleanup(); expireKnowledgeProjections(client, scope, false); client.clear(); }
});
