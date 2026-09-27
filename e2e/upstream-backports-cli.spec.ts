import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { access, mkdtemp, realpath, rm, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const suppliedBinary = process.env.MULTICA_E2E_CLI_BINARY?.trim();
const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const loopbackHosts = new Set(["localhost", "127.0.0.1", "[::1]"]);

interface Label {
  id: string;
  name: string;
  resource_type: "issue" | "skill";
}

interface Comment {
  id: string;
  content: string;
  revision: number;
  author_type: string;
  author_id: string;
}

interface CliResult {
  code: number;
  stdout: string;
  stderr: string;
}

interface CliFixture {
  api: TestApiClient;
  ownerId: string;
  workspace: { id: string; slug: string };
  directory: string;
  run: (args: string[], actor?: TestApiClient) => Promise<CliResult>;
  json: <T>(args: string[]) => Promise<T>;
}

function requiredLocalURL(name: "NEXT_PUBLIC_API_URL" | "DATABASE_URL"): URL {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`CLI integration requires an explicit ${name}`);
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    // A database URL can contain credentials; never echo it in a failure.
    throw new Error(`${name} must be a valid local URL`);
  }
  if (!loopbackHosts.has(url.hostname)) throw new Error(`${name} must use a loopback host`);
  if (name === "NEXT_PUBLIC_API_URL") {
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password ||
        url.pathname !== "/" || url.search || url.hash) {
      throw new Error("NEXT_PUBLIC_API_URL must be a credential-free local API origin");
    }
  } else if (!["postgres:", "postgresql:"].includes(url.protocol) ||
             !url.pathname.slice(1) || url.pathname === "/multica") {
    throw new Error("DATABASE_URL must name an isolated test database, not the shared multica database");
  }
  return url;
}

async function taskBinary(): Promise<string> {
  if (!suppliedBinary || !isAbsolute(suppliedBinary)) {
    throw new Error("MULTICA_E2E_CLI_BINARY must be an absolute task-owned binary path");
  }
  const binary = await realpath(suppliedBinary);
  const binaryRoot = join(await realpath(repositoryRoot), "server", "bin");
  const within = relative(binaryRoot, binary);
  if (!within || within === ".." || within.startsWith(`..${process.platform === "win32" ? "\\" : "/"}`) || isAbsolute(within)) {
    throw new Error("MULTICA_E2E_CLI_BINARY must resolve inside this checkout's server/bin directory");
  }
  await access(binary, constants.X_OK);
  return binary;
}

const test = base.extend<{ cli: CliFixture }>({
  cli: async ({}, use) => {
    const apiURL = requiredLocalURL("NEXT_PUBLIC_API_URL").origin;
    requiredLocalURL("DATABASE_URL");
    const binary = await taskBinary();
    const slug = `e2e-b1-cli-${randomUUID().slice(0, 12)}`;
    const directory = await mkdtemp(join(tmpdir(), "multica-b1-cli-"));
    const api = new TestApiClient();
    let workspace: CliFixture["workspace"] | undefined;
    try {
      const login = await api.login(`${slug}@example.test`, "B1 CLI owner");
      workspace = await api.ensureWorkspace("B1 CLI integration", slug);
      expect(workspace.slug, "the fixture must own its workspace").toBe(slug);
      const workspaceId = workspace.id;

      const run = (args: string[], actor = api): Promise<CliResult> => {
        const token = actor.getToken();
        if (!token) throw new Error("CLI fixture actor is not authenticated");
        // Never inherit a daemon's task identity or its credentials. This root
        // isolates CLI config without changing HOME or provider-tool config.
        const env = Object.fromEntries(
          Object.entries(process.env).filter(([key]) => !key.startsWith("MULTICA_")),
        );
        return new Promise((resolveResult, reject) => {
          execFile(binary, [
            "--server-url", apiURL,
            "--workspace-id", workspaceId,
            "--profile", slug,
            ...args,
          ], {
            cwd: directory,
            env: {
              ...env,
              MULTICA_TOKEN: token,
              MULTICA_SERVER_URL: apiURL,
              MULTICA_WORKSPACE_ID: workspaceId,
              MULTICA_TASK_CONFIG_ROOT: join(directory, "config"),
              MULTICA_HTTP_TIMEOUT: "5s",
            },
            encoding: "utf8",
            timeout: 15_000,
            maxBuffer: 1024 * 1024,
          }, (error, stdout, stderr) => {
            if (error && typeof error.code !== "number") {
              reject(new Error(`Task-owned CLI could not finish: ${error.code ?? error.signal ?? "process failure"}`));
              return;
            }
            resolveResult({ code: error?.code ?? 0, stdout: stdout.trim(), stderr: stderr.trim() });
          });
        });
      };
      const json = async <T,>(args: string[]): Promise<T> => {
        const result = await run([...args, "--output", "json"]);
        expect(result.code, result.stderr).toBe(0);
        return JSON.parse(result.stdout) as T;
      };
      await use({ api, ownerId: login.user.id, workspace, directory, run, json });
    } finally {
      try {
        if (workspace?.slug === slug) await api.deleteFeatureWorkspace(workspace.id);
      } finally {
        await rm(directory, { recursive: true, force: true });
      }
    }
  },
});

