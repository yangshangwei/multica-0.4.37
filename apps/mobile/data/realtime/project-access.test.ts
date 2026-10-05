import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { isProjectAccessError, markProjectAccessDenied, observeProjectAccess, onProjectAccessDenied, protectProjectQuery, revokeProjectAccess } from "./project-access";
import { patchProjectDetail } from "./project-ws-updaters";
import { projectKeys } from "../queries/projects";
vi.mock("@/data/api", () => ({ api: {} }));

describe("mobile project access boundary", () => {
  it("always treats 401 as auth loss and fails closed for unknown/malformed 403 codes", () => {
    for (const code of ["project_permission_denied", "project_evidence_forbidden", "project_updates_disabled"]) {
      expect(isProjectAccessError({ status: 401, body: { code } })).toBe(true);
    }
    for (const body of [{ code: "forbidden" }, { code: "future_scope_denial" }, { code: 42 }, {}, null]) {
      expect(isProjectAccessError({ status: 403, body })).toBe(true);
    }
    expect(isProjectAccessError({ status: 409, body: { code: "forbidden" } })).toBe(false);
  });
  it("uses one navigation responder for concurrent 403 failures", () => {
    const navigate = vi.fn();
    const stop = onProjectAccessDenied(navigate);
    try {
      markProjectAccessDenied("concurrent");
      markProjectAccessDenied("concurrent");
      expect(navigate).toHaveBeenCalledOnce();
    } finally { stop(); }
  });
  it("removes protected data synchronously, even when navigation fails", () => {
    const qc = new QueryClient();
    qc.setQueryData(["projects", "revoked", "detail", "project"], { description: "secret" });
    qc.setQueryData(["issues", "revoked", "detail", "issue"], { title: "secret evidence" });
    qc.setQueryData(["projects", "other", "list"], []);
    const stopNavigate = onProjectAccessDenied(() => { throw new Error("navigation failed"); });
    const stopObserve = observeProjectAccess(qc);
    try {
      expect(() => markProjectAccessDenied("revoked")).not.toThrow();
      expect(qc.getQueryData(["projects", "revoked", "detail", "project"])).toBeUndefined();
      expect(qc.getQueryData(["issues", "revoked", "detail", "issue"])).toBeUndefined();
      expect(qc.getQueryData(["projects", "other", "list"])).toEqual([]);
    } finally { stopNavigate(); stopObserve(); }
  });
  it("prevents an in-flight query and a subsequent WS payload from restoring data", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    let resolve!: (value: { description: string }) => void;
    const pending = qc.fetchQuery({
      queryKey: projectKeys.detail("late", "project"),
      queryFn: () => protectProjectQuery(qc, "late", () => new Promise<{ description: string }>((next) => { resolve = next; })),
    });
    const rejected = expect(pending).rejects.toThrow();
    revokeProjectAccess(qc, "late");
    resolve({ description: "secret" });
    await rejected;
    patchProjectDetail(qc, "late", { id: "project", workspace_id: "late", description: "secret" });
    expect(qc.getQueryData(projectKeys.detail("late", "project"))).toBeUndefined();
  });
});
