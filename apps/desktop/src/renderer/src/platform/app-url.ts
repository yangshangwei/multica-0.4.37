import { useConfigStore } from "@multica/core/config";
import { resolveAppUrl } from "../../../shared/runtime-config";

/**
 * Web origin for URLs this window hands to the outside world: copied links,
 * share/invite links, pages opened in the browser. Null until the runtime
 * config is accepted.
 *
 * Reads the server-published origin from the config store so the link follows
 * the deployment rather than a guess made from the API address; the precedence
 * rules and their matrix live in `shared/runtime-config`.
 */
export function useAppUrl(): string | null {
  const serverAppUrl = useConfigStore((state) => state.daemonAppUrl);
  const runtimeConfig = window.desktopAPI.runtimeConfig;
  if (!runtimeConfig.ok) return null;
  return resolveAppUrl(runtimeConfig, serverAppUrl);
}
