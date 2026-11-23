// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  deriveWsUrl,
  mergeRuntimeConfigInput,
  parseRuntimeConfig,
  resolveAppUrl,
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
      updateUrl: "http://congvc-x99.taila6fa8a.ts.net:18080/desktop",
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
      updateUrl: "http://api.multica.ai:18080/desktop",
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
      updateUrl: "http://api.example.com:18080/desktop",
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

  it.each([
    ["http://10.10.10.20:8080", "http://10.10.10.20:18080/desktop"],
    ["https://api.example.com:18443/api/?x=1#fragment", "http://api.example.com:18080/desktop"],
    ["http://localhost:80", "http://localhost:18080/desktop"],
    ["https://[2001:db8::1]:8443/api", "http://[2001:db8::1]:18080/desktop"],
  ])("defaults the update directory for %s", (apiUrl, updateUrl) => {
    expect(
      parseRuntimeConfig(
        JSON.stringify({ schemaVersion: 1, apiUrl }),
      ),
    ).toHaveProperty("updateUrl", updateUrl);
  });

  it("moves the standard update directory when the server hostname changes", () => {
    const current = parseRuntimeConfig(JSON.stringify({
      schemaVersion: 1,
      apiUrl: "https://old.example.com:8443",
      updateUrl: "http://old.example.com:18080/desktop",
    }));
    const saved = parseRuntimeConfig(JSON.stringify(
      mergeRuntimeConfigInput(current, { apiUrl: "https://new.example.com:9443" }),
    ));

    expect(saved.updateUrl).toBe("http://new.example.com:18080/desktop");
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

// Canonical matrix for the origin used by copied/share/invite links. The
// navigation suites only assert that the adapters are wired to it.
describe("resolveAppUrl", () => {
  const installed = (apiUrl: string, appUrl?: string) => ({
    source: "configured" as const,
    config: parseRuntimeConfig(
      JSON.stringify({ schemaVersion: 1, apiUrl, ...(appUrl ? { appUrl } : {}) }),
    ),
  });

  it("uses the origin the server publishes when the local one is only a guess", () => {
    // An intranet desktop is given the API address alone; the guess is that
    // same origin, where /join answers 404.
    const runtime = installed("http://10.10.10.30:8080");
    expect(runtime.config.appUrl).toBe("http://10.10.10.30:8080");
    expect(resolveAppUrl(runtime, "http://10.10.10.30:3000/")).toBe(
      "http://10.10.10.30:3000",
    );
  });

  it("keeps an appUrl that was configured on purpose", () => {
    const runtime = installed("http://10.10.10.30:8080", "https://work.corp.test");
    expect(resolveAppUrl(runtime, "http://10.10.10.30:3000")).toBe(
      "https://work.corp.test",
    );
  });

  it.each([undefined, null, "", "   "])(
    "keeps the derived origin when the server publishes nothing (%j)",
    (published) => {
      // The managed cloud omits daemon_app_url; so do older servers.
      expect(resolveAppUrl(installed("https://api.multica.ai"), published)).toBe(
        "https://multica.ai",
      );
    },
  );

  it.each([
    "10.10.10.30:3000",
    "ftp://10.10.10.30",
    "http://user:pass@10.10.10.30:3000",
    "not a url",
  ])("ignores a malformed published origin (%s)", (published) => {
    expect(resolveAppUrl(installed("http://10.10.10.30:8080"), published)).toBe(
      "http://10.10.10.30:8080",
    );
  });

  it.each(["http://localhost:3000", "http://127.0.0.1:3000", "http://[::1]:3000"])(
    "ignores a loopback origin published by a networked server (%s)",
    (published) => {
      // docker-compose.selfhost.yml defaults MULTICA_APP_URL to localhost:3000;
      // on another machine that address is the reader's own computer.
      expect(resolveAppUrl(installed("http://10.10.10.30:8080"), published)).toBe(
        "http://10.10.10.30:8080",
      );
    },
  );

  it("replaces the dev localhost:3000 guess with the environment's web origin", () => {
    const runtime = {
      source: "dev" as const,
      config: runtimeConfigFromDevEnv({ apiUrl: "http://localhost:18572" }),
    };
    expect(runtime.config.appUrl).toBe("http://localhost:3000");
    expect(resolveAppUrl(runtime, "http://localhost:13492")).toBe(
      "http://localhost:13492",
    );
  });

  it("keeps a dev VITE_APP_URL over the published origin", () => {
    const runtime = {
      source: "dev" as const,
      config: runtimeConfigFromDevEnv({
        apiUrl: "http://localhost:18572",
        appUrl: "http://localhost:4000",
      }),
    };
    expect(resolveAppUrl(runtime, "http://localhost:13492")).toBe(
      "http://localhost:4000",
    );
  });
});
