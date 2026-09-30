import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SquadAvatar } from "./squad-avatar";

const base = { name: "Release squad", initials: "RS", templateKey: "release" };

describe("SquadAvatar", () => {
  it("renders an existing built-in default as a colored circular SVG", () => {
    const { container } = render(<SquadAvatar {...base} avatarUrl="emoji:📦" size="lg" />);
    expect(screen.queryByText("📦")).toBeNull();
    expect(container.querySelector("svg.lucide-package")).not.toBeNull();
    expect(container.firstElementChild).toHaveClass("rounded-full", "text-skill-operations");
    expect(container.firstElementChild).toHaveStyle({ width: "32px", height: "32px" });
  });

  it.each([
    ["feature-delivery", "🚀", "rocket", "engineering"],
    ["bug-fix", "🐞", "bug", "quality"],
    ["review-gate", "🚧", "shield-check", "quality"],
    ["discovery", "🔭", "telescope", "research"],
    ["docs", "📚", "book-open", "writing"],
    ["maintenance", "🧹", "wrench", "operations"],
    ["release", "📦", "package", "operations"],
    ["incident", "🚨", "siren", "operations"],
  ])("renders the %s default with its category tone", (templateKey, emoji, icon, category) => {
    const { container } = render(<SquadAvatar {...base} templateKey={templateKey} avatarUrl={`emoji:${emoji}`} />);
    expect(container.querySelector(`svg.lucide-${icon}`)).not.toBeNull();
    expect(container.firstElementChild).toHaveClass(`text-skill-${category}`);
  });

  it("renders an unknown legacy emoji as a line icon", () => {
    render(<SquadAvatar {...base} avatarUrl="emoji:🌻" />);
    expect(screen.queryByText("🌻")).toBeNull();
    expect(screen.getByRole("img", { name: base.name }).tagName).toBe("svg");
  });

  it("preserves an uploaded image", () => {
    render(<SquadAvatar {...base} avatarUrl="https://example.com/avatar.png" />);
    expect(screen.getByRole("img", { name: base.name })).toHaveAttribute("src", "https://example.com/avatar.png");
  });

  it("renders legacy icons without requiring template provenance", () => {
    render(<SquadAvatar {...base} templateKey="custom" avatarUrl="emoji:📦" />);
    expect(screen.getByRole("img", { name: base.name })).toHaveClass("lucide-package");
  });

  it("uses the template icon when no avatar is set", () => {
    const { container } = render(<SquadAvatar {...base} />);
    expect(container.querySelector("svg.lucide-package")).not.toBeNull();
  });

  it("retains the circular Users fallback for a hand-built squad", () => {
    const { container } = render(<SquadAvatar {...base} templateKey={undefined} />);
    expect(container.querySelector("svg.lucide-users")).not.toBeNull();
    expect(container.firstElementChild).toHaveClass("rounded-full");
  });
});
