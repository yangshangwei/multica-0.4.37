interface DaemonQuitDependencies {
  stopBackgroundActivity: () => void;
  runExclusive: (operation: () => Promise<void>) => Promise<void>;
  loadPrefs: () => Promise<{ autoStop: boolean }>;
  stopDaemon: () => Promise<{ success: boolean; error?: string }>;
  quit: () => void;
  warn: (message: string, error: unknown) => void;
}

interface QuitEvent {
  preventDefault: () => void;
}

export function createDaemonQuitHandler(
  deps: DaemonQuitDependencies,
): (event: QuitEvent) => void {
  let state: "idle" | "pending" | "ready" = "idle";

  async function cleanup(): Promise<void> {
    try {
      deps.stopBackgroundActivity();
      await deps.runExclusive(async () => {
        const prefs = await deps.loadPrefs();
        if (prefs.autoStop) {
          const result = await deps.stopDaemon();
          if (!result.success) {
            deps.warn("[daemon] quit cleanup failed:", result.error ?? "unknown error");
          }
        }
      });
    } catch (error) {
      deps.warn("[daemon] quit cleanup failed:", error);
    } finally {
      state = "ready";
      deps.quit();
    }
  }

  return (event) => {
    if (state === "ready") return;

    // Electron only honors prevention during event delivery. Every request
    // must wait for cleanup, including a second quit while preferences load.
    event.preventDefault();
    if (state === "pending") return;
    state = "pending";
    void cleanup();
  };
}
