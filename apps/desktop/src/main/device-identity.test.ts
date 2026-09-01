// @vitest-environment node
import { mkdtemp, readFile, writeFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { describe, expect, it, vi } from "vitest";
import {
  formatDeviceName,
  generateDeviceId,
  loadOrCreateDeviceIdentity,
  parseDeviceIdentity,
} from "./device-identity";
import { isValidDeviceId } from "../shared/device-identity";

async function tempFile(name = "device-identity.json"): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "multica-device-identity-"));
  return join(dir, name);
}

describe("generateDeviceId", () => {
  it("produces an id the server will accept", () => {
    const id = generateDeviceId();
    expect(id).toMatch(/^[0-9a-f]{32}$/);
    expect(isValidDeviceId(id)).toBe(true);
    expect(generateDeviceId()).not.toBe(id);
  });
});

describe("isValidDeviceId", () => {
  it("accepts only 32-64 lowercase hex", () => {
    expect(isValidDeviceId("a".repeat(32))).toBe(true);
    expect(isValidDeviceId("a".repeat(64))).toBe(true);
    expect(isValidDeviceId("a".repeat(31))).toBe(false);
    expect(isValidDeviceId("a".repeat(65))).toBe(false);
    expect(isValidDeviceId("A".repeat(32))).toBe(false);
    expect(isValidDeviceId("z".repeat(32))).toBe(false);
    expect(isValidDeviceId(undefined)).toBe(false);
  });
});

describe("formatDeviceName", () => {
  it("names the machine the way another member would recognize it", () => {
    expect(formatDeviceName("artisan", "mac-mini")).toBe("artisan@mac-mini");
    // Bonjour appends .local on every LAN; it carries no information.
    expect(formatDeviceName("artisan", "mac-mini.local")).toBe("artisan@mac-mini");
    expect(formatDeviceName("  artisan  ", " mac-mini ")).toBe("artisan@mac-mini");
  });

  it("falls back to whichever half exists", () => {
    expect(formatDeviceName("", "mac-mini")).toBe("mac-mini");
    expect(formatDeviceName("artisan", "")).toBe("artisan");
    expect(formatDeviceName("", "")).toBe("multica-desktop");
  });
});

describe("parseDeviceIdentity", () => {
  it("keeps a stored identity and its name", () => {
    const raw = JSON.stringify({ deviceId: "b".repeat(32), deviceName: "lab-01" });
    expect(parseDeviceIdentity(raw, "fallback")).toEqual({
      deviceId: "b".repeat(32),
      deviceName: "lab-01",
    });
  });

  it("rejects anything that is not a usable id", () => {
    expect(parseDeviceIdentity("{", "fallback")).toBeNull();
    expect(parseDeviceIdentity("null", "fallback")).toBeNull();
    expect(parseDeviceIdentity(JSON.stringify({}), "fallback")).toBeNull();
    expect(parseDeviceIdentity(JSON.stringify({ deviceId: "short" }), "fallback")).toBeNull();
  });

  it("substitutes the system name when the stored one is blank", () => {
    const raw = JSON.stringify({ deviceId: "c".repeat(32), deviceName: "   " });
    expect(parseDeviceIdentity(raw, "artisan@mac-mini")?.deviceName).toBe("artisan@mac-mini");
  });
});

describe("loadOrCreateDeviceIdentity", () => {
  it("mints an identity on first boot and reuses it afterwards", async () => {
    const path = await tempFile();

    const first = loadOrCreateDeviceIdentity(path, "artisan@mac-mini");
    expect(isValidDeviceId(first.deviceId)).toBe(true);
    expect(first.deviceName).toBe("artisan@mac-mini");

    // Stability is the whole point: a new id every launch would make each
    // restart a new member, and yesterday's issues would belong to a stranger.
    const second = loadOrCreateDeviceIdentity(path, "artisan@mac-mini");
    expect(second).toEqual(first);
    expect(JSON.parse(await readFile(path, "utf-8")).deviceId).toBe(first.deviceId);
  });

  it("replaces a corrupt file rather than refusing to start", async () => {
    const path = await tempFile();
    await writeFile(path, "{ this is not json");

    const identity = loadOrCreateDeviceIdentity(path, "artisan@mac-mini");
    expect(isValidDeviceId(identity.deviceId)).toBe(true);
    // Rewritten, so the replacement identity survives the next restart.
    expect(JSON.parse(await readFile(path, "utf-8")).deviceId).toBe(identity.deviceId);
  });

  it("still returns an identity when it cannot be persisted", async () => {
    const blocker = await tempFile("blocker");
    await writeFile(blocker, "not a directory");
    const path = join(blocker, "device-identity.json");
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});

    // A session that works but does not survive a restart beats no session.
    const identity = loadOrCreateDeviceIdentity(path, "artisan@mac-mini");
    expect(isValidDeviceId(identity.deviceId)).toBe(true);
    expect(logged).toHaveBeenCalled();
    logged.mockRestore();
  });
});
