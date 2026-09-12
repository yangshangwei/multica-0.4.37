import { queryOptions } from "@tanstack/react-query";
import { getApi } from "../api";

export const changelogKeys = {
  all: ["changelog"] as const,
  feed: (deployment: string) => [...changelogKeys.all, deployment] as const,
};

/**
 * Deployment-wide, independent of workspace and binary updater preferences.
 * The caller supplies tab activity; Query's focus manager suppresses hidden
 * window polling. Re-enabling a kept-alive reader fetches immediately because
 * its data is always stale. Capture the API instance with its key so an old
 * observer can never fetch another deployment into the previous cache slot.
 */
export function changelogOptions(active = true) {
  const client = getApi();
  return queryOptions({
    queryKey: changelogKeys.feed(client.getBaseUrl()),
    queryFn: () => client.getChangelog(),
    enabled: active,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
    refetchInterval: active ? 60_000 : false,
    refetchIntervalInBackground: false,
    retry: false,
  });
}
