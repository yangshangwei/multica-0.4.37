import { mkdtemp, rename, rm, writeFile } from "node:fs/promises";
import { basename, isAbsolute, join } from "node:path";
import { expect, type Page } from "@playwright/test";
import type { TestApiClient } from "../fixtures";

/** The same real filesystem/API flow runs against Web and Electron. */
export async function verifyDeploymentMcpCatalog({
  page, api, workspaceId, agent, capture,
}: {
  page: Page;
  api: TestApiClient;
  workspaceId: string;
  agent: { id: string; name: string };
  capture: (name: string) => Promise<void>;
}) {
  const root = process.env.E2E_MCP_TEMPLATE_DIR;
  if (!root || !isAbsolute(root)) throw new Error("E2E_MCP_TEMPLATE_DIR must be the task-owned absolute API template directory");
  const dir = await mkdtemp(join(root, "e2e-mcp-"));
  const stdioDir = await mkdtemp(join(root, "e2e-stdio-"));
  const key = basename(dir);
  const title = `Deployment search ${key}`;
  const secret = "deployment-e2e-fake-token";
  const listPath = `/api/workspaces/${workspaceId}/mcp-servers`;
  const manifest = {
    schema_version: 1,
    titles: { en: title, zh: "部署目录测试" },
    descriptions: { en: "Local directory acceptance fixture" },
    category: "documentation",
    config: { type: "http", url: "https://mcp.example.test/v1" },
    inputs: [{
      key: "access_token", labels: { en: "Access token", zh: "访问令牌" },
      required: true, secret: true,
      target: { kind: "header", name: "Authorization", prefix: "Bearer " },
    }],
  };
  const publish = async () => {
    await writeFile(join(dir, "mcp.json.next"), JSON.stringify(manifest));
    await rename(join(dir, "mcp.json.next"), join(dir, "mcp.json"));
  };
  try {
    await page.getByRole("tab", { name: "MCP market", exact: true }).click();
    const market = page.getByTestId("mcp-market");
    await publish();
    await market.getByRole("button", { name: "Deployment provided", exact: true }).click();
    const preview = market.getByRole("button", { name: `View configuration: ${title}`, exact: true });
    await expect(preview).toBeVisible();
    const catalog = await api.requestJSON<{ templates: { key: string; source: string; version: string; config?: unknown }[] }>(`${listPath}/templates?language=en`);
    const template = catalog.templates.find((item) => item.source === "deployment" && item.key === key)!;
    expect(template.version).toMatch(/^sha256:[a-f0-9]{64}$/);
    expect(template).not.toHaveProperty("config");
    await capture("deployment-market");

    await preview.click();
    const dialog = page.getByRole("dialog").last();
    const token = dialog.getByLabel("Access token", { exact: false });
    await expect(token).toHaveAttribute("type", "password");
    await token.fill(secret);
    await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
    await expect(dialog.getByRole("heading", { name: "Choose agents", exact: true })).toBeVisible();
    type Summary = { id: string; name: string; template_source: string | null; template_key: string | null; template_version: string | null };
    const saved = (await api.requestJSON<Summary[]>(`${listPath}?mcp_source_version=1`)).find((item) => item.name === key)!;
    expect(saved).toMatchObject({ template_source: "deployment", template_key: key, template_version: template.version });
    expect(JSON.stringify(saved)).not.toContain(secret);
    const legacy = (await api.requestJSON<Summary[]>(listPath)).find((item) => item.id === saved.id)!;
    expect(legacy).toMatchObject({ template_source: null, template_key: null, template_version: null });
    expect(await api.requestJSON(`/api/agents/${agent.id}/mcp-servers?mcp_source_version=1`)).not.toEqual(expect.arrayContaining([expect.objectContaining({ id: saved.id })]));
    await dialog.getByRole("checkbox", { name: new RegExp(agent.name) }).check();
    await dialog.getByRole("button", { name: "Assign selected", exact: true }).click();
    await dialog.getByRole("button", { name: "Done", exact: true }).click();

    await preview.click();
    await token.fill("draft-to-clear-after-reload");
    manifest.config.url = "https://mcp.example.test/v2";
    await publish();
    // The open market must discover this change without manual refresh.
    await expect(dialog.getByText(/This template has changed/)).toBeVisible({ timeout: 40_000 });
    await expect(dialog.getByRole("button", { name: "Save and continue", exact: true })).toBeDisabled();
    await dialog.getByRole("button", { name: "Reload template", exact: true }).click();
    await expect(token).toHaveValue("");
    await expect(dialog.getByText(/This template has changed/)).toHaveCount(0);
    await capture("deployment-reloaded");
    await page.keyboard.press("Escape");

    await rm(dir, { recursive: true, force: true });
    await market.getByRole("button", { name: "Deployment provided", exact: true }).click();
    await expect(preview).toHaveCount(0);
    const retained = (await api.requestJSON<Summary[]>(`${listPath}?mcp_source_version=1`)).find((item) => item.id === saved.id);
    expect(retained).toEqual(saved);
    expect(await api.requestJSON(`/api/agents/${agent.id}/mcp-servers?mcp_source_version=1`)).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: saved.id, template_source: "deployment", enabled: true }),
    ]));
    await page.getByRole("tab", { name: "Shared configurations", exact: true }).click();
    await expect(page.getByText(key, { exact: true })).toBeVisible();
    await capture("deployment-saved-after-withdrawal");

    // A deployment stdio recipe must also work without public config or inputs.
    const stdioKey = basename(stdioDir);
    const stdioTitle = `Deployment local ${stdioKey}`;
    await writeFile(join(stdioDir, "mcp.json"), JSON.stringify({
      schema_version: 1, titles: { en: stdioTitle }, category: "coding",
      config: { command: "mcp-e2e-never-executed", args: [] },
    }));
    await page.getByRole("tab", { name: "MCP market", exact: true }).click();
    await market.getByRole("button", { name: "Deployment provided", exact: true }).click();
    await market.getByRole("button", { name: `View configuration: ${stdioTitle}`, exact: true }).click();
    await dialog.getByRole("button", { name: "Save and continue", exact: true }).click();
    await expect(dialog.getByRole("heading", { name: "Choose agents", exact: true })).toBeVisible();
    const stdioSaved = (await api.requestJSON<Summary[]>(`${listPath}?mcp_source_version=1`)).find((item) => item.name === stdioKey)!;
    expect(stdioSaved).toMatchObject({ template_source: "deployment", template_key: stdioKey, transport: "stdio" });
    await dialog.getByRole("checkbox", { name: new RegExp(agent.name) }).check();
    await dialog.getByRole("button", { name: "Assign selected", exact: true }).click();
    await dialog.getByRole("button", { name: "Done", exact: true }).click();
    expect(await api.requestJSON(`/api/agents/${agent.id}/mcp-servers?mcp_source_version=1`)).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: stdioSaved.id, template_source: "deployment", enabled: true, transport: "stdio" }),
    ]));
    await capture("deployment-stdio");
  } finally {
    await rm(dir, { recursive: true, force: true });
    await rm(stdioDir, { recursive: true, force: true });
  }
}
