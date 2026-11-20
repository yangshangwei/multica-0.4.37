// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  resolveQuickCreateSubmitState,
  type QuickCreateSubmitInput,
} from "./quick-create-submit-state";

// Everything in place: a description, a recipient, loaded lists.
const ready: QuickCreateSubmitInput = {
  submitting: false,
  uploading: false,
  hasContent: true,
  hasActor: true,
  autonomyBlocked: false,
  versionBlocked: false,
  sourceRequired: false,
  sourceReady: false,
  recipientsKnown: true,
  recipientsFailed: false,
  usableRecipientCount: 2,
};

// Every blocker at once; each case below lifts the ones ranked above it.
const everything: QuickCreateSubmitInput = {
  submitting: true,
  uploading: true,
  hasContent: false,
  hasActor: false,
  autonomyBlocked: true,
  versionBlocked: true,
  sourceRequired: true,
  sourceReady: false,
  recipientsKnown: true,
  recipientsFailed: true,
  usableRecipientCount: 0,
};

describe("quick-create submit state", () => {
  it("is ready when nothing blocks", () => {
    expect(resolveQuickCreateSubmitState(ready)).toBe("ready");
  });

  it("names the first blocker in the documented order", () => {
    let input = everything;
    const lift = (patch: Partial<QuickCreateSubmitInput>) => {
      input = { ...input, ...patch };
      return resolveQuickCreateSubmitState(input);
    };
    expect(resolveQuickCreateSubmitState(input)).toBe("submitting");
    expect(lift({ submitting: false })).toBe("uploading");
    expect(lift({ uploading: false })).toBe("no_recipients");
    // Stop being a dead end without yet having a recipient.
    expect(lift({ usableRecipientCount: 1 })).toBe("autonomy_blocked");
    expect(lift({ autonomyBlocked: false })).toBe("version_blocked");
    expect(lift({ versionBlocked: false })).toBe("source_unavailable");
    expect(lift({ sourceReady: true })).toBe("missing_description");
    expect(lift({ hasContent: true })).toBe("recipients_failed");
    expect(lift({ recipientsFailed: false, recipientsKnown: false })).toBe("loading_recipients");
    expect(lift({ recipientsKnown: true })).toBe("missing_recipient");
    expect(lift({ hasActor: true })).toBe("ready");
  });

  it("puts the dead end ahead of an empty description", () => {
    expect(resolveQuickCreateSubmitState({ ...ready, hasActor: false, hasContent: false, usableRecipientCount: 0 }))
      .toBe("no_recipients");
  });

  it("asks for the description while the lists are still loading", () => {
    expect(resolveQuickCreateSubmitState({ ...ready, hasActor: false, hasContent: false, recipientsKnown: false, usableRecipientCount: 0 }))
      .toBe("missing_description");
  });

  it("does not call an unloaded or failed list empty", () => {
    const unknown = { ...ready, hasActor: false, recipientsKnown: false, usableRecipientCount: 0 };
    expect(resolveQuickCreateSubmitState(unknown)).toBe("loading_recipients");
    expect(resolveQuickCreateSubmitState({ ...unknown, recipientsFailed: true })).toBe("recipients_failed");
  });

  it("keeps a selected recipient out of the no-recipients state", () => {
    // An explicitly requested creator that cannot create issues is still a
    // selection; its own notice explains it.
    expect(resolveQuickCreateSubmitState({ ...ready, autonomyBlocked: true, usableRecipientCount: 0 }))
      .toBe("autonomy_blocked");
  });

  it("ignores a missing source preview when no source is required", () => {
    expect(resolveQuickCreateSubmitState({ ...ready, sourceRequired: false, sourceReady: false })).toBe("ready");
  });

  it("is ready exactly when the previous button condition allowed a submit", () => {
    const flags = [
      "submitting", "uploading", "hasContent", "hasActor", "autonomyBlocked", "versionBlocked",
      "sourceRequired", "sourceReady", "recipientsKnown", "recipientsFailed",
    ] as const;
    let checked = 0;
    for (let mask = 0; mask < 1 << flags.length; mask += 1) {
      for (const usableRecipientCount of [0, 1, 3]) {
        const input = { ...ready, usableRecipientCount };
        flags.forEach((flag, bit) => { input[flag] = (mask & (1 << bit)) !== 0; });
        // The submit button's `disabled` expression before this state existed.
        const wasDisabled = !input.hasContent || !input.hasActor || input.submitting || input.versionBlocked
          || input.autonomyBlocked || input.uploading || (input.sourceRequired && !input.sourceReady);
        expect(resolveQuickCreateSubmitState(input) === "ready", JSON.stringify(input)).toBe(!wasDisabled);
        checked += 1;
      }
    }
    expect(checked).toBe(3072);
  });
});
