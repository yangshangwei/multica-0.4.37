import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { IssuePropertyValue } from "@multica/core/types";
import { renderWithI18n } from "../../../test/i18n";
import { CustomPropertyValueInput } from "./custom-property-picker";

type PropertyDefinition = React.ComponentProps<typeof CustomPropertyValueInput>["property"];
const choices = Array.from({ length: 10 }, (_, index) => ({
  id: `choice-${index + 1}`, name: `Choice ${index + 1}`, color: "#2563eb",
}));
const choiceProperty: PropertyDefinition = {
  id: "choice-property", name: "Release checks", type: "select", archived: false,
  config: { options: choices },
};

afterEach(() => vi.restoreAllMocks());

function ChoiceHarness({ type, onChange, options = choices }: {
  type: "select" | "multi_select";
  onChange: (value: IssuePropertyValue | undefined) => void;
  options?: typeof choices;
}) {
  const [value, setValue] = useState<IssuePropertyValue | undefined>(type === "select" ? "choice-2" : ["choice-2"]);
  return <CustomPropertyValueInput property={{ ...choiceProperty, type, config: { options } }} value={value}
    defaultOpen trigger="Edit choices" onChange={(next) => { setValue(next); onChange(next); }} />;
}

describe("custom property choice search", () => {
  it.each(["select", "multi_select"] as const)("filters an exact %s query and commits only the visible result", async (type) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderWithI18n(<ChoiceHarness type={type} onChange={onChange} />);
    const input = await screen.findByRole("textbox", { name: "Filter options" });
    await user.type(input, "  cHoIcE 10  ");
    expect(input).toHaveValue("  cHoIcE 10  ");
    expect(screen.getByRole("button", { name: "Choice 10", exact: true })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Choice 1", exact: true })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Choice 2", exact: true })).not.toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(type === "select" ? "choice-10" : ["choice-2", "choice-10"]);
  });

  it.each(["select", "multi_select"] as const)("keeps partial %s matches and resets browsing after closing", async (type) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderWithI18n(<ChoiceHarness type={type} onChange={onChange} />);
    const input = await screen.findByRole("textbox", { name: "Filter options" });
    await user.type(input, "choice 1");
    expect(screen.getByRole("button", { name: "Choice 1", exact: true })).toBeVisible();
    expect(screen.getByRole("button", { name: "Choice 10", exact: true })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Choice 2", exact: true })).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{Escape}");
    const trigger = screen.getByRole("button", { name: "Edit choices" });
    await waitFor(() => expect(trigger).toHaveFocus());
    await user.click(trigger);
    expect(await screen.findByRole("textbox", { name: "Filter options" })).toHaveValue("");
    const selected = screen.getByRole("button", { name: "Choice 2", exact: true });
    expect(selected).toBeVisible();
    if (type === "multi_select") expect(selected).toHaveAttribute("aria-pressed", "true");
    expect(onChange).not.toHaveBeenCalled();
  });

  it.each(["select", "multi_select"] as const)("does not write for a no-match %s Enter and still allows explicit clearing", async (type) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderWithI18n(<ChoiceHarness type={type} onChange={onChange} />);
    await user.type(await screen.findByRole("textbox", { name: "Filter options" }), "absent choice");
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    const empty = screen.getByRole("status");
    expect(empty).toHaveTextContent("No results");
    expect(empty).toBeVisible();
    const clear = screen.getByRole("button", { name: "No value", exact: true });
    expect(clear).not.toHaveAttribute("aria-pressed");
    await user.click(clear);
    expect(onChange).toHaveBeenCalledExactlyOnceWith(undefined);
    await user.click(screen.getByRole("button", { name: "Edit choices" }));
    expect(await screen.findByRole("textbox", { name: "Filter options" })).toHaveValue("");
    expect(screen.getByRole("button", { name: "Choice 2", exact: true })).toBeVisible();
  });

  it("exposes add/remove state while preserving selections outside the filter", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderWithI18n(<ChoiceHarness type="multi_select" onChange={onChange} />);
    expect(await screen.findByRole("button", { name: "Choice 2", pressed: true })).toBeVisible();
    await user.type(screen.getByRole("textbox", { name: "Filter options" }), "choice 10");
    const option = screen.getByRole("button", { name: "Choice 10", pressed: false });
    await user.click(option);
    expect(option).toHaveAttribute("aria-pressed", "true");
    expect(onChange).toHaveBeenLastCalledWith(["choice-2", "choice-10"]);
    await user.keyboard(" ");
    expect(option).toHaveAttribute("aria-pressed", "false");
    expect(onChange).toHaveBeenLastCalledWith(["choice-2"]);
    expect(screen.getByRole("textbox", { name: "Filter options" })).toBeInTheDocument();
  });

  it("keeps a short single-select as commands and resets search after a committed selection", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const view = renderWithI18n(<ChoiceHarness type="select" onChange={onChange} options={choices.slice(0, 7)} />);
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    const selected = await screen.findByRole("button", { name: "Choice 2", exact: true });
    expect(selected).not.toHaveAttribute("aria-pressed");
    await user.click(selected);
    expect(onChange).toHaveBeenCalledExactlyOnceWith("choice-2");
    view.rerender(<ChoiceHarness type="select" onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: "Edit choices" }));
    await user.type(await screen.findByRole("textbox", { name: "Filter options" }), "choice 10");
    await user.keyboard("{Enter}");
    await user.click(screen.getByRole("button", { name: "Edit choices" }));
    expect(await screen.findByRole("textbox", { name: "Filter options" })).toHaveValue("");
    expect(screen.getByRole("button", { name: "Choice 1", exact: true })).toBeVisible();
  });

  it("does not use a stale highlighted choice after a definition changes", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const view = renderWithI18n(<ChoiceHarness type="select" onChange={onChange} />);
    const input = await screen.findByRole("textbox", { name: "Filter options" });
    await user.type(input, "choice 10");
    view.rerender(<ChoiceHarness type="select" onChange={onChange} options={choices.slice(0, 9)} />);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("No results", { exact: true })).toBeVisible();
  });
});

