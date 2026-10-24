import { afterEach, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationRunDisclosure } from "./run-records";

const issue = parseCollaborationIssue(collaborationExecutionIssueFixture)!;
const run = { ...issue.execution!.runs[0]!, progress_state: "preparing" as const, forced_disposition: null };
const group = issue.execution!.groups.find((value) => value.id === run.group_id)!;
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });

const preparation = {
  version: 1,
  attachments: [{ id: "00000000-0000-4000-8000-000000000201", version: "7", name: "A deliberately long recorded attachment name.txt", size_bytes: 123 }],
  git: { object_format: "sha1", commit_oid: "a".repeat(40) },
  stages: [{ seq: "3", stage: "attachments", state: "prepared" },
    { seq: "4", stage: "git", state: "failed", code: "git_unavailable" }],
};

function recordedPage(summary: unknown = preparation) {
  return { protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: issue.id, resource_owner_user_id: issue.id,
      execution_workspace_id: issue.id, grant_id: issue.id, provider: null, model: null, account_source: null },
    recording: { status: "not_recorded", inputs_recorded: true }, preparation: summary, usage: null,
    items: [{ id: "input:one", kind: "input", command_id: run.command_id, author_user_id: issue.id,
      body: "Frozen input", attachment_ids: [] }], next_cursor: null };
}

function mount(locale: "en" | "zh-Hans" = "en", open = false) {
  setApiInstance(new ApiClient("https://server.example"));
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "",
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}>
    <ul><CollaborationRunDisclosure scope={collaborationScope(issue.id)} issue={issue} run={run} group={group} selectedRunId={open ? run.id : null} /></ul>
  </NavigationProvider></QueryClientProvider>, { locale });
}

// Core owns the malformed summary/correlation matrix; these exercise the
// protected disclosure, factual wording, compact details and localized reason.
it("lazily shows exact prepared files and failed Git preparation without asserting run completion", async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(recordedPage())));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(fetch).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: `View records for ${run.id}` }));
  const summary = await screen.findByRole("region", { name: "Run preparation" });
  await userEvent.click(within(summary).getByText("1 recorded attachment version"));
  expect(within(summary).getByText(preparation.attachments[0]!.name)).toBeVisible();
  expect(within(summary).getByText("Version 7 · 123 bytes")).toBeVisible();
  expect(within(summary).getByText(preparation.attachments[0]!.id)).toBeVisible();
  expect(within(summary).getByText(preparation.git.commit_oid)).toBeInTheDocument();
  expect(within(summary).getByText("Prepared")).toBeInTheDocument();
  expect(within(summary).getByText("The fixed Git commit could not be prepared.")).toBeInTheDocument();
  expect(within(summary).getAllByText("Not recorded")).toHaveLength(2);
  expect(screen.getByText("No business result recorded")).toBeInTheDocument();
  expect(within(summary).queryByText(/terminated|succeeded|not run/i)).not.toBeInTheDocument();
});

it("shows missing legacy preparation as not recorded", async () => {
  const page = recordedPage();
  const { preparation: _preparation, ...legacy } = page;
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(legacy))));
  mount("en", true);
  const summary = await screen.findByRole("region", { name: "Run preparation" });
  expect(within(summary).getByText("Not recorded")).toBeInTheDocument();
  expect(within(summary).queryByText(/not run|failed|prepared/i)).not.toBeInTheDocument();
});

it("renders the recorded stage reason in Chinese and leaves missing facts explicit", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(recordedPage()))));
  mount("zh-Hans", true);
  const summary = await screen.findByRole("region", { name: "运行准备" });
  expect(within(summary).getByText("无法准备固定 Git 提交。")).toBeInTheDocument();
  expect(within(summary).getByText("准备失败")).toBeInTheDocument();
  expect(within(summary).getByText("1 个已记录的附件版本")).toBeInTheDocument();
  expect(within(summary).queryByText("git_unavailable")).not.toBeInTheDocument();
});

it("removes preparation content when a later records page denies current access", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ ...recordedPage(), next_cursor: "page-two" })))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found" }), { status: 404 })));
  mount("en", true);
  expect(await screen.findByRole("region", { name: "Run preparation" })).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Load more records" }));
  expect(await screen.findByText("You no longer have access to this issue.")).toBeInTheDocument();
  expect(screen.queryByRole("region", { name: "Run preparation" })).not.toBeInTheDocument();
  expect(screen.queryByText(preparation.git.commit_oid)).not.toBeInTheDocument();
});
