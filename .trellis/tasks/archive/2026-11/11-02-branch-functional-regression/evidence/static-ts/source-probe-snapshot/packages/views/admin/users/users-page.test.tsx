import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import { AdminUsersPage } from "./users-page";
import en from "../../locales/en/admin.json";

const state = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  refetch: vi.fn(),
  items: [] as { id: string; name: string; username: string; status: string; platformRole: null; workspaceCount: number; createdAt: string }[],
}));
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  adminApiScope: () => "server",
  useAdminAccess: () => ({ identity: { userId: "actor", organizationId: "org", role: "super_admin" } }),
  useAdminUsers: () => ({ isPending: false, isError: false, refetch: state.refetch, data: {
    items: state.items, nextCursor: "next-page", asOf: "2026-10-01T00:00:00Z", dataQuality: "partial",
    registration: { enabled: true, approvalRequired: false },
  } }),
}));

function mount(search = "", administrators = false) {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{ push: state.push, replace: state.replace, back: vi.fn(), pathname: administrators ? "/admin/administrators" : "/admin/users", searchParams: new URLSearchParams(search), hash: "", getShareableUrl: path => path }}>
      <AdminUsersPage administrators={administrators} />
    </NavigationProvider>
  </I18nProvider>);
}
beforeEach(() => { vi.clearAllMocks(); state.items = []; });

describe("account directory partial scans", () => {
  it("keeps continuation available without declaring a complete empty result", () => {
    render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
      <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/admin/users", searchParams: new URLSearchParams("status=disabled"), hash: "", getShareableUrl: (path) => path }}>
        <AdminUsersPage />
      </NavigationProvider>
    </I18nProvider>);
    expect(screen.getByText("These results are incomplete. Continue to the next page to find more matching accounts.")).toBeVisible();
    expect(screen.queryByText("No accounts match these filters")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next page" })).toBeEnabled();
  });
});

describe("account filter and investigation context", () => {
  it("accepts a same-day UTC range and includes the final calendar day", () => {
    mount("cursor=old&timezone=Asia%2FShanghai");
    fireEvent.change(screen.getByLabelText(/Created from/), { target: { value: "2026-10-01" } });
    fireEvent.change(screen.getByLabelText(/Created (before|through)/), { target: { value: "2026-10-01" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
    const params = new URL(state.replace.mock.calls[0]![0], "https://test.invalid").searchParams;
    expect(new Date(params.get("time_from")!).toISOString()).toBe("2026-10-01T00:00:00.000Z");
    expect(new Date(params.get("time_to")!).toISOString()).toBe("2026-10-02T00:00:00.000Z");
    expect(params.has("cursor")).toBe(false);
    expect(params.get("timezone")).toBe("Asia/Shanghai");
  });
  it("identifies a reversed date range and focuses the invalid field without navigating", async () => {
    mount();
    fireEvent.change(screen.getByLabelText(/Created from/), { target: { value: "2026-10-03" } });
    fireEvent.change(screen.getByLabelText(/Created (before|through)/), { target: { value: "2026-10-01" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
    const invalid = document.querySelector<HTMLElement>('[aria-invalid="true"]');
    expect(invalid).not.toBeNull();
    expect(invalid).toHaveAccessibleDescription();
    await waitFor(() => expect(invalid).toHaveFocus());
    expect(state.replace).not.toHaveBeenCalled();
  });
  it("shows the inclusive final date when restoring the URL's exclusive end boundary", () => {
    mount("time_from=2026-10-01T00%3A00%3A00Z&time_to=2026-10-02T00%3A00%3A00Z");
    expect(screen.getByLabelText(/Created (before|through)/)).toHaveValue("2026-10-01");
  });
  it("preserves administrator filters and timezone when opening account details", () => {
    state.items = [{ id: "account", name: "Example account", username: "example", status: "active", platformRole: null, workspaceCount: 2, createdAt: "2026-10-01T00:00:00Z" }];
    mount("role=platform_observer&cursor=next&timezone=Asia%2FShanghai", true);
    const href = screen.getAllByRole("link", { name: "Example account" })[0]!.getAttribute("href")!;
    const detail = new URL(href, "https://test.invalid");
    expect(detail.searchParams.get("timezone")).toBe("Asia/Shanghai");
    expect(detail.searchParams.get("return_to")).toBe("/admin/administrators?role=platform_observer&cursor=next&timezone=Asia%2FShanghai");
  });
  it("offers a compact account summary with status and reachable secondary details", () => {
    state.items = [{ id: "account", name: "Example account", username: "example", status: "active", platformRole: null, workspaceCount: 2, createdAt: "2026-10-01T00:00:00Z" }];
    mount();
    const summary = screen.getByRole("list", { name: "Accounts" });
    expect(within(summary).getByText("Active")).toBeInTheDocument();
    expect(within(summary).getByRole("link", { name: "Example account" })).toBeInTheDocument();
    const details = summary.querySelector("details")!;
    expect(details).not.toHaveAttribute("open");
    fireEvent.click(within(summary).getByText("More account details"));
    expect(details).toHaveAttribute("open");
    expect(within(details).getByText("account")).toBeVisible();
    expect(within(details).getByText(/UTC/)).toBeVisible();
  });
});

it("resets active account filters while retaining the administrator directory", () => {
  mount("role=platform_observer&status=disabled&cursor=next", true);
  fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
  expect(state.replace).toHaveBeenCalledWith("/admin/administrators");
});
it("clears invalid account drafts when resetting an already unfiltered URL", () => {
  mount();
  fireEvent.change(screen.getByLabelText(/time\s?zone/i), { target: { value: "Mars/Phobos" } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(screen.getByLabelText(/time\s?zone/i)).toHaveAttribute("aria-invalid", "true");
  fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
  expect(screen.getByLabelText(/time\s?zone/i)).not.toHaveAttribute("aria-invalid");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
it("refreshes current account data and returns paginated refreshes to the first page", () => {
  const first = mount("status=disabled");
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  expect(state.refetch).toHaveBeenCalledOnce();
  first.unmount();
  mount("status=disabled&cursor=next");
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  expect(state.push).toHaveBeenCalledWith("/admin/users?status=disabled");
  expect(state.refetch).toHaveBeenCalledOnce();
});
