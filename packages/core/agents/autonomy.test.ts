// @vitest-environment node
import { describe, expect, it } from "vitest";
import { canAgentCreateIssues } from "./autonomy";

describe("canAgentCreateIssues", () => {
  it("mirrors the server: only a declared level below contributor is refused", () => {
    expect(canAgentCreateIssues("observer")).toBe(false);
    for (const level of ["contributor", "coordinator", "operator"]) {
      expect(canAgentCreateIssues(level)).toBe(true);
    }
  });

  it("treats an absent or unknown level as no policy, never as observer", () => {
    for (const level of [undefined, null, "", "supervisor", "Observer"]) {
      expect(canAgentCreateIssues(level)).toBe(true);
    }
  });
});
