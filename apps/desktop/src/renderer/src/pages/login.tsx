import { useCallback, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Loader2, Server } from "lucide-react";
import { LoginPage } from "@multica/views/auth";
import { DragStrip } from "@multica/views/platform";
import { useT } from "@multica/views/i18n";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@multica/ui/components/ui/card";
import { Button } from "@multica/ui/components/ui/button";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { useAuthStore } from "@multica/core/auth";
import { useConfigStore } from "@multica/core/config";
import { api } from "@multica/core/api";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { DesktopEndpointSetupPage } from "./endpoint-setup";

/**
 * Post-logout / pre-login page for a desktop build that only ever talks to the
 * backend named in `desktop.json`. Three things make it different from the web
 * login page:
 *
 * 1. The connected deployment is the title, and its address sits in the card
 *    footer with a live reachability dot. A private deployment must not look
 *    like the managed cloud.
 * 2. Sign-in methods are rendered from what the server DECLARED in /api/config,
 *    never from what this client can imagine: Google only with a client id, and
 *    a deployment with intranet device auth gets a single "continue as this
 *    device" button instead of an email form. That last one is load-bearing —
 *    those deployments have no mail relay and no reachable OAuth, and
 *    `logout()` deliberately does not re-arm AuthInitializer's boot-time device
 *    login (it would make signing out a no-op), so without this button the only
 *    way back in is restarting the app.
 * 3. "Change server" is reachable from here. Settings live behind the very door
 *    the user is standing at, so this is the only entry point that works.
 */

type Reachability =
  | { kind: "checking" }
  | { kind: "connected" }
  | { kind: "unreachable"; message: string };

function requireRuntimeConfig() {
  const runtimeConfig = window.desktopAPI.runtimeConfig;
  if (!runtimeConfig.ok) {
    throw new Error(
      "Invariant violated: DesktopLoginPage rendered before App accepted runtime config",
    );
  }
  return runtimeConfig.config;
}

/** Host of the configured backend, or the raw value if it will not parse. */
function hostLabel(apiUrl: string): string {
  try {
    return new URL(apiUrl).host;
  } catch {
    return apiUrl;
  }
}

/**
 * Which deployment this client is talking to, plus the way out to another one.
 * Secondary by construction: the address is a caption, "change server" is a
 * text button, and neither competes with the sign-in action above it.
 */
function ServerIdentity({
  apiUrl,
  reachability,
  serverVersion,
  onRetry,
  onChangeServer,
}: {
  apiUrl: string;
  reachability: Reachability;
  serverVersion: string;
  onRetry: () => void;
  onChangeServer: () => void;
}) {
  const { t } = useT("auth");
  const dot =
    reachability.kind === "connected"
      ? "bg-success"
      : reachability.kind === "unreachable"
        ? "bg-destructive"
        : "bg-muted-foreground";
  const status =
    reachability.kind === "connected"
      ? serverVersion
        ? t(($) => $.desktop.signin.server_connected_version, {
            version: serverVersion,
          })
        : t(($) => $.desktop.signin.server_connected)
      : reachability.kind === "unreachable"
        ? t(($) => $.desktop.signin.server_unreachable)
        : t(($) => $.desktop.signin.server_checking);

  return (
    <div className="flex w-full items-start gap-2 text-left">
      <Server
        aria-hidden
        className="mt-0.5 size-3.5 shrink-0 text-muted-foreground"
      />
      <div className="min-w-0 flex-1 space-y-1">
        <p className="text-caption break-all">{apiUrl}</p>
        <p className="text-caption flex flex-wrap items-center gap-1.5 text-muted-foreground">
          <span aria-hidden className={`size-1.5 shrink-0 rounded-full ${dot}`} />
          {status}
          {reachability.kind === "unreachable" && (
            <>
              <span aria-hidden>·</span>
              <button
                type="button"
                onClick={onRetry}
                className="text-primary underline underline-offset-4"
              >
                {t(($) => $.desktop.signin.retry)}
              </button>
            </>
          )}
        </p>
        {reachability.kind === "unreachable" && reachability.message && (
          <p className="text-caption break-words text-destructive">
            {reachability.message}
          </p>
        )}
      </div>
      <button
        type="button"
        onClick={onChangeServer}
        className="text-caption shrink-0 text-primary underline underline-offset-4"
      >
        {t(($) => $.desktop.signin.change_server)}
      </button>
    </div>
  );
}

