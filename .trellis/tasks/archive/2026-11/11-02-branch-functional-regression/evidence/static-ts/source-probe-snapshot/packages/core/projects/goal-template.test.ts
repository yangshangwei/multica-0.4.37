// @vitest-environment node
import { expect, it } from "vitest";
import { projectGoalTemplateAppend } from "./goal-template";
it("preserves existing prose and appends only selected missing headings", () => {
  const sections = [{ id: "goal", title: "Goal", body: "Outcome" }, { id: "scope", title: "Scope", body: "Included" }];
  expect(projectGoalTemplateAppend("Original prose\n\n## Goal\nOriginal goal", sections, ["goal", "scope"])).toBe("## Scope\n\nIncluded");
  expect(projectGoalTemplateAppend("## Goal\n\n## Scope", sections, ["goal", "scope"])).toBe("");
});
