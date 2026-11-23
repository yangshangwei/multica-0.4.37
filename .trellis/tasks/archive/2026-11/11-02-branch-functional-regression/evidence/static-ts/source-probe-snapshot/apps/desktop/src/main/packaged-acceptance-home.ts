import { isAbsolute, join, resolve } from "node:path";

export const PACKAGED_ACCEPTANCE_MODE_ENV = "MULTICA_PACKAGED_ACCEPTANCE";
export const PACKAGED_ACCEPTANCE_HOME_ENV = "MULTICA_PACKAGED_ACCEPTANCE_HOME";

type SetPath = (name: "home" | "userData", value: string) => void;

/**
 * Selects a task-owned home for a packaged acceptance run.
 *
 * Production keeps Electron's normal home and userData paths. The override is
 * deliberately opt-in and requires the explicit acceptance marker so a random
 * environment variable cannot silently redirect an installed client.
 */
export function configurePackagedAcceptanceHome(options: {
  isDev: boolean;
  env?: NodeJS.ProcessEnv;
  setPath: SetPath;
}): string | null {
  const env = options.env ?? process.env;
  const rawHome = env[PACKAGED_ACCEPTANCE_HOME_ENV]?.trim();
  if (!rawHome) return null;
  if (
    options.isDev ||
    env[PACKAGED_ACCEPTANCE_MODE_ENV] !== "1" ||
    !isAbsolute(rawHome)
  ) {
    throw new Error(
      `${PACKAGED_ACCEPTANCE_HOME_ENV} requires packaged acceptance mode and an absolute path`,
    );
  }

  const home = resolve(rawHome);
  options.setPath("home", home);
  options.setPath("userData", join(home, ".config", "Multica"));
  return home;
}
