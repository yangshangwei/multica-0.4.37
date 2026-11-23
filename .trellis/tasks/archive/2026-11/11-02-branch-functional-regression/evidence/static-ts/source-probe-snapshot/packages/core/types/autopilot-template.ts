import type {
  Autopilot,
  AutopilotAssigneeType,
  AutopilotSubscriberInput,
  AutopilotTrigger,
} from "./autopilot";

/**
 * A built-in autopilot template: one standing automation a workspace can adopt
 * in one step.
 *
 * Adopting one produces an ORDINARY autopilot plus an ordinary schedule
 * trigger — the scheduler and every edit surface treat them exactly as they
 * treat hand-built rows. What the template contributes is content and
 * provenance: `prompt` is byte-for-byte what lands on the created autopilot,
 * which is why the picker can show it before the person commits.
 */
export interface AutopilotTemplate {
  /** Stable identity, recorded on created autopilots as `template_key`. */
  key: string;
  version: number;
  /** Stable English slug (e.g. `repo-health`). Group by this, not by the label. */
  category: string;
  /** Localized category copy. Follows the requested language. */
  category_label: string;
  /** Localized card title. Also what gets stored as the autopilot's title. */
  title: string;
  /**
   * Localized card copy. Display only — the text stored on the autopilot row
   * is `prompt`, because an autopilot has no separate prompt column and its
   * description IS the brief handed to the agent.
   */
  description: string;
  /** Five-field cron the created schedule trigger runs on. */
  cron_expression: string;
  /**
   * Widened to string at the boundary: a mode this client has never heard of
   * must still render as a card rather than collapse the list. Any switch on
   * it needs a `default` branch.
   */
  execution_mode: string;
  avatar_emoji: string;
  /** The canonical Chinese prompt, exactly as it will be copied onto the row. */
  prompt: string;
}

/**
 * The creation input.
 *
 * Everything the template decides is deliberately absent — no title, prompt,
 * cron or execution mode. The server takes those from the template so a client
 * cannot claim a template's provenance while supplying its own prompt. All of
 * them are editable afterwards through the ordinary autopilot endpoints.
 */
export interface CreateAutopilotFromTemplateRequest {
  template_key: string;
  /** Required: a template cannot know which agent a workspace owns. */
  assignee_id: string;
  /** Omitted means "agent", matching `CreateAutopilotRequest`. */
  assignee_type?: AutopilotAssigneeType;
  project_id?: string | null;
  /** The schedule trigger's timezone. Omitted means UTC. */
  timezone?: string;
  /** Selects the localized title; the prompt retains its canonical Chinese body. */
  language?: string;
  subscribers?: AutopilotSubscriberInput[];
}

/**
 * Both rows the call wrote, in one round trip. The endpoint's whole point is
 * that these two either exist together or not at all — there is no state in
 * which the autopilot landed and its schedule did not.
 */
export interface CreateAutopilotFromTemplateResponse {
  autopilot: Autopilot;
  trigger: AutopilotTrigger;
}
