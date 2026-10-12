// @vitest-environment jsdom

import { useState, type ComponentProps, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AgentRoleTemplate, SquadTemplate } from "@multica/core/types";
import enSettings from "../locales/en/settings.json";
import zhSettings from "../locales/zh-Hans/settings.json";
import enAgents from "../locales/en/agents.json";
import zhAgents from "../locales/zh-Hans/agents.json";
import { McpAgentPicker, type McpAgentChoice } from "./mcp-agent-picker";

type PickerProps = ComponentProps<typeof McpAgentPicker>;

const roleTemplates: AgentRoleTemplate[] = [{
  key: "reviewer", version: 1, name: "Reviewer", title: "Code reviewer", description: "Review",
  autonomy_level: "observer", avatar_emoji: "", max_concurrent_tasks: 1, skill_names: [], instructions: "Review",
}];
const squadTemplates: SquadTemplate[] = [{
  key: "delivery", version: 1, name: "Delivery", title: "Delivery", description: "Delivery",
  avatar_emoji: "", instructions: "Route",
  leader: { template_key: "delivery-lead", title: "Delivery lead", name: "Lead", autonomy_level: "coordinator", avatar_emoji: "", role: "Route" },
  members: [],
}];
const agents: McpAgentChoice[] = [
  { id: "general", name: "Personal helper", description: "Release planning" },
  { id: "specialist", name: "Payments expert", description: "Review payments", template_key: "reviewer" },
  { id: "lead", name: "Release owner", description: "Route release work", template_key: "delivery-lead" },
  { id: "custom", name: "Research helper", description: "Release research", template_key: "reviewer", category: "Research" },
];

function Harness({ props }: { props: Partial<PickerProps> }) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  return (
    <McpAgentPicker
      agents={agents}
      roleTemplates={roleTemplates}
      squadTemplates={squadTemplates}
      selected={selected}
      succeeded={new Set()}
      failures={[]}
      disabled={false}
      showTools
      onClearSelection={() => setSelected(new Set())}
      onSelectionChange={(ids, checked) => setSelected((previous) => {
        const next = new Set(previous);
        ids.forEach((id) => checked ? next.add(id) : next.delete(id));
        return next;
      })}
      {...props}
    />
  );
}

function renderPicker(props: Partial<PickerProps> = {}, locale: "en" | "zh-Hans" = "en") {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <I18nProvider locale={locale} resources={{ en: { settings: enSettings, agents: enAgents }, "zh-Hans": { settings: zhSettings, agents: zhAgents } }}>
      {children}
    </I18nProvider>
  );
  const view = render(<Harness props={props} />, { wrapper });
  return {
    ...view,
    refresh: (updates: Partial<PickerProps>) => view.rerender(<Harness props={{ ...props, ...updates }} />),
  };
}

