import type { ReactNode } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { IterationParticipation } from "../../iterations/iteration-participation";
import { alpha, source, ws } from "../../iterations/test-fixtures";
import projects from "../../locales/en/projects.json";
import { IssueDetailRoute } from "./issue-detail-route";

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "10000000-0000-4000-8000-000000000001",
}));
vi.mock("@multica/core/paths", async () => {
  const actual = await vi.importActual<typeof import("@multica/core/paths")>("@multica/core/paths");
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("../../i18n", () => ({
  useLocale: () => "en",
  useT: () => ({ t: (selector: (translations: typeof projects) => string) => selector(projects) }),
}));
vi.mock("./issue-detail", async () => {
  const { IterationParticipation } = await import("../../iterations/iteration-participation");
  return {
    // The browser suite owns the complete detail and Next RSC boundary. This
    // leaf replacement keeps the real route, provider, canonical query and
    // disclosure/query wiring without mounting unrelated editor/modal APIs.
    IssueDetail: (props: {
      issueId: string;
      iterationHistoryOpen?: boolean;
      onIterationHistoryOpenChange?: (open: boolean) => void;
    }) => {
      const disclosureProps = { open: props.iterationHistoryOpen, onOpenChange: props.onIterationHistoryOpenChange };
      return <IterationParticipation wsId="10000000-0000-4000-8000-000000000001" issueId={props.issueId} {...disclosureProps} />;
    },
    IssueDetailSkeleton: () => <div>Loading issue</div>,
    IssueNotFound: () => <div>Issue not found</div>,
    IssueLoadError: () => <div>Issue unavailable</div>,
  };
});

const history = { items: [{ ...source, name: "Metadata cycle", status: "completed" as const }], next_cursor: null };

function clientFixture(listIterations: () => Promise<typeof history>) {
  const getIssue = vi.fn().mockResolvedValue(alpha);
  const getIterationCapabilities = vi.fn().mockResolvedValue({ supported: true });
  const readHistory = vi.fn(listIterations);
  setApiInstance({ getIssue, getIterationCapabilities, listIterations: readHistory } as unknown as ApiClient);
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false, gcTime: Infinity } } });
  return { client, readHistory };
}

function surface(client: QueryClient, pathname: string, replace: NavigationAdapter["replace"], children: ReactNode) {
  return (
    <QueryClientProvider client={client}>
      <NavigationProvider value={{ pathname, replace, push: vi.fn(), back: vi.fn(), hash: "", searchParams: new URLSearchParams(), getShareableUrl: (path) => path }}>
        {children}
      </NavigationProvider>
    </QueryClientProvider>
  );
}

describe("routed iteration participation disclosure", () => {
  it("retains an expansion made after replace started across the same issue route remount", async () => {
    const user = userEvent.setup();
    let release!: (value: typeof history) => void;
    const heldHistory = new Promise<typeof history>((done) => { release = done; });
    const { client, readHistory } = clientFixture(() => heldHistory);
    const replace = vi.fn();
    const view = render(surface(client, `/acme/issues/${alpha.id}`, replace, <IssueDetailRoute key={alpha.id} routeId={alpha.id} />));
    try {
      const summary = (await screen.findByText("Iteration history")).closest("summary")!;
      await waitFor(() => expect(replace).toHaveBeenCalledWith(`/acme/issues/${alpha.identifier}`, { preserveView: true }));
      await user.click(summary);
      await waitFor(() => expect(readHistory).toHaveBeenCalled());
      expect(summary.closest("details")).toHaveProperty("open", true);

      // Commit the new page segment only after the user expanded the old one;
      // NavigationProvider and the actual QueryClient retain their identity.
      view.rerender(surface(client, `/acme/issues/${alpha.identifier}`, replace, <IssueDetailRoute key={alpha.identifier} routeId={alpha.identifier} />));
      await waitFor(() => expect(screen.getByText("Iteration history").closest("details")).toHaveProperty("open", true));
      expect(screen.queryByRole("link", { name: "Metadata cycle" })).not.toBeInTheDocument();
      await act(async () => { release(history); });
      expect(await screen.findByRole("link", { name: "Metadata cycle" })).toBeVisible();
    } finally {
      release(history);
      view.unmount();
      client.clear();
    }
  });

  it("keeps a non-routed embedded disclosure local and reads only after expansion", async () => {
    const user = userEvent.setup();
    const { client, readHistory } = clientFixture(async () => history);
    const view = render(surface(client, "/acme/inbox", vi.fn(), <IterationParticipation wsId={ws} issueId={alpha.id} />));
    try {
      const summary = (await screen.findByText("Iteration history")).closest("summary")!;
      expect(summary.closest("details")).toHaveProperty("open", false);
      expect(readHistory).not.toHaveBeenCalled();
      await user.click(summary);
      expect(await screen.findByRole("link", { name: "Metadata cycle" })).toBeVisible();
      view.rerender(surface(client, "/acme/settings", vi.fn(), <IterationParticipation wsId={ws} issueId={alpha.id} />));
      expect(summary.closest("details")).toHaveProperty("open", true);
    } finally {
      view.unmount();
      client.clear();
    }
  });
});
