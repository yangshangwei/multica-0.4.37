// @vitest-environment node
import { describe, expect, it } from "vitest";
import { adjacentOrder } from "./adjacent-order";

describe("adjacentOrder loaded group boundaries", () => {
  it("moves only to a loaded sibling without mutating the input", () => {
    const ids = ["a", "b", "c"];
    expect(adjacentOrder(ids, "b", -1)).toEqual(["b", "a", "c"]);
    expect(adjacentOrder(ids, "b", 1)).toEqual(["a", "c", "b"]);
    expect(ids).toEqual(["a", "b", "c"]);
  });
  it("rejects missing ids and group edges", () => {
    expect(adjacentOrder(["a", "b"], "a", -1)).toBeNull();
    expect(adjacentOrder(["a", "b"], "b", 1)).toBeNull();
    expect(adjacentOrder(["a", "b"], "missing", 1)).toBeNull();
  });
  it("does not invent an end anchor when later siblings are unloaded", () => {
    expect(adjacentOrder(["a", "b", "c"], "b", 1, true)).toBeNull();
    expect(adjacentOrder(["a", "b", "c"], "a", 1, true)).toEqual(["b", "a", "c"]);
    expect(adjacentOrder(["a", "b", "c"], "c", -1, true)).toEqual(["a", "c", "b"]);
  });
});
