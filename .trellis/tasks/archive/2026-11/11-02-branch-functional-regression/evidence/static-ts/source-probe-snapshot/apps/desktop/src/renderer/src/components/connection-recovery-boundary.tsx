import { useCallback, useEffect, useState, type ReactNode } from "react";
import type { AuthRecoveryState } from "@multica/core/auth";
import { Button } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "@multica/views/i18n";
import { DesktopAuthRecoveryPage } from "../pages/auth-recovery";
import { DesktopEndpointSetupPage } from "../pages/endpoint-setup";

/** Owns editor lifetime independently of authentication and workspace recovery. */
export function DesktopConnectionRecoveryBoundary({
  recovery,
  onRetry,
  onEditorOpenChange,
  children,
}: {
  recovery: AuthRecoveryState | null;
  onRetry: () => void;
  onEditorOpenChange?: (open: boolean) => void;
  children: ReactNode;
}) {
  const { t } = useT("auth");
  const locale = useLocale();
  const [editingServer, setEditingServer] = useState(false);
  const isMainWindow = window.desktopAPI.windowContext?.kind !== "issue";
  const openEditor = useCallback(() => {
    onEditorOpenChange?.(true);
    setEditingServer(true);
  }, [onEditorOpenChange]);
  const closeEditor = () => {
    setEditingServer(false);
    onEditorOpenChange?.(false);
  };

  useEffect(() => {
    if (!isMainWindow) return;
    return window.desktopAPI.onRuntimeConfigEditorOpen(openEditor);
  }, [isMainWindow, openEditor]);

  const changeServer = async () => {
    if (isMainWindow) {
      openEditor();
      return;
    }
    const result = await window.desktopAPI.openRuntimeConfigEditor();
    if (!result.ok) throw new Error("Could not open main server editor");
  };

  if (editingServer) {
    return <DesktopEndpointSetupPage
      initialApiUrl={window.desktopAPI.runtimeConfig.ok ? window.desktopAPI.runtimeConfig.config.apiUrl : undefined}
      locale={locale}
      notice={t(($) => $.desktop.recovery.switch_notice)}
      saveLabel={t(($) => $.desktop.recovery.save_connect)}
      secondaryAction={<Button variant="ghost" className="h-auto min-h-9 w-full whitespace-normal py-2" onClick={closeEditor}>{t(($) => $.common.back)}</Button>}
    />;
  }
  if (recovery) {
    return <DesktopAuthRecoveryPage recovery={recovery} onRetry={onRetry} onChangeServer={changeServer} />;
  }
  return children;
}
