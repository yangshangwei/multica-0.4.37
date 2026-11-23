import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { ProjectIterations } from "./project-iterations";
import { alpha, beta, issuePage, source, targetA, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";

vi.mock("../navigation", () => ({ AppLink: ({ href, children, ...props }: React.ComponentProps<"a">) => <a href={href} {...props}>{children}</a> }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ iterationDetail: (id: string) => `/iterations/${id}`, issueDetail: (id: string) => `/issues/${id}` }) }));
vi.mock("@multica/core/api", async (original) => ({
  ...await original<typeof import("@multica/core/api")>(),
  api: {
    getBaseUrl: () => "test", getSessionScope: () => "session",
    getIterationCapabilities: vi.fn(), listIterations: vi.fn(), getIterationIssues: vi.fn(),
  },
}));

const translations = createInstance();
const clients: QueryClient[] = [];
beforeAll(async () => { await translations.init({ lng: "en", fallbackLng: "en", interpolation: { escapeValue: false }, resources: { en: { projects } } }); });
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({ workspace_id: ws, schema_version: 1, supported: true, enabled: true, manual: true, atomic_handoff: true });
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA], next_cursor: null });
  vi.mocked(api.getIterationIssues).mockImplementation(async (_ws, id, params) => ({ ...issuePage(params?.cursor ? [beta] : [alpha]), iteration_id: id, total: 2, next_cursor: params?.cursor ? null : "second-page" }));
});
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); });

function mount(projectId = "project-a") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const node = (id: string) => <I18nextProvider i18n={translations}><QueryClientProvider client={client}><ProjectIterations wsId={ws} projectId={id} /></QueryClientProvider></I18nextProvider>;
  const view = render(node(projectId));
  return { user: userEvent.setup(), rerender: (id: string) => view.rerender(node(id)) };
}

async function selectIteration(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole("combobox", { name: "Iterations" }));
  await user.click(await screen.findByRole("option", { name }));
}

describe("project iteration task navigation", () => {
  it("returns to the previous task page and clears cursor history when selection changes", async () => {
    const { user } = mount();
    await user.click(await screen.findByText("Iterations", { selector: "summary" }));
    await selectIteration(user, source.name);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.queryByRole("button", { name: "Previous page" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` });
    await user.click(screen.getByRole("button", { name: "Previous page" }));
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await selectIteration(user, targetA.name);
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, targetA.id, { project_id: "project-a" }, expect.any(Object)));
    expect(screen.queryByRole("button", { name: "Previous page" })).not.toBeInTheDocument();
  });

  it("does not reuse the previous project's selection or cursor after identity changes", async () => {
    const { user, rerender } = mount();
    await user.click(await screen.findByText("Iterations", { selector: "summary" }));
    await selectIteration(user, source.name);
    await screen.findByRole("button", { name: "Next page" });
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` });
    rerender("project-b");
    await user.click(await screen.findByText("Iterations", { selector: "summary" }));
    await selectIteration(user, source.name);
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { project_id: "project-b" }, expect.any(Object)));
    expect(screen.queryByRole("button", { name: "Previous page" })).not.toBeInTheDocument();
  });
});
