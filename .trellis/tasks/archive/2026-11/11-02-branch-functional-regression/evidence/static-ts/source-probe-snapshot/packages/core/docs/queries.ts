import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

/**
 * Server state for the in-app documentation.
 *
 * TanStack Query owns this, not Zustand: the content is fetched from the API and
 * belongs to the deployment, not to the view.
 *
 * Keys carry no `wsId`, which is the one place this deviates from every other
 * key in the codebase. The bundle is a property of the server build — every
 * workspace on a deployment reads the identical bytes — so scoping the cache per
 * workspace would refetch 45 pages on each workspace switch to produce the same
 * result. The route is workspace-scoped for URL reasons only.
 */
export const docsKeys = {
  all: ["docs"] as const,
  manifest: () => [...docsKeys.all, "manifest"] as const,
  page: (slug: string) => [...docsKeys.all, "page", slug] as const,
};

/**
 * The navigation tree, plus the server version that published it.
 *
 * Immutable for the lifetime of a server build, so it is cached hard: refetching
 * can only produce a different answer after an upgrade, and an upgrade
 * disconnects the websocket and reloads the client anyway.
 */
export function docsManifestOptions() {
  return queryOptions({
    queryKey: docsKeys.manifest(),
    queryFn: () => api.getDocsManifest(),
    staleTime: Infinity,
    gcTime: Infinity,
    // A deployment older than this feature has no /api/docs route and answers
    // 404 for every attempt; retrying cannot change that, and the reader needs
    // the failure promptly to show its "not available on this server" state.
    retry: false,
  });
}

/**
 * One page's markdown body and heading outline.
 *
 * `enabled` guards the empty slug so the directory route can call this
 * unconditionally.
 */
export function docsPageOptions(slug: string) {
  return queryOptions({
    queryKey: docsKeys.page(slug),
    queryFn: () => api.getDocsPage(slug),
    enabled: slug.length > 0,
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });
}
