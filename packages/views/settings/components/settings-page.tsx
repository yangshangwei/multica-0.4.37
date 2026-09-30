"use client";

import React from "react";
import { ChevronDown, X } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger,
} from "@multica/ui/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { useIsMobile } from "@multica/ui/hooks/use-mobile";
import { useFeatureEnabled } from "@multica/core/config";
import {
  BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
  PLUGINS_V1_FLAG,
} from "@multica/core/feature-flags";
import { useNavigation } from "../../navigation";
import { AccountTab } from "./account-tab";
import { PreferencesTab } from "./preferences-tab";
import { TokensTab } from "./tokens-tab";
import { WorkspaceTab } from "./workspace-tab";
import { MembersTab } from "./members-tab";
import { RepositoriesTab } from "./repositories-tab";
import { IntegrationsTab } from "./integrations-tab";
import { NotificationsTab } from "./notifications-tab";
import { LabelsTab } from "./labels-tab";
import { IssueStatusesTab } from "./issue-statuses-tab";
import { PropertiesTab } from "./properties-tab";
import { QuickActionsTab } from "./quick-actions-tab";
import { KeyboardShortcutsTab } from "./keyboard-shortcuts-tab";
import { PluginsTab } from "./plugins-tab";
import { McpTab } from "./mcp-tab";
import { BillingTab } from "./billing-tab";
import {
  SETTINGS_NAV_GROUPS,
  visibleSettingsNavGroups,
} from "./settings-nav";
import { CollapsedNavTrigger } from "../../layout/page-header";
import { useT } from "../../i18n";

export interface ExtraSettingsTab {
  value: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  content: React.ReactNode;
}

interface SettingsPageProps {
  /** Additional tabs injected by platform (e.g. desktop daemon settings) */
  extraDesktopTabs?: ExtraSettingsTab[];
}

const DEFAULT_TAB = "profile";
const TAB_QUERY_KEY = "tab";

// Legacy `?tab=…` values that no longer name a tab of their own. Lark used to
// be its own top-level workspace tab; it now lives inside Integrations. So
// did GitHub — and the Go backend still redirects installing users back to
// `/settings?tab=github` after the GitHub App install callback
// (server/internal/handler/github.go, githubSettingsURL), so the GitHub
// mapping is a backend-driven URL contract, not an internal shim.
const LEGACY_WORKSPACE_TAB_REDIRECTS: Record<string, string> = {
  lark: "integrations",
  github: "integrations",
};

const SETTINGS_TAB_TRIGGER_CLASS =
  "h-auto min-h-11 w-full flex-none justify-start px-2 py-1.5 text-left whitespace-normal [overflow-wrap:anywhere] hover:bg-surface-hover data-active:!bg-surface-selected data-active:!text-surface-selected-foreground data-active:hover:!bg-surface-selected focus-visible:outline-2 focus-visible:outline-foreground focus-visible:outline-offset-2 focus-visible:ring-0 after:hidden md:min-h-8 md:py-1 pointer-coarse:min-h-11";

