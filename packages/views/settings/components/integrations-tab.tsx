"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ChevronRight, GitBranch, Plug } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { composioToolkitsOptions } from "@multica/core/composio";
import { useConfigStore, useFeatureEnabled } from "@multica/core/config";
import { COMPOSIO_MCP_APPS_FLAG } from "@multica/core/feature-flags";
import { deriveGitHubSettings, githubInstallationsOptions } from "@multica/core/github";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentWorkspace } from "@multica/core/paths";
import { vcsConnectionsOptions } from "@multica/core/vcs";
import { memberListOptions } from "@multica/core/workspace/queries";
import { cn } from "@multica/ui/lib/utils";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { GitHubTab } from "./github-tab";
import { GitHubMark } from "./github-mark";
import { LarkTab } from "./lark-tab";
import { ComposioTab } from "./composio-tab";
import { SlackTab } from "./slack-tab";
import { DingTalkTab } from "./dingtalk-tab";
import { VCSTab } from "./vcs-tab";
import { WecomTab } from "./wecom-tab";
import { TelegramTab } from "./telegram-tab";
import { SettingsSection, SettingsTab } from "./settings-layout";
import { IntegrationChannelIcon } from "./integration-channel-icon";
import { buildSettingsHref, getRequestedIntegration } from "./settings-integration-navigation";

const PROVIDER_TABS = {
  github: GitHubTab, vcs: VCSTab, lark: LarkTab, slack: SlackTab,
  dingtalk: DingTalkTab, wecom: WecomTab, telegram: TelegramTab,
};
type Provider = keyof typeof PROVIDER_TABS;
const CHANNELS = ["lark", "slack", "dingtalk", "wecom", "telegram"] as const;
type CatalogEntry = { id: Provider; planned?: false } | {
  id: "ones" | "plane" | "kaneo" | "fuxin";
  planned: true;
};

// Only the selected provider mounts its form. Composio keeps its existing
// deployment-gated surface and callback handling, outside this catalog change.
export function IntegrationsTab() {
  const { t } = useT("settings");
  const navigation = useNavigation();
  const messagingEnabled = useConfigStore((s) => s.messagingIntegrationsEnabled);
  const vcsAvailable = useConfigStore((s) => s.vcsIntegrationAvailable);
  const composioEnabled = useFeatureEnabled(COMPOSIO_MCP_APPS_FLAG, false);
  const composioToolkits = useQuery({ ...composioToolkitsOptions(), enabled: composioEnabled });
  const composioUnconfigured = composioToolkits.error instanceof ApiError && composioToolkits.error.status === 503;
  const requested = getRequestedIntegration(navigation.searchParams);
  const availableProviders: Provider[] = ["github", ...(vcsAvailable ? ["vcs" as const] : []), ...(messagingEnabled ? CHANNELS : [])];
  // Config loads independently of auth. Keep gated deep links intact so a
  // slow config response can still reveal the requested, authorized detail.
  const selected = availableProviders.find((id) => id === requested);
  const catalogHref = buildSettingsHref(navigation, "integrations");
  const previousProvider = useRef<Provider | null>(null);
  const cardRefs = useRef(new Map<string, HTMLAnchorElement>());
  const backRef = useRef<HTMLAnchorElement>(null);

  useEffect(() => {
    if (selected) {
      previousProvider.current = selected;
      backRef.current?.focus();
    } else if (previousProvider.current) {
      cardRefs.current.get(previousProvider.current)?.focus();
      previousProvider.current = null;
    }
  }, [selected]);

  if (selected) {
    const ProviderTab = PROVIDER_TABS[selected];
    return (
      <div className="space-y-5 [overflow-wrap:anywhere]">
        <AppLink ref={backRef} href={catalogHref} className="inline-flex min-h-11 items-center gap-2 rounded-md text-body text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-foreground focus-visible:outline-offset-2">
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t(($) => $.integrations.back)}
        </AppLink>
        <SettingsTab
          title={<span className="flex items-center gap-3"><ProviderIcon id={selected} />{t(($) => $.integrations.providers[selected].name)}</span>}
          description={t(($) => $[selected].page_description)}
        >
          <ProviderTab />
        </SettingsTab>
      </div>
    );
  }

  const groups: { id: "code" | "tasks" | "chat"; entries: CatalogEntry[] }[] = [
    { id: "code", entries: [{ id: "github" }, ...(vcsAvailable ? [{ id: "vcs" as const }] : [])] },
    { id: "tasks", entries: [{ id: "ones", planned: true }, { id: "plane", planned: true }, { id: "kaneo", planned: true }] },
    { id: "chat", entries: [...(messagingEnabled ? CHANNELS.map((id) => ({ id })) : []), { id: "fuxin", planned: true }] },
  ];

  return (
    <div className="@container [overflow-wrap:anywhere]">
      <SettingsTab title={t(($) => $.page.tabs.integrations)} description={t(($) => $.integrations.description)}>
        {requested && !selected && (
          <p role="status" className="text-body text-muted-foreground">{t(($) => $.integrations.unavailable)}</p>
        )}
        {groups.filter((group) => group.entries.length > 0).map((group) => (
          <SettingsSection key={group.id} title={t(($) => $.integrations.groups[group.id].title)} description={t(($) => $.integrations.groups[group.id].description)}>
            <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
              {group.entries.map((entry) => (
                <IntegrationCard key={entry.id} entry={entry} href={entry.planned ? undefined : buildSettingsHref(navigation, "integrations", entry.id)} registerRef={(element) => {
                  if (element) cardRefs.current.set(entry.id, element);
                  else cardRefs.current.delete(entry.id);
                }} />
              ))}
            </div>
          </SettingsSection>
        ))}
        {composioEnabled && !composioUnconfigured && (
          <SettingsSection title={t(($) => $.composio.section_title)}>
            <ComposioTab />
          </SettingsSection>
        )}
      </SettingsTab>
    </div>
  );
}

