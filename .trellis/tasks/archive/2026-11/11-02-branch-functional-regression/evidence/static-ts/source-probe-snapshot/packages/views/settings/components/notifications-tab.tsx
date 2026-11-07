"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { notificationPreferenceOptions } from "@multica/core/notification-preferences/queries";
import { useUpdateNotificationPreferences } from "@multica/core/notification-preferences/mutations";
import type { NotificationGroupKey, NotificationPreferences } from "@multica/core/types";
import { Switch } from "@multica/ui/components/ui/switch";
import { Button } from "@multica/ui/components/ui/button";
import { toast } from "sonner";
import { useT } from "../../i18n";
import { BrowserNotificationSetting } from "./browser-notification-setting";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
  SettingsTab,
} from "./settings-layout";

// Inbox event groups rendered in the per-event toggle list. `system_notifications`
// is a sibling preference key but lives in its own section below.
const INBOX_GROUP_KEYS = [
  "assignments",
  "status_changes",
  "comments",
  "mentions",
  "updates",
  "agent_activity",
] as const;
type InboxGroupKey = (typeof INBOX_GROUP_KEYS)[number];

export function NotificationsTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const query = useQuery(notificationPreferenceOptions(wsId));
  const { data } = query;
  const mutation = useUpdateNotificationPreferences();

  const preferences = data?.preferences ?? {};

  const handleToggle = (key: NotificationGroupKey, enabled: boolean) => {
    if (data === undefined) return;
    const updated: NotificationPreferences = {
      ...preferences,
      [key]: enabled ? "all" : "muted",
    };
    // Remove keys set to "all" (default) to keep the object clean
    if (enabled) {
      delete updated[key];
    }
    mutation.mutate(updated, {
      onSuccess: () =>
        toast.success(t(($) => $.auto_save.toast_saved), {
          id: "settings-auto-save",
        }),
      onError: (err) =>
        toast.error(
          err instanceof Error && err.message
            ? err.message
            : t(($) => $.notifications.toast_failed),
        ),
    });
  };

  const systemEnabled = preferences.system_notifications !== "muted";
  const failed = query.isError || (query.isFetching && query.errorUpdatedAt > query.dataUpdatedAt);
  const feedback = failed ? (
    <div role="alert" className="flex flex-wrap items-center gap-3 text-body">
      <p className="min-w-0 flex-1 basis-48 text-muted-foreground">
        {data === undefined
          ? t(($) => $.notifications.load_failed)
          : t(($) => $.notifications.refresh_failed)}
      </p>
      <Button variant="outline" size="sm" disabled={query.isFetching} onClick={() => void query.refetch()}>
        {query.isFetching ? t(($) => $.notifications.retrying) : t(($) => $.notifications.retry)}
      </Button>
    </div>
  ) : null;

  if (data === undefined) {
    return (
      <SettingsTab title={t(($) => $.page.tabs.notifications)}>
        {feedback ?? <p role="status" className="text-body text-muted-foreground">{t(($) => $.notifications.loading)}</p>}
      </SettingsTab>
    );
  }

  return (
    <SettingsTab title={t(($) => $.page.tabs.notifications)}>
      {feedback}
      <SettingsSection
        title={t(($) => $.notifications.title)}
        description={t(($) => $.notifications.description)}
      >
        <SettingsCard>
            {INBOX_GROUP_KEYS.map((key: InboxGroupKey) => {
              const enabled = preferences[key] !== "muted";
              return (
                <SettingsRow
                  key={key}
                  label={t(($) => $.notifications.groups[key].label)}
                  description={t(($) => $.notifications.groups[key].description)}
                >
                  <Switch
                    checked={enabled}
                    aria-label={t(($) => $.notifications.groups[key].label)}
                    onCheckedChange={(checked) => handleToggle(key, checked)}
                  />
                </SettingsRow>
              );
            })}
        </SettingsCard>
      </SettingsSection>

      <SettingsSection
        title={t(($) => $.notifications.system.title)}
        description={t(($) => $.notifications.system.description)}
      >
        <SettingsCard>
          <SettingsRow
            label={t(($) => $.notifications.system.label)}
            description={t(($) => $.notifications.system.hint)}
          >
              <Switch
                checked={systemEnabled}
                aria-label={t(($) => $.notifications.system.label)}
                onCheckedChange={(checked) => handleToggle("system_notifications", checked)}
              />
          </SettingsRow>
        </SettingsCard>

        {/* Web-only: the browser permission banners require. Renders nothing on
            desktop (OS-native delivery) or where the Notification API is absent. */}
        <BrowserNotificationSetting />
      </SettingsSection>
    </SettingsTab>
  );
}
