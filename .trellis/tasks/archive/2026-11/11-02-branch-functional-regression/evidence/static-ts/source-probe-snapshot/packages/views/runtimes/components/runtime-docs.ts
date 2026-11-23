import { DOCS_ANCHORS, DOCS_SLUGS } from "@multica/core/docs";
import { paths } from "@multica/core/paths";

/**
 * Links from the runtimes UI into the documentation.
 *
 * These used to build `https://multica.ai/docs/<lang>/daemon-runtimes` and were
 * dead on any deployment without public internet access. They now address the
 * documentation this deployment serves, which is why they take a workspace slug
 * instead of a language: the in-app reader is workspace-scoped, and P0 ships
 * Chinese content only, so there is no locale segment to compute.
 *
 * The slug is nullable and the result is null without one. These links render
 * inside controls that also appear outside a workspace route, where
 * `useWorkspacePaths()` throws — returning null lets a caller drop the link
 * instead of taking the surrounding dialog down with it.
 *
 * The anchor comes from `DOCS_ANCHORS` rather than being written here, so the
 * bundle parity test can prove it matches a real heading.
 */
export function daemonRuntimesDocsHref(
  workspaceSlug: string | null,
): string | null {
  if (!workspaceSlug) return null;
  return paths.workspace(workspaceSlug).docsPage(DOCS_SLUGS.daemonRuntimes);
}

export function customRuntimeDocsHref(
  workspaceSlug: string | null,
): string | null {
  if (!workspaceSlug) return null;
  return paths
    .workspace(workspaceSlug)
    .docsPage(DOCS_SLUGS.daemonRuntimes, DOCS_ANCHORS.customRuntimeProfiles);
}
