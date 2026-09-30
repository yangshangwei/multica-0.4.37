import { SettingsPage } from "@multica/views/settings";
import { useT } from "@multica/views/i18n";
import { AppWindow, Cpu, Download, Network } from "lucide-react";
import { DaemonSettingsTab } from "./daemon-settings-tab";
import { UpdatesSettingsTab } from "./updates-settings-tab";
import { RuntimeConfigSettingsTab } from "./runtime-config-settings-tab";
import { DesktopBehaviorSettingsTab } from "./desktop-behavior-settings-tab";

/**
 * Wraps `SettingsPage` so the desktop-only extra tabs can pull their labels
 * from i18n. The route element has to be a component (not a literal JSX
 * value) for `useT` to run.
 */
export function DesktopSettingsRoute() {
  const { t } = useT("settings");
  const os = window.desktopAPI.appInfo.os;
  const supportsCloseBehavior = os === "windows" || os === "linux";
  return (
    <SettingsPage
      extraDesktopTabs={[
        {
          value: "daemon",
          label: t(($) => $.desktop.tabs.daemon),
          icon: Cpu,
          content: <DaemonSettingsTab />,
        },
        {
          value: "server",
          label: t(($) => $.desktop.tabs.server),
          icon: Network,
          content: <RuntimeConfigSettingsTab />,
        },
        {
          value: "updates",
          label: t(($) => $.desktop.tabs.updates),
          icon: Download,
          content: <UpdatesSettingsTab />,
        },
        ...(supportsCloseBehavior
          ? [{
              value: "behavior",
              label: t(($) => $.desktop.tabs.behavior),
              icon: AppWindow,
              content: <DesktopBehaviorSettingsTab />,
            }]
          : []),
      ]}
    />
  );
}
