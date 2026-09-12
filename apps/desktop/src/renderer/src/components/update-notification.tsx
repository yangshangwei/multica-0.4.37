import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { RefreshCw, X } from "lucide-react";
import type { SupportedLocale } from "@multica/core/i18n";
import { paths } from "@multica/core/paths";
import { useNavigation } from "@multica/views/navigation";
import en from "@multica/views/locales/en/changelog.json";
import zhHans from "@multica/views/locales/zh-Hans/changelog.json";
import ja from "@multica/views/locales/ja/changelog.json";
import ko from "@multica/views/locales/ko/changelog.json";

type OpenChangelog = ((version: string) => void) | null;
const NotificationNavigation = createContext<{
  open: OpenChangelog;
  register: (action: OpenChangelog) => void;
} | null>(null);

/** Keep the event listener at App scope; only the navigation capability moves. */
export function UpdateNotificationNavigationProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<OpenChangelog>(null);
  const register = useCallback((action: OpenChangelog) => setOpen(() => action), []);
  const value = useMemo(() => ({ open, register }), [open, register]);
  return <NotificationNavigation.Provider value={value}>{children}</NotificationNavigation.Provider>;
}

/** Mount only within the desktop navigation provider, using its resolved workspace. */
export function UpdateNotificationNavigationBridge({ workspaceSlug }: { workspaceSlug: string | null }) {
  const navigation = useNavigation();
  const register = useContext(NotificationNavigation)?.register;
  useEffect(() => {
    register?.(workspaceSlug ? (version) => navigation.push(paths.workspace(workspaceSlug).changelog({ version })) : null);
    return () => register?.(null);
  }, [workspaceSlug, navigation, register]);
  return null;
}

// The prompt also renders before CoreProvider exists (endpoint setup/login).
// Use the already-resolved App locale and shared resources at this boundary.
const COPY = { en: en.desktop, "zh-Hans": zhHans.desktop, ja: ja.desktop, ko: ko.desktop };

// Downloads run silently in the background (main process has
// autoDownload=true). The renderer only renders UI once the package is fully
// downloaded and waiting for a restart.
type UpdateState =
  | { status: "idle" }
  | { status: "ready"; version: string };

export function UpdateNotification({ locale }: { locale: SupportedLocale }) {
  const [state, setState] = useState<UpdateState>({ status: "idle" });
  const [dismissed, setDismissed] = useState(false);
  const openChangelog = useContext(NotificationNavigation)?.open;
  const copy = COPY[locale];

  useEffect(() => {
    const cleanup = window.updater.onUpdateDownloaded((info) => {
      setState({ status: "ready", version: info.version });
      setDismissed(false);
    });
    return cleanup;
  }, []);

  if (state.status === "idle") return null;
  if (dismissed) return null;

  return (
    <div role="status" aria-label={copy.ready} className="fixed bottom-4 right-4 z-50 w-80 max-w-[calc(100vw-2rem)] rounded-lg border border-border bg-background p-4 motion-safe:animate-in motion-safe:slide-in-from-bottom-2 motion-safe:fade-in duration-300">
      <button
        type="button"
        aria-label={copy.dismiss}
        onClick={() => setDismissed(true)}
        className="absolute top-2 right-2 rounded-md p-1 text-muted-foreground hover:text-foreground transition-colors"
      >
        <X aria-hidden="true" className="size-3.5" />
      </button>

      <div className="flex items-start gap-3">
        <div className="mt-0.5 rounded-md bg-success/10 p-1.5">
          <RefreshCw aria-hidden="true" className="size-4 text-success" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-body font-medium">{copy.ready}</p>
          <p className="text-caption text-muted-foreground mt-0.5 break-words">
            {copy.description.replace("{{version}}", state.version.startsWith("v") ? state.version : `v${state.version}`)}
          </p>
          {!openChangelog ? <p className="mt-1 text-caption text-muted-foreground">{copy.workspace_required}</p> : null}
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            <button
              type="button"
              disabled={!openChangelog}
              onClick={() => openChangelog?.(state.version)}
              className="inline-flex items-center rounded-md border border-border bg-background px-3 py-1.5 text-caption font-medium text-foreground hover:bg-accent transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {copy.changelog}
            </button>
            <button
              type="button"
              onClick={() => window.updater.installUpdate()}
              className="inline-flex items-center rounded-md bg-primary px-3 py-1.5 text-caption font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
            >
              {copy.restart}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
