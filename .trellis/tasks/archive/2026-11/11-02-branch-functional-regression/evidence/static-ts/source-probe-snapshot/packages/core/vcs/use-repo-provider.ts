import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useConfigStore } from "../config";
import { githubInstallationsOptions } from "../github/queries";
import { vcsConnectionsOptions } from "./queries";
import { resolveRepoProvider, type RepoProviderPresentation } from "./repo-provider";

/**
 * Presentation for the project repo picker: which Git host the workspace is
 * connected to, so the copy and example URLs can name it. Both source queries
 * are member-readable; a missing or degraded response resolves to neutral.
 */
export function useRepoProvider(wsId: string): RepoProviderPresentation {
  // The VCS endpoint answers "unavailable" on deployments without the
  // integration, so gate on the config flag instead of paying a request.
  const vcsAvailable = useConfigStore((s) => s.vcsIntegrationAvailable);
  const { data: vcs } = useQuery({
    ...vcsConnectionsOptions(wsId),
    enabled: !!wsId && vcsAvailable,
  });
  const { data: github } = useQuery(githubInstallationsOptions(wsId));
  const connections = vcs?.connections;
  const installations = github?.installations;
  return useMemo(
    () =>
      resolveRepoProvider({
        vcsConnections: connections,
        githubInstallations: installations,
      }),
    [connections, installations],
  );
}
