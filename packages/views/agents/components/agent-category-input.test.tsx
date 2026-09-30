import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { AgentCategoryInput } from "./agent-category-input";

const listAgents = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api: { listAgents } }));

function CategoryField({ initialValue = "", disabled = false }: { initialValue?: string; disabled?: boolean }) {
  const [value, setValue] = useState(initialValue);
  return <AgentCategoryInput workspaceId="workspace-1" value={value} onChange={setValue} disabled={disabled} />;
}

function renderField(props: { initialValue?: string; disabled?: boolean } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <QueryClientProvider client={client}><CategoryField {...props} /></QueryClientProvider>,
  );
}

beforeEach(() => {
  listAgents.mockReset();
  listAgents.mockResolvedValue([{ category: "Research" }]);
});

describe("AgentCategoryInput", () => {
  // Category normalization and deduplication live in core/agents/category.test.ts.
  it("opens a visible list from the arrow and selects an existing category", async () => {
    const user = userEvent.setup();
    renderField();
    const input = screen.getByRole("combobox", { name: "Category" });
    await user.click(screen.getByRole("button", { name: "Show categories" }));
    await user.click(await screen.findByRole("option", { name: "Research" }));
    expect(input).toHaveValue("Research");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(listAgents).toHaveBeenCalledWith({ workspace_id: "workspace-1", include_archived: true });
  });

  it("guides free entry and lets the keyboard select an existing category", async () => {
    const user = userEvent.setup();
    renderField();
    const input = screen.getByRole("combobox", { name: "Category" });
    await user.type(input, "Operations");
    expect(await screen.findByRole("option", { name: 'Use new category "Operations"' })).toBeVisible();
    expect(screen.getByText(/Saved when you create the agent/)).toBeVisible();
    await user.tab();
    expect(input).toHaveValue("Operations");
    await user.clear(input);
    await user.type(input, "Res");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(input).toHaveValue("Research");
  });

  it("shows all choices on reopen even when a category is already selected", async () => {
    listAgents.mockResolvedValue([{ category: "Research" }, { category: "Content" }]);
    const user = userEvent.setup();
    renderField({ initialValue: "Research" });
    await user.click(screen.getByRole("button", { name: "Show categories" }));
    expect(await screen.findByRole("option", { name: "Content" })).toBeVisible();
    await user.click(screen.getByRole("option", { name: "Default category" }));
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveValue("");
    await user.keyboard("{Escape}{ArrowDown}");
    expect(await screen.findByRole("option", { name: "Default category" })).toHaveAttribute("aria-selected", "true");
  });

  it("always offers the three defaults and allows a new custom category", async () => {
    listAgents.mockResolvedValue([]);
    const user = userEvent.setup();
    renderField();
    await user.click(screen.getByRole("button", { name: "Show categories" }));
    expect(await screen.findByText(/No custom categories yet/)).toBeVisible();
    expect(screen.getAllByRole("option").map((option) => option.textContent)).toEqual([
      "General-purpose agents", "Specialists", "Coordinators", "Default category",
    ]);
    await user.click(screen.getByRole("option", { name: "Specialists" }));
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveValue("Specialists");
    await user.clear(screen.getByRole("combobox", { name: "Category" }));
    await user.type(screen.getByRole("combobox", { name: "Category" }), "Design");
    await user.click(screen.getByRole("option", { name: 'Use new category "Design"' }));
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveValue("Design");
  });

  it("deduplicates saved defaults and keeps custom suggestions after them", async () => {
    listAgents.mockResolvedValue([{ category: "Specialists" }, { category: "Research" }]);
    const user = userEvent.setup();
    renderField();
    await user.click(screen.getByRole("button", { name: "Show categories" }));
    await screen.findByRole("option", { name: "Research" });
    expect(screen.getAllByRole("option").map((option) => option.textContent)).toEqual([
      "General-purpose agents", "Specialists", "Coordinators", "Research", "Default category",
    ]);
  });

  it("disables typing and the dropdown for a read-only agent", () => {
    renderField({ disabled: true, initialValue: "Research" });
    expect(screen.getByRole("combobox", { name: "Category" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Show categories" })).toBeDisabled();
  });

  it("allows entering a category when suggestions cannot load", async () => {
    listAgents.mockRejectedValue(new Error("Offline"));
    const user = userEvent.setup();
    renderField();
    const input = screen.getByRole("combobox", { name: "Category" });
    await user.click(screen.getByRole("button", { name: "Show categories" }));
    expect(await screen.findByText(/Could not load categories/)).toBeVisible();
    expect(screen.getByRole("option", { name: "General-purpose agents" })).toBeVisible();
    await user.type(input, "Research");
    expect(input).toHaveValue("Research");
    expect(input).toBeEnabled();
  });
});
