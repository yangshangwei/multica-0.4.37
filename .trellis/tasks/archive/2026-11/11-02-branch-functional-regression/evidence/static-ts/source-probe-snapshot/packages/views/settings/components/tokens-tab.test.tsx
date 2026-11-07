import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiClient } from "@multica/core/api/client";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";

const mocks = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), revoke: vi.fn(), toastError: vi.fn(), toastSuccess: vi.fn(),
}));
const authState = vi.hoisted(() => ({ user: { id: "user-1" } }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: {
    listPersonalAccessTokens: mocks.list,
    createPersonalAccessToken: mocks.create,
    revokePersonalAccessToken: mocks.revoke,
  },
}));
vi.mock("@multica/core/auth", async (importOriginal) => {
  return {
    ...await importOriginal<typeof import("@multica/core/auth")>(),
    useAuthStore: Object.assign(
      (selector: (value: typeof authState) => unknown) => selector(authState),
      { getState: () => authState },
    ),
  };
});
vi.mock("sonner", () => ({ toast: { error: mocks.toastError, success: mocks.toastSuccess } }));

import { TokensTab } from "./tokens-tab";

const token = {
  id: "token-1", name: "Test CLI", token_prefix: "mca_test",
  created_at: "2026-10-01T00:00:00Z", expires_at: null, last_used_at: null,
};
const credential = "mca_test_only_new_credential";
const clients: QueryClient[] = [];

function renderTab() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  clients.push(client);
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>
      <I18nProvider locale="en" resources={{ en: { settings: enSettings, common: enCommon } }}>
        {children}
      </I18nProvider>
    </QueryClientProvider>;
  }
  return { client, ...render(<TokensTab />, { wrapper: Wrapper }) };
}

async function createToken(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByText(token.name);
  await user.type(screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder }), "New CLI");
  await user.click(screen.getByRole("button", { name: "Create" }));
  return screen.findByRole("dialog", { name: enSettings.tokens.created_dialog.title });
}

beforeEach(() => {
  vi.resetAllMocks();
  authState.user = { id: "user-1" };
  mocks.list.mockResolvedValue([token]);
  mocks.create.mockResolvedValue({ ...token, token: credential });
  mocks.revoke.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  vi.unstubAllGlobals();
});

