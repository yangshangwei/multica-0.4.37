import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationActivity } from "./activity";

// Core information-schema/queries tests own exact ordering, cursor, denial and
// denominator matrices. This suite owns lazy shared history and paging controls.
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
const id = collaborationIssueFixture.id;
const repairId = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const olderId = "00000000-0000-4000-8000-000000000004";
const eventId = "00000000-0000-4000-8000-000000000005";
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
afterEach(() => { client.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const provenance = { state: "legacy_unlinked", event_id: null, event_sequence: null };
const missing = { state: "unavailable", event_id: null, event_sequence: null, reason_code: "information_context_unavailable" };
const repair = { kind: "information_repair", handoff_id: repairId, created_at: "2026-11-18T08:00:00.000900Z",
  provenance: { state: "linked", event_id: eventId, event_sequence: "9" }, actor_user_id: id, from_user_id: id, to_user_id: id,
  note: "Actual repair reason", previous_status: "blocked", resolution: "open", prior_active_request_id: requestId,
  restored_status: "todo", target_user_id: id, basis: { kind: "human_selected" }, superseded_request_count: "2" };
const request = { kind: "information_request", handoff_id: requestId, created_at: "2026-11-18T08:00:00.000800Z", provenance,
  actor_user_id: id, from_user_id: null, to_user_id: id, note: "Original legacy reason", previous_status: "todo", resolution: "superseded",
  requester_user_id: null, parent_request_id: null, block: { protocol_version: 1, state: "legacy" }, superseded_by_repair_handoff_id: repairId };
const older = { kind: "information_request", handoff_id: olderId, created_at: "2026-11-18T08:00:00.000001Z", provenance: missing };
const envelope = { protocol_version: 1, issue_id: id, collaboration_revision: "2", lifecycle_epoch: "1" };
const upper = { handoff_id: repairId, created_at: repair.created_at };

it("loads real history pages and retains legacy or unavailable affected IDs until the full denominator is read", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  let failAffected = true;
  const fetch = vi.fn(async (input: string) => {
    const url = new URL(input);
    if (url.pathname.endsWith("/activity")) return json({ issue_id: id, items: [], next_cursor: null });
    if (url.pathname.endsWith("/superseded-requests")) {
      if (url.searchParams.has("cursor") && failAffected) { failAffected = false; return json({ error: "Temporarily unavailable" }, 503); }
      return json({ ...envelope, repair_handoff_id: repairId, superseded_request_count: "2", upper_handoff: { handoff_id: requestId, created_at: request.created_at },
        items: [{ request_id: url.searchParams.has("cursor") ? olderId : requestId, request_event_id: null,
          provenance: url.searchParams.has("cursor") ? missing : provenance, resolution: "superseded", superseded_by_repair_handoff_id: repairId }],
        next_cursor: url.searchParams.has("cursor") ? null : "affected-page-2" });
    }
    if (url.pathname.endsWith("/handoffs")) return json({ ...envelope, upper_handoff: upper,
      items: url.searchParams.has("cursor") ? [older] : [repair, request], next_cursor: url.searchParams.has("cursor") ? null : "history-page-2" });
    if (url.pathname.endsWith(`/handoffs/${requestId}`)) return json({ protocol_version: 1, issue_id: id, item: request });
    throw new Error(`Unexpected history test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationActivity scope={collaborationScope(id)} destination={{ view: "participating", issueId: id, commentId: null }} /></QueryClientProvider>);
  await screen.findByText("No activity yet.");
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith("/handoffs"))).toBe(false);
  const user = userEvent.setup();
  await user.click(screen.getByText("Information handoff history", { selector: "summary" }));
  const history = screen.getByRole("region", { name: "Information handoff history" });
  await within(history).findByText("Actual repair reason");
  expect(within(history).getByText("Original legacy reason")).toBeInTheDocument();
  await user.click(within(history).getByRole("button", { name: "Load more information handoffs" }));
  await waitFor(() => expect(history.querySelector(`[data-information-handoff-id="${olderId}"]`)).toBeInTheDocument());
  expect(within(history).getByText("This record's details are unavailable. Its identity is retained.")).toBeInTheDocument();
  await user.click(within(history).getByText("Affected requests", { selector: "summary" }));
  const affected = within(history).getByRole("region", { name: "Affected requests" });
  await within(affected).findByText("1 of 2 affected requests loaded");
  expect(within(affected).queryByText("All 2 affected request identities are loaded.")).toBeNull();
  await user.click(within(affected).getByRole("button", { name: "Load more affected requests" }));
  await within(affected).findByRole("alert");
  expect(within(affected).getByText("1 of 2 affected requests loaded")).toBeInTheDocument();
  expect(within(affected).queryByText("All 2 affected request identities are loaded.")).toBeNull();
  await user.click(within(affected).getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(within(affected).queryByRole("alert")).toBeNull());
  await user.click(within(affected).getByRole("button", { name: "Load more affected requests" }));
  await within(affected).findByText("All 2 affected request identities are loaded.");
  expect(affected.querySelector(`[data-information-request-id="${requestId}"]`)).toBeInTheDocument();
  expect(affected.querySelector(`[data-information-request-id="${olderId}"]`)).toBeInTheDocument();
  await user.click(within(affected.querySelector<HTMLElement>(`[data-information-request-id="${requestId}"]`)!).getByRole("button", { name: "View the original record" }));
  const detail = within(history).getByRole("region", { name: "View the original record" });
  await within(detail).findByText("Original legacy reason");
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith(`/handoffs/${requestId}`))).toBe(true);
});
