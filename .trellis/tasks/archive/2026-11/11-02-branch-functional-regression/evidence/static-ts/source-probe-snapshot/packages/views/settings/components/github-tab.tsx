"use client";

import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ExternalLink, GitCommitHorizontal, Link2, PanelRight } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Label } from "@multica/ui/components/ui/label";
import { Switch } from "@multica/ui/components/ui/switch";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentWorkspace } from "@multica/core/paths";
import { memberListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import {
  deriveGitHubSettings,
  githubInstallationsOptions,
} from "@multica/core/github";
import { api } from "@multica/core/api";
import type { Workspace } from "@multica/core/types";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { GitHubMark } from "./github-mark";
import { buildSettingsHref } from "./settings-integration-navigation";
import { getSettingsQueryState, SettingsQueryFeedback } from "./settings-query-feedback";

type SettingsKey =
  | "github_enabled"
  | "github_pr_sidebar_enabled"
  | "co_authored_by_enabled"
  | "github_auto_link_prs_enabled";

export function GitHubTab() {
  const { t } = useT("settings");
  const workspace = useCurrentWorkspace();
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const navigation = useNavigation();
  const user = useAuthStore((s) => s.user);

  const membersQuery = useQuery(memberListOptions(wsId));
  const memberState = getSettingsQueryState(membersQuery);
  const currentMember = memberState.hasData
    ? membersQuery.data?.find((m) => m.user_id === user?.id) ?? null
    : null;
  // `canView` gates the read-only installation list (every workspace member
  // sees it after MUL-2413); `canManage` gates the Connect / Disconnect
  // actions and comes from the backend response (`can_manage`) so the
  // frontend never claims management rights the server would reject.
  const canView = !!currentMember;

  const installationsQuery = useQuery({
    ...githubInstallationsOptions(wsId),
    enabled: !!wsId && canView,
  });
  const installationState = getSettingsQueryState(installationsQuery);
  const installationData = installationState.hasData ? installationsQuery.data : undefined;
  const installations = installationData?.installations ?? [];
  const configured = installationData?.configured ?? false;
  const canManage = canView && installationData?.can_manage === true;
  const connected = installations.length > 0;
  const primaryInstallation = installations[0] ?? null;

  const flags = deriveGitHubSettings(workspace);
  const [savingKey, setSavingKey] = useState<SettingsKey | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const pendingRef = useRef(false);
  const controlsPending = savingKey !== null || connecting || disconnecting;

  async function persistSetting(key: SettingsKey, next: boolean) {
    if (!workspace || !canManage || pendingRef.current) return;
    pendingRef.current = true;
    setSavingKey(key);
    try {
      const merged = {
        ...((workspace.settings as Record<string, unknown>) ?? {}),
        [key]: next,
      };
      const updated = await api.updateWorkspace(workspace.id, { settings: merged });
      qc.setQueryData(workspaceKeys.list(), (old: Workspace[] | undefined) =>
        old?.map((ws) => (ws.id === updated.id ? updated : ws)),
      );
      toast.success(t(($) => $.auto_save.toast_saved), {
        id: "settings-auto-save",
      });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.github.toast_failed));
    } finally {
      pendingRef.current = false;
      setSavingKey(null);
    }
  }

  async function handleConnect() {
    if (!canManage || !configured || pendingRef.current) return;
    pendingRef.current = true;
    setConnecting(true);
    try {
      const resp = await api.getGitHubConnectURL(wsId);
      if (!resp.configured || !resp.url) {
        toast.error(t(($) => $.github.toast_not_configured));
        return;
      }
      window.open(resp.url, "_blank", "noopener");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.github.toast_open_failed));
    } finally {
      pendingRef.current = false;
      setConnecting(false);
    }
  }

  async function handleDisconnect() {
    if (!disconnectTarget || !canManage || pendingRef.current) return;
    pendingRef.current = true;
    setDisconnecting(true);
    try {
      await api.deleteGitHubInstallation(wsId, disconnectTarget);
      await qc.invalidateQueries({ queryKey: ["github", wsId] });
      toast.success(t(($) => $.github.toast_disconnected));
      setDisconnectTarget(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.github.toast_disconnect_failed));
    } finally {
      pendingRef.current = false;
      setDisconnecting(false);
    }
  }

  if (!workspace) return null;

  const repositoriesHref = buildSettingsHref(navigation, "repositories");
  const feedback = <>
    <SettingsQueryFeedback
      query={membersQuery}
      loadingMessage={t(($) => $.query_feedback.permissions_loading)}
      errorMessage={t(($) => $.query_feedback.permissions_load_failed)}
      refreshMessage={t(($) => $.query_feedback.permissions_refresh_failed)}
    />
    {canView ? (
      <SettingsQueryFeedback
        query={installationsQuery}
        loadingMessage={t(($) => $.integrations.loading)}
        errorMessage={t(($) => $.integrations.load_failed, { provider: t(($) => $.github.section_title) })}
        refreshMessage={t(($) => $.integrations.refresh_failed, { provider: t(($) => $.github.section_title) })}
      />
    ) : memberState.hasData ? (
      <p role="alert" className="text-body text-muted-foreground">{t(($) => $.query_feedback.access_denied)}</p>
    ) : null}
  </>;

  if (!canView || !installationState.hasData) return <div className="space-y-3">{feedback}</div>;

  // Section body for the GitHub block on the Integrations page. The section
  // heading and description come from the host (integrations-tab.tsx); the
  // h3 sub-headings stay one level below the host's h2.
  return (
    <div className="min-w-0 space-y-6 [overflow-wrap:anywhere] max-md:[&_[data-slot=button]]:min-h-11 pointer-coarse:[&_[data-slot=button]]:min-h-11 max-md:[&_[data-slot=switch]]:after:-inset-y-3.5 pointer-coarse:[&_[data-slot=switch]]:after:-inset-y-3.5">
      {feedback}
      <p role="status" className="text-caption text-muted-foreground">
        {savingKey ? t(($) => $.auto_save.saving) : null}
      </p>
      <Card>
        <CardContent>
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 flex-1 items-start gap-3">
              <div className="hidden shrink-0 rounded-md border bg-muted/50 p-2 text-muted-foreground sm:block">
                <GitHubMark className="h-4 w-4" />
              </div>
              <div className="min-w-0 space-y-1">
                <Label htmlFor="github-master" className="text-body font-medium">
                  {t(($) => $.github.section_master)}
                </Label>
                <p className="text-body text-muted-foreground">
                  {flags.enabled
                    ? t(($) => $.github.master_description_on)
                    : t(($) => $.github.master_description_off)}
                </p>
              </div>
            </div>
            <Switch
              id="github-master"
              checked={flags.enabled}
              onCheckedChange={(v) => persistSetting("github_enabled", v)}
              disabled={!canManage || controlsPending}
            />
          </div>
        </CardContent>
      </Card>

      <section className="space-y-3">
        <h3 className="text-body font-semibold">{t(($) => $.github.section_connection)}</h3>
        <Card>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex min-w-0 flex-1 basis-48 items-start gap-3">
                <GitHubMark className="h-6 w-6 mt-0.5 shrink-0" />
                <div className="min-w-0 space-y-1">
                  <p className="text-body font-medium">{t(($) => $.github.connection_title)}</p>
                  {connected ? (
                    <>
                      <p className="text-caption text-muted-foreground">
                        {t(($) => $.github.connected_to, {
                          login: installations.map((i) => i.account_login).join(", "),
                        })}
                      </p>
                      {primaryInstallation?.connected_by && (
                        <p className="text-caption text-muted-foreground">
                          {t(($) => $.github.connected_by, {
                            name: primaryInstallation.connected_by!,
                          })}
                        </p>
                      )}
                    </>
                  ) : canManage ? (
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.github.connection_description_prefix)}{" "}
                      <code className="rounded bg-muted px-1 py-0.5 text-micro">
                        {t(($) => $.github.connection_identifier_example)}
                      </code>{" "}
                      {t(($) => $.github.connection_description_suffix)}{" "}
                      <strong>{t(($) => $.github.connection_description_done)}</strong>.
                    </p>
                  ) : (
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.github.contact_admin_to_connect)}
                    </p>
                  )}
                </div>
              </div>
              {canManage && (
                <div className="flex items-center gap-2">
                  {connected && primaryInstallation ? (
                    // Disconnect must stay reachable even when the master switch
                    // is off — disconnect is a separate intent (revoke the App
                    // grant) from hiding the feature.
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setDisconnectTarget(primaryInstallation.id)}
                      disabled={controlsPending}
                    >
                      {t(($) => $.github.disconnect)}
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      onClick={handleConnect}
                      disabled={controlsPending || !configured}
                      title={
                        !configured
                          ? t(($) => $.github.connect_disabled_tooltip)
                          : undefined
                      }
                    >
                      {connecting
                        ? t(($) => $.github.connect_opening)
                        : t(($) => $.github.connect_github)}
                    </Button>
                  )}
                </div>
              )}
            </div>

            {canManage && !configured && (
              <p className="text-caption text-muted-foreground">
                {t(($) => $.github.not_configured)}{" "}
                <code className="rounded bg-muted px-1 py-0.5 text-micro">GITHUB_APP_SLUG</code>{" "}
                {t(($) => $.github.not_configured_and)}{" "}
                <code className="rounded bg-muted px-1 py-0.5 text-micro">GITHUB_WEBHOOK_SECRET</code>.
              </p>
            )}

            {!canManage && connected && (
              <p className="text-caption text-muted-foreground">
                {t(($) => $.github.read_only_hint)}
              </p>
            )}
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h3 className="text-body font-semibold">{t(($) => $.github.section_features)}</h3>
        <Card className="gap-0 py-0">
          <CardContent className="divide-y divide-surface-border px-0">
            <FeatureRow
              id="github-pr-sidebar"
              icon={<PanelRight className="h-4 w-4" />}
              label={t(($) => $.github.feature_pr_sidebar_label)}
              description={
                <p className="text-body text-muted-foreground">
                  {t(($) => $.github.feature_pr_sidebar_description)}
                </p>
              }
              checked={flags.prSidebar}
              disabled={!canManage || !flags.enabled || controlsPending}
              onCheckedChange={(v) => persistSetting("github_pr_sidebar_enabled", v)}
            />

            <FeatureRow
              id="github-coauthor"
              icon={<GitCommitHorizontal className="h-4 w-4" />}
              label={t(($) => $.github.feature_co_author_label)}
              description={
                <p className="text-body text-muted-foreground">
                  {t(($) => $.github.feature_co_author_description_prefix)}{" "}
                  <code className="rounded bg-muted px-1 py-0.5 text-caption whitespace-normal">
                    {"Co-authored-by: multica-agent <github@multica.ai>"}
                  </code>{" "}
                  {t(($) => $.github.feature_co_author_description_suffix)}
                </p>
              }
              checked={flags.coAuthor}
              disabled={!canManage || !flags.enabled || controlsPending}
              onCheckedChange={(v) => persistSetting("co_authored_by_enabled", v)}
            />

            <FeatureRow
              id="github-auto-link"
              icon={<Link2 className="h-4 w-4" />}
              label={t(($) => $.github.feature_auto_link_label)}
              description={
                <p className="text-body text-muted-foreground">
                  {t(($) => $.github.feature_auto_link_description)}
                </p>
              }
              checked={flags.autoLinkPRs}
              disabled={!canManage || !flags.enabled || controlsPending}
              onCheckedChange={(v) => persistSetting("github_auto_link_prs_enabled", v)}
            />
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <h3 className="text-body font-semibold">{t(($) => $.github.section_repositories)}</h3>
        <Card>
          <CardContent>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-body font-medium">
                {t(($) => $.github.repositories_shortcut_label)}
              </p>
              <Button
                variant="outline"
                size="sm"
                render={<AppLink href={repositoriesHref} />}
                nativeButton={false}
              >
                <ExternalLink className="h-3 w-3" />
                {t(($) => $.github.repositories_shortcut_link)}
              </Button>
            </div>
          </CardContent>
        </Card>
      </section>

      <AlertDialog
        open={canManage && !!disconnectTarget}
        onOpenChange={(v) => {
          if (!v && !pendingRef.current) setDisconnectTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.github.disconnect_confirm_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.github.disconnect_confirm_description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="max-md:min-h-11 pointer-coarse:min-h-11" disabled={controlsPending}>
              {t(($) => $.github.disconnect_confirm_cancel)}
            </AlertDialogCancel>
            <AlertDialogAction className="max-md:min-h-11 pointer-coarse:min-h-11" onClick={handleDisconnect} disabled={controlsPending}>
              {disconnecting
                ? t(($) => $.github.disconnecting)
                : t(($) => $.github.disconnect_confirm_action)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function FeatureRow({
  id,
  icon,
  label,
  description,
  checked,
  disabled,
  onCheckedChange,
}: {
  id: string;
  icon: React.ReactNode;
  label: string;
  description: React.ReactNode;
  checked: boolean;
  disabled: boolean;
  onCheckedChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4 px-4 py-3.5">
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <div className="hidden shrink-0 rounded-md border bg-muted/50 p-2 text-muted-foreground sm:block">{icon}</div>
        <div className="min-w-0 space-y-1">
          <Label htmlFor={id} className="text-body font-medium">
            {label}
          </Label>
          {description}
        </div>
      </div>
      <Switch
        id={id}
        checked={checked}
        disabled={disabled}
        onCheckedChange={onCheckedChange}
      />
    </div>
  );
}
