import { expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderWithI18n } from "../test/i18n";
import { CollaborationRunHandoff } from "./run-handoff";

const id = "00000000-0000-4000-8000-000000000001";
const run = "00000000-0000-4000-8000-000000000002";
const query = vi.hoisted(() => ({ data: undefined as unknown, isPending: false, error: null, refetch: vi.fn() }));
vi.mock("@tanstack/react-query", async (original) => ({ ...await original<typeof import("@tanstack/react-query")>(), useQuery: () => query }));
vi.mock("../navigation", () => ({ AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));

it("shows current handoff source evidence and explicit unknown reasons without claiming a delivered answer", () => {
  const page = { total: 0, returned: 0, truncated: false, items: [] };
  query.data = { mode: "live", authority: { handler_id: id }, assignment: { role: "member" }, memory: { availability: "absent" },
    inputs: page, results: { ...page, items: [{ run_id: run, result_state: "absent" }] }, wakeup_causes: { ...page, completeness: "unknown" }, missing: [],
    sources: [{ kind: "delegation", id, trust: "admitted_assignment", availability: "available", content: "Review the retained evidence", truncated: false, run_id: run }] };
  const client = new QueryClient();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationRunHandoff scope={{ server: "https://handoff.example", apiScope: "https://handoff.example", userId: id }} issueId={id} runId={run}
    handler={{ user_id: id, name: "Current issue handler", avatar_url: "", external: true }} /></QueryClientProvider>);
  expect(screen.getByText("Current issue handler")).toBeInTheDocument();
  expect(screen.getByText(id)).toBeInTheDocument();
  expect(screen.getByText("Current source context.")).toBeInTheDocument();
  expect(screen.getByText("Earlier reasons were not recorded.")).toBeInTheDocument();
  expect(screen.getByText("No result recorded")).toBeInTheDocument();
  expect(screen.getByText("Review the retained evidence")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Open source run" })).toHaveAttribute("href", expect.stringContaining(run));
  client.clear();
});

it("does not label a historical handler with another current account's name", () => {
  const page = { total: 0, returned: 0, truncated: false, items: [] };
  query.data = { mode: "snapshot", authority: { handler_id: id }, assignment: { role: "agent" }, memory: { availability: "absent" },
    inputs: page, results: page, wakeup_causes: { ...page, completeness: "unknown" }, missing: [], sources: [] };
  const client = new QueryClient();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationRunHandoff scope={{ server: "https://handoff.example", apiScope: "https://handoff.example", userId: id }} issueId={id} runId={run}
    handler={{ user_id: run, name: "Another handler", avatar_url: "", external: false }} /></QueryClientProvider>);
  expect(screen.queryByText("Another handler")).not.toBeInTheDocument();
  expect(screen.getByText("Unavailable author")).toBeInTheDocument();
  expect(screen.getByText(id)).toBeInTheDocument();
  client.clear();
});

it("keeps unsupported handoff information unavailable", () => {
  query.data = undefined;
  const client = new QueryClient();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationRunHandoff scope={{ server: "https://handoff.example", apiScope: "https://handoff.example", userId: id }} issueId={id} runId={run} /></QueryClientProvider>);
  expect(screen.getByText("Handoff context is unavailable on this server.")).toBeInTheDocument();
  client.clear();
});

it("labels a live read's launch base and its later latest memory separately", () => {
  // Regression for audit ENV-1: the live read's latest version was shown as
  // "Started from", contradicting the run row's recorded base.
  const page = { total: 0, returned: 0, truncated: false, items: [] };
  const live = { mode: "live", authority: {}, assignment: { role: "agent" }, inputs: page, results: page, wakeup_causes: { ...page, completeness: "recorded" }, missing: [], sources: [] };
  const render = (memory: Record<string, string>) => {
    query.data = { ...live, memory: { availability: "available", sha256: "a".repeat(64), ...memory } };
    const client = new QueryClient();
    const view = renderWithI18n(<QueryClientProvider client={client}><CollaborationRunHandoff scope={{ server: "https://handoff.example", apiScope: "https://handoff.example", userId: id }} issueId={id} runId={run} /></QueryClientProvider>);
    return { client, view };
  };
  let { client, view } = render({ version: "4", base_version: "3" });
  expect(screen.getByRole("link", { name: "Started from v3" })).toHaveAttribute("href", expect.stringContaining("3"));
  expect(screen.getByRole("link", { name: "Latest is v4" })).toBeInTheDocument();
  expect(screen.queryByText("Started from v4")).not.toBeInTheDocument();
  client.clear();
  view.unmount();

  ({ client, view } = render({ version: "4" }));
  expect(screen.queryByText(/Started from/)).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Latest is v4" })).toBeInTheDocument();
  client.clear();
  view.unmount();

  ({ client, view } = render({ version: "2", base_version: "0" }));
  expect(screen.getByText("Started from an empty document")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Latest is v2" })).toBeInTheDocument();
  client.clear();
  view.unmount();

  ({ client } = render({ version: "3", base_version: "3" }));
  expect(screen.getByRole("link", { name: "Started from v3" })).toBeInTheDocument();
  expect(screen.queryByText(/Latest is/)).not.toBeInTheDocument();
  client.clear();
});
