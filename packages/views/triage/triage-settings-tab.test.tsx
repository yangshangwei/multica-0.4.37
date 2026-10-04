import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriageSettingsTab } from "./triage-settings-tab";
const state = vi.hoisted(() => ({
  role: "admin",
  pending: 0,
  enabled: false,
  supported: true,
  error: null as Error | null,
  mutate: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof en) => string) => selector(en) }),
}));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({ triage: () => "/acme/triage" }),
}));
vi.mock("../navigation", () => ({
  AppLink: ({
    href,
    children,
  }: {
    href: string;
    children: React.ReactNode;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: state.role }),
}));
vi.mock("./triage-fields", () => ({
  AcceptanceStatusSelect: () => null,
  ReviewerSelect: () => null,
  TriageSelect: () => null,
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useUpdateTriageSettings: () => ({ mutateAsync: state.mutate }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({ queryKey }: { queryKey: string[] }) =>
    queryKey[2] === "counts"
      ? { data: { pending: state.pending, ready: 0, snoozed: state.pending } }
      : {
          data: {
            supported: state.supported,
            enabled: state.enabled,
            acceptance_status: "backlog",
            require_priority: false,
            responsibility_mode: "none",
            responsibility_member_id: null,
            revision: 2,
          },
          isError: !!state.error,
          error: state.error,
        },
}));
beforeEach(() => {
  state.role = "admin";
  state.pending = 0;
  state.enabled = false;
  state.supported = true;
  state.error = null;
  state.mutate.mockReset();
});
it("lets an admin explicitly enable triage with a revision-protected save", () => {
  state.mutate.mockResolvedValue({});
  render(<TriageSettingsTab />);
  fireEvent.click(screen.getByRole("switch", { name: en.enable }));
  fireEvent.click(screen.getByRole("button", { name: en.save }));
  expect(state.mutate).toHaveBeenCalledWith(
    expect.objectContaining({ enabled: true, expected_revision: 2 }),
  );
});
it("shows member settings read-only", () => {
  state.role = "member";
  render(<TriageSettingsTab />);
  expect(screen.getByRole("switch", { name: en.enable })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  expect(screen.getByRole("button", { name: en.save })).toBeDisabled();
  expect(screen.getByText(en.admin_only)).toBeVisible();
});
it("cannot disable while every unresolved task is snoozed", () => {
  state.enabled = true;
  state.pending = 3;
  render(<TriageSettingsTab />);
  expect(screen.getByRole("switch", { name: en.enable })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  expect(screen.getByText(en.pending_block)).toBeVisible();
});
it("does not mistake a network failure for unsupported triage", () => {
  state.error = new Error("Service unavailable");
  render(<TriageSettingsTab />);
  expect(screen.getByRole("alert")).toHaveTextContent("Service unavailable");
  expect(screen.queryByText(en.unsupported)).not.toBeInTheDocument();
});
