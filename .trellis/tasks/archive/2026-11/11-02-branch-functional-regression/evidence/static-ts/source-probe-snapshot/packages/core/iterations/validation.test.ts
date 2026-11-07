// @vitest-environment node
import { describe, expect, it } from "vitest";
import { validateIterationName } from "./validation";
describe("iteration name contract", () => {
  it.each(["", " \r\n\t "])(
    "requires a name after normalization: %j",
    (value) => {
      expect(validateIterationName(value)).toBe("required");
    },
  );
  it.each([
    "x",
    "x".repeat(200),
    "🚀".repeat(200),
    ` \r\n${"🚀".repeat(200)}\r\n `,
  ])("accepts at most 200 normalized Unicode codepoints", (value) => {
    expect(validateIterationName(value)).toBeNull();
  });
  it.each(["x".repeat(201), "🚀".repeat(201), "a" + "🚀".repeat(200)])(
    "rejects more than 200 codepoints",
    (value) => {
      expect(validateIterationName(value)).toBe("too_long");
    },
  );
});
