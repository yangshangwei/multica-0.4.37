import { detectWebOS } from "@/platform/client-os";

/**
 * Per-browser identity for intranet device auth.
 *
 * The desktop app keys this off a file in its user-data directory; a browser
 * has no equivalent, so the id lives in localStorage and its scope is "this
 * browser profile on this machine" rather than "this machine". Clearing site
 * data joins as a new member on the next visit, exactly as deleting the
 * desktop app's `device-identity.json` does.
 *
 * The id is not a secret and gates nothing: on a deployment with device auth
 * on, any client that can reach the backend can mint its own.
 */
const STORAGE_KEY = "multica_device_id";

/** Mirrors the server's accepted device_id shape (32-64 lowercase hex). */
const DEVICE_ID_PATTERN = /^[0-9a-f]{32,64}$/;

// 16 bytes → 32 hex characters, the shortest id the server accepts.
const DEVICE_ID_BYTES = 16;

export interface WebDeviceIdentity {
  deviceId: string;
  deviceName: string;
}

export function generateDeviceId(): string {
  const bytes = new Uint8Array(DEVICE_ID_BYTES);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Label other members see. Desktop sends `<os user>@<hostname>`; a browser can
 * read neither, so the OS bucket plus a slice of the id stands in. The id
 * slice is what keeps two browsers on the same OS distinguishable in a member
 * list — which is the whole reason identity here is per client and not one
 * shared account.
 */
export function formatWebDeviceName(os: string, deviceId: string): string {
  const suffix = deviceId.slice(0, 8);
  return os && os !== "unknown" ? `web-${os}-${suffix}` : `web-${suffix}`;
}

/** The slice of the Storage API this needs, so a test can hand it a hostile one. */
type DeviceIdStore = Pick<Storage, "getItem" | "setItem">;

function defaultStore(): DeviceIdStore | undefined {
  return typeof window === "undefined" ? undefined : window.localStorage;
}

/**
 * Reads this browser's identity, creating it on first visit. Returns null
 * during SSR and when storage is unavailable (private-mode quota, blocked
 * cookies): device auth is then simply not offered and the login page stands.
 *
 * `store` is injected the way detectWebOS injects `navigator` — the default is
 * the real one.
 */
export function loadOrCreateWebDeviceIdentity(
  store: DeviceIdStore | null | undefined = defaultStore(),
): WebDeviceIdentity | null {
  if (!store) return null;

  let deviceId: string | null = null;
  try {
    const stored = store.getItem(STORAGE_KEY);
    if (stored && DEVICE_ID_PATTERN.test(stored)) {
      deviceId = stored;
    }
  } catch {
    return null;
  }

  if (!deviceId) {
    deviceId = generateDeviceId();
    try {
      store.setItem(STORAGE_KEY, deviceId);
    } catch {
      // Unwritable storage would hand out a new identity on every page load,
      // scattering one person's issues across a growing pile of members.
      // Better to leave this browser on the login page.
      return null;
    }
  }

  return {
    deviceId,
    deviceName: formatWebDeviceName(detectWebOS(), deviceId),
  };
}
