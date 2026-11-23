import type { AgentRuntime } from "@multica/core/types";
import { buildRuntimeMachines, splitRuntimeName } from "./runtime-machines";

function deviceName(runtime: AgentRuntime): string | null {
  return splitRuntimeName(runtime.name).hostname?.trim() ||
    runtime.device_info?.split(" · ")[0]?.trim() || null;
}

function identityKeys(runtime: AgentRuntime): string[] {
  const keys = [`runtime:${runtime.id}`];
  if (runtime.daemon_id) keys.push(`daemon:${runtime.daemon_id}`);
  const device = deviceName(runtime);
  if (device) keys.push(`device:${device.toLowerCase()}`);
  return keys;
}

function ownedLocalRuntime(runtime: AgentRuntime, userId: string, wsId: string): boolean {
  return runtime.owner_id === userId && runtime.workspace_id === wsId &&
    runtime.runtime_mode === "local";
}

export function computerBaseline(runtimes: AgentRuntime[], userId: string, wsId: string): Set<string> {
  return new Set(runtimes
    .filter((runtime) => ownedLocalRuntime(runtime, userId, wsId))
    .flatMap(identityKeys));
}

export function newlyConnectedComputer(
  runtimes: AgentRuntime[],
  baseline: Set<string>,
  userId: string,
  wsId: string,
) {
  const candidates = runtimes.filter((runtime) => {
    const identities = identityKeys(runtime);
    return ownedLocalRuntime(runtime, userId, wsId) &&
      runtime.status === "online" && identities.length > 1 &&
      // Keep waiting when the server cannot identify the computer by name.
      !!(deviceName(runtime) || runtime.custom_name?.trim()) &&
      !identities.some((key) => baseline.has(key));
  });
  return buildRuntimeMachines(candidates, { now: Date.now(), currentUserId: userId })[0] ?? null;
}
