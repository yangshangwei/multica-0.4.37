// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ApiError } from "@multica/core/api/client";
import { getSettingsQueryState } from "./settings-query-feedback";

const saved = {
  data: { name: "Saved settings" }, error: null, isError: false,
  isFetching: false, errorUpdatedAt: 0, dataUpdatedAt: 1,
};

describe("Settings query presentation", () => {
  it("keeps a previous failure visible during a cold pending retry", () => {
    expect(getSettingsQueryState({
      ...saved, data: undefined, dataUpdatedAt: 0, errorUpdatedAt: 2, isFetching: true,
    })).toEqual({ hasData: false, failed: true, accessDenied: false });
  });

  it("treats successful null configuration as loaded data", () => {
    expect(getSettingsQueryState({ ...saved, data: null })).toEqual({
      hasData: true, failed: false, accessDenied: false,
    });
  });

  it("hides a denied snapshot while keeping a transient cached read available", () => {
    expect(getSettingsQueryState({ ...saved, isError: true, error: new ApiError("denied", 403, "Forbidden") }))
      .toEqual({ hasData: false, failed: true, accessDenied: true });
    expect(getSettingsQueryState({ ...saved, isError: true, error: new Error("offline") }))
      .toEqual({ hasData: true, failed: true, accessDenied: false });
  });

  it("does not label a stale credential response as current access denial", () => {
    const error = new ApiError("old session", 401, "Unauthorized", undefined, () => false);
    expect(getSettingsQueryState({ ...saved, isError: true, error })).toEqual({
      hasData: true, failed: true, accessDenied: false,
    });
  });
});
