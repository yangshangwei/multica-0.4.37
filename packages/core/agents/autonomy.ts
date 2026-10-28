import { AGENT_AUTONOMY_LEVELS, isKnownAutonomyLevel } from "../types";
import type { AgentAutonomyLevel } from "../types";

/**
 * The level an agent needs to file an issue itself. Create-with-agent hands the
 * request to the agent, whose only step is `multica issue create`; the server
 * refuses that call below this level and refuses the quick-create up front with
 * `agent_autonomy_insufficient`.
 */
export const QUICK_CREATE_MIN_AUTONOMY: AgentAutonomyLevel = "contributor";

/**
 * Whether an agent at this declared level may create issues. An absent or
 * unknown level means no policy is declared, which the server lets through, so
 * it must pass here too or the UI would claim a limit nobody enforces.
 */
export function canAgentCreateIssues(level: string | null | undefined): boolean {
  if (!isKnownAutonomyLevel(level)) return true;
  return AGENT_AUTONOMY_LEVELS.indexOf(level) >= AGENT_AUTONOMY_LEVELS.indexOf(QUICK_CREATE_MIN_AUTONOMY);
}
