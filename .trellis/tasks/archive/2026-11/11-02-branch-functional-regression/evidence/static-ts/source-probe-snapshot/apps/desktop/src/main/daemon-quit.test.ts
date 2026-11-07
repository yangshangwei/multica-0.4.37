// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { createDaemonQuitHandler } from "./daemon-quit";
import { DaemonOperationGate } from "./daemon-recovery";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function setup() {
  const lifecycleOperations = new DaemonOperationGate();
  const deps = {
    stopBackgroundActivity: vi.fn(),
    runExclusive: vi.fn((operation: () => Promise<void>) =>
      lifecycleOperations.runForeground(operation),
    ),
    loadPrefs: vi.fn(async () => ({ autoStop: true })),
    stopDaemon: vi.fn(async (): Promise<{ success: boolean; error?: string }> => ({
      success: true,
    })),
    quit: vi.fn(),
    warn: vi.fn(),
  };
  const handler = createDaemonQuitHandler(deps);
  return { deps, handler, lifecycleOperations };
}

describe("daemon quit cleanup", () => {
  it("prevents exit synchronously while preferences are still loading", async () => {
    const { deps, handler } = setup();
    const prefs = deferred<{ autoStop: boolean }>();
    deps.loadPrefs.mockImplementation(() => prefs.promise);
    const event = { preventDefault: vi.fn() };

    handler(event);

    expect(event.preventDefault).toHaveBeenCalledOnce();
    expect(deps.stopBackgroundActivity).toHaveBeenCalledOnce();
    await vi.waitFor(() => expect(deps.loadPrefs).toHaveBeenCalledOnce());
    expect(event.preventDefault.mock.invocationCallOrder[0]).toBeLessThan(
      deps.loadPrefs.mock.invocationCallOrder[0],
    );
    expect(deps.stopDaemon).not.toHaveBeenCalled();
    expect(deps.quit).not.toHaveBeenCalled();

    prefs.resolve({ autoStop: true });
    await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
    expect(deps.stopDaemon).toHaveBeenCalledOnce();
  });

  it.each([true, false])(
    "waits for in-flight startup before final cleanup and exit (auto-stop: %s)",
    async (autoStop) => {
      const { deps, handler, lifecycleOperations } = setup();
      const startup = deferred<void>();
      const activity: string[] = [];
      const starting = lifecycleOperations.runBackground(async () => {
        activity.push("starting");
        await startup.promise;
        activity.push("running");
      });
      deps.loadPrefs.mockResolvedValue({ autoStop });
      deps.stopDaemon.mockImplementation(async () => {
        activity.push("stopped");
        return { success: true };
      });
      deps.quit.mockImplementation(() => {
        activity.push("quit");
      });
      const event = { preventDefault: vi.fn() };

      handler(event);

      expect(event.preventDefault).toHaveBeenCalledOnce();
      expect(deps.stopBackgroundActivity).toHaveBeenCalledOnce();
      await new Promise<void>((resolve) => setImmediate(resolve));
      expect(deps.quit).not.toHaveBeenCalled();
      expect(deps.stopDaemon).not.toHaveBeenCalled();
      expect(activity).toEqual(["starting"]);

      startup.resolve();
      await starting;
      await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
      expect(activity).toEqual(
        autoStop
          ? ["starting", "running", "stopped", "quit"]
          : ["starting", "running", "quit"],
      );
    },
  );

  it("prevents repeated exit requests until the single daemon stop finishes", async () => {
    const { deps, handler } = setup();
    const stopped = deferred<{ success: boolean }>();
    deps.stopDaemon.mockImplementation(() => stopped.promise);
    handler({ preventDefault: vi.fn() });
    const whileLoading = { preventDefault: vi.fn() };
    handler(whileLoading);
    expect(whileLoading.preventDefault).toHaveBeenCalledOnce();
    await vi.waitFor(() => expect(deps.stopDaemon).toHaveBeenCalledOnce());

    const whileStopping = { preventDefault: vi.fn() };
    handler(whileStopping);
    expect(whileStopping.preventDefault).toHaveBeenCalledOnce();
    expect(deps.loadPrefs).toHaveBeenCalledOnce();
    expect(deps.stopBackgroundActivity).toHaveBeenCalledOnce();
    expect(deps.quit).not.toHaveBeenCalled();

    const resumedExit = { preventDefault: vi.fn() };
    deps.quit.mockImplementation(() => handler(resumedExit));
    stopped.resolve({ success: true });
    await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
    expect(resumedExit.preventDefault).not.toHaveBeenCalled();
    expect(deps.stopDaemon).toHaveBeenCalledOnce();
    expect(deps.warn).not.toHaveBeenCalled();
  });

  it("resumes exit without stopping the daemon when auto-stop is disabled", async () => {
    const { deps, handler } = setup();
    deps.loadPrefs.mockResolvedValue({ autoStop: false });
    const resumedExit = { preventDefault: vi.fn() };
    deps.quit.mockImplementation(() => handler(resumedExit));

    handler({ preventDefault: vi.fn() });

    await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
    expect(deps.stopDaemon).not.toHaveBeenCalled();
    expect(resumedExit.preventDefault).not.toHaveBeenCalled();
  });

  it.each(["loadPrefs", "stopDaemon"] as const)(
    "logs a %s rejection and still resumes exit",
    async (operation) => {
      const { deps, handler } = setup();
      const error = new Error("cleanup failed");
      deps[operation].mockRejectedValue(error);
      const resumedExit = { preventDefault: vi.fn() };
      deps.quit.mockImplementation(() => handler(resumedExit));

      handler({ preventDefault: vi.fn() });

      await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
      expect(deps.warn).toHaveBeenCalledWith(
        "[daemon] quit cleanup failed:",
        error,
      );
      expect(resumedExit.preventDefault).not.toHaveBeenCalled();
    },
  );

  it("logs an unsuccessful daemon stop result and still resumes exit", async () => {
    const { deps, handler } = setup();
    deps.stopDaemon.mockResolvedValue({ success: false, error: "stop timed out" });

    handler({ preventDefault: vi.fn() });

    await vi.waitFor(() => expect(deps.quit).toHaveBeenCalledOnce());
    expect(deps.warn).toHaveBeenCalledWith(
      "[daemon] quit cleanup failed:",
      "stop timed out",
    );
  });
});
