// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountBootstrapDestination } from "./account-bootstrap";

describe("authenticated account boot", () => {
  const ready = { authenticated: true, restricted: false, activeScope: "none" as const, workspaceListReady: false, workspaceListUnavailable: false, workspaceCount: 0 };
  it.each([
    { workspaceListReady: false, workspaceListUnavailable: false },
    { workspaceListReady: false, workspaceListUnavailable: true },
    { workspaceListReady: true, workspaceListUnavailable: false },
  ])("opens account independently of workspace list: %o", (list) => {
    expect(accountBootstrapDestination({ ...ready, ...list })).toBe("account");
  });
  it("preserves restored account selection with available workspaces", () => {
    expect(accountBootstrapDestination({ ...ready, activeScope: "account", workspaceListReady: true, workspaceCount: 3 })).toBeNull();
  });
  it("keeps workspace tabs during loading but falls back to account on initial list failure", () => {
    expect(accountBootstrapDestination({ ...ready, activeScope: "workspace" })).toBeNull();
    expect(accountBootstrapDestination({ ...ready, activeScope: "workspace", workspaceListUnavailable: true })).toBe("account");
  });
  it.each([{ authenticated: false }, { restricted: true }])("keeps identity and required setup/password gates: %o", (auth) => {
    expect(accountBootstrapDestination({ ...ready, ...auth })).toBeNull();
  });
});
