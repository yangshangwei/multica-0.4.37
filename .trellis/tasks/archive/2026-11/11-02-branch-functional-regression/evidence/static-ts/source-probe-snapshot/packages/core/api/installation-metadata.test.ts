// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const proof = () => `mip_a.${btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 300 }))}.b`;
afterEach(() => vi.unstubAllGlobals());

describe("installation attribution transport", () => {
  it("sends hints only for business writes and clears them when credentials change", async () => {
    const fetch = vi.fn().mockImplementation(async () => new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    const api = new ApiClient("https://task.example");
    api.setToken("current-human-token");
    const value = { serverUrl: "https://task.example", userId: "00000000-0000-4000-8000-000000000001", authVersion: "1", proof: proof() };
    expect(api.setInstallationMetadataProof(value)).toBe(true);
    await api.subscribeToIssue("00000000-0000-4000-8000-000000000002");
    expect(fetch.mock.calls.at(-1)?.[1].headers["X-Installation-Proof"]).toBe(value.proof);
    await api.getAdminMe();
    expect(fetch.mock.calls.at(-1)?.[1].headers["X-Installation-Proof"]).toBeUndefined();
    api.setToken("replacement-human-token");
    await api.subscribeToIssue("00000000-0000-4000-8000-000000000002");
    expect(fetch.mock.calls.at(-1)?.[1].headers["X-Installation-Proof"]).toBeUndefined();
  });

  it("refuses another endpoint and expired or malformed hints", () => {
    const api = new ApiClient("https://task.example");
    const value = { serverUrl: "https://other.example", userId: "00000000-0000-4000-8000-000000000001", authVersion: "1", proof: proof() };
    expect(api.setInstallationMetadataProof(value)).toBe(false);
    expect(api.setInstallationMetadataProof({ ...value, serverUrl: "https://task.example", proof: "not-a-proof" })).toBe(false);
    expect(api.setInstallationMetadataProof({ ...value, serverUrl: "https://task.example", proof: `mip_a.${btoa('{"exp":1}')}.b` })).toBe(false);
  });
});