describe("MCP agent category projection", () => {
  // Canonical category normalization/provenance matrices live in core/agents/category.test.ts
  // and discovery.test.ts. These tests pin picker composition and recovery wiring.
  it("uses provenance after renaming, respects an explicit category, and counts the full directory", async () => {
    const user = userEvent.setup();
    renderPicker();
    const group = screen.getByRole("group", { name: "Agent categories" });
    expect(within(group).getAllByRole("button").map((button) => button.textContent)).toEqual([
      "All (4)", "General-purpose agents (1)", "Specialists (1)", "Coordinators (1)", "Research (1)",
    ]);
    await user.click(screen.getByRole("button", { name: "Specialists (1)" }));
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
    expect(screen.getByRole("checkbox", { name: "Payments expert" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Select results" }));
    await user.type(screen.getByRole("searchbox"), "payments");
    await user.click(screen.getByRole("button", { name: "Show selected only" }));
    expect(within(group).getAllByRole("button").map((button) => button.textContent)).toEqual([
      "All (4)", "General-purpose agents (1)", "Specialists (1)", "Coordinators (1)", "Research (1)",
    ]);
    await user.click(screen.getByRole("button", { name: "Coordinators (1)" }));
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Selected: 1");
  });

  it("marks preset counts/results incomplete during first metadata load while explicit categories stay usable", async () => {
    const user = userEvent.setup();
    renderPicker({
      agents: [...agents, { id: "explicit", name: "Saved specialist", category: "专业角色" }],
      roleTemplates: undefined, squadTemplates: undefined, rolesReady: false, metadataPending: true,
    });
    expect(screen.getByRole("button", { name: "General-purpose agents (1+)" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Specialists (1+)" })).toBeVisible();
    expect(screen.getByRole("button", { name: "All (5)" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "Specialists (1+)" }));
    await user.type(screen.getByRole("searchbox"), "payments");
    expect(screen.queryByText("No agents match your search.")).toBeNull();
    expect(screen.getByText("More agents may appear when category information is available.")).toBeVisible();
    expect(screen.getByText("Agents found: 0+ · Category results incomplete.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Research (1)" }));
    await user.click(screen.getByRole("button", { name: "Clear search" }));
    expect(screen.getByRole("checkbox", { name: "Research helper" })).toBeVisible();
    expect(screen.getByText("Agents found: 1")).toBeVisible();
  });

  it("retains category, query and selected IDs while unavailable metadata retries", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    const view = renderPicker({ onRetryMetadata: retry });
    await user.click(screen.getByRole("button", { name: "Specialists (1)" }));
    await user.click(screen.getByRole("checkbox", { name: "Payments expert" }));
    await user.type(screen.getByRole("searchbox"), "payments");
    view.refresh({ roleTemplates: undefined, squadTemplates: undefined, rolesReady: false, metadataError: true });
    expect(screen.getByRole("button", { name: "Specialists (0+)" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("searchbox")).toHaveValue("payments");
    expect(screen.getByText("Selected: 1")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
    view.refresh({ rolesReady: true });
    expect(screen.getByRole("button", { name: "Specialists (1)" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("checkbox", { name: "Payments expert" })).toBeChecked();
  });

  it("keeps cached classification and recovery controls on a metadata refresh failure", async () => {
    const user = userEvent.setup();
    renderPicker({ metadataError: true, rolesReady: true, onRetryMetadata: vi.fn() });
    await user.click(screen.getByRole("button", { name: "Coordinators (1)" }));
    expect(screen.getByRole("checkbox", { name: "Release owner" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
    expect(screen.getByText("Agents found: 1")).toBeVisible();
  });

  it("resets only a proven missing category and retains query and hidden selection", async () => {
    const user = userEvent.setup();
    const view = renderPicker();
    await user.click(screen.getByRole("checkbox", { name: "Personal helper" }));
    await user.click(screen.getByRole("button", { name: "Research (1)" }));
    await user.type(screen.getByRole("searchbox"), "release");
    view.refresh({ agents: agents.filter((agent) => agent.id !== "custom"), metadataError: true });
    expect(screen.getByRole("button", { name: /Research/ })).toHaveAttribute("aria-pressed", "true");
    view.refresh({ agents: agents.filter((agent) => agent.id !== "custom") });
    expect(screen.getByRole("button", { name: "All (3)" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("searchbox")).toHaveValue("release");
    expect(screen.getByRole("checkbox", { name: "Personal helper" })).toBeChecked();
  });

  it("scopes bulk controls to unfinished visible rows while preserving other categories", async () => {
    const user = userEvent.setup();
    renderPicker({ succeeded: new Set(["lead"]) });
    await user.click(screen.getByRole("checkbox", { name: "Research helper" }));
    await user.click(screen.getByRole("button", { name: "Specialists (1)" }));
    await user.click(screen.getByRole("button", { name: "Select results" }));
    expect(screen.getByText("Selected: 2")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Deselect results" }));
    expect(screen.getByText("Selected: 1")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Coordinators (1)" }));
    expect(screen.getByRole("checkbox", { name: "Release owner" })).toBeChecked();
    expect(screen.getByRole("button", { name: "Deselect results" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "All (4)" }));
    expect(screen.getByRole("checkbox", { name: "Research helper" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Payments expert" })).not.toBeChecked();
  });

  it("uses the existing Chinese directory presets while preserving custom names", () => {
    renderPicker({}, "zh-Hans");
    const group = screen.getByRole("group", { name: "智能体分类" });
    expect(group).toHaveTextContent("通用智能体");
    expect(group).toHaveTextContent("专业角色");
    expect(group).toHaveTextContent("统筹角色");
    expect(group).toHaveTextContent("Research");
  });
});