export function DesktopLoginPage() {
  const config = requireRuntimeConfig();
  const { t } = useT("auth");
  const qc = useQueryClient();
  const googleClientId = useConfigStore((s) => s.googleClientId);
  const deviceAuthAvailable = useConfigStore((s) => s.deviceAuthAvailable);
  const serverVersion = useConfigStore((s) => s.serverVersion);
  const [changingServer, setChangingServer] = useState(false);
  const [reachability, setReachability] = useState<Reachability>({
    kind: "checking",
  });
  const [probeGeneration, setProbeGeneration] = useState(0);
  const [devicePending, setDevicePending] = useState(false);
  const [deviceError, setDeviceError] = useState("");

  const host = useMemo(() => hostLabel(config.apiUrl), [config.apiUrl]);
  const device = window.desktopAPI.deviceIdentity;

  // Advisory only: the probe is the same manual `/health` check the endpoint
  // editor offers, and it reports failure for a deployment behind a redirecting
  // proxy that the app itself can still talk to. So it colours the footer and
  // never gates the sign-in action.
  useEffect(() => {
    let cancelled = false;
    setReachability({ kind: "checking" });
    void (async () => {
      try {
        const result = await window.desktopAPI.testRuntimeConfig(config.apiUrl);
        if (cancelled) return;
        setReachability(
          result.ok
            ? { kind: "connected" }
            : { kind: "unreachable", message: result.message },
        );
      } catch {
        if (cancelled) return;
        setReachability({ kind: "unreachable", message: "" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [config.apiUrl, probeGeneration]);

  const handleGoogleLogin = useCallback(() => {
    // Open web login in the default browser with the platform=desktop flag. The
    // web callback redirects back through the multica:// deep link with a token.
    window.desktopAPI.openExternal(`${config.appUrl}/login?platform=desktop`);
  }, [config.appUrl]);

  const continueAsDevice = useCallback(async () => {
    if (!device) return;
    setDevicePending(true);
    setDeviceError("");
    try {
      await useAuthStore
        .getState()
        .loginWithDevice(device.deviceId, device.deviceName);
      // Seed the workspace list the way the verification step does, so the
      // shell's first render sees a settled workspace state. This component is
      // already unmounting by now; the cache it writes to outlives it.
      try {
        const wsList = await api.listWorkspaces();
        qc.setQueryData(workspaceKeys.list(), wsList);
      } catch {
        // The shell's own workspace query owns retries from here.
      }
    } catch (err) {
      setDeviceError(
        err instanceof Error
          ? err.message
          : t(($) => $.desktop.signin.device_failed),
      );
      setDevicePending(false);
    }
  }, [device, qc, t]);

  const serverIdentity = (
    <ServerIdentity
      apiUrl={config.apiUrl}
      reachability={reachability}
      serverVersion={serverVersion}
      onRetry={() => setProbeGeneration((n) => n + 1)}
      onChangeServer={() => setChangingServer(true)}
    />
  );

  // Same editor as first run, prefilled with the current address — an empty
  // form would ask the user to retype something they already configured.
  // Deliberately not a WindowOverlay: <WindowOverlay /> lives inside the
  // dashboard shell, which is not mounted while nobody is signed in.
  if (changingServer) {
    return (
      <div className="flex h-screen flex-col">
        <DragStrip />
        <div className="flex-1 overflow-y-auto">
          <DesktopEndpointSetupPage
            embedded
            initialApiUrl={config.apiUrl}
            secondaryAction={
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                onClick={() => setChangingServer(false)}
              >
                {t(($) => $.desktop.signin.back_to_signin)}
              </Button>
            }
          />
        </div>
      </div>
    );
  }

  // Intranet device auth: no email form, because no code would ever arrive.
  if (deviceAuthAvailable && device) {
    return (
      <div className="flex h-screen flex-col">
        <DragStrip />
        <main className="flex flex-1 items-center justify-center p-8">
          <Card className="w-full max-w-sm">
            <CardHeader className="text-center">
              <div className="mx-auto mb-4">
                <MulticaIcon bordered size="lg" />
              </div>
              <CardTitle className="text-display-sm break-words">
                {t(($) => $.desktop.signin.device_title, { host })}
              </CardTitle>
              <CardDescription>
                {t(($) => $.desktop.signin.device_description)}
              </CardDescription>
            </CardHeader>
            <CardFooter className="flex flex-col gap-3">
              <Button
                type="button"
                autoFocus
                className="w-full"
                size="lg"
                disabled={devicePending}
                onClick={() => void continueAsDevice()}
              >
                {devicePending && <Loader2 className="animate-spin" />}
                {devicePending
                  ? t(($) => $.desktop.signin.device_connecting)
                  : t(($) => $.desktop.signin.device_continue)}
              </Button>
              <p className="text-caption w-full break-words text-muted-foreground">
                {t(($) => $.desktop.signin.device_hint, {
                  device: device.deviceName,
                })}
              </p>
              {deviceError && (
                <p className="text-body w-full break-words text-destructive">
                  {deviceError}
                </p>
              )}
              <div className="w-full border-t border-surface-border pt-3">
                {serverIdentity}
              </div>
            </CardFooter>
          </Card>
        </main>
      </div>
    );
  }

  return (
    <div className="flex h-screen flex-col">
      <DragStrip />
      <LoginPage
        logo={<MulticaIcon bordered size="lg" />}
        title={t(($) => $.desktop.signin.title, { host })}
        description={t(($) => $.desktop.signin.description)}
        onSuccess={() => {
          // Auth store update triggers AppContent re-render → shows DesktopShell.
          // Initial workspace navigation happens in routes.tsx via IndexRedirect.
        }}
        // Rendered from the server's declaration, not from this client's guess:
        // a deployment with no GOOGLE_CLIENT_ID would send the user to an
        // external browser with nowhere to land.
        onGoogleLogin={googleClientId ? handleGoogleLogin : undefined}
        footer={serverIdentity}
      />
    </div>
  );
}
