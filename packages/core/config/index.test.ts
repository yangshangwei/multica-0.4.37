// @vitest-environment node
import { afterEach, describe, expect, it } from "vitest";
import { configStore } from "./index";

afterEach(() => configStore.setState(configStore.getInitialState()));

describe("daemon setup config", () => {
  it("clears platform overrides when a later config omits them", () => {
    configStore.getState().setDaemonConfig({
      daemonServerUrl: "https://api.internal",
      daemonAppUrl: "https://app.internal",
      cliInstallCommand: "legacy installer",
      cliInstallCommands: { windows: "windows installer", linux: "linux installer" },
    });
    configStore.getState().setDaemonConfig({
      cliInstallCommands: { macos: "macos installer" },
    });
    expect(configStore.getState()).toMatchObject({
      daemonServerUrl: "",
      daemonAppUrl: "",
      cliInstallCommand: "",
      cliInstallCommands: { macos: "macos installer" },
    });
    configStore.getState().setDaemonConfig({});
    expect(configStore.getState().cliInstallCommands).toEqual({});
  });
});
