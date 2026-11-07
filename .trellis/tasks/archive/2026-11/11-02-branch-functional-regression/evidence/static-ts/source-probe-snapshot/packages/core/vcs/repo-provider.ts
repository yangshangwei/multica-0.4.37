import type { VCSConnection } from "../types";

/**
 * Which Git hosting brand the project repo picker should present itself as.
 * `git` is the neutral fallback when nothing is connected or more than one
 * provider is, so the copy never names a host the workspace does not use.
 */
export type RepoProviderKind = "github" | "gitlab" | "forgejo" | "gitea" | "git";

export interface RepoProviderPresentation {
  kind: RepoProviderKind;
  /** Brand name for UI copy ("GitLab"). "Git" for the neutral fallback. */
  name: string;
  /** Example clone URLs for the paste box, derived from the connected instance. */
  httpsExample: string;
  sshExample: string;
}

const PROVIDER_NAMES: Record<RepoProviderKind, string> = {
  github: "GitHub",
  gitlab: "GitLab",
  forgejo: "Forgejo",
  gitea: "Gitea",
  git: "Git",
};

const GENERIC: RepoProviderPresentation = {
  kind: "git",
  name: PROVIDER_NAMES.git,
  httpsExample: "https://git.example.com/group/repo",
  sshExample: "git@git.example.com:group/repo.git",
};

const GITHUB: RepoProviderPresentation = {
  kind: "github",
  name: PROVIDER_NAMES.github,
  httpsExample: "https://github.com/owner/repo",
  sshExample: "git@github.com:owner/repo.git",
};

export interface ResolveRepoProviderInput {
  /** Token-based connections (Forgejo / Gitea / GitLab) from the VCS endpoint. */
  vcsConnections?: readonly Pick<VCSConnection, "provider" | "instance_url">[] | null;
  /** GitHub App installations; only the count matters here. */
  githubInstallations?: readonly unknown[] | null;
}

function tokenProviderKind(provider: string): RepoProviderKind {
  switch (provider) {
    case "gitlab":
      return "gitlab";
    case "forgejo":
      return "forgejo";
    case "gitea":
      return "gitea";
    default:
      // Server-driven enum: an unknown provider still counts as a connection,
      // it just cannot be branded.
      return "git";
  }
}

function hostOf(instanceURL: string): string {
  try {
    return new URL(instanceURL).host;
  } catch {
    return "";
  }
}

/**
 * Picks the single provider a workspace is connected to. When exactly one
 * brand is connected the picker adopts its name and example URLs (built from
 * the token provider's instance URL); otherwise it stays neutral.
 */
export function resolveRepoProvider(input: ResolveRepoProviderInput): RepoProviderPresentation {
  const connections = Array.isArray(input.vcsConnections) ? input.vcsConnections : [];
  const hasGitHub = Array.isArray(input.githubInstallations) && input.githubInstallations.length > 0;

  const kinds = new Set<RepoProviderKind>();
  if (hasGitHub) kinds.add("github");
  for (const c of connections) kinds.add(tokenProviderKind(c.provider));

  let kind: RepoProviderKind | undefined;
  for (const k of kinds) kind = k;
  if (kinds.size !== 1 || kind === undefined) return GENERIC;
  if (kind === "github") return GITHUB;
  if (kind === "git") return GENERIC;

  // Exactly one token provider brand. If it spans several instances the
  // brand still applies but no single host can stand in as the example.
  const instances = connections.map((c) => c.instance_url.replace(/\/+$/, ""));
  const hosts = new Set(instances.map(hostOf).filter(Boolean));
  let host: string | undefined;
  for (const h of hosts) host = h;
  const base = instances[0];
  if (hosts.size !== 1 || host === undefined || base === undefined) {
    return { ...GENERIC, kind, name: PROVIDER_NAMES[kind] };
  }
  return {
    kind,
    name: PROVIDER_NAMES[kind],
    httpsExample: `${base}/group/repo`,
    sshExample: `git@${host}:group/repo.git`,
  };
}
