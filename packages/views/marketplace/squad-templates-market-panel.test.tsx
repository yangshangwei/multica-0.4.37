import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithI18n } from "../test/i18n";
import { SquadTemplatesMarketPanel } from "./squad-templates-market-panel";

const state = vi.hoisted(() => ({
  query: {
    data: [],
    isError: false,
    isPending: false,
    isSuccess: true,
    refetch: vi.fn(),
  },
  enabled: true,
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { enabled?: boolean }) => {
    state.enabled = options.enabled ?? true;
    return state.query;
  },
}));
vi.mock("@multica/core/workspace/queries", () => ({
  squadListOptions: (workspaceId: string) => ({ queryKey: ["squads", workspaceId] }),
}));
vi.mock("../squads/components/builtin-squad-catalog", () => ({
  BuiltinSquadCatalog: ({ squads, active }: { squads: unknown[]; active: boolean }) => (
    <div data-testid="catalog" data-count={squads.length} data-active={active} />
  ),
}));

beforeEach(() => {
  vi.clearAllMocks();
  state.query = {
    data: [],
    isError: false,
    isPending: false,
    isSuccess: true,
    refetch: vi.fn(),
  };
  state.enabled = true;
});

describe("SquadTemplatesMarketPanel", () => {
  it("loads the saved-squad inventory only while active and renders the catalog on success", () => {
    renderWithI18n(<SquadTemplatesMarketPanel workspaceId="ws-1" active />);

    expect(state.enabled).toBe(true);
    expect(screen.getByTestId("catalog")).toHaveAttribute("data-count", "0");
    expect(screen.getByTestId("catalog")).toHaveAttribute("data-active", "true");
  });

  it("shows loading feedback instead of treating an unresolved inventory as empty", () => {
    state.query = { ...state.query, isPending: true, isSuccess: false };
    renderWithI18n(<SquadTemplatesMarketPanel workspaceId="ws-1" active />);

    expect(screen.getByRole("status")).toHaveTextContent("Loading saved squads...");
    expect(screen.queryByTestId("catalog")).not.toBeInTheDocument();
  });

  it("shows retry feedback when the saved-squad inventory fails", () => {
    state.query = { ...state.query, isError: true, isSuccess: false };
    renderWithI18n(<SquadTemplatesMarketPanel workspaceId="ws-1" active />);

    expect(screen.getByRole("alert")).toHaveTextContent("Could not load saved squads.");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.query.refetch).toHaveBeenCalledOnce();
    expect(screen.queryByTestId("catalog")).not.toBeInTheDocument();
  });

  it("does not fetch or show a false empty catalog while inactive and uncached", () => {
    state.query = { ...state.query, isPending: true, isSuccess: false };
    renderWithI18n(<SquadTemplatesMarketPanel workspaceId="ws-1" active={false} />);

    expect(state.enabled).toBe(false);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.queryByTestId("catalog")).not.toBeInTheDocument();
  });
});
