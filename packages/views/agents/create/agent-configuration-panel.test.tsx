// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen } from "@testing-library/react";
import type { AgentDraft } from "@multica/core/agents";
import zhSkills from "../../locales/zh-Hans/skills.json";
import { renderWithI18n } from "../../test/i18n";
import { AgentConfigurationPanel } from "./agent-configuration-panel";

const draft: AgentDraft = {
  name: "Reviewer",
  description: "",
  instructions: "",
  conversationStarters: [],
  avatarUrl: null,
  runtimeId: "",
  model: "",
  thinkingLevel: "",
  serviceTier: "",
  skillIds: new Set(),
  permissionScope: "private",
  memberIds: new Set(),
  teamIds: new Set(),
};

describe("AgentConfigurationPanel role skills", () => {
  it("localizes trusted template skill labels without changing template instructions or the draft", () => {
    const onChange = vi.fn();
    const instructions = "# Reviewer\n\nRead the diff before reporting findings.";
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    renderWithI18n(
      <QueryClientProvider client={queryClient}>
        <AgentConfigurationPanel
          draft={draft}
          onChange={onChange}
          runtimes={[]}
          runtimesLoading={false}
          members={[]}
          currentUserId="user-1"
          nameError={null}
          onNameChange={vi.fn()}
          roleTemplate={{
            instructions,
            skillNames: ["multica-code-review", "multica-security-review", "future-role-skill"],
          }}
        />
      </QueryClientProvider>,
      { locale: "zh-Hans" },
    );

    expect(screen.getByText("代码审查")).toHaveAttribute(
      "title",
      zhSkills.builtin_role_skills["multica-code-review"].description,
    );
    expect(screen.getByText("安全审查")).toBeInTheDocument();
    expect(screen.getByText("future-role-skill")).toBeInTheDocument();
    expect(screen.getByText(/Read the diff before reporting findings\./).textContent).toBe(instructions);
    expect(onChange).not.toHaveBeenCalled();
  });
});
