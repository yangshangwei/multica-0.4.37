import type { Agent } from "../types";
import { AGENT_CATEGORY_MAX_LENGTH } from "./constants";
import type { AgentRoleKind, AgentRoleMetadata } from "./discovery";

export interface AgentDirectoryCategory {
  key: string;
  preset: AgentRoleKind | null;
  name: string;
}

export const AGENT_CATEGORY_PRESET_ORDER: readonly AgentRoleKind[] = ["other", "specialist", "coordinator"];

const PRESET_NAMES: Record<AgentRoleKind, readonly [string, string]> = {
  other: ["General-purpose agents", "通用智能体"],
  specialist: ["Specialists", "专业角色"],
  coordinator: ["Coordinators", "统筹角色"],
};

function getCategoryPreset(name: string): AgentRoleKind | null {
  const normalized = name.trim().toLowerCase();
  return AGENT_CATEGORY_PRESET_ORDER.find((preset) =>
    PRESET_NAMES[preset].some((label) => label.toLowerCase() === normalized),
  ) ?? null;
}

/** Convert stored names to language-independent directory filter keys. */
export function getAgentCategoryKey(name: string): string {
  const preset = getCategoryPreset(name);
  return preset ? `preset:${preset}` : `custom:${name.trim()}`;
}

/** Explicit categories override provenance; unresolved template fallback stays unknown. */
export function resolveAgentDirectoryCategory(
  agent: Pick<Agent, "category" | "template_key">,
  role: AgentRoleMetadata | null,
  rolesReady = true,
): AgentDirectoryCategory | null {
  const name = getAgentCategory(agent);
  if (name) {
    return { key: getAgentCategoryKey(name), preset: getCategoryPreset(name), name };
  }
  if (agent.template_key && !rolesReady) return null;
  const preset = role?.kind ?? "other";
  return { key: `preset:${preset}`, preset, name: PRESET_NAMES[preset][0] };
}

/** Matches the server's trimmed Unicode-code-point limit and control check. */
export function isAgentCategoryValid(value: string): boolean {
  const normalized = value.trim();
  return [...normalized].length <= AGENT_CATEGORY_MAX_LENGTH && !/\p{Cc}/u.test(normalized);
}

/** Raw stored category name; an empty string leaves directory classification automatic. */
export function getAgentCategory(agent: Pick<Agent, "category">): string {
  return typeof agent.category === "string" ? agent.category.trim() : "";
}

/** Suggestions come only from agents already visible to the current caller. */
export function getAgentCategoryNames(agents: readonly Pick<Agent, "category">[]): string[] {
  return [...new Set(agents.map(getAgentCategory).filter(Boolean))]
    .sort((a, b) => a.localeCompare(b));
}
