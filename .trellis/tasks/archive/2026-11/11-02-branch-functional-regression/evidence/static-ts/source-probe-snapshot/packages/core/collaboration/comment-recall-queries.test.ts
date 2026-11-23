// @vitest-environment node
import { expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions } from "./content-queries";

it("keeps comment projection budgets in query identity", () => {
  const client = new QueryClient();
  const scope = { server: "https://server.example", apiScope: "https://server.example", userId: "00000000-0000-4000-8000-000000000001" };
  const issue = "00000000-0000-4000-8000-000000000002";
  const normal = collaborationCommentsOptions(client, scope, issue);
  expect(normal.queryKey).toEqual([...collaborationCommentsOptions(client, scope, issue, { limit: 50, maxBytes: 65536 }).queryKey]);
  expect(normal.queryKey).not.toEqual(collaborationCommentsOptions(client, scope, issue, { maxBytes: 4096 }).queryKey);
  expect(normal.queryKey).not.toEqual(collaborationCommentsOptions(client, scope, issue, { limit: 1 }).queryKey);
  client.clear();
});
