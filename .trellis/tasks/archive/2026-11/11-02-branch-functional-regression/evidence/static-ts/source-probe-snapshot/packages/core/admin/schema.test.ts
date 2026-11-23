// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAdminMe } from "./schema";

const response = {
  user_id: "11111111-1111-4111-8111-111111111111",
  organization_id: "22222222-2222-4222-8222-222222222222",
  role: "super_admin",
  supported: true,
  allowed_actions: ["view", "manage_roles"],
};

describe("admin identity boundary", () => {
  it("converts a validated identity to camelCase", () => {
    expect(parseAdminMe(response)).toEqual({
      userId: response.user_id,
      organizationId: response.organization_id,
      role: "super_admin",
      supported: true,
      allowedActions: ["view", "manage_roles"],
    });
  });

  it.each([
    null,
    {},
    { ...response, role: "owner" },
    { ...response, supported: "true" },
    { ...response, supported: false },
    { ...response, user_id: "" },
    { ...response, organization_id: null },
    { ...response, allowed_actions: null },
    { ...response, allowed_actions: [true] },
  ])("fails closed for malformed or unsupported authorization: %j", (value) => {
    expect(parseAdminMe(value)).toBeNull();
  });

  it("does not invent actions for an observer", () => {
    expect(parseAdminMe({ ...response, role: "platform_observer", allowed_actions: [] })?.allowedActions).toEqual([]);
  });
});
