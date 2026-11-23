export interface InstallationMetadataProof {
  serverUrl: string;
  userId: string;
  authVersion: string;
  proof: string;
}

/** Reads only the session's scope version; this never authenticates a JWT. */
export function managementAuthVersion(token: string, userId: string): string | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3 || !parts[1]) return null;
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const bytes = Uint8Array.from(atob(base64), (character) => character.charCodeAt(0));
    let versionSource: string | undefined;
    const claims: unknown = JSON.parse(new TextDecoder().decode(bytes), (key: string, value: unknown, context?: { source?: string }) => {
      if (key === "auth_version") versionSource = context?.source;
      return value;
    });
    if (!claims || typeof claims !== "object" || !("sub" in claims) || claims.sub !== userId || !("auth_version" in claims)) return null;
    if (versionSource === undefined && typeof claims.auth_version === "number" && Number.isSafeInteger(claims.auth_version)) versionSource = String(claims.auth_version);
    if (!versionSource || !/^[1-9][0-9]*$/.test(versionSource) || BigInt(versionSource) > 9_223_372_036_854_775_807n) return null;
    return versionSource;
  } catch { return null; }
}

export function matchesInstallationMetadata(value: unknown, scope: { serverUrl: string; userId: string; authVersion: string | null }): value is InstallationMetadataProof {
  if (!value || typeof value !== "object" || scope.authVersion === null) return false;
  const payload = value as Partial<InstallationMetadataProof>;
  try {
    const normalize = (url: string) => new URL(url).toString().replace(/\/+$/, "");
    return typeof payload.serverUrl === "string" && normalize(payload.serverUrl) === normalize(scope.serverUrl) && payload.userId === scope.userId && payload.authVersion === scope.authVersion &&
      typeof payload.proof === "string" && payload.proof.startsWith("mip_") && payload.proof.length <= 16_384;
  } catch { return false; }
}
