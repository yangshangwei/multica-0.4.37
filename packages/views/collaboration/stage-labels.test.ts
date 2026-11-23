// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { useCollaborationStageLabels } from "./stage-labels";

const selection = vi.hoisted(() => ({ locale: "en" }));
vi.mock("../i18n", async () => {
  const en = (await import("../locales/en/collaboration.json")).default;
  const zh = (await import("../locales/zh-Hans/collaboration.json")).default;
  return { useT: () => ({ t: (selector: (value: { stage: { refusals: typeof en.stage.refusals } }) => string) => selector(selection.locale === "en" ? en : zh) }) };
});

describe("execution and rollback refusal labels", () => {
  it.each([
    ["stage_execution_unavailable", "Execution integration is unavailable.", "执行集成暂不可用。"],
    ["stage_execution_target_changed", "The execution target changed. Review it again.", "执行目标已变化，请重新审阅。"],
    ["stage_execution_pending", "An execution request for this target is still pending.", "此目标的执行请求仍待确认。"],
    ["stage_execution_conflict", "The execution observation conflicts with the recorded request.", "执行观测与已记录的请求冲突。"],
    ["stage_rollback_observation_unavailable", "Rollback evidence is unavailable.", "回滚证据暂不可用。"],
    ["stage_rollback_observation_changed", "Rollback evidence changed. Review it again.", "回滚证据已变化，请重新审阅。"],
  ])("shows the bilingual reason for %s", (code, english, chinese) => {
    selection.locale = "en";
    expect(useCollaborationStageLabels().refusal(code)).toBe(english);
    selection.locale = "zh-Hans";
    expect(useCollaborationStageLabels().refusal(code)).toBe(chinese);
  });
  it.each(["en", "zh-Hans"])("keeps an unfamiliar reason on the %s unknown fallback", (locale) => {
    selection.locale = locale;
    const labels = useCollaborationStageLabels();
    expect(labels.refusal("stage_future_execution_gate")).toBe(labels.refusal(null));
  });
});