function IntegrationCard({ entry, href, registerRef }: {
  entry: CatalogEntry;
  href?: string;
  registerRef: (element: HTMLAnchorElement | null) => void;
}) {
  const { t } = useT("settings");
  const id = useId();
  const content = <>
    <span className="flex size-9 shrink-0 items-center justify-center" aria-hidden="true">
      {entry.planned ? <Plug className="size-6 text-muted-foreground" /> : <ProviderIcon id={entry.id} />}
    </span>
    <span className="min-w-0 flex-1 space-y-1">
      <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span id={`${id}-title`} className="text-body font-semibold">{t(($) => $.integrations.providers[entry.id].name)}</span>
        <span id={`${id}-status`} className="inline-flex flex-wrap gap-1">
        {entry.planned && <StatusBadge>{t(($) => $.integrations.planned)}</StatusBadge>}
        {entry.id === "github" && <GitHubCatalogStatus />}
        {entry.id === "vcs" && <VCSCatalogStatus />}
        </span>
      </span>
      <span id={`${id}-description`} className="block text-caption leading-5 text-muted-foreground">{t(($) => $.integrations.providers[entry.id].description)}</span>
    </span>
    {!entry.planned && <ChevronRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />}
  </>;
  const className = "flex min-h-24 min-w-0 items-center gap-3 rounded-xl border border-surface-border px-4 py-4";
  return entry.planned ? <div className={className}>{content}</div> : (
    <AppLink ref={registerRef} href={href!} aria-labelledby={`${id}-title`} aria-describedby={`${id}-description ${id}-status`} className={cn(className, "bg-card transition-colors hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-foreground focus-visible:outline-offset-2")}>
      {content}
    </AppLink>
  );
}

function ProviderIcon({ id }: { id: Provider }) {
  if (id === "github") return <GitHubMark className="size-7 shrink-0" aria-hidden="true" />;
  if (id === "vcs") return <GitBranch className="size-7 shrink-0 text-muted-foreground" aria-hidden="true" />;
  return <IntegrationChannelIcon channel={id} />;
}

function StatusBadge({ children, connected = false }: { children: ReactNode; connected?: boolean }) {
  return <span className={cn("rounded px-1.5 py-0.5 text-micro font-medium", connected ? "bg-success/10 text-success" : "bg-muted text-muted-foreground")}>{children}</span>;
}

function ConnectionStatus({ pending, error, connected, configured }: {
  pending: boolean; error: boolean; connected: boolean; configured: boolean;
}) {
  const { t } = useT("settings");
  const label = error ? "status_unavailable" : pending ? "loading" : connected ? "connected" : configured ? "not_connected" : "not_configured";
  return <StatusBadge connected={!pending && !error && connected}>{t(($) => $.integrations[label])}</StatusBadge>;
}

function GitHubCatalogStatus() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const workspace = useCurrentWorkspace();
  const user = useAuthStore((s) => s.user);
  const members = useQuery(memberListOptions(wsId));
  const canView = members.data?.some((member) => member.user_id === user?.id) === true;
  const installations = useQuery({ ...githubInstallationsOptions(wsId), enabled: !!wsId && canView });
  return <>
    <ConnectionStatus
      pending={members.isPending || (canView && installations.isPending)}
      error={members.isError || installations.isError || (!members.isPending && !canView)}
      connected={(installations.data?.installations?.length ?? 0) > 0}
      configured={installations.data?.configured === true}
    />
    {" "}
    {workspace && !deriveGitHubSettings(workspace).enabled && <StatusBadge>{t(($) => $.integrations.features_off)}</StatusBadge>}
  </>;
}

function VCSCatalogStatus() {
  const wsId = useWorkspaceId();
  const connections = useQuery(vcsConnectionsOptions(wsId));
  return <ConnectionStatus pending={connections.isPending} error={connections.isError} connected={(connections.data?.connections?.length ?? 0) > 0} configured={connections.data?.configured === true} />;
}