describe("TokensTab", () => {
  it.each(["Escape", "close button", "backdrop", "Done"] as const)(
    "requires acknowledgement on the %s dismissal path", async (path) => {
      const user = userEvent.setup();
      const { client } = renderTab();
      const dialog = await createToken(user);
      act(() => within(dialog).getByRole("button", { name: "Copy token" }).focus());
      const dismiss = async () => {
        if (path === "Escape") await user.keyboard("{Escape}");
        else if (path === "close button") await user.click(within(dialog).getByRole("button", { name: "Close" }));
        else if (path === "Done") await user.click(within(dialog).getByRole("button", { name: "Done" }));
        else {
          const backdrop = document.querySelector('[data-slot="dialog-overlay"]');
          if (!(backdrop instanceof HTMLElement)) throw new Error("Token dialog backdrop missing");
          await user.click(backdrop);
        }
      };

      await dismiss();
      expect(screen.getByRole("dialog")).toBe(dialog);
      expect(within(dialog).getByText(credential, { exact: true })).toBeVisible();
      expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain(credential);
      await user.click(within(dialog).getByRole("checkbox", { name: enSettings.tokens.created_dialog.confirm_stored }));
      await dismiss();
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    },
  );

  it("blocks repeated revocation and retains the same confirmation after failure for retry", async () => {
    const user = userEvent.setup();
    let rejectRevoke!: (error: Error) => void;
    mocks.revoke.mockReturnValue(new Promise((_resolve, reject) => { rejectRevoke = reject; }));
    renderTab();
    await user.click(await screen.findByRole("button", { name: "Revoke Test CLI" }));
    const dialog = screen.getByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", { name: "Revoke" });
    await user.click(confirm);
    expect(confirm).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    await user.keyboard("{Escape}");
    fireEvent.click(confirm);
    expect(mocks.revoke).toHaveBeenCalledTimes(1);
    await act(async () => rejectRevoke(new Error("Revocation unavailable")));
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(within(dialog).getByRole("alert")).toHaveTextContent("Revocation unavailable");
    expect(screen.getByText(token.name)).toBeInTheDocument();
    mocks.revoke.mockResolvedValue(undefined);
    mocks.list.mockResolvedValue([]);
    await user.click(within(dialog).getByRole("button", { name: "Revoke" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(mocks.revoke).toHaveBeenCalledTimes(2);
    expect(screen.queryByText(token.name)).not.toBeInTheDocument();
  });

  it("locks token creation controls while one request is pending", async () => {
    const user = userEvent.setup();
    let finish!: (value: unknown) => void;
    mocks.create.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    renderTab();
    await screen.findByText(token.name);
    const name = screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder });
    await user.type(name, "New CLI");
    const create = screen.getByRole("button", { name: "Create" });
    act(() => { fireEvent.click(create); fireEvent.click(create); });
    expect(name).toBeDisabled();
    await waitFor(() => expect(mocks.create).toHaveBeenCalledTimes(1));
    const expiry = screen.getByRole("combobox", { name: "Token expiry" });
    expect(expiry).toBeDisabled();
    await user.click(expiry);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    await act(async () => finish({ ...token, token: credential }));
    expect(await screen.findByRole("dialog")).toBeVisible();
  });

  it("keeps the one-time credential out of query and mutation caches throughout creation", async () => {
    const user = userEvent.setup();
    const { client } = renderTab();
    const cacheSnapshots: string[] = [];
    const recordCaches = () => {
      cacheSnapshots.push(JSON.stringify({
        queries: client.getQueryCache().getAll().map((query) => query.state),
        mutations: client.getMutationCache().getAll().map((mutation) => mutation.state),
      }));
    };
    const stopQueryObservation = client.getQueryCache().subscribe(recordCaches);
    const stopMutationObservation = client.getMutationCache().subscribe(recordCaches);

    try {
      const dialog = await createToken(user);
      expect(within(dialog).getByText(credential, { exact: true })).toBeVisible();
      await waitFor(() => expect(client.isFetching()).toBe(0));
      await waitFor(() => expect(client.isMutating()).toBe(0));
      expect(client.getMutationCache().getAll()).toHaveLength(1);
      expect(client.getMutationCache().getAll()[0]?.state).toMatchObject({
        status: "success",
        data: undefined,
        variables: { name: "New CLI", expires_in_days: 90 },
      });
      recordCaches();
      expect(cacheSnapshots.length).toBeGreaterThan(0);
      expect(cacheSnapshots.join("\n")).not.toContain(credential);
    } finally {
      stopQueryObservation();
      stopMutationObservation();
    }
  });

  it("recovers from a malformed real API list response without claiming an empty list", async () => {
    const user = userEvent.setup();
    const api = new ApiClient("https://tokens.test");
    vi.stubGlobal("fetch", vi.fn(async () => new Response("null", { status: 200 })));
    mocks.list.mockImplementation(() => api.listPersonalAccessTokens());
    renderTab();
    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(enSettings.tokens.empty)).not.toBeInTheDocument();
    mocks.list.mockResolvedValue([token]);
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(token.name)).toBeVisible();
  });

  it("keeps a malformed create result out of the success dialog and retains the draft", async () => {
    const user = userEvent.setup();
    const api = new ApiClient("https://tokens.test");
    vi.stubGlobal("fetch", vi.fn(async () => new Response("null", { status: 200 })));
    mocks.create.mockImplementation((request) => api.createPersonalAccessToken(request));
    renderTab();
    await screen.findByText(token.name);
    const name = screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder });
    await user.type(name, "New CLI");
    await user.click(screen.getByRole("button", { name: "Create" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(enSettings.tokens.create_response_invalid);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(name).toHaveValue("New CLI");
    expect(mocks.create).toHaveBeenCalledTimes(1);
    expect(mocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("retains token rows and the name input during a failed list refresh", async () => {
    const user = userEvent.setup();
    const { client } = renderTab();
    await screen.findByText(token.name);
    const name = screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder });
    await user.type(name, "Unsaved name");
    mocks.list.mockRejectedValue(new Error("List unavailable"));
    await act(async () => client.invalidateQueries());
    expect(await screen.findByRole("alert")).toHaveTextContent(enSettings.tokens.refresh_failed);
    expect(screen.getByText(token.name)).toBeVisible();
    expect(screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder })).toBe(name);
    expect(name).toHaveFocus();
    expect(name).toHaveValue("Unsaved name");
  });

  it("does not reveal the previous account's one-time token when the account changes", async () => {
    const user = userEvent.setup();
    const view = renderTab();
    await createToken(user);
    mocks.list.mockResolvedValue([{ ...token, id: "token-2", name: "Another account CLI" }]);
    authState.user = { id: "user-2" };
    view.rerender(<TokensTab />);
    await screen.findByText("Another account CLI");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.queryByText(credential, { exact: true })).not.toBeInTheDocument();
  });

  it("does not start an old account's list refresh after a late create response", async () => {
    const user = userEvent.setup();
    let finish!: (value: unknown) => void;
    mocks.create.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const view = renderTab();
    await screen.findByText(token.name);
    await user.type(screen.getByRole("textbox", { name: enSettings.tokens.name_placeholder }), "Old account CLI");
    await user.click(screen.getByRole("button", { name: "Create" }));
    mocks.list.mockResolvedValue([{ ...token, id: "token-2", name: "Another account CLI" }]);
    authState.user = { id: "user-2" };
    view.rerender(<TokensTab />);
    await screen.findByText("Another account CLI");
    const reads = mocks.list.mock.calls.length;
    await act(async () => finish({ ...token, token: credential }));
    expect(mocks.list).toHaveBeenCalledTimes(reads);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
