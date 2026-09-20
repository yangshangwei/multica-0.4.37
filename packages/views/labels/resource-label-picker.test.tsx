// @vitest-environment jsdom

import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Label } from "@multica/core/types";
import { renderWithI18n } from "../test/i18n";
import { ResourceLabelPicker } from "./resource-label-picker";

// The picker has two modes that share one popover. Draft mode (no
// `resourceId`) is what the create-skill dialog relies on: it must never read
// or write the resource's labels, only report ids through
// `onSelectedIdsChange`. Attached mode keeps hitting attach/detach.

const mockApi = vi.hoisted(() => ({
  listLabels: vi.fn(),
  listLabelsForResource: vi.fn(),
  attachLabelToResource: vi.fn(),
  detachLabelFromResource: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({ api: mockApi }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

const quality: Label = {
  id: "lbl-1",
  workspace_id: "ws-1",
  resource_type: "skill",
  name: "quality",
  color: "#22c55e",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};
const review: Label = { ...quality, id: "lbl-2", name: "review", color: "#3b82f6" };

function popup(): HTMLElement {
  const el = document.querySelector<HTMLElement>('[data-slot="popover-content"]');
  if (!el) throw new Error("popover content not mounted");
  return el;
}

function renderPicker(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithI18n(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

// Holds the draft selection like the create dialog does, so the test drives
// the real select -> chip -> deselect loop instead of a frozen prop.
function DraftHarness({ initial = [] }: { initial?: string[] }) {
  const [ids, setIds] = useState<string[]>(initial);
  return (
    <ResourceLabelPicker
      resourceType="skill"
      selectedIds={ids}
      onSelectedIdsChange={setIds}
      canEdit
    />
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockApi.listLabels.mockResolvedValue({ labels: [quality, review], total: 2 });
  mockApi.listLabelsForResource.mockResolvedValue({ labels: [quality] });
  mockApi.attachLabelToResource.mockResolvedValue({ labels: [quality, review] });
  mockApi.detachLabelFromResource.mockResolvedValue({ labels: [] });
});

describe("ResourceLabelPicker draft mode", () => {
  it("selects and deselects through onSelectedIdsChange without touching the resource endpoints", async () => {
    renderPicker(<DraftHarness />);

    fireEvent.click(screen.getByRole("button", { name: "Add labels" }));
    fireEvent.click(await within(popup()).findByRole("button", { name: "quality" }));

    // The chip now sits in the trigger, rendered from the workspace catalog.
    await screen.findByTitle("quality");
    expect(screen.getByTitle("quality")).toHaveStyle({ backgroundColor: "#22c55e" });

    fireEvent.click(within(popup()).getByRole("button", { name: "quality" }));
    await waitFor(() => expect(screen.queryByTitle("quality")).toBeNull());

    expect(mockApi.listLabels).toHaveBeenCalledWith("skill");
    expect(mockApi.listLabelsForResource).not.toHaveBeenCalled();
    expect(mockApi.attachLabelToResource).not.toHaveBeenCalled();
    expect(mockApi.detachLabelFromResource).not.toHaveBeenCalled();
  });

  it("drops a selected id whose label no longer exists in the catalog", async () => {
    renderPicker(<DraftHarness initial={["lbl-1", "lbl-gone"]} />);
    await screen.findByTitle("quality");
    expect(screen.queryByTitle("lbl-gone")).toBeNull();
    expect(screen.getAllByTitle(/./).filter((el) => el.tagName === "SPAN")).toHaveLength(1);
  });

  it("shows the settings hint when the workspace has no skill labels yet", async () => {
    mockApi.listLabels.mockResolvedValue({ labels: [], total: 0 });
    renderPicker(<DraftHarness />);
    fireEvent.click(screen.getByRole("button", { name: "Add labels" }));
    expect(
      await within(popup()).findByText("Create labels in Settings first"),
    ).toBeInTheDocument();
  });
});

describe("ResourceLabelPicker attached mode", () => {
  it("reads the resource's labels and attaches a new one on click", async () => {
    renderPicker(
      <ResourceLabelPicker resourceType="skill" resourceId="skill-1" canEdit />,
    );
    await screen.findByTitle("quality");
    expect(mockApi.listLabelsForResource).toHaveBeenCalledWith("skill", "skill-1");

    fireEvent.click(screen.getByRole("button", { name: "quality" }));
    fireEvent.click(await within(popup()).findByRole("button", { name: "review" }));
    await waitFor(() =>
      expect(mockApi.attachLabelToResource).toHaveBeenCalledWith("skill", "skill-1", "lbl-2"),
    );
  });

  it("renders read-only chips without a trigger when the viewer cannot edit", async () => {
    renderPicker(
      <ResourceLabelPicker resourceType="skill" resourceId="skill-1" canEdit={false} />,
    );
    await screen.findByTitle("quality");
    expect(screen.queryByRole("button")).toBeNull();
  });
});