async function readComment(cli: CliFixture, issueId: string, commentId: string): Promise<Comment> {
  const comments = await cli.api.requestJSON<Comment[]>(`/api/issues/${issueId}/comments?full=true`);
  const comment = comments.find((item) => item.id === commentId);
  if (!comment) throw new Error("Fixture comment was not returned by the API");
  return comment;
}

test.describe("B1 compiled CLI against the isolated API", () => {
  test.skip(!suppliedBinary, "Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY");

  test("skill labels remain typed and idempotent while issue labels still work", async ({ cli }) => {
    const skill = await cli.api.requestJSON<{ id: string }>("/api/skills", {
      method: "POST",
      body: { name: "b1-cli-label-target", description: "Disposable skill", content: "# B1 CLI skill\n\nTest-owned content.\n" },
    });
    const skillLabel = await cli.json<Label>(["label", "create", "--resource-type", "skill", "--name", "B1 skill label", "--color", "#336699"]);
    const issueLabel = await cli.json<Label>(["label", "create", "--name", "B1 issue label", "--color", "#993366"]);
    expect(skillLabel.resource_type).toBe("skill");
    expect(issueLabel.resource_type).toBe("issue");
    const skillCatalog = await cli.json<Label[]>(["label", "list", "--resource-type", "skill"]);
    expect(skillCatalog.map((label) => label.id)).toContain(skillLabel.id);
    expect(skillCatalog.map((label) => label.id)).not.toContain(issueLabel.id);
    const issueCatalog = await cli.json<Label[]>(["label", "list"]);
    expect(issueCatalog.map((label) => label.id)).toContain(issueLabel.id);
    expect(issueCatalog.map((label) => label.id)).not.toContain(skillLabel.id);

    for (let attempt = 0; attempt < 2; attempt += 1) {
      const labels = await cli.json<Label[]>(["skill", "label", "add", skill.id, skillLabel.id]);
      expect(labels.map((label) => label.id)).toEqual([skillLabel.id]);
    }
    const beforeWrongType = await cli.api.requestJSON<{ labels: Label[] }>(`/api/skills/${skill.id}/labels`);
    const rejected = await cli.run(["skill", "label", "add", skill.id, issueLabel.id, "--output", "json"]);
    // HTTP 404 maps to exit 4; a missing command or transport failure must not pass.
    expect(rejected.code, rejected.stderr).toBe(4);
    expect(await cli.api.requestJSON(`/api/skills/${skill.id}/labels`)).toEqual(beforeWrongType);
    expect((await cli.json<Label[]>(["skill", "label", "list", skill.id])).map((label) => label.id)).toEqual([skillLabel.id]);
    expect(await cli.json<Label[]>(["skill", "label", "remove", skill.id, skillLabel.id])).toEqual([]);

    const issue = await cli.api.createIssue("B1 issue-label compatibility");
    expect((await cli.json<Label[]>(["issue", "label", "add", issue.id, issueLabel.id])).map((label) => label.id)).toEqual([issueLabel.id]);
    expect((await cli.json<Label[]>(["issue", "label", "list", issue.id])).map((label) => label.id)).toEqual([issueLabel.id]);
    expect(await cli.json<Label[]>(["issue", "label", "remove", issue.id, issueLabel.id])).toEqual([]);
  });

  test("comment updates preserve UTF-8 files and reject stale revisions", async ({ cli }) => {
    const issue = await cli.api.createIssue("B1 comment revision integration");
    const comment = await cli.api.requestJSON<Comment>(`/api/issues/${issue.id}/comments`, {
      method: "POST", body: { content: "Original test-owned comment" },
    });
    expect(comment.revision).toBeGreaterThan(0);
    expect(comment.author_type).toBe("member");
    expect(comment.author_id).toBe(cli.ownerId);
    const original = await readComment(cli, issue.id, comment.id);
    const invalid = await cli.run(["issue", "comment", "update", comment.id, "--expected-revision", "0", "--content", "Rejected edit"]);
    expect(invalid.code).toBe(1);
    expect(invalid.stderr).toContain("--expected-revision");
    expect(await readComment(cli, issue.id, comment.id)).toEqual(original);

    // Existing CLI text input trims the trailing newline; keep this fixture
    // focused on interior newlines, Unicode and literal backslashes.
    const content = "修复说明：缓存命中率包含写入。\n\n保留 UTF-8：你好，世界 🌍\n字面反斜杠：\\n 不应变成换行。";
    await writeFile(join(cli.directory, "comment.md"), content, "utf8");
    const updated = await cli.json<Comment>([
      "issue", "comment", "update", comment.id,
      "--expected-revision", String(comment.revision), "--content-file", "comment.md",
    ]);
    expect(updated).toMatchObject({ id: comment.id, content, revision: comment.revision + 1, author_type: "member", author_id: cli.ownerId });
    const stored = await readComment(cli, issue.id, comment.id);
    expect(stored.content).toBe(content);
    const stale = await cli.run([
      "issue", "comment", "update", comment.id,
      "--expected-revision", String(comment.revision), "--content", "Stale edit must not win",
    ]);
    expect(stale.code, stale.stderr).toBe(1);
    expect(stale.stderr).toMatch(/changed|revision|conflict|冲突/i);
    expect(await readComment(cli, issue.id, comment.id)).toEqual(stored);
  });

  test("an unrelated member and an outsider cannot update another member's comment", async ({ cli }) => {
    const issue = await cli.api.createIssue("B1 comment authorization integration");
    const comment = await cli.api.requestJSON<Comment>(`/api/issues/${issue.id}/comments`, {
      method: "POST", body: { content: "Only the author or an administrator may edit this." },
    });
    const member = new TestApiClient();
    const memberEmail = `e2e-b1-member-${randomUUID()}@example.test`;
    const memberLogin = await member.login(memberEmail, "B1 unrelated member");
    const invitation = await cli.api.requestJSON<{ id: string }>(`/api/workspaces/${cli.workspace.id}/members`, {
      method: "POST", body: { email: memberEmail, role: "member" },
    });
    await member.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
    const members = await cli.api.requestJSON<{ user_id: string; role: string }[]>(`/api/workspaces/${cli.workspace.id}/members`);
    expect(members.find((item) => item.user_id === memberLogin.user.id)?.role).toBe("member");
    const outsider = new TestApiClient();
    await outsider.login(`e2e-b1-outsider-${randomUUID()}@example.test`, "B1 outsider");
    expect((await outsider.getWorkspaces()).some((workspace) => workspace.id === cli.workspace.id)).toBe(false);

    const original = await readComment(cli, issue.id, comment.id);
    const args = ["issue", "comment", "update", comment.id, "--expected-revision", String(comment.revision), "--content", "Unauthorized edit"];
    const memberResult = await cli.run(args, member);
    expect(memberResult.code, memberResult.stderr).toBe(3);
    expect(await readComment(cli, issue.id, comment.id)).toEqual(original);
    const outsiderResult = await cli.run(args, outsider);
    expect(outsiderResult.code, outsiderResult.stderr).toBe(4);
    expect(await readComment(cli, issue.id, comment.id)).toEqual(original);
  });
});
