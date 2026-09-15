import type { ComponentType } from "react";
import {
  Bell,
  Blocks,
  CircleDot,
  CreditCard,
  FolderGit2,
  Key,
  Keyboard,
  Plug,
  Server,
  Settings,
  SlidersHorizontal,
  Tag,
  User,
  Users,
  Zap,
} from "lucide-react";
import {
  BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
  PLUGINS_V1_FLAG,
} from "@multica/core/feature-flags";
import type enSettings from "../../locales/en/settings.json";

type SettingsPage = (typeof enSettings)["page"];

/** i18n key under `page.tabs` naming a static nav item. */
export type SettingsTabKey = keyof SettingsPage["tabs"];
/** i18n key under `page.groups` naming a nav group heading. */
export type SettingsNavGroupId = keyof SettingsPage["groups"];

/**
 * Nav-relevant subset of a platform-injected settings tab. `ExtraSettingsTab`
 * in settings-page.tsx satisfies this structurally (it adds `content`); the
 * narrower shape keeps the filtering logic free of React nodes.
 */
export interface InjectedSettingsTab {
  value: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
}

/** A nav item from the shared IA, before feature-flag filtering. */
interface StaticNavItem {
  /** i18n key under `page.tabs`. */
  key: SettingsTabKey;
  /** `?tab=` value; defaults to `key`. */
  value?: string;
  icon: ComponentType<{ className?: string }>;
  /** Feature flag that must be enabled for the item to appear. */
  flag?: string;
}

export interface SettingsNavGroupDef {
  id: SettingsNavGroupId;
  items: readonly StaticNavItem[];
}

/** A nav entry after flag filtering, ready to render. */
export type SettingsNavEntry =
  | {
      kind: "static";
      /** `?tab=` value — the Tabs trigger value. */
      value: string;
      tabKey: SettingsTabKey;
      icon: ComponentType<{ className?: string }>;
    }
  | {
      kind: "injected";
      value: string;
      /** Already localized by the injecting platform. */
      label: string;
      icon: ComponentType<{ className?: string }>;
    };

export interface SettingsNavGroup {
  id: SettingsNavGroupId;
  items: readonly SettingsNavEntry[];
}

// Four-group settings IA: account-level preferences first, then workspace
// administration, issue-model configuration, and external connections. Group
// and item order is part of the design contract — do not reorder casually.
// `value ?? key` keeps every historical `?tab=` URL stable, including the
// backend-driven GitHub App callback (`/settings?tab=github` is redirected
// to Integrations in settings-page.tsx).
export const SETTINGS_NAV_GROUPS: readonly SettingsNavGroupDef[] = [
  {
    id: "account",
    items: [
      { key: "profile", icon: User },
      { key: "preferences", icon: SlidersHorizontal },
      { key: "notifications", icon: Bell },
      { key: "shortcuts", icon: Keyboard },
      { key: "tokens", icon: Key },
    ],
  },
  {
    id: "workspace",
    items: [
      { key: "general", value: "workspace", icon: Settings },
      { key: "members", icon: Users },
      {
        key: "billing",
        icon: CreditCard,
        flag: BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
      },
    ],
  },
  {
    id: "issue",
    items: [
      { key: "issue_statuses", value: "issue-statuses", icon: CircleDot },
      { key: "labels", icon: Tag },
      { key: "properties", icon: SlidersHorizontal },
      { key: "quick_actions", value: "quick-actions", icon: Zap },
    ],
  },
  {
    id: "connections",
    items: [
      { key: "repositories", icon: FolderGit2 },
      { key: "integrations", icon: Plug },
      { key: "mcp", icon: Server },
      { key: "plugins", icon: Blocks, flag: PLUGINS_V1_FLAG },
    ],
  },
];

/**
 * Resolve the visible nav groups: drop flag-gated items, append
 * platform-injected tabs to the account group, and drop groups left with no
 * items at all (a heading with nothing under it reads as a bug, not an
 * empty state). Injected tabs are appended before the empty-group filter so
 * they keep the account group alive even if every static item were gated.
 */
export function visibleSettingsNavGroups(
  defs: readonly SettingsNavGroupDef[],
  flags: Readonly<Record<string, boolean>>,
  injectedAccountTabs?: readonly InjectedSettingsTab[],
): SettingsNavGroup[] {
  return defs
    .map((group) => {
      const items: SettingsNavEntry[] = group.items
        .filter((item) => item.flag === undefined || flags[item.flag] === true)
        .map((item) => ({
          kind: "static" as const,
          value: item.value ?? item.key,
          tabKey: item.key,
          icon: item.icon,
        }));
      if (group.id === "account" && injectedAccountTabs?.length) {
        items.push(
          ...injectedAccountTabs.map((tab) => ({
            kind: "injected" as const,
            value: tab.value,
            label: tab.label,
            icon: tab.icon,
          })),
        );
      }
      return { id: group.id, items };
    })
    .filter((group) => group.items.length > 0);
}
