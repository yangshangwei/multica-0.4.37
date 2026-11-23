// @vitest-environment node
import { describe, expect, it } from "vitest";

import {
  buildSettingsHref,
  getRequestedIntegration,
} from "./settings-integration-navigation";

describe("getRequestedIntegration", () => {
  it.each([
    ["", null],
    ["tab=integrations", null],
    ["tab=integrations&integration=", null],
    ["tab=integrations&integration=github", "github"],
    ["tab=integrations&integration=unknown", "unknown"],
    ["tab=github", "github"],
    ["tab=lark", "lark"],
    ["tab=github&integration=slack", "github"],
    ["tab=lark&integration=github", "lark"],
    ["tab=profile&integration=github", null],
    ["integration=github", null],
  ])("resolves %s to %s", (search, expected) => {
    expect(getRequestedIntegration(new URLSearchParams(search))).toBe(expected);
  });
});

describe("buildSettingsHref", () => {
  const location = {
    pathname: "/acme/settings",
    searchParams: new URLSearchParams("tab=integrations&integration=github&from=install&label=one&label=two"),
    hash: "#connections",
  };

  it("opens a provider while preserving unrelated parameters and the fragment", () => {
    expect(buildSettingsHref(location, "integrations", "lark")).toBe(
      "/acme/settings?tab=integrations&integration=lark&from=install&label=one&label=two#connections",
    );
    expect(location.searchParams.get("integration")).toBe("github");
  });

  it.each([undefined, null, ""])("clears the provider when selection is %s", (integration) => {
    expect(buildSettingsHref(location, "integrations", integration)).toBe(
      "/acme/settings?tab=integrations&from=install&label=one&label=two#connections",
    );
  });

  it("clears provider state when switching the main tab", () => {
    expect(buildSettingsHref(location, "repositories")).toBe(
      "/acme/settings?tab=repositories&from=install&label=one&label=two#connections",
    );
  });

  it("adds a tab to a location without search parameters or a fragment", () => {
    expect(buildSettingsHref({ pathname: "/acme/settings", searchParams: new URLSearchParams(), hash: "" }, "integrations", "github")).toBe(
      "/acme/settings?tab=integrations&integration=github",
    );
  });
});
