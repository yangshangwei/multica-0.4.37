import { useCallback, useEffect, useRef, useState } from "react";
import { AlertCircle, ArrowDownToLine, Check, Loader2 } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "@multica/views/i18n";
import { SettingsCard, SettingsRow, SettingsTab } from "@multica/views/settings";
import { toast } from "sonner";

type CheckState =
  | { status: "idle" }
  | { status: "checking" }
  | { status: "up-to-date" }
  | { status: "available"; latestVersion: string }
  | { status: "error"; message: string };

export function UpdatesSettingsTab() {
  const { t } = useT("settings");
  const [state, setState] = useState<CheckState>({ status: "idle" });
  const [automaticUpdates, setAutomaticUpdates] = useState(true);
  const [preferencesReady, setPreferencesReady] = useState(false);
  const [savingPreference, setSavingPreference] = useState(false);
  const checkingRef = useRef(false);
  const currentVersion = window.desktopAPI.appInfo.version;

  useEffect(() => {
    let mounted = true;
    void window.updater
      .getPreferences()
      .then((preferences) => {
        if (mounted) setAutomaticUpdates(preferences.automaticUpdates);
      })
      .catch(() => {
        // The main process falls back to enabled when preferences cannot be
        // read. Keep the same safe default if IPC itself becomes unavailable.
      })
      .finally(() => {
        if (mounted) setPreferencesReady(true);
      });

    return () => {
      mounted = false;
    };
  }, []);

  const handleAutomaticUpdatesChange = useCallback(
    async (enabled: boolean) => {
      setSavingPreference(true);
      try {
        const preferences = await window.updater.setAutomaticUpdates(enabled);
        setAutomaticUpdates(preferences.automaticUpdates);
        toast.success(t(($) => $.auto_save.toast_saved), {
          id: "settings-auto-save",
        });
      } catch {
        toast.error(t(($) => $.desktop.updates.automatic_updates_save_failed));
      } finally {
        setSavingPreference(false);
      }
    },
    [t],
  );

  const handleCheck = useCallback(async () => {
    // aria-disabled preserves native focus, so also guard synchronous activation.
    if (checkingRef.current) return;
    checkingRef.current = true;
    setState({ status: "checking" });
    try {
      const result = await window.updater.checkForUpdates();
      if (!result.ok) {
        setState({ status: "error", message: result.error });
        return;
      }
      setState(
        result.available
          ? { status: "available", latestVersion: result.latestVersion }
          : { status: "up-to-date" },
      );
    } catch {
      setState({ status: "error", message: t(($) => $.desktop.updates.check_failed) });
    } finally {
      checkingRef.current = false;
    }
  }, [t]);

  return (
    <SettingsTab
      title={t(($) => $.desktop.updates.title)}
      description={t(($) => $.desktop.updates.description)}
    >
      <SettingsCard>
        <SettingsRow label={t(($) => $.desktop.updates.current_version)}>
          <span className="font-mono text-caption text-muted-foreground">
            v{currentVersion}
          </span>
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.updates.automatic_updates_title)}
          description={t(($) => $.desktop.updates.automatic_updates_description)}
        >
          <Switch
            checked={automaticUpdates}
            onCheckedChange={handleAutomaticUpdatesChange}
            disabled={!preferencesReady || savingPreference}
            aria-label={t(($) => $.desktop.updates.automatic_updates_title)}
          />
        </SettingsRow>

        <SettingsRow
          label={t(($) => $.desktop.updates.check_section_title)}
          align="start"
          description={
            <>
              <p>{t(($) => $.desktop.updates.check_section_description)}</p>
              <div role="status" aria-live="polite" aria-atomic="true">
              {state.status === "up-to-date" && (
                <p className="mt-2 inline-flex items-center gap-1.5">
                  <Check className="size-3.5 text-success" />
                  {t(($) => $.desktop.updates.up_to_date)}
                </p>
              )}
              {state.status === "available" && (
                <p className="mt-2 inline-flex items-center gap-1.5">
                  <ArrowDownToLine className="size-3.5 text-primary" />
                  {t(($) => $.desktop.updates.downloading, {
                    version: state.latestVersion,
                  })}
                </p>
              )}
              {state.status === "error" && (
                <p className="mt-2 inline-flex items-center gap-1.5 text-destructive">
                  <AlertCircle className="size-3.5" />
                  {state.message}
                </p>
              )}
              </div>
            </>
          }
        >
          <Button
            variant="outline"
            size="sm"
            className="aria-disabled:pointer-events-none aria-disabled:opacity-50"
            onClick={handleCheck}
            aria-disabled={state.status === "checking"}
          >
            {state.status === "checking" ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                {t(($) => $.desktop.updates.checking)}
              </>
            ) : (
              t(($) => $.desktop.updates.check_now)
            )}
          </Button>
        </SettingsRow>
      </SettingsCard>
    </SettingsTab>
  );
}
