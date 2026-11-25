const fetchCodes = new Set(["ECONNREFUSED", "ECONNRESET", "ETIMEDOUT", "ENOTFOUND", "EHOSTUNREACH", "UND_ERR_CONNECT_TIMEOUT", "UND_ERR_SOCKET"]);
const integer = value => Number.isSafeInteger(value) && value >= 100 && value <= 599 ? value : null;

/** Select only non-secret booleans from the native ownership probe. */
export function collaborationProcessEvidence(value) {
  return {
    source_matches: value?.source_matches === true,
    ...Object.fromEntries(["api", "web"].map(service => [service, Object.fromEntries(
      ["present", "alive", "path_matches", "start_matches"].map(key => [key, value?.[service]?.[key] === true]),
    )])),
  };
}

export async function probeCollaborationApi(setup) {
  const probe = {};
  for (const [key, route] of [["health", "/health"], ["readiness", "/healthz"], ["config", "/api/config"]]) {
    const item = { status: null, valid: false, error_kind: null, cause_code: null };
    try {
      const response = await fetch(`${setup.api}${route}`, { signal: AbortSignal.timeout(5000), redirect: "error" });
      item.status = integer(response.status);
      const body = await response.json();
      item.valid = response.status === 200 && (key === "health"
        ? body.status === "ok" && body.pid === setup.pid && body.commit === setup.sourceCommit
        : key === "readiness" ? body.status === "ok"
          : body.auth_mode === "password" && body.managed_installation_supported === true && body.deployment_id === setup.environment.MULTICA_DEPLOYMENT_ID);
    } catch (error) {
      item.error_kind = ["TypeError", "SyntaxError", "TimeoutError", "AbortError"].includes(error.name) ? error.name : "unknown";
      item.cause_code = fetchCodes.has(error.cause?.code) ? error.cause.code : null;
    }
    probe[key] = item;
  }
  return probe;
}
