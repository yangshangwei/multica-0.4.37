import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useProjectsRealtime } from "./use-projects-realtime";
import { projectKeys } from "../queries/projects";
import { allowProjectAccess } from "./project-access";

type Handler = (payload: Record<string, unknown>) => void;
type MockWS = { on: (type: string, fn: Handler) => () => void; onAny: (fn: Handler) => () => void; onReconnect: (fn: () => void) => () => void };
const state = vi.hoisted(() => ({ qc: null as QueryClient | null, setup: null as null | ((ws: MockWS, wsId: string) => (() => void)[]) }));
vi.mock("@tanstack/react-query", async (original) => ({
  ...await original<typeof import("@tanstack/react-query")>(), useQueryClient: () => state.qc,
}));
vi.mock("@/data/api", () => ({ api: {} }));
vi.mock("@/data/auth-store", () => ({ useAuthStore: (select: (value: { user: { id: string } }) => unknown) => select({ user: { id: "self" } }) }));
vi.mock("@/lib/use-ws-subscriptions", () => ({ useWSSubscriptions: (setup: typeof state.setup) => { state.setup = setup; } }));
beforeEach(() => { state.qc = new QueryClient(); allowProjectAccess("workspace"); });

describe("mobile project realtime wiring", () => {
  function connect() {
    const handlers = new Map<string, Handler>();
    useProjectsRealtime();
    state.setup?.({
      on: (type, fn) => { handlers.set(type, fn); return () => {}; },
      onAny: (fn) => { handlers.set("any", fn); return () => {}; },
      onReconnect: (fn) => { handlers.set("reconnect", fn); return () => {}; },
    }, "workspace");
    return handlers;
  }
  it("clears only self workspace on member removal", () => {
    const handlers = connect();
    const key = projectKeys.detail("workspace", "project");
    state.qc?.setQueryData(key, { description: "secret" });
    handlers.get("member:removed")?.({ workspace_id: "workspace", user_id: "other" });
    expect(state.qc?.getQueryData(key)).toBeDefined();
    handlers.get("member:removed")?.({ workspace_id: "workspace", user_id: "self" });
    expect(state.qc?.getQueryData(key)).toBeUndefined();
  });
  it("invalidates ID-only updates, admission/category changes and reconnect", () => {
    const handlers = connect();
    const invalidate = vi.spyOn(state.qc!, "invalidateQueries");
    for (const type of ["project:update_published", "project:update_corrected", "triage:updated", "issue_status:changed", "issue:updated"]) {
      handlers.get("any")?.({ type, payload: { workspace_id: "workspace", project_id: "project" } });
    }
    handlers.get("reconnect")?.({});
    expect(invalidate).toHaveBeenCalledTimes(6);
    expect(invalidate).toHaveBeenLastCalledWith({ queryKey: projectKeys.all("workspace") });
  });
});
