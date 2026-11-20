"use client";

import type { UseQueryResult } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api/client";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

type SquadQuerySnapshot = Pick<UseQueryResult<unknown>,
  "data" | "error" | "isError" | "isFetching" | "errorUpdatedAt" | "dataUpdatedAt"
>;

export function getSquadQueryState(query: SquadQuerySnapshot) {
  const unavailable = query.error instanceof ApiError && !query.error.isStaleAuthResponse &&
    [401, 403, 404].includes(query.error.status);
  return {
    hasData: query.data !== undefined && !unavailable,
    unavailable,
    failed: query.isError || (query.isFetching && query.errorUpdatedAt > query.dataUpdatedAt),
  };
}

/** Keep read feedback beside retained forms so a retry never remounts drafts. */
export function SquadQueryFeedback({ query, loadingMessage, errorMessage, refreshMessage }: {
  query: SquadQuerySnapshot & Pick<UseQueryResult<unknown>, "refetch">;
  loadingMessage: string;
  errorMessage: string;
  refreshMessage?: string;
}) {
  const { t } = useT("squads");
  const { hasData, failed } = getSquadQueryState(query);
  if (!failed) return hasData ? null : <p role="status" className="text-body text-muted-foreground">{loadingMessage}</p>;
  return (
    <div role="alert" className="flex flex-wrap items-center gap-2 text-body text-muted-foreground">
      <p className="min-w-0 flex-1">{hasData ? refreshMessage ?? errorMessage : errorMessage}</p>
      <Button variant="outline" size="sm" disabled={query.isFetching} onClick={() => {
        if (!query.isFetching) void query.refetch({ cancelRefetch: false });
      }}>{t(($) => $.recovery.retry)}</Button>
    </div>
  );
}
