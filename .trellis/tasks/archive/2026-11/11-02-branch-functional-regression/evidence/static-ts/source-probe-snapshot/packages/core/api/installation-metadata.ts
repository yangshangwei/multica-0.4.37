export interface InstallationMetadataProofInput {
  serverUrl: string;
  userId: string;
  authVersion: string;
  proof: string;
}

// Reading exp only avoids sending an expired hint. This is not verification:
// the server authenticates the session and validates the separate proof key.
export function installationProofExpiry(proof: string): number | null {
  if (!proof.startsWith("mip_") || proof.length > 4096) return null;
  const parts = proof.slice(4).split(".");
  if (parts.length !== 3 || !parts[1]) return null;
  try {
    const claims: unknown = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
    if (!claims || typeof claims !== "object" || !("exp" in claims) ||
      typeof claims.exp !== "number" || !Number.isSafeInteger(claims.exp) ||
      claims.exp <= 0 || claims.exp > Number.MAX_SAFE_INTEGER / 1000) return null;
    return claims.exp * 1000;
  } catch {
    return null;
  }
}
