"use client";

import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

export function ChatQueryFeedback({
  kind,
  loading = false,
  failed,
  hasData,
  retrying,
  onRetry,
}: {
  kind: "sessions" | "messages";
  loading?: boolean;
  failed: boolean;
  hasData: boolean;
  retrying: boolean;
  onRetry: () => void;
}) {
  const { t } = useT("chat");
  if (loading && !failed) {
    return <p role="status" className="px-3 py-2 text-caption text-muted-foreground">{t(($) => $.recovery.loading_sessions)}</p>;
  }
  if (!failed) return null;
  const message = kind === "sessions"
    ? hasData ? t(($) => $.recovery.sessions_refresh_failed) : t(($) => $.recovery.sessions_failed)
    : hasData ? t(($) => $.recovery.messages_refresh_failed) : t(($) => $.recovery.messages_failed);
  return (
    <div role="alert" className="flex shrink-0 flex-wrap items-center gap-2 px-3 py-2 text-caption">
      <p className="min-w-0 flex-1 basis-36 text-muted-foreground">{message}</p>
      <Button variant="outline" size="sm" disabled={retrying} onClick={onRetry}>
        {retrying ? t(($) => $.recovery.retrying) : t(($) => $.recovery.retry)}
      </Button>
    </div>
  );
}
