import { test, expect } from "@playwright/test";
import type { ProjectExecutionSquad } from "../packages/core/types/project";
import { TestApiClient } from "./fixtures";

type Template = {
  key: string;
  title: string;
  description: string;
  leader: { title: string };
  members: { title: string }[];
};
type Project = {
  id: string;
  execution_squads: ProjectExecutionSquad[];
};
type Squad = { id: string; name: string; description: string; template_key: string; template_version: number };

for (const labels of [
  {
    locale: "en", language: "en", title: "Security Remediation Squad",
    catalog: "Squad templates", members: "8 members", project: "Project", runtime: "Execution runtime",
    summary: "Triage security risks, repair them outside production, and verify with independent review.",
    apply: "Apply to project", confirm: "Use for project",
    dialog: "Use Security Remediation Squad for a project",
  },
  {
    locale: "zh-Hans", language: "zh", title: "安全漏洞修复小队",
    catalog: "AI小队模板", members: "8 名成员", project: "项目", runtime: "执行运行时",
    summary: "分诊安全风险，在非生产环境修复，并由独立成员验证和审查。",
    apply: "应用到项目", confirm: "用于项目", dialog: "将安全漏洞修复小队用于项目",
  },
]) {
  test(`security remediation catalog and project adoption (${labels.locale})`, async ({ page }, info) => {
    const api = new TestApiClient();
    const slug = `security-squad-${labels.language}-${Date.now().toString(36)}-${process.pid}`;
    await api.login(`${slug}@multica.ai`, "Security squad tester");
    const workspace = await api.ensureWorkspace("Security squad fixture", slug);
    const pageErrors: string[] = [];
    page.on("pageerror", (error) => pageErrors.push(error.message));

    try {
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: labels.locale } });
      const token = api.getToken();
      if (!token) throw new Error("Fixture login failed");
      await page.addInitScript(({ token, locale }) => {
        localStorage.setItem("multica_token", token);
        localStorage.setItem("multica:chat:isOpen", "false");
        document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
        document.cookie = `multica-locale=${locale}; path=/; SameSite=Lax`;
      }, { token, locale: labels.locale });

      // This inserts runtime metadata only. No daemon or model can claim work.
      const runtime = await api.seedProjectRuntime();
      const projectTitle = "Dependency and authorization remediation";
      const project = await api.requestJSON<Project>("/api/projects", {
        method: "POST", body: { title: projectTitle, execution_squads: [] },
      });
      const { templates } = await api.requestJSON<{ templates: Template[] }>(`/api/squads/templates?language=${labels.language}`);
      const template = templates.find((entry) => entry.key === "security-remediation");
      if (!template) throw new Error("Security remediation template was not returned");

      const catalogPath = `/${workspace.slug}/squads?view=templates`;
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(catalogPath);
      const catalog = page.getByRole("region", { name: labels.catalog, exact: true });
      await expect(catalog.getByRole("listitem")).toHaveCount(9);
      const entry = catalog.getByRole("listitem", { name: labels.title, exact: true });
      await expect(entry.getByText(labels.summary, { exact: true })).toBeVisible();
      await expect(entry.getByText(labels.members, { exact: true })).toBeVisible();
      await expect(entry.getByText([template.leader, ...template.members].map((role) => role.title).join(" · "), { exact: true })).toBeVisible();
      await expect(entry.locator("svg.lucide-shield-check")).toBeVisible();
      await entry.screenshot({ path: info.outputPath(`security-template-${labels.locale}.png`), animations: "disabled" });

      await entry.getByRole("button", { name: labels.apply, exact: true }).click();
      const dialog = page.getByRole("dialog", { name: labels.dialog, exact: true });
      await expect(dialog.getByText(template.description, { exact: true })).toBeVisible();
      await dialog.getByRole("button", { name: labels.project, exact: true }).click();
      await page.getByRole("button", { name: projectTitle, exact: true }).click();
      await expect(dialog.getByRole("combobox", { name: labels.runtime, exact: true })).toContainText("E2E isolated runtime");
      const configured = page.waitForResponse((response) => response.request().method() === "PUT"
        && response.url().endsWith(`/api/projects/${project.id}/execution-squads`));
      await dialog.getByRole("button", { name: labels.confirm, exact: true }).click();
      const response = await configured;
      expect(response.status()).toBe(200);
      expect(response.request().postDataJSON()).toEqual({
        squads: [{ template_key: "security-remediation", runtime_id: runtime.id, language: labels.language }],
      });
      await expect(page).toHaveURL(new RegExp(`/projects/${project.id}(?:\\?|$)`));

      const saved = await api.requestJSON<Project>(`/api/projects/${project.id}`);
      expect(saved.execution_squads).toHaveLength(1);
      const selection = saved.execution_squads[0]!;
      // The API persists configuration; live readiness is derived separately.
      expect(selection).toMatchObject({ state: "configured", template_key: "security-remediation" });
      expect(selection.squad_id).toBeTruthy();
      const squad = await api.requestJSON<Squad>(`/api/squads/${selection.squad_id}`);
      expect(squad).toMatchObject({ name: labels.title, template_key: "security-remediation", template_version: 1 });

      const customName = "Application security response";
      const customDescription = "Our application-specific review and repair policy.";
      await api.requestJSON(`/api/squads/${squad.id}`, {
        method: "PUT", body: { name: customName, description: customDescription },
      });
      await page.goto(catalogPath);
      await entry.getByRole("link", { name: customName, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/squads/${squad.id}$`));
      expect(await api.requestJSON<Squad>(`/api/squads/${squad.id}`)).toMatchObject({
        name: customName, description: customDescription, template_key: "security-remediation",
      });
      expect(pageErrors).toEqual([]);
    } finally {
      await api.deleteFeatureWorkspace(workspace.id);
    }
  });
}
