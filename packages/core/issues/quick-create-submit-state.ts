export type QuickCreateSubmitState =
  | "submitting"
  | "uploading"
  | "no_recipients"
  | "autonomy_blocked"
  | "version_blocked"
  | "source_unavailable"
  | "missing_description"
  | "recipients_failed"
  | "loading_recipients"
  | "missing_recipient"
  | "ready";

export interface QuickCreateSubmitInput {
  submitting: boolean;
  uploading: boolean;
  hasContent: boolean;
  hasActor: boolean;
  autonomyBlocked: boolean;
  versionBlocked: boolean;
  /** The dialog was opened from a comment and needs its captured context. */
  sourceRequired: boolean;
  sourceReady: boolean;
  /** Agents, members and squads all have data and preferences are hydrated. */
  recipientsKnown: boolean;
  /** One of those lists errored without any data to fall back on. */
  recipientsFailed: boolean;
  /** Agents and squads that could actually take the request. */
  usableRecipientCount: number;
}

/**
 * Why agent create cannot be submitted right now, or "ready". When several
 * blockers hold at once, this names the one to deal with first. It only
 * describes a blocked submit: "ready" is exactly the set of inputs the submit
 * button accepted before this function existed.
 */
export function resolveQuickCreateSubmitState(input: QuickCreateSubmitInput): QuickCreateSubmitState {
  if (input.submitting) return "submitting";
  if (input.uploading) return "uploading";
  // A dead end: no description can get past it, so it outranks the empty one.
  if (!input.hasActor && input.recipientsKnown && input.usableRecipientCount === 0) return "no_recipients";
  if (input.autonomyBlocked) return "autonomy_blocked";
  if (input.versionBlocked) return "version_blocked";
  if (input.sourceRequired && !input.sourceReady) return "source_unavailable";
  // Loading ends by itself; writing the description is what the user can do.
  if (!input.hasContent) return "missing_description";
  if (!input.hasActor) {
    if (input.recipientsFailed) return "recipients_failed";
    if (!input.recipientsKnown) return "loading_recipients";
    return "missing_recipient";
  }
  return "ready";
}
