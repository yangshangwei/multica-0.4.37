import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import type { Squad, SquadTemplate } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { BuiltinSquadCatalog } from "./builtin-squad-catalog";

const mocks = vi.hoisted(() => ({
  templates: [] as SquadTemplate[],
  pending: false,
  failed: false,
  retry: vi.fn(),
}));

vi.mock("../../agents/create/use-role-templates", () => ({
  useSquadTemplates: () => ({
    data: mocks.templates,
    isPending: mocks.pending,
    isError: mocks.failed,
    refetch: mocks.retry,
  }),
}));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
// Project configuration behavior is covered in use-squad-for-project-dialog.test.tsx.
// Keep this suite focused on handing the original template to that workflow.
vi.mock("../../projects/components/use-squad-for-project-dialog", () => ({
  UseSquadForProjectDialog: ({ template, onClose }: { template: SquadTemplate; onClose: () => void }) => (
    <div role="dialog" aria-label={template.title}>
      <p>{template.description}</p>
      <p>{template.instructions}</p>
      <button onClick={onClose}>Close</button>
    </div>
  ),
}));

const TEMPLATE: SquadTemplate = {
  key: "feature-delivery",
  version: 1,
  name: "Feature Delivery Squad",
  title: "Feature delivery",
  description: "The full server description remains available for project setup.",
  avatar_emoji: "🚀",
  instructions: "Coordinate each stage of delivery.",
  leader: { template_key: "delivery-lead", title: "Delivery lead", name: "Lead", role: "Coordinate", autonomy_level: "coordinator", avatar_emoji: "" },
  members: [
    { template_key: "implementer", title: "Implementer", name: "Developer", role: "Implement", autonomy_level: "contributor", avatar_emoji: "" },
    { template_key: "qa-engineer", title: "QA engineer", name: "QA", role: "Verify", autonomy_level: "contributor", avatar_emoji: "" },
  ],
};

function squad(id: string, name: string, overrides: Partial<Squad> = {}): Squad {
  return {
    id, name, workspace_id: "ws-1", description: "Custom squad description", instructions: "Custom instructions",
    avatar_url: null, leader_id: "leader-1", creator_id: "user-1", created_at: "", updated_at: "",
    archived_at: null, archived_by: null, template_key: TEMPLATE.key, ...overrides,
  };
}

function renderCatalog(squads: Squad[] = []) {
  const navigation: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads",
    searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
  };
  return {
    ...renderWithI18n(<NavigationProvider value={navigation}><BuiltinSquadCatalog squads={squads} /></NavigationProvider>),
    navigation,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.templates = [TEMPLATE];
  mocks.pending = false;
  mocks.failed = false;
});

describe("BuiltinSquadCatalog", () => {
  it("shows templates immediately with use-case copy and the full role roster", () => {
    renderCatalog();
    const catalog = screen.getByRole("region", { name: "Squad templates" });
    expect(within(catalog).getByRole("heading", { name: "Squad templates" })).toBeVisible();
    const template = within(catalog).getByRole("listitem", { name: TEMPLATE.title });
    expect(within(template).getByText("Take a feature from requirements through implementation and review.")).toBeVisible();
    expect(within(template).getByText("3 members")).toBeVisible();
    expect(within(template).getByText("Delivery lead · Implementer · QA engineer")).toBeVisible();
    expect(within(template).getByText("🚀")).toBeVisible();
    expect(within(template).getByRole("button", { name: "Apply to project" })).toBeVisible();
    expect(catalog.querySelector("[aria-expanded]")).toBeNull();
  });

  it("links every active instance by template identity, even after it is renamed", () => {
    const { navigation } = renderCatalog([
      squad("payments", "Payments delivery"),
      squad("onboarding", "Onboarding delivery"),
      squad("archived", "Archived delivery", { archived_at: "2026-09-24" }),
      squad("custom", TEMPLATE.title, { template_key: undefined }),
    ]);
    const payments = screen.getByRole("link", { name: "Payments delivery" });
    expect(payments).toHaveAttribute("href", "/acme/squads/payments");
    expect(screen.getByRole("link", { name: "Onboarding delivery" })).toHaveAttribute("href", "/acme/squads/onboarding");
    expect(screen.getAllByRole("link")).toHaveLength(2);
    fireEvent.click(payments);
    expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/payments");
  });

  it("opens project setup with the unchanged template and closes it independently", () => {
    renderCatalog();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply to project" }));
    const dialog = screen.getByRole("dialog", { name: TEMPLATE.title });
    expect(within(dialog).getByText(TEMPLATE.description)).toBeVisible();
    expect(within(dialog).getByText(TEMPLATE.instructions)).toBeVisible();
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps unknown templates usable and falls back to their server copy", () => {
    mocks.templates = [{ ...TEMPLATE, key: "future-template", title: "", name: "Future workflow", members: [] }];
    renderCatalog();
    expect(screen.getByRole("heading", { name: "Future workflow" })).toBeVisible();
    expect(screen.getByText(TEMPLATE.description)).toBeVisible();
    expect(screen.getByText("1 member")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Apply to project" }));
    expect(screen.getByRole("dialog")).toBeVisible();
  });

  it("announces loading before templates are available", () => {
    mocks.pending = true;
    mocks.templates = [];
    renderCatalog();
    expect(screen.getByRole("region", { name: "Squad templates" })).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("status")).toHaveTextContent("Loading squad templates...");
    expect(screen.queryByRole("button", { name: "Apply to project" })).not.toBeInTheDocument();
  });

  it("exposes an error and retry instead of cached template actions", () => {
    mocks.failed = true;
    renderCatalog();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load squad templates.");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.retry).toHaveBeenCalledOnce();
    expect(screen.queryByRole("button", { name: "Apply to project" })).not.toBeInTheDocument();
  });

  it("shows the empty state when the response has no identifiable templates", () => {
    mocks.templates = [{ ...TEMPLATE, key: "" }];
    renderCatalog();
    expect(screen.getByText("No squad templates are available.")).toBeVisible();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
