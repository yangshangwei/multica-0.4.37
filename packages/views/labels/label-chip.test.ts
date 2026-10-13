// @vitest-environment node

import { describe, expect, it } from "vitest";
import { labelChipColors } from "./label-chip";

// Evaluate the rendered foreground/background pair against the WCAG formula;
// do not assert which foreground the implementation happens to prefer.
function luminance(hex: string) {
  const rgb = hex.slice(1).match(/../g)!.map((channel) => {
    const srgb = Number.parseInt(channel, 16) / 255;
    return srgb <= 0.04045 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4;
  });
  return rgb[0]! * 0.2126 + rgb[1]! * 0.7152 + rgb[2]! * 0.0722;
}

function contrastRatio(first: string, second: string) {
  const lighter = Math.max(luminance(first), luminance(second));
  const darker = Math.min(luminance(first), luminance(second));
  return (lighter + 0.05) / (darker + 0.05);
}

describe("label chip colors", () => {
  it.each(["#ef4444", "#3b82f6", "#8b5cf6", "#777777", "#000000", "#ffffff"])(
    "keeps %s and gives its small text at least 4.5:1 contrast",
    (backgroundColor) => {
      const colors = labelChipColors(backgroundColor);
      expect(colors.backgroundColor).toBe(backgroundColor);
      expect(contrastRatio(colors.color, colors.backgroundColor)).toBeGreaterThanOrEqual(4.5);
    },
  );

  it("guarantees readable text across the RGB range, including the candidate crossover", () => {
    let minimum = Infinity;
    for (let red = 0; red <= 255; red += 17) {
      for (let green = 0; green <= 255; green += 17) {
        for (let blue = 0; blue <= 255; blue += 17) {
          const backgroundColor = `#${[red, green, blue].map((c) => c.toString(16).padStart(2, "0")).join("")}`;
          const colors = labelChipColors(backgroundColor);
          expect(colors.backgroundColor).toBe(backgroundColor);
          minimum = Math.min(minimum, contrastRatio(colors.color, colors.backgroundColor));
        }
      }
    }
    expect(minimum).toBeGreaterThanOrEqual(4.5);
  });

  it("normalizes six-digit backend hex without changing its color", () => {
    expect(labelChipColors("EF4444").backgroundColor).toBe("#ef4444");
  });

  it.each(["", "#abc", "#gg0000", "#12345g", "red", "#12345678", "##123456"])(
    "uses a readable opaque fallback for malformed color %j",
    (color) => {
      const colors = labelChipColors(color);
      expect(colors.backgroundColor).toMatch(/^#[0-9a-f]{6}$/);
      expect(contrastRatio(colors.color, colors.backgroundColor)).toBeGreaterThanOrEqual(4.5);
    },
  );
});
