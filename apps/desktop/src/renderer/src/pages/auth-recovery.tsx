import { useEffect, useState } from "react";
import type { AuthRecoveryState } from "@multica/core/auth";
import { Button } from "@multica/ui/components/ui/button";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { copyText } from "@multica/ui/lib/clipboard";
import { useLocale, useT } from "@multica/views/i18n";
import { DragStrip } from "@multica/views/platform";
import { ChevronDown, Loader2 } from "lucide-react";

function displayEndpoint(value: string): string {
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    return url.origin + (url.pathname === "/" ? "" : url.pathname);
  } catch {
    return "";
  }
}

export function DesktopAuthRecoveryPage({
  recovery,
  onRetry,
  onChangeServer,
}: {
  recovery: AuthRecoveryState;
  onRetry: () => void;
  onChangeServer: () => void | Promise<void>;
}) {
  const { t } = useT("auth");
  const locale = useLocale();
  const [now, setNow] = useState(Date.now);
  const [copyState, setCopyState] = useState<"idle" | "copying" | "copied" | "failed">("idle");
  const [changeState, setChangeState] = useState<"idle" | "opening" | "opened" | "failed">("idle");
  const { failure, isRetrying, nextRetryAt } = recovery;
  const configuredEndpoint = window.desktopAPI.runtimeConfig.ok
    ? window.desktopAPI.runtimeConfig.config.apiUrl
    : "";
  const endpoint = displayEndpoint(configuredEndpoint) || t(($) => $.desktop.recovery.unknown);

  useEffect(() => {
    if (nextRetryAt === null) return;
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), 1_000);
    return () => clearInterval(timer);
  }, [nextRetryAt]);

  const failureMessage = (() => {
    switch (failure?.kind) {
      case "timeout": return t(($) => $.desktop.recovery.failure_timeout);
      case "network": return t(($) => $.desktop.recovery.failure_network);
      case "server": return t(($) => $.desktop.recovery.failure_server);
      default: return t(($) => $.desktop.recovery.failure_unknown);
    }
  })();
  const failureTime = failure && Number.isFinite(failure.occurredAt)
    ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "medium" }).format(failure.occurredAt)
    : t(($) => $.desktop.recovery.unknown);
  // Build the disclosure and clipboard from this allowlist, never from an error object.
  const details = [
    { label: t(($) => $.desktop.recovery.server), value: endpoint },
    { label: t(($) => $.desktop.recovery.client_version), value: window.desktopAPI.appInfo.version },
    { label: t(($) => $.desktop.recovery.last_failure), value: failureTime },
    { label: t(($) => $.desktop.recovery.failure_category), value: failureMessage },
    ...(failure?.status !== undefined ? [{ label: t(($) => $.desktop.recovery.http_status), value: String(failure.status) }] : []),
  ];
  const copyDetails = async () => {
    setCopyState("copying");
    const copied = await copyText(details.map(({ label, value }) => `${label}: ${value}`).join("\n"));
    setCopyState(copied ? "copied" : "failed");
  };
  const changeServer = async () => {
    setChangeState("opening");
    try {
      await onChangeServer();
      setChangeState("opened");
    } catch {
      setChangeState("failed");
    }
  };
  const seconds = nextRetryAt === null ? null : Math.max(0, Math.ceil((nextRetryAt - now) / 1_000));

  return (
    <div className="flex h-screen flex-col bg-background text-foreground">
      <DragStrip />
      <main className="flex min-h-0 flex-1 flex-col overflow-y-auto px-6 py-8 sm:px-8">
        <section className="my-auto flex w-full max-w-sm shrink-0 flex-col items-center self-center text-center">
          <MulticaIcon bordered size="lg" />
          <h1 className="mt-6 text-title font-semibold">
            {t(($) => $.desktop.recovery.title)}
          </h1>
          <p className="mt-2 text-body text-muted-foreground">
            {t(($) => $.desktop.recovery.description)}
          </p>
          <p className="mt-5 w-full select-text text-body font-medium [overflow-wrap:anywhere]">{endpoint}</p>
          <p role="status" className="mt-2 text-body text-muted-foreground">
            {isRetrying ? t(($) => $.desktop.recovery.checking) : failureMessage}
          </p>
          <p className="mt-1 text-caption text-muted-foreground" aria-live="off">
            {isRetrying
              ? t(($) => $.desktop.recovery.retrying_hint)
              : seconds !== null
                ? seconds > 0
                  ? t(($) => $.desktop.recovery.retry_in, { count: seconds })
                  : t(($) => $.desktop.recovery.retry_soon)
                : t(($) => $.desktop.recovery.manual_hint)}
          </p>
          <div className="mt-6 flex w-full flex-wrap justify-center gap-2">
            <Button disabled={isRetrying} onClick={onRetry} className="h-auto min-h-9 min-w-0 whitespace-normal px-4 py-2">
              {isRetrying && <Loader2 aria-hidden className="motion-safe:animate-spin" />}
              {isRetrying ? t(($) => $.desktop.recovery.retrying) : t(($) => $.desktop.recovery.retry)}
            </Button>
            <Button variant="outline" disabled={changeState === "opening"} onClick={() => void changeServer()} className="h-auto min-h-9 min-w-0 whitespace-normal px-4 py-2">
              {changeState === "opening" ? t(($) => $.desktop.recovery.change_pending) : t(($) => $.desktop.recovery.change_server)}
            </Button>
          </div>
          {changeState === "failed" && <p role="alert" className="mt-3 text-caption text-destructive">{t(($) => $.desktop.recovery.change_failed)}</p>}
          {changeState === "opened" && window.desktopAPI.windowContext?.kind === "issue" && <p role="status" className="mt-3 text-caption text-muted-foreground">{t(($) => $.desktop.recovery.child_opened)}</p>}
          <details className="group mt-6 w-full text-left">
            <summary className="flex min-h-9 cursor-pointer list-none items-center justify-center gap-1 rounded-md text-caption text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring">
              {t(($) => $.desktop.recovery.details)}
              <ChevronDown aria-hidden className="size-3.5 group-open:rotate-180" />
            </summary>
            <dl className="mt-3 space-y-3 select-text rounded-lg bg-muted/50 p-4 text-caption">
              {details.map(({ label, value }) => <div key={label}><dt className="text-muted-foreground">{label}</dt><dd className="mt-0.5 [overflow-wrap:anywhere]">{value}</dd></div>)}
            </dl>
            <Button variant="outline" className="mt-3 h-auto min-h-9 w-full whitespace-normal py-2" disabled={copyState === "copying"} onClick={() => void copyDetails()}>
              {copyState === "copying" ? t(($) => $.desktop.recovery.copying_details) : t(($) => $.desktop.recovery.copy_details)}
            </Button>
            <p role="status" className="mt-2 text-center text-caption text-muted-foreground">
              {copyState === "copied" ? t(($) => $.desktop.recovery.copied) : copyState === "failed" ? t(($) => $.desktop.recovery.copy_failed) : null}
            </p>
          </details>
        </section>
      </main>
    </div>
  );
}
