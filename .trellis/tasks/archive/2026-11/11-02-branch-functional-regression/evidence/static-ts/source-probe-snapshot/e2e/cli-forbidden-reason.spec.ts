import { execFile } from "node:child_process";
import { createHash, randomBytes } from "node:crypto";
import { constants } from "node:fs";
import { access, mkdtemp, realpath, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { legacyLaneMissing, loginLegacyOwner, sql } from "./fixtures/lane-guards";

// 584dcd436: a 403 now prints "Permission denied: <server reason>" (zh:
// "权限不足：") instead of the generic workspace hint (server/internal/cli/
// errors.go serverMessagePrefixes / kindMessages). Exit code stays 3.
// The Observer denial is autonomyDenialMessage in handler/agent_autonomy.go,
// reached through a real `mat_` task token, as a daemon would hand an agent.
const suppliedBinary = process.env.MULTICA_E2E_CLI_BINARY?.trim();
const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const OBSERVER_REASON = "your autonomy level is observer, which cannot change an issue's status or assignee — that requires contributor. Report what is needed and hand it to a human or an agent at that level instead.";
const GENERIC_HINT = { en: "right workspace", zh: "请确认当前 workspace 是否正确" } as const;

interface CliResult { code: number; stdout: string; stderr: string }

async function taskBinary(): Promise<string> {
  if (!suppliedBinary || !isAbsolute(suppliedBinary)) throw new Error("MULTICA_E2E_CLI_BINARY must be an absolute task-owned binary path");
  const binary = await realpath(suppliedBinary);
  const within = relative(join(await realpath(repositoryRoot), "server", "bin"), binary);
  if (!within || within.startsWith("..") || isAbsolute(within)) throw new Error("MULTICA_E2E_CLI_BINARY must resolve inside this checkout's server/bin directory");
  await access(binary, constants.X_OK);
  return binary;
}

/** Runs the compiled CLI with an isolated config root and no inherited Multica identity or locale. */
async function runCli(args: string[], token: string, workspaceId: string, locale: "en" | "zh"): Promise<CliResult> {
  const binary = await taskBinary();
  const apiURL = new URL(process.env.NEXT_PUBLIC_API_URL!).origin;
  const directory = await mkdtemp(join(tmpdir(), "multica-supp-cli-"));
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) =>
    !key.startsWith("MULTICA_") && !["LC_ALL", "LC_MESSAGES", "LANG"].includes(key)));
  try {
    return await new Promise((resolveResult, reject) => {
      execFile(binary, ["--server-url", apiURL, "--workspace-id", workspaceId, "--profile", "forbidden-reason", ...args], {
        cwd: directory, encoding: "utf8", timeout: 15_000, maxBuffer: 1024 * 1024,
        env: { ...env, LANG: locale === "zh" ? "zh_CN.UTF-8" : "en_US.UTF-8", MULTICA_TOKEN: token, MULTICA_SERVER_URL: apiURL,
          MULTICA_WORKSPACE_ID: workspaceId, MULTICA_TASK_CONFIG_ROOT: join(directory, "config"), MULTICA_HTTP_TIMEOUT: "5s" },
      }, (error, stdout, stderr) => {
        if (error && typeof error.code !== "number") {
          reject(new Error(`Task-owned CLI could not finish: ${error.code ?? error.signal ?? "process failure"}`));
          return;
        }
        resolveResult({ code: error ? Number(error.code) : 0, stdout: stdout.trim(), stderr: stderr.trim() });
      });
    });
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

/** An Observer staffed from its built-in template, holding a running task's token. */
async function seedObserver() {
  const owner = await loginLegacyOwner("e2e-supp-cli", "CLI forbidden owner");
  const runtime = await owner.api.seedProjectRuntime();
  const observer = await owner.api.requestJSON<{ id: string; autonomy_level?: string }>("/api/agents/from-template", {
    method: "POST", body: { template_key: "product-analyst", runtime_id: runtime.id, language: "en", permission_mode: "private" },
  });
  expect(observer.autonomy_level).toBe("observer");
  const [task] = await sql<{ id: string }>(
    "INSERT INTO agent_task_queue (agent_id, status, runtime_id) VALUES ($1, 'running', $2) RETURNING id", [observer.id, runtime.id]);
  const token = `mat_${randomBytes(24).toString("hex")}`;
  await sql(
    `INSERT INTO task_token (token_hash, task_id, agent_id, workspace_id, user_id, expires_at)
     VALUES ($1, $2, $3, $4, $5, now() + interval '1 hour')`,
    [createHash("sha256").update(token).digest("hex"), task!.id, observer.id, owner.workspace.id, owner.userId]);
  const issue = await owner.api.createIssue("CLI forbidden-reason target", { status: "todo" }) as { id: string; status: string };
  expect(issue.status).toBe("todo");
  return { ...owner, observerToken: token, issue };
}

test.describe("CLI surfaces the server's 403 reason", () => {
  test.skip(!suppliedBinary || legacyLaneMissing() !== null,
    legacyLaneMissing() ?? "Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY");

  for (const locale of ["en", "zh"] as const) {
    test(`an Observer agent's status change prints the autonomy reason, not the workspace hint (${locale})`, async () => {
      const fixture = await seedObserver();
      const result = await runCli(["issue", "status", fixture.issue.id, "in_progress"], fixture.observerToken, fixture.workspace.id, locale);
      expect(result.code, result.stderr).toBe(3);
      expect(result.stderr).toContain(`${locale === "zh" ? "权限不足：" : "Permission denied: "}${OBSERVER_REASON}`);
      expect(result.stderr).not.toContain(GENERIC_HINT[locale]);
      expect(result.stderr).not.toContain(`/api/issues/${fixture.issue.id}`);
      expect(await fixture.api.requestJSON(`/api/issues/${fixture.issue.id}`)).toMatchObject({ status: "todo" });
    });
  }

  test("an ordinary member editing someone else's comment sees the server's reason", async () => {
    const fixture = await seedObserver();
    const comment = await fixture.api.requestJSON<{ id: string; revision: number; content: string }>(`/api/issues/${fixture.issue.id}/comments`, {
      method: "POST", body: { content: "Only the author or an administrator may edit this." },
    });
    const member = new TestApiClient();
    const memberEmail = `${fixture.slug}-member@example.test`;
    await member.login(memberEmail, "CLI forbidden member");
    const invitation = await fixture.api.requestJSON<{ id: string }>(`/api/workspaces/${fixture.workspace.id}/members`, {
      method: "POST", body: { email: memberEmail, role: "member" },
    });
    await member.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
    const memberToken = member.getToken();
    if (!memberToken) throw new Error("The member fixture has no token");
    const result = await runCli(["issue", "comment", "update", comment.id, "--expected-revision", String(comment.revision),
      "--content", "Unauthorized edit"], memberToken, fixture.workspace.id, "en");
    expect(result.code, result.stderr).toBe(3);
    expect(result.stderr).toContain("Permission denied: only comment author or admin can edit");
    expect(result.stderr).not.toContain(GENERIC_HINT.en);
    const comments = await fixture.api.requestJSON<{ id: string; content: string }[]>(`/api/issues/${fixture.issue.id}/comments?full=true`);
    expect(comments.find((item) => item.id === comment.id)?.content).toBe(comment.content);
  });
});
