import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { IterationsPage } from "./iteration-page";
vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "10000000-0000-4000-8000-000000000001",
}));
vi.mock("@multica/core/auth", () => ({ useAuthStore: (select: (state: { user: { id: string } }) => unknown) => select({ user: { id: "actor" } }) }));
vi.mock("@multica/core/api", () => ({
  api: { getBaseUrl: () => "", getIterationCapabilities: async () => null },
}));
vi.mock("../i18n", () => ({
  useLocale: () => "en",
  useT: () => ({
    t: (fn: (x: unknown) => string) =>
      fn({
        iterations: {
          title: "Iterations",
          unsupported: "Iterations are unavailable on this server.",
          loading: "Loading",
        },
      }),
  }),
}));
describe("iteration capability gate", () => {
  it("shows unavailable without mounting write controls on an older server", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <IterationsPage />
      </QueryClientProvider>,
    );
    expect(
      await screen.findByText("Iterations are unavailable on this server."),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Create iteration" }),
    ).toBeNull();
  });
});
