// @vitest-environment node

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";

const metadata = {
  id: "token-1", name: "Test CLI", token_prefix: "mca_test", created_at: "2026-10-01T00:00:00Z",
};
const completeMetadata = { ...metadata, expires_at: null, last_used_at: null };
const fetchMock = vi.fn();
const warn = vi.fn();
const client = new ApiClient("https://tokens.test");

beforeEach(() => {
  fetchMock.mockReset();
  warn.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  setSchemaLogger({ ...noopLogger, warn });
});

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
});

function reply(value: unknown) {
  fetchMock.mockResolvedValue(new Response(JSON.stringify(value), {
    status: 200, headers: { "Content-Type": "application/json" },
  }));
}

describe("personal access token response boundary", () => {
  it("accepts metadata from older servers and never includes credential fields in list results", async () => {
    reply([{ ...metadata, token: "must-not-enter-query-cache", future: true }]);
    expect(await client.listPersonalAccessTokens()).toEqual([completeMetadata]);
  });

  it.each([null, {}, "not a list", [null], [{}], [{ ...completeMetadata, id: " " }]])(
    "rejects unusable list data without inventing an empty successful list (%j)", async (value) => {
      reply(value);
      await expect(client.listPersonalAccessTokens()).rejects.toThrow();
      expect(warn).toHaveBeenCalledOnce();
    },
  );

  it("preserves a genuine empty list", async () => {
    reply([]);
    expect(await client.listPersonalAccessTokens()).toEqual([]);
    expect(warn).not.toHaveBeenCalled();
  });

  it("returns the one-time secret only from a valid create response", async () => {
    reply({ ...metadata, token: "mca_fixture_credential" });
    expect(await client.createPersonalAccessToken({ name: metadata.name })).toEqual({
      ...completeMetadata, token: "mca_fixture_credential",
    });
  });

  it.each([null, {}, { ...completeMetadata }, { ...completeMetadata, token: "" },
    { ...completeMetadata, token: "  " }, { ...completeMetadata, token: 123 }])(
    "rejects an unusable create response without retrying creation (%j)", async (value) => {
      reply(value);
      await expect(client.createPersonalAccessToken({ name: metadata.name })).rejects.toMatchObject({
        body: { code: "invalid_personal_access_token" },
      });
      expect(fetchMock).toHaveBeenCalledOnce();
    },
  );

  it("redacts both payloads and validation details for malformed credential responses", async () => {
    const secret = "mca_never_log_this_fixture_value";
    reply({ ...completeMetadata, token: { secret } });
    let failure: unknown;
    try { await client.createPersonalAccessToken({ name: metadata.name }); } catch (error) { failure = error; }
    expect(warn).toHaveBeenCalledWith(
      "API response failed schema validation: POST /api/tokens",
      { endpoint: "POST /api/tokens" },
    );
    expect(JSON.stringify(warn.mock.calls)).not.toContain(secret);
    expect(String(failure)).not.toContain(secret);
  });

  it("does not leak token text from a JSON syntax error", async () => {
    fetchMock.mockResolvedValue(new Response('mca_private_fixture_value{', { status: 200 }));
    await expect(client.createPersonalAccessToken({ name: metadata.name })).rejects.toMatchObject({
      body: { code: "invalid_personal_access_token" },
    });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("mca_private_fixture_value");
  });
});
