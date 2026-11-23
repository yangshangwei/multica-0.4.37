import { act, cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { DesktopInstallationMetadataBridge } from "./installation-metadata-bridge";

const fixture = vi.hoisted(() => ({
  auth: { user: { id: "user-a" }, status: "authenticated" },
  epoch: "1", serverUrl: "https://example.test", setProof: vi.fn(),
}));
vi.mock("@multica/core/api", () => {
  const api = { getSessionScope: () => fixture.epoch, getBaseUrl: () => fixture.serverUrl, setInstallationMetadataProof: fixture.setProof };
  return { getApi: () => api };
});
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign((selector: (state: typeof fixture.auth) => unknown) => selector(fixture.auth), { getState: () => fixture.auth }),
}));

let deliver: (value: unknown) => void;
let resolveSnapshot: (value: unknown) => void;
const metadata = { serverUrl: "https://example.test", userId: "user-a", authVersion: "1", proof: "mip_fixture" };
function installToken(userId: string, version = 1) {
  window.localStorage.setItem("multica_token", `header.${btoa(JSON.stringify({ sub: userId, auth_version: version }))}.signature`);
}

beforeEach(() => {
  fixture.auth = { user: { id: "user-a" }, status: "authenticated" }; fixture.epoch = "1"; fixture.serverUrl = "https://example.test"; fixture.setProof.mockClear();
  installToken("user-a");
  Object.defineProperty(window, "daemonAPI", { configurable: true, value: {
    getInstallationMetadata: () => new Promise((resolve) => { resolveSnapshot = resolve; }),
    onInstallationMetadata: (callback: (value: unknown) => void) => { deliver = callback; return vi.fn(); },
  } });
});
afterEach(() => { cleanup(); window.localStorage.clear(); });

it("keeps a fresh event when an older getter resolves later", async () => {
  render(<DesktopInstallationMetadataBridge />);
  act(() => deliver(metadata));
  expect(fixture.setProof).toHaveBeenLastCalledWith(metadata);
  await act(async () => resolveSnapshot(null));
  expect(fixture.setProof).toHaveBeenLastCalledWith(metadata);
});

it("ignores wrong scope and old credential events", () => {
  const view = render(<DesktopInstallationMetadataBridge />);
  for (const change of [{ serverUrl: "https://other.test" }, { userId: "user-b" }, { authVersion: "2" }]) act(() => deliver({ ...metadata, ...change }));
  expect(fixture.setProof).toHaveBeenCalledTimes(1);
  fixture.epoch = "2";
  act(() => deliver(metadata));
  expect(fixture.setProof).toHaveBeenCalledTimes(1);
  view.rerender(<DesktopInstallationMetadataBridge />);
  act(() => deliver(metadata));
  expect(fixture.setProof).toHaveBeenLastCalledWith(metadata);
});

it("clears metadata when the renderer switches accounts", () => {
  const view = render(<DesktopInstallationMetadataBridge />);
  act(() => deliver(metadata));
  fixture.auth = { user: { id: "user-b" }, status: "authenticated" }; fixture.epoch = "2"; installToken("user-b");
  view.rerender(<DesktopInstallationMetadataBridge />);
  expect(fixture.setProof).toHaveBeenLastCalledWith(null);
  act(() => deliver(metadata));
  expect(fixture.setProof).toHaveBeenLastCalledWith(null);
});
