"use client";

import type { UseQueryResult } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api/client";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

type SettingsQuerySnapshot = Pick<
  UseQueryResult<unknown>,
  "data" | "error" | "isError" | "isFetching" | "errorUpdatedAt" | "dataUpdatedAt"
>;

export function getSettingsQueryState(query: SettingsQuerySnapshot) {
  const accessDenied = query.error instanceof ApiError && !query.error.isStaleAuthResponse &&
    (query.error.status === 401 || query.error.status === 403);
  return {
    hasData: query.data !== undefined && !accessDenied,
    accessDenied,
    // A cold refetch changes Query's status back to pending. Keep its prior
    // error and Retry mounted until a successful response replaces them.
    failed: query.isError ||
      (query.isFetching && query.errorUpdatedAt > query.dataUpdatedAt),
  };
}

/** Render as a stable sibling of content gated by getSettingsQueryState().hasData. */
export function SettingsQueryFeedback({
  query,
  loadingMessage,
  errorMessage,
  refreshMessage,
}: {
  query: SettingsQuerySnapshot & Pick<UseQueryResult<unknown>, "refetch">;
  loadingMessage: string;
  errorMessage: string;
  refreshMessage: string;
}) {
  const { t } = useT("settings");
  const { hasData, failed, accessDenied } = getSettingsQueryState(query);

  if (failed) {
    return (
      <div role="alert" className="flex flex-wrap items-center gap-3 text-body">
        <p className="min-w-0 flex-1 basis-48 text-muted-foreground">
          {accessDenied
            ? t(($) => $.query_feedback.access_denied)
            : hasData ? refreshMessage : errorMessage}
        </p>
        <Button
          variant="outline"
          size="sm"
          disabled={query.isFetching}
          onClick={() => {
            if (!query.isFetching) void query.refetch({ cancelRefetch: false });
          }}
        >
          {query.isFetching
            ? t(($) => $.query_feedback.retrying)
            : t(($) => $.query_feedback.retry)}
        </Button>
      </div>
    );
  }

  return hasData ? null : (
    <p role="status" className="text-body text-muted-foreground">{loadingMessage}</p>
  );
}
