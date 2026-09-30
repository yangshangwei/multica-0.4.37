// @vitest-environment jsdom

import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";
import { fireEvent, screen } from "@testing-library/react";
import type { Agent, Label, SkillSummary } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { SkillCard } from "./skill-card";
import type { PresentedSkillRow } from "./skills-page";

vi.mock("@multica/ui/components/common/actor-avatar", () => ({
  ActorAvatar: ({ name }: { name: string }) => <span data-testid="avatar">{name}</span>,
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: () => null,
}));
vi.mock("./skill-list-actions", () => ({
  SkillRowActions: () => <button type="button" aria-label="Actions" />,
}));
vi.mock("@multica/core/workspace/avatar-url", () => ({
  resolvePublicFileUrl: (u: string | null) => u,
}));

const adapter: NavigationAdapter = {
  push: vi.fn(), replace: vi.fn(), back: vi.fn(),
  pathname: "/acme/skills", searchParams: new URLSearchParams(), hash: "",
  getShareableUrl: (path) => path,
  openInNewTab: vi.fn(),
};
beforeEach(() => vi.clearAllMocks());
function renderCard(element: React.ReactElement) {
  return renderWithI18n(<NavigationProvider value={adapter}>{element}</NavigationProvider>);
}

const labels = ["a", "b", "c", "d", "e"].map(
  (name, i) => ({ id: `l${i}`, name, color: "#3b82f6" }) as Label,
);

const skill: SkillSummary = {
  id: "skill-1",
  workspace_id: "ws-1",
  name: "lint-fixer",
  description: "Fixes lint findings before review.",
  config: {},
  created_by: "user-1",
  created_at: "2026-07-28T18:11:37Z",
  updated_at: "2026-07-28T18:14:40Z",
  labels,
};

const agents = ["Alpha", "Beta", "Gamma", "Delta"].map(
  (name, i) => ({ id: `a${i}`, name, avatar_url: null }) as unknown as Agent,
);

function makeRow(overrides: Partial<PresentedSkillRow> = {}): PresentedSkillRow {
  return {
    skill,
    presentation: {
      name: skill.name,
      description: skill.description,
      searchNames: [skill.name],
      searchText: skill.name,
      isBuiltin: false,
    },
    meta: { category: "engineering", icon: "bug" },
    labels,
    agents,
    creator: null,
    runtime: null,
    originType: "github",
    canEdit: false,
    ...overrides,
  };
}

const ctx = { wsId: "ws-1", agents: [], currentUserId: "user-1", isAdmin: false };

describe("SkillCard", () => {
  it("opens the title with Enter exactly once without bubbling to the card", async () => {
    const user = userEvent.setup();
    const onCardClick = vi.fn();
    renderCard(<SkillCard href="/acme/skills/skill-1" row={makeRow()} ctx={ctx} selected={false} onToggleSelected={() => {}} linkProps={{ onClick: onCardClick }} />);
    const link = screen.getByRole("link", { name: "lint-fixer" });
    await user.tab();
    expect(link).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(adapter.push).toHaveBeenCalledExactlyOnceWith("/acme/skills/skill-1");
    expect(onCardClick).not.toHaveBeenCalled();
  });

  it("exposes a named checked control and identifies manual origin as metadata", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    const onCardClick = vi.fn();
    renderCard(<SkillCard href="/acme/skills/skill-1" row={makeRow({ originType: "manual" })} ctx={ctx} selected onToggleSelected={onToggle} linkProps={{ onClick: onCardClick }} />);
    const checkbox = screen.getByRole("checkbox", { name: "Select lint-fixer" });
    expect(checkbox).toBeChecked();
    checkbox.focus();
    await user.keyboard(" ");
    expect(onToggle).toHaveBeenCalledOnce();
    expect(onCardClick).not.toHaveBeenCalled();
    expect(screen.getByText("Created manually")).toBeInTheDocument();
  });

  it("uses the localized display title for the complete-title hint", () => {
    const row = makeRow();
    row.presentation.name = "发布后与 Canary 验证";
    renderCard(<SkillCard href="/acme/skills/skill-1" row={row} ctx={ctx} selected={false} onToggleSelected={() => {}} linkProps={{}} />);
    expect(screen.getByRole("link", { name: row.presentation.name })).toHaveAttribute("title", row.presentation.name);
  });

  it("renders the category tile, name, label chips with overflow, agent count and origin", () => {
    renderCard(
      <SkillCard href="/acme/skills/skill-1" row={makeRow()} ctx={ctx} selected={false} onToggleSelected={() => {}} linkProps={{}} />,
    );
    const card = screen.getByTestId("skill-card");
    expect(card.querySelector("[data-category=engineering]")).not.toBeNull();
    expect(screen.getByText("lint-fixer")).toBeInTheDocument();
    expect(screen.getByText("Fixes lint findings before review.")).toBeInTheDocument();
    // Workspace labels render as colored chips (LabelChip), capped at three.
    expect(screen.getByTitle("a")).toHaveStyle({ backgroundColor: "#3b82f6" });
    expect(screen.getByText("c")).toBeInTheDocument();
    expect(screen.queryByText("d")).not.toBeInTheDocument();
    expect(screen.getByText("+2")).toBeInTheDocument();
    expect(screen.getAllByTestId("avatar")).toHaveLength(3);
    expect(screen.getByText("+1")).toBeInTheDocument();
    expect(screen.getByText("4 agents")).toBeInTheDocument();
    expect(card.querySelector("[data-origin=github]")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Actions" })).toBeInTheDocument();
  });

  it("shows the unused label without avatars", () => {
    renderCard(
      <SkillCard href="/acme/skills/skill-1" row={makeRow({ agents: [] })} ctx={ctx} selected={false} onToggleSelected={() => {}} linkProps={{}} />,
    );
    expect(screen.getByText("Unused")).toBeInTheDocument();
    expect(screen.queryAllByTestId("avatar")).toHaveLength(0);
  });

  it("toggles selection from the checkbox without triggering the card link", () => {
    const onToggle = vi.fn();
    const onCardClick = vi.fn();
    renderCard(
      <SkillCard href="/acme/skills/skill-1"
        row={makeRow()}
        ctx={ctx}
        selected={false}
        onToggleSelected={onToggle}
        linkProps={{ onClick: onCardClick }}
      />,
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "Select lint-fixer" }));
    expect(onToggle).toHaveBeenCalledOnce();
    expect(onCardClick).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("Fixes lint findings before review."));
    expect(onCardClick).toHaveBeenCalledOnce();
  });

  it("exposes data-selected when checked", () => {
    renderCard(
      <SkillCard href="/acme/skills/skill-1" row={makeRow()} ctx={ctx} selected onToggleSelected={() => {}} linkProps={{}} />,
    );
    expect(screen.getByTestId("skill-card")).toHaveAttribute("data-selected");
  });
});

describe("SkillCard labels", () => {
  it("renders no chips and no overflow badge for a summary from an older server", () => {
    renderCard(
      <SkillCard href="/acme/skills/skill-1"
        row={makeRow({ labels: [], agents: [] })}
        ctx={ctx}
        selected={false}
        onToggleSelected={() => {}}
        linkProps={{}}
      />,
    );
    expect(screen.queryByText("a")).not.toBeInTheDocument();
    expect(screen.queryByText(/^\+\d+$/)).not.toBeInTheDocument();
  });
});
