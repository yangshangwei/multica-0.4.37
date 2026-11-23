// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountBootstrapDestination } from "./account-bootstrap";

describe("authenticated account boot", () => {
  const ready = { authenticated: true, restricted: false, hasOnboarded: true, activeScope: "none" as const, workspaceListReady: false, workspaceListUnavailable: false, workspaceCount: 0 };
  it.each([
    { workspaceListReady: false, workspaceListUnavailable: true },
    { workspaceListReady: true, workspaceListUnavailable: false },
  ])("opens account when no workspace can be chosen: %o", (list) => {
    expect(accountBootstrapDestination({ ...ready, ...list })).toBe("account");
  });
  it.each([
    { workspaceListReady: false, workspaceListUnavailable: true },
    { workspaceListReady: true, workspaceListUnavailable: false },
  ])("leaves a new account's first-run routing in control: %o", (list) => {
    expect(accountBootstrapDestination({ ...ready, ...list, hasOnboarded: false })).toBeNull();
  });
  it("waits for a pending list instead of defaulting a fresh session to account", () => {
    expect(accountBootstrapDestination(ready)).toBeNull();
  });
  // Regression: a fresh sign-in with workspaces used to stay on the account tab,
  // hiding the workspace sidebar (projects-p1-desktop, triage-desktop, ...).
  it("leaves a fresh session with workspaces to first-workspace selection", () => {
    expect(accountBootstrapDestination({ ...ready, workspaceListReady: true, workspaceCount: 2 })).toBeNull();
  });
  it("preserves restored account selection with available workspaces", () => {
    expect(accountBootstrapDestination({ ...ready, activeScope: "account", workspaceListReady: true, workspaceCount: 3 })).toBeNull();
  });
  it("preserves explicit account selection before onboarding", () => {
    expect(accountBootstrapDestination({ ...ready, hasOnboarded: false, activeScope: "account", workspaceListReady: true })).toBeNull();
  });
  it("keeps workspace tabs during loading but falls back to account on initial list failure", () => {
    expect(accountBootstrapDestination({ ...ready, activeScope: "workspace" })).toBeNull();
    expect(accountBootstrapDestination({ ...ready, activeScope: "workspace", workspaceListUnavailable: true })).toBe("account");
  });
  it.each([{ authenticated: false }, { restricted: true }])("keeps identity and required setup/password gates: %o", (auth) => {
    expect(accountBootstrapDestination({ ...ready, ...auth })).toBeNull();
  });
});
