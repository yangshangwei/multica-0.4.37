// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SkillSummary } from "@multica/core/types";
import zhSkills from "../../locales/zh-Hans/skills.json";
import { renderWithI18n } from "../../test/i18n";
import { SkillPickerList } from "./skill-picker-list";

const builtinSkill: SkillSummary = {
  id: "skill-review",
  workspace_id: "ws-1",
  name: "multica-code-review",
  description:
    "Use when reviewing a diff: what to look for, how to state a finding so it is actionable, and what not to report.",
  config: {
    origin: { type: "builtin_role_skill", name: "multica-code-review" },
  },
  created_by: null,
  created_at: "2026-09-12T00:00:00Z",
  updated_at: "2026-09-12T00:00:00Z",
};

const customSkill: SkillSummary = {
  ...builtinSkill,
  id: "skill-custom",
  name: "custom-explainer",
  description: "Explain project conventions.",
  config: {},
};

// The provenance and custom-content matrix lives in skill-presentation.test.ts.
describe("SkillPickerList built-in presentation", () => {
  it("shows a Chinese name and purpose while returning the original selection", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    renderWithI18n(
      <SkillPickerList
        skills={[builtinSkill]}
        selectedIds={new Set([builtinSkill.id])}
        onToggle={onToggle}
      />,
      { locale: "zh-Hans" },
    );

    const option = screen.getByRole("button", { name: /代码审查/ });
    expect(option).toHaveAttribute("aria-pressed", "true");
    expect(option).toHaveTextContent(zhSkills.builtin_role_skills["multica-code-review"].description);

    await user.click(option);

    expect(onToggle).toHaveBeenCalledOnce();
    expect(onToggle.mock.calls[0]?.[0]).toBe(builtinSkill);
  });

  it.each([
    ["zh-Hans", "代码审查"],
    ["en", "multica-code-review"],
  ] as const)("supports Chinese and English search in %s", async (locale, displayName) => {
    const user = userEvent.setup();
    renderWithI18n(
      <SkillPickerList
        skills={[builtinSkill, customSkill]}
        selectedIds={new Set()}
        onToggle={vi.fn()}
      />,
      { locale },
    );
    const search = screen.getByRole("textbox");

    await user.type(search, "代码审查");
    expect(screen.getByRole("button", { name: new RegExp(displayName) })).toBeInTheDocument();
    expect(screen.queryByText(customSkill.name)).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "MULTICA-CODE-REVIEW");
    expect(screen.getByRole("button", { name: new RegExp(displayName) })).toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "actionable");
    expect(screen.getByRole("button", { name: new RegExp(displayName) })).toBeInTheDocument();
  });
});
