import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { ProjectIcon } from "./project-icon";
import { ProjectIconPicker } from "./project-icon-picker";

describe("Project Lucide icons", () => {
  it("renders saved legacy emoji as a line icon", () => {
    const { container } = renderWithI18n(<ProjectIcon project={{ icon: "🚀" }} />);
    expect(container.querySelector("svg.lucide-rocket")).not.toBeNull();
    expect(container).not.toHaveTextContent("🚀");
  });

  it("renders a project without an icon using the shared default", () => {
    const { container } = renderWithI18n(<ProjectIcon />);
    expect(container.querySelector("svg.lucide-package")).not.toBeNull();
  });

  it("persists Lucide markers and reflects the selected legacy icon", () => {
    const onSelect = vi.fn();
    renderWithI18n(<ProjectIconPicker value="🚀" onSelect={onSelect} />);
    expect(screen.getByRole("button", { name: "Rocket" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Package" }));
    expect(onSelect).toHaveBeenCalledWith("icon:package");
  });
});
