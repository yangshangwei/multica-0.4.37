// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  deriveWsUrl,
  mergeRuntimeConfigInput,
  parseRuntimeConfig,
  runtimeConfigFromDevEnv,
} from "./runtime-config";

describe("runtime config", () => {
  it("derives https/wss compatible URLs from apiUrl", () => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({
          schemaVersion: 1,
          apiUrl: "https://congvc-x99.taila6fa8a.ts.net:18443",
        }),
      ),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://congvc-x99.taila6fa8a.ts.net:18443",
      wsUrl: "wss://congvc-x99.taila6fa8a.ts.net:18443/ws",
      appUrl: "https://congvc-x99.taila6fa8a.ts.net:18443",
    });
  });

  it("strips the leading api. label when deriving appUrl", () => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({ schemaVersion: 1, apiUrl: "https://api.multica.ai" }),
      ),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.multica.ai",
      wsUrl: "wss://api.multica.ai/ws",
      appUrl: "https://multica.ai",
    });
  });

  it("derives ws for http api URLs", () => {
    expect(deriveWsUrl("http://localhost:8080")).toBe("ws://localhost:8080/ws");
  });

  it("accepts explicit appUrl and wsUrl", () => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({
          schemaVersion: 1,
          apiUrl: "https://api.example.com/",
          wsUrl: "wss://ws.example.com/socket/",
          appUrl: "https://app.example.com/",
        }),
      ),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.example.com",
      wsUrl: "wss://ws.example.com/socket",
      appUrl: "https://app.example.com",
    });
  });

  it("accepts and normalizes an optional updateUrl", () => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({
          schemaVersion: 1,
          apiUrl: "https://api.example.com",
          updateUrl: "https://updates.example.com/desktop/?x=1#frag",
        }),
      ),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.example.com",
      wsUrl: "wss://api.example.com/ws",
      appUrl: "https://example.com",
      updateUrl: "https://updates.example.com/desktop",
    });
  });

  it("omits updateUrl when it is not configured", () => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({ schemaVersion: 1, apiUrl: "https://api.example.com" }),
      ),
    ).not.toHaveProperty("updateUrl");
  });

  it("rejects an updateUrl that is empty, non-http, or carries credentials", () => {
    for (const updateUrl of ["", "ftp://updates.example.com", "https://u:p@updates.example.com"]) {
      expect(() =>
        parseRuntimeConfig(
          JSON.stringify({ schemaVersion: 1, apiUrl: "https://api.example.com", updateUrl }),
        ),
      ).toThrow(/updateUrl/);
    }
  });

  it("preserves a configured updateUrl when the login page saves server URLs", () => {
    const current = parseRuntimeConfig(
      JSON.stringify({
        schemaVersion: 1,
        apiUrl: "https://api.old.example.com",
        updateUrl: "https://updates.example.com/desktop",
      }),
    );

    expect(
      parseRuntimeConfig(
        JSON.stringify(mergeRuntimeConfigInput(current, { apiUrl: "https://api.new.example.com" })),
      ),
    ).toMatchObject({
      apiUrl: "https://api.new.example.com",
      updateUrl: "https://updates.example.com/desktop",
    });
    expect(mergeRuntimeConfigInput(null, { apiUrl: "https://api.new.example.com" })).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.new.example.com",
    });
  });

  it("rejects invalid JSON", () => {
    expect(() => parseRuntimeConfig("{")).toThrow(/Invalid desktop runtime config JSON/);
  });

  it("rejects unsupported schema versions", () => {
    expect(() =>
      parseRuntimeConfig(JSON.stringify({ schemaVersion: 2, apiUrl: "https://api.example.com" })),
    ).toThrow(/schemaVersion/);
  });

  it("rejects non-http api schemes", () => {
    expect(() =>
      parseRuntimeConfig(JSON.stringify({ schemaVersion: 1, apiUrl: "file:///tmp/multica" })),
    ).toThrow(/apiUrl must use http or https/);
  });

  it("rejects URLs containing credentials", () => {
    expect(() =>
      parseRuntimeConfig(
        JSON.stringify({ schemaVersion: 1, apiUrl: "https://user:secret@example.com" }),
      ),
    ).toThrow(/credentials/);
  });

  it("rejects non-ws websocket schemes", () => {
    expect(() =>
      parseRuntimeConfig(
        JSON.stringify({
          schemaVersion: 1,
          apiUrl: "https://api.example.com",
          wsUrl: "https://api.example.com/ws",
        }),
      ),
    ).toThrow(/wsUrl must use ws or wss/);
  });

  it("preserves electron-vite dev env precedence", () => {
    expect(
      runtimeConfigFromDevEnv({
        apiUrl: "http://dev-api.example.test:8080/",
        wsUrl: "ws://dev-api.example.test:8080/ws/",
        appUrl: "http://dev-app.example.test:3000/",
      }),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "http://dev-api.example.test:8080",
      wsUrl: "ws://dev-api.example.test:8080/ws",
      appUrl: "http://dev-app.example.test:3000",
    });
  });

  it("falls back to local web URL when dev apiUrl is localhost", () => {
    expect(runtimeConfigFromDevEnv({ apiUrl: "http://localhost:8080" })).toEqual({
      schemaVersion: 1,
      apiUrl: "http://localhost:8080",
      wsUrl: "ws://localhost:8080/ws",
      appUrl: "http://localhost:3000",
    });
  });

  it("derives dev appUrl by stripping the leading api. label", () => {
    // When the dev renderer is pointed at a remote backend (e.g. a test
    // environment), copy-link / share URLs must reflect that environment's
    // public web host, not the api host. Multica's convention exposes the
    // api at `api.<web-host>`, so stripping the leading label gives the
    // right web origin without a separate VITE_APP_URL.
    expect(
      runtimeConfigFromDevEnv({ apiUrl: "https://api.test.multica.ai" }),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.test.multica.ai",
      wsUrl: "wss://api.test.multica.ai/ws",
      appUrl: "https://test.multica.ai",
    });
  });

  it("dev VITE_APP_URL still wins over apiUrl-derived value", () => {
    expect(
      runtimeConfigFromDevEnv({
        apiUrl: "https://api.test.multica.ai",
        appUrl: "https://staging.multica.ai",
      }),
    ).toEqual({
      schemaVersion: 1,
      apiUrl: "https://api.test.multica.ai",
      wsUrl: "wss://api.test.multica.ai/ws",
      appUrl: "https://staging.multica.ai",
    });
  });
});
