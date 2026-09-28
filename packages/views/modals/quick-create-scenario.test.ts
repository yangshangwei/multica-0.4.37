// @vitest-environment node
import { createInstance } from "i18next";
import { beforeAll, describe, expect, it } from "vitest";
import en from "../locales/en/modals.json";
import zh from "../locales/zh-Hans/modals.json";
import { getQuickCreateScenario } from "./quick-create-scenario";

const i18n = createInstance();
beforeAll(async () => {
  await i18n.init({
    lng: "en",
    fallbackLng: "en",
    resources: { en: { modals: en }, "zh-Hans": { modals: zh } },
    interpolation: { escapeValue: false },
  });
});

describe.each([["en", en], ["zh-Hans", zh]] as const)("quick-create scenarios (%s)", (locale, copy) => {
  const t = i18n.getFixedT(locale, "modals");
  const agent = { name: "Renamed partner", description: "" };
  const squad = { name: "Renamed squad", description: "" };

  it.each([
    "product-analyst", "architect", "implementer", "qa-engineer", "diagnostician",
    "code-reviewer", "security-reviewer", "release-engineer", "technical-writer",
    "progress-reporter", "reliability-engineer", "agent-evaluator",
    "experience-validation-engineer", "migration-reviewer",
  ] as const)("uses the %s role even when the agent was renamed", (template_key) => {
    expect(getQuickCreateScenario(t, { ...agent, template_key }))
      .toBe(copy.create_issue.scenarios[template_key]);
  });

  it.each([
    "feature-delivery", "bug-fix", "review-gate", "discovery", "docs",
    "maintenance", "release", "incident",
  ] as const)("uses the %s squad workflow and its lead's workflow", (template_key) => {
    const reviewer = { ...agent, template_key: "code-reviewer" };
    expect(getQuickCreateScenario(t, reviewer, { ...squad, template_key }))
      .toBe(copy.create_issue.scenarios[template_key]);
    expect(getQuickCreateScenario(t, { ...agent, template_key: `${template_key}-lead` }))
      .toBe(copy.create_issue.scenarios[template_key]);
  });

  it("recognizes the workspace assistant by system identity, not name", () => {
    expect(getQuickCreateScenario(t, { ...agent, system_key: "mika" }))
      .toBe(copy.create_issue.scenarios.mika);
    expect(getQuickCreateScenario(t, { ...agent, name: "Mika" }))
      .not.toBe(copy.create_issue.scenarios.mika);
  });

  it("uses a custom agent's actual description without inferring skills from its name", () => {
    const custom = { name: "Security Reviewer", description: "  Reviews\nAPI documentation  " };
    expect(getQuickCreateScenario(t, custom)).toBe(t(($) => $.create_issue.scenarios.custom_agent, {
      name: custom.name, description: "Reviews API documentation",
    }));
  });

  it("keeps a custom squad's responsibilities separate from its leader", () => {
    const custom = { ...squad, description: "Checks SDK compatibility" };
    expect(getQuickCreateScenario(t, { ...agent, template_key: "implementer" }, custom))
      .toBe(t(($) => $.create_issue.scenarios.custom_squad, custom));
  });

  it.each(["future-template", "constructor", "__proto__"])("handles unknown template %s without inventing a specialty", (template_key) => {
    expect(getQuickCreateScenario(t, { ...agent, template_key, description: "  " }))
      .toBe(t(($) => $.create_issue.scenarios.generic_agent, agent));
    expect(getQuickCreateScenario(t, agent, { ...squad, template_key }))
      .toBe(t(($) => $.create_issue.scenarios.generic_squad, squad));
  });

  it("shows a development example while no actor is available", () => {
    expect(getQuickCreateScenario(t)).toBe(copy.create_issue.agent.prompt_placeholder);
  });
});
