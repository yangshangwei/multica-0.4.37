import { MANAGEMENT_PENDING_LABEL_KEYS } from "../../../shared/daemon-types";
import { useState, useEffect, useCallback, type ReactNode } from "react";
import { AlertCircle, Info, LogIn } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "@multica/views/i18n";
import { toast } from "sonner";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
  SettingsTab,
} from "@multica/views/settings";
import { daemonDiagnosticText } from "../platform/daemon-diagnostics";
import { reauthenticateDaemon } from "../platform/daemon-reauth";
import type { DaemonDiagnostic, DaemonPrefs, DaemonStatus } from "../../../shared/daemon-types";
import {
  DAEMON_STATE_COLORS,
  formatUptime,
} from "../../../shared/daemon-types";

// One row inside the diagnostics block. Values that are likely to be
// long IDs / URLs render as monospaced + truncated with a tooltip.
function DiagnosticsRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="grid grid-cols-[140px_minmax(0,1fr)] items-baseline gap-3 py-1.5">
      <span className="text-caption text-muted-foreground">{label}</span>
      <span
        className={cn(
          "min-w-0 truncate text-body",
          mono && "font-mono text-caption",
        )}
        title={typeof value === "string" ? value : undefined}
      >
        {value}
      </span>
    </div>
  );
}

function DiagnosticCard({ kind, diagnostic }: { kind: "cli" | "connection"; diagnostic?: DaemonDiagnostic }) {
  const { t, i18n } = useT("settings");
  const text = daemonDiagnosticText(diagnostic, t, kind === "cli" ? "cli_unavailable" : "operation_failed");
  const title = t($ => $.desktop.daemon.diagnostic[kind === "cli" ? "cli_title" : "connection_title"]);
  const time = diagnostic?.occurredAt && Number.isFinite(diagnostic.occurredAt) && !Number.isNaN(new Date(diagnostic.occurredAt).getTime())
    ? new Date(diagnostic.occurredAt).toLocaleString(i18n.language) : null;
  return (
    <SettingsCard>
      <section aria-label={title} className="min-w-0 space-y-2 break-words px-4 py-4 text-body">
        <h3 className="font-medium">{title}</h3>
        <p className="text-caption text-muted-foreground">{t($ => $.desktop.daemon.diagnostic.stage, { stage: text.stage })}</p>
        <p>{text.summary}</p>
        {text.fields && <p>{t($ => $.desktop.daemon.diagnostic.fields, { fields: text.fields })}</p>}
        {diagnostic?.httpStatus && Number.isInteger(diagnostic.httpStatus) && diagnostic.httpStatus >= 400 && diagnostic.httpStatus <= 599 && (
          <p>{t($ => $.desktop.daemon.diagnostic.http_status, { status: diagnostic.httpStatus })}</p>
        )}
        <ol className="list-decimal space-y-2 pl-5 text-muted-foreground">
          {text.steps.map(step => <li key={step}>{step}</li>)}
        </ol>
        {time && <p className="text-caption text-muted-foreground">{t($ => $.desktop.daemon.diagnostic.record, { code: text.code, time })}</p>}
      </section>
    </SettingsCard>
  );
}

