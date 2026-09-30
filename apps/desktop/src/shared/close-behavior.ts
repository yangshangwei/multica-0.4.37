/**
 * Cross-process contract for the close-behavior feature.
 *
 * Types and IPC channel constants shared between `main/` and `preload/`.
 * Implementation lives in `main/close-behavior.ts`; the renderer-facing
 * API surface is exposed by `preload/index.ts` over the channels declared
 * here.
 */

export type CloseBehavior = "quit" | "minimize" | "ask";

export const DEFAULT_CLOSE_BEHAVIOR: CloseBehavior = "ask";

export const CLOSE_BEHAVIOR_VALUES: readonly CloseBehavior[] = [
  "quit",
  "minimize",
  "ask",
] as const;

export function isCloseBehavior(value: unknown): value is CloseBehavior {
  return (
    typeof value === "string" &&
    (CLOSE_BEHAVIOR_VALUES as readonly string[]).includes(value)
  );
}

export interface CloseBehaviorPromptRequest {
  requestId: string;
}

export interface CloseBehaviorPromptResult {
  action: CloseBehavior;
  remember: boolean;
}

export interface CloseBehaviorSetResult {
  ok: boolean;
  reason?: "invalid_value" | "persist_failed";
}

export const CLOSE_BEHAVIOR_CHANNELS = {
  get: "close-behavior:get",
  set: "close-behavior:set",
  isTraySupported: "close-behavior:is-tray-supported",
  prompt: "close-behavior:prompt",
  respond: "close-behavior:respond",
} as const;
