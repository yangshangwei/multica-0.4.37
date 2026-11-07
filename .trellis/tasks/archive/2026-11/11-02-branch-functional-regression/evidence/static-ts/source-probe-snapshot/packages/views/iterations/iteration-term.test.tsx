import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { IterationTerm } from "./iteration-term";
import projects from "../locales/en/projects.json";
import chineseProjects from "../locales/zh-Hans/projects.json";

const translations = createInstance();
beforeAll(async () => {
  await translations.init({
    lng: "en", fallbackLng: "en", interpolation: { escapeValue: false },
    resources: { en: { projects }, "zh-Hans": { projects: chineseProjects } },
  });
});
beforeEach(async () => { await translations.changeLanguage("en"); });

function mount(props: Partial<React.ComponentProps<typeof IterationTerm>> = {}) {
  return render(<I18nextProvider i18n={translations}>
    <IterationTerm term="original" {...props}>{projects.iterations.original}</IterationTerm>
    <button type="button">Elsewhere</button>
  </I18nextProvider>);
}

describe("iteration term explanations", () => {
  it("explains a hovered label and dismisses with Escape without changing its accessible name", async () => {
    const user = userEvent.setup();
    mount();
    const label = screen.getByRole("button", { name: projects.iterations.original });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    await user.hover(label);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(projects.iterations.termHelp.original);
    expect(label).toHaveAccessibleName(projects.iterations.original);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());
  });

  it("supports keyboard focus and localizes the explanation, including a frozen-history note", async () => {
    await translations.changeLanguage("zh-Hans");
    const user = userEvent.setup();
    mount({ closed: true });
    await user.tab();
    const label = screen.getByRole("button", { name: projects.iterations.original });
    expect(label).toHaveFocus();
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent(chineseProjects.iterations.termHelp.original);
    expect(tooltip).toHaveTextContent(chineseProjects.iterations.termHelp.frozen);
    expect(label).toHaveAccessibleDescription(tooltip.textContent!);
    await user.tab();
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());
  });

  it("opens on the first tap and dismisses outside without requiring hover", async () => {
    const user = userEvent.setup();
    mount();
    const label = screen.getByRole("button", { name: projects.iterations.original });
    await user.pointer([{ keys: "[TouchA>]", target: label }, { keys: "[/TouchA]", target: label }]);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(projects.iterations.termHelp.original);
    await user.click(screen.getByRole("button", { name: "Elsewhere" }));
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());
  });

  it("composes onto an existing control without nesting a button or replacing its action", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    mount({ render: <button type="button" onClick={onClick} /> });
    const label = screen.getByRole("button", { name: projects.iterations.original });
    expect(label.querySelector("button")).toBeNull();
    await user.hover(label);
    expect(await screen.findByRole("tooltip")).toBeVisible();
    await user.click(label);
    expect(onClick).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());
  });
});
