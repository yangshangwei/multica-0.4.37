import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { URL } from "node:url";
import { avatarIconColor, avatarIconNodes, resolveAvatarIcon, resolveProjectIcon } from "./avatar-icon";

describe("mobile avatar icons", () => {
  it.each(["bot", "users", "siren", "telescope", "git-pull-request"])("resolves %s markers", (name) => {
    expect(resolveAvatarIcon(`icon:${name}`)).toBe(name);
  });

  it("maps legacy avatars and preserves image URLs", () => {
    expect(resolveAvatarIcon("emoji:🚑")).toBe("bug");
    expect(resolveAvatarIcon("emoji:🛡️")).toBe("shield-check");
    expect(resolveAvatarIcon("emoji:🐙")).toBe("workflow");
    expect(resolveAvatarIcon("emoji:unknown")).toBe("bot");
    expect(resolveAvatarIcon("https://example.com/avatar.png")).toBeNull();
    expect(resolveAvatarIcon("icon:unknown")).toBeNull();
    expect(resolveAvatarIcon("icon:toString")).toBeNull();
    expect(resolveAvatarIcon("emoji:")).toBeNull();
  });

  it("resolves raw and marked project icons using the web fallback", () => {
    expect(resolveProjectIcon(null)).toBe("package");
    expect(resolveProjectIcon("📁")).toBe("package");
    expect(resolveProjectIcon("🚀")).toBe("rocket");
    expect(resolveProjectIcon("emoji:🚀")).toBe("rocket");
    expect(resolveProjectIcon("icon:database")).toBe("database");
    expect(resolveProjectIcon("icon:unknown")).toBe("package");
  });

  it("covers the web icon registry and both theme palettes", () => {
    const source = readFileSync(new URL("../../../packages/ui/lib/avatar-icon.ts", import.meta.url), "utf8");
    const block = source.match(/AVATAR_ICON_COMPONENTS = \{([\s\S]*?)\} satisfies/)![1];
    const names = [...block.matchAll(/\s*["']?([\w-]+)["']?: (\w+),/g)].map((match) => match[1]);
    expect(Object.keys(avatarIconNodes).sort()).toEqual(names.sort());
    for (const name of Object.keys(avatarIconNodes) as (keyof typeof avatarIconNodes)[]) {
      expect(avatarIconColor(name, false)).toMatch(/^#[0-9a-f]{6}$/);
      expect(avatarIconColor(name, true)).toMatch(/^#[0-9a-f]{6}$/);
      for (const [tag] of avatarIconNodes[name]) {
        expect(["path", "rect", "circle", "ellipse", "line", "polygon", "polyline"]).toContain(tag);
      }
    }
  });
});
