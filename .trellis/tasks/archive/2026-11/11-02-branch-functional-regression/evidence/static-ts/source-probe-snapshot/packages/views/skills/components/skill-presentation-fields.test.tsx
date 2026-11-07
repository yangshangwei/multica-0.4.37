// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EMPTY_SKILL_PRESENTATION, type SkillPresentationMeta } from "@multica/core/skills";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSkills from "../../locales/en/skills.json";
import { SkillPresentationFields } from "./skill-presentation-fields";

const TEST_RESOURCES = { en: { common: enCommon, skills: enSkills } };

function renderFields(
  value: SkillPresentationMeta = EMPTY_SKILL_PRESENTATION,
  extra: Partial<Parameters<typeof SkillPresentationFields>[0]> = {},
) {
  const onChange = vi.fn();
  render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <SkillPresentationFields value={value} onChange={onChange} {...extra} />
    </I18nProvider>,
  );
  return { onChange };
}

beforeEach(() => {
  cleanup();
});

describe("SkillPresentationFields category", () => {
  it("lists every category and emits the picked one", async () => {
    const user = userEvent.setup();
    const { onChange } = renderFields();
    await user.click(screen.getByRole("combobox", { name: "Category" }));
    expect(await screen.findAllByRole("option")).toHaveLength(8);
    await user.click(screen.getByRole("option", { name: "Development & integration" }));
    expect(onChange).toHaveBeenCalledWith({
      ...EMPTY_SKILL_PRESENTATION,
      category: "engineering",
    });
  });

  it("keeps a null icon when the category changes", async () => {
    const user = userEvent.setup();
    const { onChange } = renderFields({ category: "writing", icon: null });
    await user.click(screen.getByRole("combobox", { name: "Category" }));
    await user.click(await screen.findByRole("option", { name: "Data & automation" }));
    expect(onChange).toHaveBeenCalledWith({ category: "data", icon: null });
  });
});

describe("SkillPresentationFields icon", () => {
  it("picks an icon from the whitelist grid", async () => {
    const user = userEvent.setup();
    const { onChange } = renderFields();
    await user.click(screen.getByRole("button", { name: "Pick an icon" }));
    const rocket = await screen.findByRole("button", { name: "rocket" });
    expect(rocket.getAttribute("aria-pressed")).toBe("false");
    await user.click(rocket);
    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_SKILL_PRESENTATION, icon: "rocket" });
  });

  it("marks the current icon pressed and resets to follow-category", async () => {
    const user = userEvent.setup();
    const { onChange } = renderFields({ category: "research", icon: "bug" });
    await user.click(screen.getByRole("button", { name: "Pick an icon" }));
    expect(
      (await screen.findByRole("button", { name: "bug" })).getAttribute("aria-pressed"),
    ).toBe("true");
    await user.click(screen.getByRole("button", { name: "Follow category" }));
    expect(onChange).toHaveBeenCalledWith({ category: "research", icon: null });
  });

  it("disables both fields for read-only viewers and renders no free-text tag input", () => {
    renderFields({ category: "research", icon: "bug" }, { disabled: true });
    expect(screen.getByRole("combobox", { name: "Category" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Pick an icon" })).toBeDisabled();
    // Labels are workspace labels handled by ResourceLabelPicker; the fields
    // must not grow a free-text tag editor again.
    expect(screen.queryByRole("textbox")).toBeNull();
  });
});
