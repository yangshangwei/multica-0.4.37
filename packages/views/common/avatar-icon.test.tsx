import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { AVATAR_ICON_COMPONENTS, formatAvatarIcon, parseAvatarIcon, resolveAvatarIcon, type AvatarIconName } from "@multica/ui/lib/avatar-icon";

describe("Lucide avatar markers", () => {
  it.each([
    "emoji:🦄",
    "/api/avatars/builtin/afu-seal-v1.png",
    "https://api.example.test/api/avatars/builtin/afu-seal-v1.png",
  ])("preserves the built-in Fu identity for %s", (avatarUrl) => {
    const { container } = render(<ActorAvatar name="Renamed assistant" initials="RA" avatarUrl={avatarUrl} isAgent />);
    expect(screen.getByRole("img", { name: "Renamed assistant" })).toHaveTextContent("孚");
    expect(container.firstElementChild).toHaveClass("rounded-full", "bg-skill-quality/12", "text-skill-quality");
    expect(container.querySelector("svg, img")).toBeNull();
  });
  it("round trips every selectable icon", () => {
    for (const name of Object.keys(AVATAR_ICON_COMPONENTS) as AvatarIconName[]) {
      expect(parseAvatarIcon(formatAvatarIcon(name))).toBe(name);
    }
  });
  it.each(["icon:constructor", "icon:unknown", "https://example.com/a.png", null])("rejects non-icons: %s", (value) => {
    expect(parseAvatarIcon(value)).toBeNull();
  });
  it("maps legacy role avatars and unknown emojis to line icons", () => {
    expect(resolveAvatarIcon("emoji:📦")).toBe("package");
    expect(resolveAvatarIcon("emoji:🚑")).toBe("bug");
    expect(resolveAvatarIcon("emoji:🧭")).toBe("compass");
    expect(resolveAvatarIcon("emoji:🌻")).toBe("bot");
  });
  it("renders an agent icon in a colored circle", () => {
    const { container } = render(<ActorAvatar name="Release lead" initials="RL" avatarUrl="icon:package" isAgent />);
    expect(screen.getByRole("img", { name: "Release lead" })).toHaveClass("lucide-package");
    expect(container.firstElementChild).toHaveClass("rounded-full", "text-skill-operations");
    expect(container.querySelector("img")).toBeNull();
  });
  it("preserves uploaded images and falls back after load failure", () => {
    const { container } = render(<ActorAvatar name="Agent" initials="A" avatarUrl="https://example.com/a.png" isAgent />);
    const img = screen.getByRole("img", { name: "Agent" });
    expect(img).toHaveAttribute("src", "https://example.com/a.png");
    fireEvent.error(img);
    expect(container.querySelector("svg.lucide-bot")).not.toBeNull();
  });
});