export function SettingsPage({ extraDesktopTabs }: SettingsPageProps = {}) {
  const { t } = useT("settings");
  const navigation = useNavigation();
  const isMobile = useIsMobile();
  const [directoryOpen, setDirectoryOpen] = React.useState(false);
  const navigationId = React.useId();
  const selectedTabRef = React.useRef<HTMLButtonElement>(null);
  React.useEffect(() => {
    if (!isMobile && directoryOpen) {
      setDirectoryOpen(false);
      selectedTabRef.current?.focus();
    }
  }, [isMobile, directoryOpen]);
  const pluginsEnabled = useFeatureEnabled(PLUGINS_V1_FLAG, false);
  const billingEnabled = useFeatureEnabled(
    BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
    false,
  );

  const flagState = React.useMemo(
    () => ({
      [PLUGINS_V1_FLAG]: pluginsEnabled,
      [BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG]: billingEnabled,
    }),
    [pluginsEnabled, billingEnabled],
  );

  const visibleGroups = React.useMemo(
    () => visibleSettingsNavGroups(SETTINGS_NAV_GROUPS, flagState, extraDesktopTabs),
    [flagState, extraDesktopTabs],
  );

  // Whitelist of valid tab values; unknown ?tab=… values silently fall back to
  // the default. Whitelisting also blocks junk like ?tab=<script> from
  // surfacing in the DOM via Radix Tabs internals.
  const validTabs = React.useMemo(
    () => new Set(visibleGroups.flatMap((g) => g.items.map((i) => i.value))),
    [visibleGroups],
  );

  const tabFromUrl = navigation.searchParams.get(TAB_QUERY_KEY);
  const candidateTab = tabFromUrl
    ? tabFromUrl === "billing" && !billingEnabled
      ? "workspace"
      : LEGACY_WORKSPACE_TAB_REDIRECTS[tabFromUrl] ?? tabFromUrl
    : null;
  const activeTab =
    candidateTab && validTabs.has(candidateTab) ? candidateTab : DEFAULT_TAB;
  const activeEntry = visibleGroups.flatMap((group) => group.items)
    .find((item) => item.value === activeTab);
  const activeLabel = activeEntry?.kind === "injected"
    ? activeEntry.label
    : t(($) => $.page.tabs[activeEntry?.tabKey ?? "profile"]);

  // replace (not push) so settings tab switches don't pollute browser history.
  // Preserve any other query params the page may carry.
  const handleTabChange = (next: string) => {
    setDirectoryOpen(false);
    const params = new URLSearchParams(navigation.searchParams);
    params.set(TAB_QUERY_KEY, next);
    navigation.replace(`${navigation.pathname}?${params.toString()}`);
  };

  const navList = (
    <TabsList
      aria-labelledby={`${navigationId}-title`}
      variant="line"
      className="flex h-auto w-full flex-col items-stretch justify-start gap-4 p-0"
    >
      {visibleGroups.map((group) => (
        <div
          key={group.id}
          role="group"
          aria-labelledby={`${navigationId}-${group.id}`}
          className="flex min-w-0 flex-col gap-1"
        >
          <h2
            id={`${navigationId}-${group.id}`}
            className="px-2 pb-1 text-caption font-medium text-muted-foreground"
          >
            {t(($) => $.page.groups[group.id])}
          </h2>
          {group.items.map((item) => (
            <TabsTrigger
              key={item.value}
              value={item.value}
              ref={item.value === activeTab ? selectedTabRef : undefined}
              onClick={() => setDirectoryOpen(false)}
              className={SETTINGS_TAB_TRIGGER_CLASS}
            >
              <item.icon className="size-4" aria-hidden="true" />
              {item.kind === "static"
                ? t(($) => $.page.tabs[item.tabKey])
                : item.label}
            </TabsTrigger>
          ))}
        </div>
      ))}
    </TabsList>
  );

  return (
    <Tabs
      value={activeTab}
      onValueChange={handleTabChange}
      orientation="vertical"
      className="flex flex-1 min-h-0 flex-col gap-0 overflow-y-auto md:flex-row md:overflow-hidden"
    >
      {isMobile ? (
        <div className="flex shrink-0 items-center gap-2 border-b border-surface-border p-2 [&>[data-slot=sidebar-trigger]]:size-11">
          <CollapsedNavTrigger />
          <h1 className="sr-only">{t(($) => $.page.title)}</h1>
          <Sheet open={directoryOpen} onOpenChange={setDirectoryOpen}>
            <SheetTrigger render={
              <Button
                variant="outline"
                aria-label={`${t(($) => $.page.directory)}: ${activeLabel}`}
                className="h-auto min-h-11 min-w-0 flex-1 justify-between gap-3 px-3 py-2 text-left whitespace-normal focus-visible:ring-2 focus-visible:ring-foreground"
              />
            }>
              <span className="min-w-0 [overflow-wrap:anywhere]">
                <span className="block text-caption text-muted-foreground">{t(($) => $.page.directory)}</span>
                <span className="block">{activeLabel}</span>
              </span>
              <ChevronDown className="size-4" aria-hidden="true" />
            </SheetTrigger>
            <SheetContent
              side="left"
              showCloseButton={false}
              initialFocus={selectedTabRef}
              aria-describedby={undefined}
              className="gap-0 data-[side=left]:w-full data-[side=left]:max-w-sm"
            >
              <SheetHeader className="flex-row items-center justify-between gap-2">
                <SheetTitle id={`${navigationId}-title`}>{t(($) => $.page.directory)}</SheetTitle>
                <SheetClose render={
                  <Button variant="ghost" size="icon" className="size-11 focus-visible:ring-2 focus-visible:ring-foreground" aria-label={t(($) => $.page.close_directory)} />
                }>
                  <X className="size-4" aria-hidden="true" />
                </SheetClose>
              </SheetHeader>
              <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">{navList}</div>
            </SheetContent>
          </Sheet>
        </div>
      ) : (
        // Keep the desktop navigation on the content surface so it merges
        // into the active window tab. The divider supplies the zoning.
        <div className="w-56 shrink-0 overflow-y-auto border-r border-surface-border p-4">
          <div className="mb-6 flex items-center">
            <CollapsedNavTrigger />
            <h1 id={`${navigationId}-title`} className="px-2 text-body font-semibold">{t(($) => $.page.title)}</h1>
          </div>
          {navList}
        </div>
      )}

      {/* Right content */}
      <div className="min-w-0 flex-1 md:overflow-y-auto">
        <div className={`mx-auto w-full p-4 sm:p-6 md:p-8 ${activeTab === "labels" || activeTab === "issue-statuses" || activeTab === "properties" || activeTab === "quick-actions"
              ? "max-w-5xl"
              : "max-w-3xl"}`}>
          <TabsContent aria-label={activeLabel} value="profile"><AccountTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="preferences"><PreferencesTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="shortcuts"><KeyboardShortcutsTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="notifications"><NotificationsTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="tokens"><TokensTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="workspace"><WorkspaceTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="members"><MembersTab /></TabsContent>
          {/* Flag-gated tabs render their content unconditionally: validTabs
              already excludes a gated value, so the panel can never activate
              while the flag is off. */}
          <TabsContent aria-label={activeLabel} value="billing"><BillingTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="issue-statuses"><IssueStatusesTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="labels"><LabelsTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="properties"><PropertiesTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="quick-actions"><QuickActionsTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="repositories"><RepositoriesTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="integrations"><IntegrationsTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="mcp"><McpTab /></TabsContent>
          <TabsContent aria-label={activeLabel} value="plugins"><PluginsTab /></TabsContent>
          {extraDesktopTabs?.map((tab) => (
            <TabsContent aria-label={activeLabel} key={tab.value} value={tab.value}>{tab.content}</TabsContent>
          ))}
        </div>
      </div>
    </Tabs>
  );
}
