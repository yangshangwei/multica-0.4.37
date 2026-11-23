import { SettingsTab } from "@multica/views/settings";
import { useT } from "@multica/views/i18n";
import { DesktopEndpointSetupPage } from "../pages/endpoint-setup";

/** Reuses the first-run endpoint editor from the desktop Settings surface. */
export function RuntimeConfigSettingsTab() {
  const { t } = useT("settings");
  const runtimeConfig = window.desktopAPI.runtimeConfig;

  return (
    <SettingsTab
      title={t(($) => $.desktop.runtime_config.title)}
      description={t(($) => $.desktop.runtime_config.description)}
    >
      {runtimeConfig.ok ? (
        <DesktopEndpointSetupPage
          embedded
          initialApiUrl={runtimeConfig.config.apiUrl}
        />
      ) : (
        <DesktopEndpointSetupPage
          embedded
          initialError={runtimeConfig.error.message}
        />
      )}
    </SettingsTab>
  );
}
