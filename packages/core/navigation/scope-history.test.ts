// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { accountPath } from "../paths";
import { navigationScope, resolveScopePath, useScopeHistoryStore } from "./scope-history";

const owner = '["server","viewer"]';
const issue = "00000000-0000-4000-8000-000000000001";

beforeEach(() => useScopeHistoryStore.getState().setOwner(null));

describe("scope history", () => {
  it("retains the full independent workspace and account locations", () => {
    const store = useScopeHistoryStore.getState();
    store.setOwner(owner);
    const workspace = "/second/projects/p1?tab=overview#description";
    const account = accountPath({ view: "participating", issueId: issue, anchor: "memory" });
    store.record(owner, workspace);
    store.record(owner, account);
    const state = useScopeHistoryStore.getState();
    expect(resolveScopePath(state, owner, "workspace", ["first", "second"])).toBe(workspace);
    expect(resolveScopePath(state, owner, "account")).toBe(account);
  });

  it("rejects global flows, foreign URLs and invalid account destinations", () => {
    const store = useScopeHistoryStore.getState();
    store.setOwner(owner);
    store.record(owner, "/space/issues/I-1");
    for (const path of ["/login", "/admin/collaboration", "/workspaces/new", "/invite/x", "//evil.test/space/issues", "/\\evil.test/space/issues", "/space/issues\u0000", "/space/issues\n", "/space/issues with spaces", "/account?issue=invalid", "https://evil.test/space/issues", "/space/../login"]) {
      expect(navigationScope(path)).toBeNull();
      store.record(owner, path);
    }
    expect(useScopeHistoryStore.getState().workspacePath).toBe("/space/issues/I-1");
  });

  it("does not reuse another identity or accept its late writes", () => {
    const store = useScopeHistoryStore.getState();
    store.setOwner(owner);
    store.record(owner, "/space/issues/I-1");
    expect(resolveScopePath(useScopeHistoryStore.getState(), "other", "workspace")).toBeNull();
    store.setOwner("other");
    store.record(owner, "/space/issues/private");
    expect(useScopeHistoryStore.getState().workspacePath).toBeNull();
    expect(resolveScopePath(useScopeHistoryStore.getState(), "other", "account")).toBe(accountPath());
  });

  it("uses authoritative membership to fall back, without treating pending as empty", () => {
    const store = useScopeHistoryStore.getState();
    store.setOwner(owner);
    store.record(owner, "/gone/projects/p1");
    const state = useScopeHistoryStore.getState();
    expect(resolveScopePath(state, owner, "workspace")).toBe("/gone/projects/p1");
    expect(resolveScopePath(state, owner, "workspace", ["available"])).toBe("/available/issues");
    expect(resolveScopePath(state, owner, "workspace", [])).toBe("/workspaces/new");
  });
});
