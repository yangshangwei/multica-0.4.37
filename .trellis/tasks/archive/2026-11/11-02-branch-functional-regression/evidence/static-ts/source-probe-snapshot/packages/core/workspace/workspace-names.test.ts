// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  createWorkspaceNameGenerator,
  DEFAULT_WORKSPACE_NAME_SERIES,
  isWorkspaceNameSelection,
  WORKSPACE_NAMES,
  WORKSPACE_NAME_SERIES,
} from "./workspace-names";

describe("workspace name catalog", () => {
  it("offers six curated bilingual series with distinct safe identities", () => {
    expect(DEFAULT_WORKSPACE_NAME_SERIES).toBe("workshop");
    expect(WORKSPACE_NAME_SERIES).toEqual([
      "workshop", "computing", "ai", "space", "nature", "voyage",
    ]);
    for (const series of WORKSPACE_NAME_SERIES) {
      expect(WORKSPACE_NAMES[series].length).toBeGreaterThanOrEqual(20);
      expect(WORKSPACE_NAMES[series].length).toBeLessThanOrEqual(30);
    }
    const names = Object.values(WORKSPACE_NAMES).flat();
    for (const field of ["id", "en", "zh", "slug"] as const) {
      expect(new Set(names.map((name) => name[field])).size).toBe(names.length);
    }
    for (const entry of names) {
      expect(entry.en.trim()).toBeTruthy();
      expect(entry.zh).toMatch(/[\u4e00-\u9fff]/);
      expect(entry.slug).toMatch(/^[a-z0-9]+(?:-[a-z0-9]+)*$/);
    }
  });

  it("only accepts supported selections", () => {
    for (const series of [...WORKSPACE_NAME_SERIES, "all"]) {
      expect(isWorkspaceNameSelection(series)).toBe(true);
    }
    for (const value of [null, undefined, {}, [], 1, "", "constructor", "stars"]) {
      expect(isWorkspaceNameSelection(value)).toBe(false);
    }
  });
});

describe("workspace name generator", () => {
  it("localizes the name while retaining its English URL and four-character suffix", () => {
    const english = createWorkspaceNameGenerator(() => 0)("en", "workshop");
    const chinese = createWorkspaceNameGenerator(() => 0)("zh-Hans", "workshop");
    expect(english.name).toBe(WORKSPACE_NAMES.workshop[0]!.en);
    expect(chinese.name).toBe(WORKSPACE_NAMES.workshop[0]!.zh);
    expect(chinese.id).toBe(english.id);
    expect(chinese.slug).toBe(english.slug);
    expect(english.slug).toBe(`${WORKSPACE_NAMES.workshop[0]!.slug}-aaaa`);
    expect(createWorkspaceNameGenerator(() => 0.999)("en", "workshop").slug)
      .toMatch(/-9999$/);
  });

  it.each(WORKSPACE_NAME_SERIES)("exhausts %s before repeating and keeps going", (series) => {
    const generate = createWorkspaceNameGenerator(() => 0.999);
    const generated = WORKSPACE_NAMES[series].map(() => generate("en", series));
    expect(new Set(generated.map((entry) => entry.id)).size).toBe(generated.length);
    expect(new Set(generated.map((entry) => entry.id)))
      .toEqual(new Set(WORKSPACE_NAMES[series].map((entry) => entry.id)));
    const afterReset = generate("en", series);
    expect(afterReset.id).not.toBe(generated.at(-1)!.id);
  });

  it("shares seen identities across selections without clearing another series", () => {
    const generate = createWorkspaceNameGenerator(() => 0);
    const first = generate("en", "workshop");
    for (let count = 0; count <= WORKSPACE_NAMES.computing.length; count += 1) {
      generate("en", "computing");
    }
    const next = generate("zh-Hans", "all");
    expect(next.id).toBe(WORKSPACE_NAMES.workshop[1]!.id);
    expect(next.id).not.toBe(first.id);
    expect(next.name).toBe(WORKSPACE_NAMES.workshop[1]!.zh);
  });

  it("excludes the last identity when RNG targets it immediately after exhaustion", () => {
    let draw = 0;
    const generate = createWorkspaceNameGenerator(() => draw);
    const entries = WORKSPACE_NAMES.workshop.map(() => generate("en", "workshop"));
    // Ascending generation ends on the final item; target that item after reset.
    draw = 0.999;
    expect(generate("en", "workshop").id).not.toBe(entries.at(-1)!.id);
  });

  it("samples All series uniformly even when available pool sizes differ", () => {
    for (const [index, series] of WORKSPACE_NAME_SERIES.entries()) {
      let draw = 0;
      const generate = createWorkspaceNameGenerator(() => draw);
      // Leave only one workshop name, so weighting entries would bias this draw.
      for (let count = 1; count < WORKSPACE_NAMES.workshop.length; count += 1) {
        generate("en", "workshop");
      }
      draw = (index + 0.5) / WORKSPACE_NAME_SERIES.length;
      const generated = generate("en", "all");
      expect(WORKSPACE_NAMES[series].some((entry) => entry.id === generated.id)).toBe(true);
    }
  });

  it("exhausts the entire catalog in All and never immediately repeats after reset", () => {
    const generate = createWorkspaceNameGenerator(() => 0.999);
    const names = Object.values(WORKSPACE_NAMES).flat();
    const ids = names.map(() => generate("en", "all").id);
    expect(new Set(ids).size).toBe(names.length);
    expect(generate("en", "all").id).not.toBe(ids.at(-1));
  });

  it("starts independent creation sessions with independent history", () => {
    const first = createWorkspaceNameGenerator(() => 0);
    const second = createWorkspaceNameGenerator(() => 0);
    const initial = first("en", "space");
    first("en", "space");
    expect(second("en", "space")).toEqual(initial);
  });
});
