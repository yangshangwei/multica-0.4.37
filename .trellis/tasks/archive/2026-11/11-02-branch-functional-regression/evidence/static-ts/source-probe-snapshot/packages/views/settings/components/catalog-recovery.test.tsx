import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { labelKeys } from "@multica/core/labels";
import { propertyKeys } from "@multica/core/properties";
import { quickActionKeys } from "@multica/core/quick-actions";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type { IssueProperty, Label, QuickAction } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import en from "../../locales/en/settings.json";
import { LabelsTab } from "./labels-tab";
import { PropertiesTab } from "./properties-tab";
import { QuickActionsTab } from "./quick-actions-tab";

const api = vi.hoisted(() => ({
  listLabels: vi.fn(), listProperties: vi.fn(), listQuickActions: vi.fn(), listMembers: vi.fn(),
}));
const createProperty = vi.hoisted(() => vi.fn());
const auth = vi.hoisted(() => ({ user: { id: "u-1" } }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()), api,
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof auth) => unknown) => selector(auth),
    { getState: () => auth },
  ),
}));
vi.mock("@multica/core/labels", async (original) => ({
  ...(await original<typeof import("@multica/core/labels")>()),
  useCreateLabel: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateLabel: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteLabel: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@multica/core/properties", async (original) => ({
  ...(await original<typeof import("@multica/core/properties")>()),
  useCreateProperty: () => ({ mutate: createProperty, isPending: false }),
  useUpdateProperty: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@multica/core/quick-actions", async (original) => ({
  ...(await original<typeof import("@multica/core/quick-actions")>()),
  useCreateQuickAction: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateQuickAction: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteQuickAction: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("../../autopilots/components/pickers/agent-picker", () => ({ AgentPicker: () => null }));

const base = { id: "entry-1", workspace_id: "ws-1", created_at: "2026-10-01", updated_at: "2026-10-01" };
const label: Label = { ...base, name: "Release label", color: "#336699", resource_type: "issue" };
const property: IssueProperty = { ...base, name: "Release property", type: "select", config: {}, position: 1, archived: false };
const action: QuickAction = {
  ...base, name: "Release action", description: "", assignee_type: "agent", assignee_id: "agent-1",
  prompt: "Review the release", visibility: "public", status: "active", last_used_at: null,
  use_count: 0, created_by_id: "u-1", target_name: "Reviewer", target_public: true, target_missing: false,
};
const cases = [
  {
    name: "labels", Component: LabelsTab, read: api.listLabels,
    key: labelKeys.list("ws-1"), cached: { labels: [label], total: 1 }, empty: { labels: [], total: 0 },
    loading: en.labels.loading, emptyText: en.labels.empty.replace("{{scope}}", en.labels.scopes.issue),
    create: en.labels.new_label, error: "Could not load labels", refresh: "Could not refresh labels",
  },
  {
    name: "properties", Component: PropertiesTab, read: api.listProperties,
    key: propertyKeys.list("ws-1", true), cached: { properties: [property], total: 1 }, empty: { properties: [], total: 0 },
    loading: en.properties.loading, emptyText: en.properties.empty,
    create: en.properties.new_property, error: "Could not load properties", refresh: "Could not refresh properties",
  },
  {
    name: "quick actions", Component: QuickActionsTab, read: api.listQuickActions,
    key: quickActionKeys.list("ws-1", true), cached: { quick_actions: [action], total: 1 }, empty: { quick_actions: [], total: 0 },
    loading: en.quick_actions.loading, emptyText: en.quick_actions.empty_title,
    create: en.quick_actions.add, error: "Could not load quick actions", refresh: "Could not refresh quick actions",
  },
];
let client: QueryClient;

beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const members = [{ user_id: "u-1", role: "owner" }];
  client.setQueryData(workspaceKeys.members("ws-1"), members);
  api.listMembers.mockResolvedValue(members);
});
afterEach(() => { cleanup(); client.clear(); });

describe.each(cases)("$name settings recovery", ({ Component, read, key, cached, empty, loading, emptyText, create, error, refresh }) => {
  it("distinguishes pending/failure from an empty response and keeps cold Retry stable", async () => {
    let resolve!: (response: unknown) => void;
    read.mockRejectedValueOnce(new Error("offline"))
      .mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    renderWithI18n(<QueryClientProvider client={client}><Component /></QueryClientProvider>);
    expect(screen.getByText(loading)).toHaveAttribute("role", "status");
    expect(screen.queryByText(emptyText)).toBeNull();
    expect(await screen.findByRole("alert")).toHaveTextContent(error);
    expect(screen.queryByText(emptyText)).toBeNull();
    const retry = screen.getByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toBeDisabled());
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    fireEvent.click(retry);
    expect(read).toHaveBeenCalledTimes(2);
    await act(async () => resolve(empty));
    expect(await screen.findByText(emptyText)).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("preserves the open editor, focus and local draft when a cached refresh fails", async () => {
    let reject!: (error: Error) => void;
    client.setQueryData(key, cached);
    read.mockImplementation(() => new Promise((_resolve, fail) => { reject = fail; }));
    renderWithI18n(<QueryClientProvider client={client}><Component /></QueryClientProvider>);
    fireEvent.click(screen.getByRole("button", { name: create }));
    const name = await screen.findByRole("textbox", { name: "Name" });
    fireEvent.change(name, { target: { value: "Keep this draft" } });
    name.focus();
    await act(async () => reject(new Error("offline")));
    expect(await screen.findByRole("alert", { hidden: true })).toHaveTextContent(refresh);
    expect(screen.getByRole("textbox", { name: "Name" })).toBe(name);
    expect(name).toHaveValue("Keep this draft");
    expect(name).toHaveFocus();
  });
});

it("names the property type selector and saves all twelve drafted options", async () => {
  api.listProperties.mockResolvedValue({ properties: [], total: 0 });
  renderWithI18n(<QueryClientProvider client={client}><PropertiesTab /></QueryClientProvider>);
  await screen.findByText(en.properties.empty);
  fireEvent.click(screen.getByRole("button", { name: en.properties.new_property }));
  expect(await screen.findByRole("combobox", { name: "Type" })).toBeVisible();
  const name = screen.getByRole("textbox", { name: "Name" });
  fireEvent.change(name, { target: { value: "Release stage" } });
  for (let index = 1; index < 12; index++) fireEvent.click(screen.getByRole("button", { name: "Add option" }));
  const options = screen.getAllByPlaceholderText(en.properties.editor.option_name_placeholder);
  expect(options).toHaveLength(12);
  options.forEach((input, index) => fireEvent.change(input, { target: { value: `Stage ${index + 1}` } }));
  fireEvent.click(screen.getByRole("button", { name: en.properties.editor.save }));
  expect(createProperty).toHaveBeenCalledWith(expect.objectContaining({
    name: "Release stage",
    config: { options: options.map((_input, index) => expect.objectContaining({ name: `Stage ${index + 1}` })) },
  }), expect.any(Object));
});

it("keeps failed property permissions distinct from confirmed member access", async () => {
  client.removeQueries({ queryKey: workspaceKeys.members("ws-1") });
  api.listMembers.mockRejectedValue(new Error("offline"));
  api.listProperties.mockResolvedValue({ properties: [], total: 0 });
  renderWithI18n(<QueryClientProvider client={client}><PropertiesTab /></QueryClientProvider>);
  expect(await screen.findByRole("alert")).toHaveTextContent("Could not load workspace permissions");
  expect(screen.queryByText(en.properties.editor.admin_hint)).toBeNull();
  expect(screen.queryByRole("button", { name: en.properties.new_property })).toBeNull();
});
