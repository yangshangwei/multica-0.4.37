// @vitest-environment node

import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import { AppConfigSchema, EMPTY_APP_CONFIG } from "./schemas";

describe("messaging integration deployment config", () => {
  it("preserves messaging availability for older servers that omit the policy", () => {
    expect(AppConfigSchema.parse({}).messaging_integrations_enabled).toBe(true);
  });

  it.each([true, false])("preserves the explicit policy %s", (enabled) => {
    const config = AppConfigSchema.parse({ messaging_integrations_enabled: enabled });

    expect(config.messaging_integrations_enabled).toBe(enabled);
  });

  it.each([null, "false", "true", 0, 1, {}, []])(
    "does not enable messaging for malformed policy %j",
    (value) => {
      const config = AppConfigSchema.parse({ messaging_integrations_enabled: value });

      expect(config.messaging_integrations_enabled).toBe(false);
    },
  );

  it("keeps messaging disabled and Git available despite unrelated config drift", () => {
    const config = parseWithFallback(
      {
        messaging_integrations_enabled: false,
        vcs_integration_available: true,
        google_client_id: {},
        device_auth_available: "yes",
        feature_flags: ["unexpected"],
      },
      AppConfigSchema,
      EMPTY_APP_CONFIG,
      { endpoint: "GET /api/config" },
    );

    expect(config.messaging_integrations_enabled).toBe(false);
    expect(config.vcs_integration_available).toBe(true);
  });

  it("does not offer messaging when the entire config response is unreadable", () => {
    const config = parseWithFallback(
      null,
      AppConfigSchema,
      EMPTY_APP_CONFIG,
      { endpoint: "GET /api/config" },
    );

    expect(config.messaging_integrations_enabled).toBe(false);
  });
});
