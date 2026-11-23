// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveRepoProvider } from "./repo-provider";

const gitlab = { provider: "gitlab" as const, instance_url: "https://gitlab.corp.example" };
const forgejo = { provider: "forgejo" as const, instance_url: "https://code.example.org/" };

describe("resolveRepoProvider", () => {
  it("stays neutral when nothing is connected", () => {
    expect(resolveRepoProvider({})).toMatchObject({ kind: "git", name: "Git" });
    expect(resolveRepoProvider({ vcsConnections: [], githubInstallations: [] })).toMatchObject({
      kind: "git",
    });
  });

  it("tolerates non-array inputs from a degraded response", () => {
    expect(
      resolveRepoProvider({
        vcsConnections: null,
        githubInstallations: "nope" as unknown as unknown[],
      }).kind,
    ).toBe("git");
  });

  it("brands as GitHub when only GitHub installations exist", () => {
    expect(resolveRepoProvider({ githubInstallations: [{ id: 1 }] })).toEqual({
      kind: "github",
      name: "GitHub",
      httpsExample: "https://github.com/owner/repo",
      sshExample: "git@github.com:owner/repo.git",
    });
  });

  it("brands as GitLab and derives examples from the instance URL", () => {
    expect(resolveRepoProvider({ vcsConnections: [gitlab] })).toEqual({
      kind: "gitlab",
      name: "GitLab",
      httpsExample: "https://gitlab.corp.example/group/repo",
      sshExample: "git@gitlab.corp.example:group/repo.git",
    });
  });

  it("strips a trailing slash from the instance URL and keeps a non-default port", () => {
    expect(
      resolveRepoProvider({
        vcsConnections: [{ provider: "gitea", instance_url: "https://git.lan:8443/" }],
      }),
    ).toEqual({
      kind: "gitea",
      name: "Gitea",
      httpsExample: "https://git.lan:8443/group/repo",
      sshExample: "git@git.lan:8443:group/repo.git",
    });
  });

  it("keeps the brand but drops host examples when one brand spans two instances", () => {
    const other = { provider: "gitlab" as const, instance_url: "https://gitlab.other.example" };
    expect(resolveRepoProvider({ vcsConnections: [gitlab, other] })).toEqual({
      kind: "gitlab",
      name: "GitLab",
      httpsExample: "https://git.example.com/group/repo",
      sshExample: "git@git.example.com:group/repo.git",
    });
  });

  it("falls back to neutral when more than one brand is connected", () => {
    expect(resolveRepoProvider({ vcsConnections: [gitlab, forgejo] }).kind).toBe("git");
    expect(
      resolveRepoProvider({ vcsConnections: [gitlab], githubInstallations: [{ id: 1 }] }).kind,
    ).toBe("git");
  });

  it("treats an unknown server provider as a neutral connection", () => {
    expect(
      resolveRepoProvider({
        vcsConnections: [{ provider: "bitbucket" as never, instance_url: "https://bb.example" }],
      }),
    ).toMatchObject({ kind: "git", name: "Git" });
  });
});
