// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { SkillSummary } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSkills from "../../locales/en/skills.json";
import type { SkillRow } from "./skills-page";
import type { SkillActionsContext } from "./skill-list-actions";

const TEST_RESOURCES = { en: { common: enCommon, skills: enSkills } };

vi.mock("@multica/core/api", () => ({
  api: {
    refreshSkill: vi.fn(),
    updateSkill: vi.fn(),
    listLabels: vi.fn(),
    attachLabelToResource: vi.fn(),
    detachLabelFromResource: vi.fn(),
  },
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import type { Label } from "@multica/core/types";
import { api } from "@multica/core/api";
import { toast } from "sonner";
import {
  SkillBatchToolbar,
  UpdateSkillsDialog,
  deriveSkillLabelState,
  setSkillsLabel,
} from "./skill-list-actions";

const refreshSkill = vi.mocked(api.refreshSkill);
const updateSkill = vi.mocked(api.updateSkill);
const listLabels = vi.mocked(api.listLabels);
const attachLabelToResource = vi.mocked(api.attachLabelToResource);
const detachLabelFromResource = vi.mocked(api.detachLabelFromResource);

const bug: Label = { id: "l-bug", name: "bug", color: "#ef4444" } as Label;
const docs: Label = { id: "l-docs", name: "docs", color: "#3b82f6" } as Label;

function withLabels(row: SkillRow, labels: Label[]): SkillRow {
  return { ...row, labels };
}

function makeRow(id: string): SkillRow {
  const skill: SkillSummary = {
    id,
    workspace_id: "ws-1",
    name: `skill-${id}`,
    description: "",
    config: {
      origin: { type: "github", source_url: `https://github.com/acme/${id}` },
    },
    created_by: "user-1",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
  return {
    skill,
    labels: [],
    agents: [],
    creator: null,
    runtime: null,
    originType: "github",
    canEdit: true,
  };
}

const ctx: SkillActionsContext = {
  wsId: "ws-1",
  agents: [],
  currentUserId: "user-1",
  isAdmin: true,
};

function renderDialog(
  rows: SkillRow[],
  skippedCount = 0,
  onUpdated?: () => void,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <UpdateSkillsDialog
          rows={rows}
          skippedCount={skippedCount}
          ctx={ctx}
          open
          onOpenChange={() => {}}
          onUpdated={onUpdated}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("UpdateSkillsDialog", () => {
  it("summarizes the updatable selection and the skipped entries", async () => {
    renderDialog([makeRow("a"), makeRow("b")], 1);

    expect(
      await screen.findByText("2 skills will be updated from their sources"),
    ).toBeTruthy();
    // The never-updatable selection entries passed via skippedCount.
    expect(
      screen.getByText("1 can't be updated from a source — skipped"),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: /Update 2/ })).toBeTruthy();
  });

  it("omits the skipped line when the whole selection is updatable", async () => {
    renderDialog([makeRow("a")]);

    expect(
      await screen.findByText("1 skill will be updated from its source"),
    ).toBeTruthy();
    expect(screen.queryByText(/skipped/)).toBeNull();
  });

  it("refreshes every updatable skill and clears the selection on success", async () => {
    refreshSkill.mockResolvedValue({} as never);
    const onUpdated = vi.fn();
    renderDialog([makeRow("a"), makeRow("b")], 0, onUpdated);

    await userEvent.click(
      await screen.findByRole("button", { name: /Update 2/ }),
    );

    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
    expect(refreshSkill.mock.calls.map(([id]) => id)).toEqual(["a", "b"]);
    expect(toast.success).toHaveBeenCalled();
  });

  it("continues past a failing item and reports a partial toast", async () => {
    refreshSkill.mockImplementation((id: string) =>
      id === "a"
        ? Promise.reject(new Error("name conflict"))
        : Promise.resolve({} as never),
    );
    const onUpdated = vi.fn();
    renderDialog([makeRow("a"), makeRow("b")], 0, onUpdated);

    await userEvent.click(
      await screen.findByRole("button", { name: /Update 2/ }),
    );

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    // The failure on "a" must not strand "b".
    expect(refreshSkill.mock.calls.map(([id]) => id)).toEqual(["a", "b"]);
    // Partial success keeps the selection: onUpdated only fires on a clean run.
    expect(onUpdated).not.toHaveBeenCalled();
  });
});

function renderToolbar(rows: SkillRow[], onClear = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <SkillBatchToolbar rows={rows} ctx={ctx} onClear={onClear} />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { onClear };
}

describe("SkillBatchToolbar set category", () => {
  it("writes the category on each editable skill, keeping origin and icon", async () => {
    updateSkill.mockResolvedValue({} as never);
    const a = makeRow("a");
    a.skill.config = {
      ...a.skill.config,
      presentation: { category: "writing", icon: "bug" },
    };
    const b = makeRow("b");
    const locked = makeRow("c");
    locked.canEdit = false;
    const { onClear } = renderToolbar([a, b, locked]);

    await userEvent.click(screen.getByRole("button", { name: "Set category" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Development & integration" }));

    await waitFor(() => expect(onClear).toHaveBeenCalled());
    expect(updateSkill.mock.calls.map(([id]) => id)).toEqual(["a", "b"]);
    expect(updateSkill.mock.calls[0]?.[1]).toEqual({
      config: {
        origin: { type: "github", source_url: "https://github.com/acme/a" },
        presentation: { category: "engineering", icon: "bug" },
      },
    });
    expect(updateSkill.mock.calls[1]?.[1]).toEqual({
      config: {
        origin: { type: "github", source_url: "https://github.com/acme/b" },
        presentation: { category: "engineering" },
      },
    });
    expect(toast.success).toHaveBeenCalledWith("Category set on 2 skills");
  });

  it("reports a failure toast and keeps the selection when an update fails", async () => {
    updateSkill.mockImplementation((id: string) =>
      id === "a" ? Promise.reject(new Error("boom")) : Promise.resolve({} as never),
    );
    const { onClear } = renderToolbar([makeRow("a"), makeRow("b")]);
    await userEvent.click(screen.getByRole("button", { name: "Set category" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Data & automation" }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Failed to set category"));
    expect(updateSkill.mock.calls.map(([id]) => id)).toEqual(["a", "b"]);
    expect(onClear).not.toHaveBeenCalled();
  });

  it("disables the action when nothing selected is editable", () => {
    const locked = makeRow("a");
    locked.canEdit = false;
    renderToolbar([locked]);
    expect(screen.getByRole("button", { name: "Set category" })).toBeDisabled();
    expect(updateSkill).not.toHaveBeenCalled();
  });
});

describe("deriveSkillLabelState", () => {
  it("reports all / some / none across the editable rows only", () => {
    const a = withLabels(makeRow("a"), [bug]);
    const b = withLabels(makeRow("b"), [bug]);
    const c = withLabels(makeRow("c"), []);
    expect(deriveSkillLabelState([a, b], bug.id)).toBe("all");
    expect(deriveSkillLabelState([a, c], bug.id)).toBe("some");
    expect(deriveSkillLabelState([c], bug.id)).toBe("none");
  });

  it("ignores non-editable rows and returns none when nothing is editable", () => {
    const editableHas = withLabels(makeRow("a"), [bug]);
    const lockedMissing = withLabels(makeRow("b"), []);
    lockedMissing.canEdit = false;
    // The locked row lacks the label but must not drag the state to "some".
    expect(deriveSkillLabelState([editableHas, lockedMissing], bug.id)).toBe("all");

    const lockedOnly = withLabels(makeRow("c"), [bug]);
    lockedOnly.canEdit = false;
    expect(deriveSkillLabelState([lockedOnly], bug.id)).toBe("none");
  });
});

describe("setSkillsLabel", () => {
  it("adds the label to editable rows that lack it, skipping locked and already-tagged rows", async () => {
    attachLabelToResource.mockResolvedValue({} as never);
    const has = withLabels(makeRow("a"), [bug]);
    const missing = withLabels(makeRow("b"), []);
    const locked = withLabels(makeRow("c"), []);
    locked.canEdit = false;

    const result = await setSkillsLabel([has, missing, locked], bug.id);

    expect(result).toEqual({ action: "add", updated: 2, failed: 0, skipped: 1 });
    // Only the editable row missing the label triggers a network call.
    expect(attachLabelToResource.mock.calls).toEqual([["skill", "b", bug.id]]);
    expect(detachLabelFromResource).not.toHaveBeenCalled();
  });

  it("removes the label from every editable row when they all carry it", async () => {
    detachLabelFromResource.mockResolvedValue({} as never);
    const a = withLabels(makeRow("a"), [bug]);
    const b = withLabels(makeRow("b"), [bug]);

    const result = await setSkillsLabel([a, b], bug.id);

    expect(result).toEqual({ action: "remove", updated: 2, failed: 0, skipped: 0 });
    expect(detachLabelFromResource.mock.calls.map((c) => c[1])).toEqual(["a", "b"]);
    expect(attachLabelToResource).not.toHaveBeenCalled();
  });

  it("counts a per-item failure without stranding the rest", async () => {
    attachLabelToResource.mockImplementation((_t, id: string) =>
      id === "a" ? Promise.reject(new Error("boom")) : Promise.resolve({} as never),
    );
    const a = withLabels(makeRow("a"), []);
    const b = withLabels(makeRow("b"), []);

    const result = await setSkillsLabel([a, b], bug.id);

    expect(result).toEqual({ action: "add", updated: 1, failed: 1, skipped: 0 });
    expect(attachLabelToResource.mock.calls.map((c) => c[1])).toEqual(["a", "b"]);
  });
});

describe("SkillBatchToolbar manage labels", () => {
  it("adds a not-yet-applied label to the editable selection and reports success", async () => {
    listLabels.mockResolvedValue({ labels: [bug, docs] } as never);
    attachLabelToResource.mockResolvedValue({} as never);
    const a = withLabels(makeRow("a"), []);
    const b = withLabels(makeRow("b"), []);
    renderToolbar([a, b]);

    await userEvent.click(screen.getByRole("button", { name: "Manage labels" }));
    await userEvent.click(await screen.findByRole("button", { name: /bug/ }));

    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith('Added "bug" to 2 skills'),
    );
    expect(attachLabelToResource.mock.calls.map((c) => c[1])).toEqual(["a", "b"]);
  });

  it("marks an all-applied label pressed and removes it on click", async () => {
    listLabels.mockResolvedValue({ labels: [bug] } as never);
    detachLabelFromResource.mockResolvedValue({} as never);
    const a = withLabels(makeRow("a"), [bug]);
    const b = withLabels(makeRow("b"), [bug]);
    renderToolbar([a, b]);

    await userEvent.click(screen.getByRole("button", { name: "Manage labels" }));
    const row = await screen.findByRole("button", { name: /bug/ });
    expect(row).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(row);

    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith('Removed "bug" from 2 skills'),
    );
    expect(detachLabelFromResource.mock.calls.map((c) => c[1])).toEqual(["a", "b"]);
  });

  it("reports a partial failure toast", async () => {
    listLabels.mockResolvedValue({ labels: [bug] } as never);
    attachLabelToResource.mockImplementation((_t, id: string) =>
      id === "a" ? Promise.reject(new Error("boom")) : Promise.resolve({} as never),
    );
    renderToolbar([withLabels(makeRow("a"), []), withLabels(makeRow("b"), [])]);

    await userEvent.click(screen.getByRole("button", { name: "Manage labels" }));
    await userEvent.click(await screen.findByRole("button", { name: /bug/ }));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Updated 1, 1 failed"),
    );
  });

  it("disables manage labels when nothing selected is editable", () => {
    const locked = makeRow("a");
    locked.canEdit = false;
    renderToolbar([locked]);
    expect(screen.getByRole("button", { name: "Manage labels" })).toBeDisabled();
  });
});
