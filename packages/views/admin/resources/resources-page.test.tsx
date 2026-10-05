import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { NavigationProvider } from "../../navigation";
import { AdminResourcesPage } from "./resources-page";
import en from "../../locales/en/admin.json";
import zh from "../../locales/zh-Hans/admin.json";
const state = vi.hoisted(() => ({ role: "super_admin", userId: "actor", data: null as unknown, isError: false, error: null as unknown, refetch: vi.fn(), push: vi.fn() }));
vi.mock("@multica/core/admin", () => ({
  useAdminAccess: () => ({ status: "ready", identity: { userId: state.userId, organizationId: "org", role: state.role } }),
  adminApiScope: () => "server", useAdminResources: () => ({ data: state.data, isPending: false, isError: state.isError, error: state.error, refetch: state.refetch }),
  AdminResourceUnsupportedError: class extends Error {},
}));
vi.mock("./resource-editor", () => ({ ResourceEditor: ({ target }: { target?: { key: string } }) => <input aria-label="Draft sentinel" defaultValue={target?.key ?? ""} /> }));
const managed = { kind: "skill", key: "sample", name: "Sample", description: "Sample resource", source: "managed", state: "published", version: "r1", contentDigest: "d1", fileCount: 1, byteCount: 10, updatedAt: null, updatedBy: null };
function page(locale: "en" | "zh-Hans" = "en", params = "kind=skill&other=keep") {
  return <I18nProvider locale={locale} resources={{ en: { admin: en }, "zh-Hans": { admin: zh } }}><NavigationProvider value={{ pathname: "/admin/resources", searchParams: new URLSearchParams(params), hash: "#catalog", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: path => path }}><AdminResourcesPage /></NavigationProvider></I18nProvider>;
}
beforeEach(() => { vi.clearAllMocks(); state.role = "super_admin"; state.userId = "actor"; state.isError = false; state.error = null; state.data = { enabled: true, canPublish: true, limits: {}, items: [managed, { ...managed, key: "builtin", name: "Builtin", source: "builtin" }, { ...managed, key: "manual", name: "Manual", source: "deployment" }] }; });
it("keeps builtin and manual entries read-only and makes kind addressable without dropping URL context", () => {
  render(page());
  expect(screen.getAllByRole("button", { name: en.resources.update })).toHaveLength(1);
  expect(screen.getAllByRole("button", { name: en.resources.withdraw })).toHaveLength(1);
  fireEvent.click(screen.getByRole("tab", { name: en.resources.mcp }));
  expect(state.push).toHaveBeenCalledWith("/admin/resources?kind=mcp&other=keep#catalog");
});
it("does not offer write controls to an observer even if a stale capability says true", () => {
  state.role = "platform_observer";
  render(page("zh-Hans"));
  expect(screen.getByText(zh.resources.readonly)).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: zh.resources.update })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("Draft sentinel")).not.toBeInTheDocument();
});
it("explains disabled publishing and unsupported older servers", () => {
  state.data = { enabled: false, canPublish: false, limits: {}, items: [] };
  const view = render(page());
  expect(screen.getByText(en.resources.disabled)).toBeInTheDocument();
  state.data = undefined; state.isError = true; state.error = new ApiError("Missing", 404, "");
  view.rerender(page());
  expect(screen.getByRole("alert")).toHaveTextContent(en.resources.unsupported);
});
it("clears upload drafts on a principal change", () => {
  const view = render(page());
  fireEvent.click(screen.getByRole("button", { name: en.resources.new }));
  fireEvent.change(screen.getByLabelText("Draft sentinel"), { target: { value: "private file draft" } });
  state.userId = "different-actor";
  view.rerender(page());
  expect(screen.queryByDisplayValue("private file draft")).not.toBeInTheDocument();
});
