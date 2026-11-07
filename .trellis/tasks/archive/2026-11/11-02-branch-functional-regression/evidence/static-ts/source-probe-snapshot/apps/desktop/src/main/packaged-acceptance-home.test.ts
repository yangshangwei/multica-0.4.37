// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  configurePackagedAcceptanceHome,
  PACKAGED_ACCEPTANCE_HOME_ENV,
  PACKAGED_ACCEPTANCE_MODE_ENV,
} from "./packaged-acceptance-home";

describe("configurePackagedAcceptanceHome", () => {
  it("leaves normal packaged paths untouched when disabled", () => {
    const paths: [string, string][] = [];
    expect(
      configurePackagedAcceptanceHome({
        isDev: false,
        env: {},
        setPath: (name, value) => paths.push([name, value]),
      }),
    ).toBeNull();
    expect(paths).toEqual([]);
  });

  it("requires the explicit marker and an absolute home", () => {
    const setPath = () => {};
    expect(() =>
      configurePackagedAcceptanceHome({
        isDev: false,
        env: { [PACKAGED_ACCEPTANCE_HOME_ENV]: "/tmp/acceptance" },
        setPath,
      }),
    ).toThrow("packaged acceptance mode");
    expect(() =>
      configurePackagedAcceptanceHome({
        isDev: false,
        env: {
          [PACKAGED_ACCEPTANCE_MODE_ENV]: "1",
          [PACKAGED_ACCEPTANCE_HOME_ENV]: "relative-home",
        },
        setPath,
      }),
    ).toThrow("absolute path");
    expect(() =>
      configurePackagedAcceptanceHome({
        isDev: true,
        env: {
          [PACKAGED_ACCEPTANCE_MODE_ENV]: "1",
          [PACKAGED_ACCEPTANCE_HOME_ENV]: "/tmp/acceptance",
        },
        setPath,
      }),
    ).toThrow("packaged acceptance mode");
  });

  it("maps both home and userData into the task-owned tree", () => {
    const paths: [string, string][] = [];
    const home = configurePackagedAcceptanceHome({
      isDev: false,
      env: {
        [PACKAGED_ACCEPTANCE_MODE_ENV]: "1",
        [PACKAGED_ACCEPTANCE_HOME_ENV]: "/tmp/multica-acceptance/../acceptance",
      },
      setPath: (name, value) => paths.push([name, value]),
    });
    expect(home).toBe("/tmp/acceptance");
    expect(paths).toEqual([
      ["home", "/tmp/acceptance"],
      ["userData", "/tmp/acceptance/.config/Multica"],
    ]);
  });
});