// The existing .test.ts owns the read-only/type-compatibility matrix. These
// tests exercise actual inputs, names and native submission with real popovers.
describe("custom property value input labels", () => {
  it.each([
    { type: "text", name: "Acceptance criterion", input: "  Reviewed by the owner  ", expected: "Reviewed by the owner" },
    { type: "number", name: "Estimated days", input: "1.25", expected: 1.25 },
    { type: "url", name: "Reference URL", input: "  https://example.invalid/review  ", expected: "https://example.invalid/review" },
  ])("names the actual $type input after its property and commits the typed value", async ({ type, name, input, expected }) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderWithI18n(<CustomPropertyValueInput property={{ ...choiceProperty, type, name }} value={undefined}
      defaultOpen trigger="Edit field" onChange={onChange} />);
    const editor = await screen.findByRole(type === "number" ? "spinbutton" : "textbox", { name });
    expect(screen.getByText(name, { selector: "label" })).toBeVisible();
    if (type === "number") {
      expect(editor).toHaveAttribute("step", "any");
      expect(editor).toHaveAttribute("inputmode", "decimal");
    }
    await user.type(editor, input);
    expect(editor).toHaveValue(type === "number" ? expected : input);
    expect(editor).toHaveFocus();
    if (type === "number") {
      expect(editor).toHaveProperty("value", input);
      expect(editor).toBeValid();
      // user-event excludes number inputs from implicit Enter submission.
      // Chromium owns that key path; this DOM case submits the real form.
      fireEvent.submit(editor.closest("form")!);
    } else await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(expected);
  });

  it("keeps the URL open action separate from editing and clearing", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const url = "https://example.invalid/original";
    renderWithI18n(<CustomPropertyValueInput property={{ ...choiceProperty, type: "url", name: "Reference URL" }} value={url}
      defaultOpen trigger="Edit field" onChange={onChange} />);
    const editor = await screen.findByRole("textbox", { name: "Reference URL" });
    expect(editor).toHaveValue(url);
    await user.click(screen.getByRole("button", { name: "Open link" }));
    expect(open).toHaveBeenCalledExactlyOnceWith(url, "_blank", "noopener,noreferrer");
    expect(onChange).not.toHaveBeenCalled();
    await user.clear(editor);
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(undefined);
  });

  it("gives two mounted editors for the same property independent label IDs", async () => {
    renderWithI18n(<>
      <CustomPropertyValueInput property={{ ...choiceProperty, type: "text", name: "First acceptance criterion" }} value={undefined}
        defaultOpen trigger="Edit first field" onChange={vi.fn()} />
      <CustomPropertyValueInput property={{ ...choiceProperty, type: "text", name: "Second acceptance criterion" }} value={undefined}
        defaultOpen trigger="Edit second field" onChange={vi.fn()} />
    </>);
    const first = await screen.findByRole("textbox", { name: "First acceptance criterion" });
    const second = await screen.findByRole("textbox", { name: "Second acceptance criterion" });
    expect(first.id).not.toBe(second.id);
    expect(first.id).not.toBe("");
    expect(second.id).not.toBe("");
  });
});
