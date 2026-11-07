import { app } from "electron";
import { randomBytes } from "crypto";
import { mkdirSync, readFileSync, writeFileSync } from "fs";
import { hostname, userInfo } from "os";
import { dirname, join } from "path";

import { isValidDeviceId, type DeviceIdentity } from "../shared/device-identity";

// 16 bytes → 32 hex characters, the shortest id the server accepts. This is an
// identifier, not a secret that gates anything: on a deployment with device
// auth on, any client that can reach the port can mint its own.
const DEVICE_ID_BYTES = 16;

export function generateDeviceId(): string {
  return randomBytes(DEVICE_ID_BYTES).toString("hex");
}

/**
 * The label other members see in the assignee picker and on comments. User plus
 * machine, because on an intranet that is what tells two clients apart.
 */
export function formatDeviceName(user: string, host: string): string {
  const cleanUser = user.trim();
  // Bonjour appends .local to every hostname on a LAN. It carries no
  // information and would eat a third of the visible name.
  const cleanHost = host.trim().replace(/\.local\.?$/i, "");
  if (cleanUser && cleanHost) return `${cleanUser}@${cleanHost}`;
  return cleanUser || cleanHost || "multica-desktop";
}

export function systemDeviceName(): string {
  let user = "";
  try {
    user = userInfo().username;
  } catch {
    // No passwd entry for this uid (happens in containers). The host alone
    // still identifies the machine.
  }
  return formatDeviceName(user, hostname());
}

/** Parses a stored identity, returning null for anything unusable. */
export function parseDeviceIdentity(
  raw: string,
  fallbackName: string,
): DeviceIdentity | null {
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!data || typeof data !== "object") return null;
  const { deviceId, deviceName } = data as {
    deviceId?: unknown;
    deviceName?: unknown;
  };
  if (!isValidDeviceId(deviceId)) return null;
  const name =
    typeof deviceName === "string" && deviceName.trim()
      ? deviceName.trim()
      : fallbackName;
  return { deviceId, deviceName: name };
}

export function deviceIdentityFilePath(): string {
  return join(app.getPath("userData"), "device-identity.json");
}

/**
 * Reads this installation's identity, creating it on first boot.
 *
 * A missing or corrupt file mints a NEW identity instead of failing: the
 * alternative is a client that cannot get in at all, and the cost of
 * regenerating is one extra member row, not a broken deployment. A write
 * failure is not fatal either — the session works, it just will not survive a
 * restart — so an identity comes back either way.
 */
export function loadOrCreateDeviceIdentity(
  filePath: string,
  fallbackName: string,
): DeviceIdentity {
  try {
    const existing = parseDeviceIdentity(
      readFileSync(filePath, "utf-8"),
      fallbackName,
    );
    if (existing) return existing;
  } catch {
    // ENOENT on first boot, or an unreadable file: fall through and mint one.
  }

  const identity: DeviceIdentity = {
    deviceId: generateDeviceId(),
    deviceName: fallbackName,
  };
  try {
    mkdirSync(dirname(filePath), { recursive: true });
    writeFileSync(filePath, `${JSON.stringify(identity, null, 2)}\n`, {
      mode: 0o600,
    });
  } catch (err) {
    console.error(
      "[device-identity] failed to persist identity; this session will not survive a restart:",
      err,
    );
  }
  return identity;
}
