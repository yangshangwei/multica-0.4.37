import type { Agent, Squad } from "@multica/core/types";
import type { useT } from "../i18n";

const agentScenarios = [
  "product-analyst",
  "architect",
  "implementer",
  "qa-engineer",
  "diagnostician",
  "code-reviewer",
  "security-reviewer",
  "release-engineer",
  "technical-writer",
  "progress-reporter",
  "reliability-engineer",
  "agent-evaluator",
  "experience-validation-engineer",
  "migration-reviewer",
] as const;

const squadScenarios = [
  "feature-delivery",
  "bug-fix",
  "review-gate",
  "discovery",
  "docs",
  "maintenance",
  "release",
  "incident",
] as const;

type ScenarioAgent = Pick<Agent, "name" | "description" | "template_key" | "system_key">;
type ScenarioSquad = Pick<Squad, "name" | "description" | "template_key">;

export function getQuickCreateScenario(
  t: ReturnType<typeof useT<"modals">>["t"],
  agent?: ScenarioAgent,
  squad?: ScenarioSquad,
): string {
  // A squad's leader is the execution target, not the identity of the example.
  if (squad) {
    const scenario = squadScenarios.find((key) => key === squad.template_key);
    if (scenario) return t(($) => $.create_issue.scenarios[scenario]);
  } else if (agent) {
    if (agent.system_key === "mika") return t(($) => $.create_issue.scenarios.mika);
    const scenario = agentScenarios.find((key) => key === agent.template_key)
      ?? squadScenarios.find((key) => `${key}-lead` === agent.template_key);
    if (scenario) return t(($) => $.create_issue.scenarios[scenario]);
  }

  const actor = squad ?? agent;
  if (!actor) return t(($) => $.create_issue.agent.prompt_placeholder);

  // Custom capabilities come from the saved description, never an editable name.
  const description = actor.description?.replace(/\s+/g, " ").trim();
  const values = { name: actor.name, description };
  if (squad) {
    return description
      ? t(($) => $.create_issue.scenarios.custom_squad, values)
      : t(($) => $.create_issue.scenarios.generic_squad, values);
  }
  return description
    ? t(($) => $.create_issue.scenarios.custom_agent, values)
    : t(($) => $.create_issue.scenarios.generic_agent, values);
}