export function DaemonSettingsTab() {
  const { t } = useT("settings");
  const [prefs, setPrefs] = useState<DaemonPrefs>({ autoStart: true, autoStop: false });
  const [cli, setCli] = useState<
    { state: "checking" } | { state: "failed" } | { state: "ready"; path: string | null }
  >({ state: "checking" });
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState<DaemonStatus>({ state: "stopped" });
  const [statusLoading, setStatusLoading] = useState(true);
  const [reauthLoading, setReauthLoading] = useState(false);
  const user = useAuthStore((state) => state.user);

  useEffect(() => {
    window.daemonAPI.getPrefs().then(setPrefs);
    window.daemonAPI.getCliPath().then(
      (path) => setCli({ state: "ready", path }),
      () => setCli({ state: "failed" }),
    );
    window.daemonAPI.getStatus().then((nextStatus) => {
      setStatus(nextStatus);
      setStatusLoading(false);
    });
    return window.daemonAPI.onStatusChange((nextStatus) => {
      setStatus(nextStatus);
      setStatusLoading(false);
    });
  }, []);

  const handleReauth = useCallback(async () => {
    setReauthLoading(true);
    try {
      await reauthenticateDaemon(t);
    } finally {
      setReauthLoading(false);
    }
  }, [t]);

  const updatePref = useCallback(
    async (key: keyof DaemonPrefs, value: boolean) => {
      setSaving(true);
      try {
        const updated = await window.daemonAPI.setPrefs({ [key]: value });
        setPrefs(updated);
        toast.success(t(($) => $.desktop.daemon.toast_saved), {
          id: "settings-auto-save",
        });
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : t(($) => $.desktop.daemon.toast_save_failed),
        );
      } finally {
        setSaving(false);
      }
    },
    [t],
  );

  const cliMissing = cli.state === "ready" && cli.path === null;

  // The daemon runs somewhere the app can't drive (e.g. inside WSL2 behind a
  // Windows desktop): /health is reachable but the lifecycle CLI can't reach
  // its process. Auto-start/auto-stop can't work, so disable them and say why
  // rather than letting the toggles silently no-op. See #3916.
  const externallyManaged = status.externallyManaged === true;
  const hasSession = Boolean(user && localStorage.getItem("multica_token"));
  const isTransitioning =
    status.state === "stopping" ||
    status.state === "installing_cli";
  const reauthDisabled =
    reauthLoading || statusLoading || externallyManaged || !hasSession || isTransitioning;
  const uptime = formatUptime(status.uptime).replace(
    /(\d+(?:\.\d+)?)(h|m|s)\b/g,
    (_, value: string, unit: "h" | "m" | "s") =>
      t(($) => $.desktop.daemon.uptime_units[unit], { value }),
  );

  return (
    <SettingsTab
      title={t(($) => $.desktop.daemon.title)}
      description={t(($) => $.desktop.daemon.description)}
    >

      {status.state === "auth_expired" && (
        <div className="mt-4 flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 px-4 py-3">
          <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium text-destructive">
              {t(($) => $.desktop.daemon.auth_expired_title)}
            </p>
            <p className="mt-0.5 text-body text-muted-foreground">
              {t(($) => $.desktop.daemon.auth_expired_description)}
            </p>
          </div>
        </div>
      )}

      {externallyManaged && (
        <div className="mt-4 flex items-start gap-3 rounded-lg border bg-muted/30 px-4 py-3">
          <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p className="min-w-0 text-body text-muted-foreground">
            {t(($) => $.desktop.daemon.externally_managed_prefix)}{" "}
            <code className="font-mono text-caption">multica daemon start</code> /{" "}
            <code className="font-mono text-caption">multica daemon stop</code>
            {t(($) => $.desktop.daemon.externally_managed_suffix)}
          </p>
        </div>
      )}

      <SettingsCard>
        <SettingsRow
          label={t(($) => $.desktop.daemon.auto_start_title)}
          description={t(($) => $.desktop.daemon.auto_start_description)}
        >
          <Switch
            aria-label={t(($) => $.desktop.daemon.auto_start_title)}
            checked={prefs.autoStart}
            onCheckedChange={(checked) => updatePref("autoStart", checked)}
            disabled={saving || externallyManaged}
          />
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.daemon.auto_stop_title)}
          description={t(($) => $.desktop.daemon.auto_stop_description)}
        >
          <Switch
            aria-label={t(($) => $.desktop.daemon.auto_stop_title)}
            checked={prefs.autoStop}
            onCheckedChange={(checked) => updatePref("autoStop", checked)}
            disabled={saving || externallyManaged}
          />
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.daemon.sign_in_again)}
          description={t(($) => $.desktop.daemon.sign_in_again_description)}
        >
          <Button
            variant="outline"
            size="sm"
            onClick={handleReauth}
            disabled={reauthDisabled}
          >
            <LogIn className="mr-1.5 size-3.5" />
            {reauthLoading
              ? t(($) => $.desktop.daemon.sign_in_again_loading)
              : t(($) => $.desktop.daemon.sign_in_again)}
          </Button>
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.daemon.cli_status)}
          description={
            <>
              <p>
                {cli.state === "checking"
                  ? t(($) => $.desktop.daemon.cli_checking)
                  : cli.state === "failed"
                    ? t(($) => $.desktop.daemon.cli_lookup_failed)
                    : cli.path
                      ? t(($) => $.desktop.daemon.cli_installed)
                      : t(($) => $.desktop.daemon.cli_missing)}
              </p>
              {cli.state === "ready" && cli.path && (
                <dl className="mt-2">
                  <dt>{t(($) => $.desktop.daemon.cli_installation_path)}</dt>
                  <dd className="mt-0.5">
                    <code className="block select-text whitespace-pre-wrap break-all font-mono text-caption text-foreground">{cli.path}</code>
                  </dd>
                </dl>
              )}
            </>
          }
        >
          {cliMissing && (
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                window.desktopAPI.openExternal(
                  "https://github.com/multica-ai/multica#cli-installation",
                )
              }
            >
              {t(($) => $.desktop.daemon.installation_guide)}
            </Button>
          )}
          {!cliMissing && <span />}
        </SettingsRow>
      </SettingsCard>

      {/* Diagnostics — moved out of the logs panel so the panel can focus
          on logs. These fields matter for support tickets and bug reports,
          not for everyday use. */}
      <SettingsSection
        title={t(($) => $.desktop.daemon.diagnostics_title)}
        description={t(($) => $.desktop.daemon.diagnostics_description)}
      >
        <div aria-live="polite" className="space-y-4">
          {(status.diagnostics?.cli || status.state === "cli_not_found") && <DiagnosticCard kind="cli" diagnostic={status.diagnostics?.cli} />}
          {status.diagnostics?.connection && <DiagnosticCard kind="connection" diagnostic={status.diagnostics.connection} />}
        </div>
        <SettingsCard>
          <div className="px-4 py-2">
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.state)}
            value={
              <span className="inline-flex items-center gap-1.5">
                <span
                  className={cn(
                    "size-1.5 rounded-full",
                    DAEMON_STATE_COLORS[status.state],
                  )}
                />
                {status.managementPendingReason ? t(($) => $.desktop.daemon[MANAGEMENT_PENDING_LABEL_KEYS[status.managementPendingReason ?? "switching"]]) : t(($) => $.desktop.daemon.states[status.state])}
              </span>
            }
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.uptime)}
            value={status.uptime ? uptime : "—"}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.pid)}
            value={status.pid ?? "—"}
            mono={!!status.pid}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.daemon_id)}
            value={status.daemonId ?? "—"}
            mono={!!status.daemonId}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.profile)}
            value={status.profile || t(($) => $.desktop.daemon.default_profile)}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.server_url)}
            value={status.serverUrl ?? "—"}
            mono={!!status.serverUrl}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.device_name)}
            value={status.deviceName ?? "—"}
          />
          <DiagnosticsRow
            label={t(($) => $.desktop.daemon.workspaces)}
            value={
              typeof status.workspaceCount === "number"
                ? status.workspaceCount
                : "—"
            }
          />
          </div>
        </SettingsCard>
      </SettingsSection>
    </SettingsTab>
  );
}
