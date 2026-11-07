import { useEffect } from "react";
import { getApi } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { managementAuthVersion, matchesInstallationMetadata } from "../../../shared/managed-installation";

/** Metadata-only proof, scoped to the current renderer credential and server. */
export function DesktopInstallationMetadataBridge() {
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const status = useAuthStore((state) => state.status);
  const sessionScope = useAuthStore(() => getApi().getSessionScope());
  const api = getApi();

  useEffect(() => {
    api.setInstallationMetadataProof(null);
    if (status !== "authenticated" || !userId) return;
    const token = window.localStorage.getItem("multica_token");
    const authVersion = token ? managementAuthVersion(token, userId) : null;
    if (authVersion === null) return;
    let cancelled = false;
    let receivedEvent = false;
    const accept = (value: unknown) => {
      if (cancelled || getApi() !== api || api.getSessionScope() !== sessionScope || useAuthStore.getState().user?.id !== userId || useAuthStore.getState().status !== "authenticated") return;
      if (value === null) { api.setInstallationMetadataProof(null); return; }
      const currentToken = window.localStorage.getItem("multica_token");
      const currentVersion = currentToken ? managementAuthVersion(currentToken, userId) : null;
      if (!matchesInstallationMetadata(value, { serverUrl: api.getBaseUrl(), userId, authVersion: currentVersion })) return;
      api.setInstallationMetadataProof(value);
    };
    const unsubscribe = window.daemonAPI.onInstallationMetadata?.((value) => { receivedEvent = true; accept(value); });
    void window.daemonAPI.getInstallationMetadata?.().then((value) => { if (!receivedEvent) accept(value); }).catch(() => undefined);
    return () => { cancelled = true; unsubscribe?.(); api.setInstallationMetadataProof(null); };
  }, [api, userId, status, sessionScope]);

  return null;
}
