// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  PLUGINS_V1_FLAG,
  BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
} from "@multica/core/feature-flags";
import {
  SETTINGS_NAV_GROUPS,
  visibleSettingsNavGroups,
  type InjectedSettingsTab,
  type SettingsNavGroupDef,
} from "./settings-nav";

const Glyph = () => null;

const INJECTED: readonly InjectedSettingsTab[] = [
  { value: "daemon", label: "Daemon", icon: Glyph },
  { value: "updates", label: "Updates", icon: Glyph },
];

// Canonical ordering matrix for the settings IA. The DOM suite in
// settings-page.test.tsx deliberately does not re-run this through a mount.
describe("visibleSettingsNavGroups", () => {
  it("renders the design IA with every flag off", () => {
    const groups = visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, {});

    expect(groups.map((g) => g.id)).toEqual([
      "account",
      "workspace",
      "issue",
      "connections",
    ]);
    expect(groups.map((g) => g.items.map((i) => i.value))).toEqual([
      ["profile", "preferences", "notifications", "shortcuts", "tokens"],
      ["workspace", "members", "labels", "triage", "iterations"],
      ["issue-statuses", "properties", "quick-actions"],
      ["repositories", "integrations", "mcp"],
    ]);
  });

  it("keeps every historical ?tab= value stable", () => {
    const groups = visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, {});
    const values = groups.flatMap((g) => g.items.map((i) => i.value));

    // `general` maps to `workspace`; `issue_statuses` / `quick_actions` map
    // to their hyphenated forms. These are bookmarks and backend-built URLs.
    for (const legacy of [
      "profile",
      "preferences",
      "notifications",
      "shortcuts",
      "tokens",
      "workspace",
      "members",
      "issue-statuses",
      "labels",
      "properties",
      "quick-actions",
      "repositories",
      "integrations",
      "mcp",
    ]) {
      expect(values).toContain(legacy);
    }
  });

  it("gates billing and plugins behind their feature flags", () => {
    const groups = visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, {
      [PLUGINS_V1_FLAG]: true,
      [BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG]: true,
    });

    expect(groups.map((g) => g.items.map((i) => i.value))).toEqual([
      ["profile", "preferences", "notifications", "shortcuts", "tokens"],
      ["workspace", "members", "labels", "triage", "iterations", "billing"],
      ["issue-statuses", "properties", "quick-actions"],
      ["repositories", "integrations", "mcp", "plugins"],
    ]);
  });

  it("drops a group whose items are all filtered out (no orphan heading)", () => {
    const defs: readonly SettingsNavGroupDef[] = [
      { id: "connections", items: [{ key: "mcp", icon: Glyph, flag: "off" }] },
    ];

    expect(visibleSettingsNavGroups(defs, {})).toEqual([]);
    expect(visibleSettingsNavGroups(defs, { off: true })).toHaveLength(1);
  });

  it("places injected tabs in a final desktop group after workspace configuration", () => {
    const groups = visibleSettingsNavGroups(
      SETTINGS_NAV_GROUPS,
      {},
      INJECTED,
    );

    expect(groups.map((g) => g.id)).toEqual([
      "account",
      "workspace",
      "issue",
      "connections",
      "desktop",
    ]);
    expect(groups.at(-1)?.items).toMatchObject([
      { kind: "injected", value: "daemon", label: "Daemon" },
      { kind: "injected", value: "updates", label: "Updates" },
    ]);
    expect(groups.slice(0, -1)).toEqual(
      visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, {}),
    );
  });

  it("hides the desktop group when a platform supplies no desktop tabs", () => {
    const groups = visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, {}, []);

    expect(groups.map((g) => g.id)).not.toContain("desktop");
  });
});
