import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { NavigationProvider } from "../../navigation";
import { AdminResourcesPage } from "./resources-page";
import en from "../../locales/en/admin.json";

const state = vi.hoisted(() => ({ version: "r1", mutate: vi.fn(), refetch: vi.fn(), reset: vi.fn() }));
const resource = { kind: "skill", key: "sample", name: "Sample", description: "", source: "managed", state: "published", contentDigest: "d1", fileCount: 1, byteCount: 10, updatedAt: null, updatedBy: null };
vi.mock("@multica/core/admin", () => ({
  adminApiScope: () => "server",
  useAdminAccess: () => ({ status: "ready", identity: { userId: "actor", organizationId: "org", role: "super_admin" } }),
  useAdminResources: () => ({ data: { enabled: true, canPublish: true, limits: {}, items: [{ ...resource, version: state.version }] }, isPending: false, isError: false, refetch: state.refetch }),
  useAdminResourcePreview: () => ({ mutateAsync: vi.fn(), reset: state.reset }),
  useAdminResourceMutation: () => ({ mutateAsync: state.mutate, reset: state.reset }),
  useAdminResourceLookup: () => ({ mutateAsync: vi.fn(), reset: state.reset }),
  AdminResourceUnsupportedError: class extends Error {},
  AdminResourceUncertainError: class extends Error {},
}));
function page() {
  return <I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/resources", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: path => path }}><AdminResourcesPage /></NavigationProvider></I18nProvider>;
}
beforeEach(() => { vi.resetAllMocks(); state.version = "r1"; });

it("pins withdrawal confirmation while polling and requires explicit reselection before using the new revision", async () => {
  const view = render(page());
  fireEvent.click(screen.getByRole("button", { name: en.resources.withdraw }));
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "Remove the reviewed revision" } });
  state.version = "r2";
  view.rerender(page());
  expect(screen.getByText("Target: sample · current revision: r1")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: en.resources.confirmWithdraw })).toBeDisabled();
  expect(screen.getByRole("alert")).toHaveTextContent("Cancel and select Withdraw again");
  expect(state.mutate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: en.resources.cancel }));
  fireEvent.click(screen.getByRole("button", { name: en.resources.withdraw }));
  expect(screen.getByText("Target: sample · current revision: r2")).toBeInTheDocument();
  expect(screen.getByLabelText(en.resources.reason)).toHaveValue("");
  state.mutate.mockResolvedValue({ operationId: "operation", replayed: false, resource: { ...resource, version: "r3", state: "withdrawn" } });
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "Remove the newly reviewed revision" } });
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmWithdraw }));
  await waitFor(() => expect(state.mutate).toHaveBeenCalledWith(expect.objectContaining({ action: "withdraw", expectedVersion: "r2" })));
});

it("sends only the selected revision if another publication races before the next poll", async () => {
  state.mutate.mockRejectedValue(new ApiError("Changed", 409, "", { code: "resource_changed" }));
  render(page());
  fireEvent.click(screen.getByRole("button", { name: en.resources.withdraw }));
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "Remove reviewed revision" } });
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmWithdraw }));
  await screen.findByRole("alert");
  expect(state.mutate).toHaveBeenCalledWith(expect.objectContaining({ expectedVersion: "r1" }));
  expect(state.mutate).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("button", { name: en.resources.confirmWithdraw })).toBeDisabled();
});
