// Presentation metadata for a workspace skill: category and icon.
//
// Stored in `skill.config.presentation` (JSONB) alongside `origin` and
// `template_source`. This file is the single source of truth for the category
// enum and the icon whitelist — the Go side mirrors these constants in
// `server/internal/skill/presentation.go` and a parity test keeps both lists
// identical. Views map `SkillIconName` to Lucide components in
// `packages/views/skills/lib/skill-presentation-icon.ts`; this module stays
// headless and never imports lucide.
//
// Labels are NOT part of presentation: a skill's labels are workspace labels
// (`resource_type = "skill"`) attached through `skill_to_label` and embedded
// on `SkillSummary.labels` by the list endpoint.

// Fixed display order: the first five trace the delivery path from planning to
// release; the last three are supporting capabilities that span multiple
// stages. `design` and `quality` were added after the original six — the six
// stable keys are preserved (no JSONB migration) and only gain new display
// names. See docs/skills for the primary-category vs. label model.
export const SKILL_CATEGORIES = [
  "research",
  "design",
  "engineering",
  "quality",
  "operations",
  "writing",
  "data",
  "other",
] as const;

export type SkillCategory = (typeof SKILL_CATEGORIES)[number];

export const DEFAULT_SKILL_CATEGORY: SkillCategory = "other";

// Curated Lucide icon names in kebab-case (the official Lucide identifier).
// Kept deliberately small: every entry is a named import in views, and the Go
// validator rejects anything outside this list.
export const SKILL_ICON_NAMES = [
  "bar-chart",
  "bell",
  "book-open",
  "book-open-text",
  "bot",
  "braces",
  "brain",
  "bug",
  "calendar",
  "chart-line",
  "chart-no-axes-column",
  "chart-pie",
  "clipboard-check",
  "cloud",
  "code",
  "compass",
  "database",
  "file-spreadsheet",
  "file-text",
  "filter",
  "flask-conical",
  "git-branch",
  "git-pull-request",
  "globe",
  "key",
  "landmark",
  "languages",
  "lightbulb",
  "list-checks",
  "lock",
  "mail",
  "megaphone",
  "message-circle-question",
  "microscope",
  "newspaper",
  "package",
  "palette",
  "pen-line",
  "presentation",
  "receipt",
  "repeat",
  "rocket",
  "search",
  "server",
  "shield-check",
  "sparkles",
  "table",
  "terminal",
  "test-tube",
  "timer",
  "workflow",
  "wrench",
] as const;

export type SkillIconName = (typeof SKILL_ICON_NAMES)[number];

/** Icon shown when a skill has no explicit icon override. */
export const SKILL_CATEGORY_DEFAULT_ICON: Record<SkillCategory, SkillIconName> = {
  research: "list-checks",
  design: "landmark",
  engineering: "code",
  quality: "clipboard-check",
  operations: "rocket",
  writing: "book-open-text",
  data: "database",
  other: "wrench",
};

export interface SkillPresentationMeta {
  category: SkillCategory;
  /** Explicit override, or null to follow the category default. */
  icon: SkillIconName | null;
}

export const EMPTY_SKILL_PRESENTATION: SkillPresentationMeta = {
  category: DEFAULT_SKILL_CATEGORY,
  icon: null,
};

export function isSkillCategory(value: unknown): value is SkillCategory {
  return typeof value === "string" && (SKILL_CATEGORIES as readonly string[]).includes(value);
}

export function isSkillIconName(value: unknown): value is SkillIconName {
  return typeof value === "string" && (SKILL_ICON_NAMES as readonly string[]).includes(value);
}

/**
 * Tolerant reader: any missing, mistyped, or out-of-whitelist field falls back
 * to its default. Never throws — list rendering must survive hand-edited
 * config. An icon equal to the category default is reported as `null` so the
 * UI's "follow category" state is canonical. Unknown keys (including a legacy
 * `tags` array) are ignored.
 */
export function readSkillPresentationMeta(
  config: Record<string, unknown> | null | undefined,
): SkillPresentationMeta {
  const raw = config?.presentation;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return EMPTY_SKILL_PRESENTATION;
  }
  const rec = raw as Record<string, unknown>;
  const category = isSkillCategory(rec.category) ? rec.category : DEFAULT_SKILL_CATEGORY;
  const icon =
    isSkillIconName(rec.icon) && rec.icon !== SKILL_CATEGORY_DEFAULT_ICON[category]
      ? rec.icon
      : null;
  if (category === DEFAULT_SKILL_CATEGORY && icon === null) {
    return EMPTY_SKILL_PRESENTATION;
  }
  return { category, icon };
}

/**
 * Resolve the icon to draw: explicit override, else the category default.
 */
export function resolveSkillIconName(meta: SkillPresentationMeta): SkillIconName {
  return meta.icon ?? SKILL_CATEGORY_DEFAULT_ICON[meta.category];
}

/**
 * Merge presentation metadata back into a config object without touching
 * sibling keys (`origin`, `template_source`, ...). Returns a new object; the
 * input is not mutated. Writes the canonical shape: no `icon` key when
 * following the category default, and drops the whole `presentation` key when
 * everything is at its default so untouched skills keep a clean config.
 */
export function writeSkillPresentationMeta(
  config: Record<string, unknown> | null | undefined,
  meta: SkillPresentationMeta,
): Record<string, unknown> {
  const { presentation: _dropped, ...rest } = config ?? {};
  void _dropped;
  const category = isSkillCategory(meta.category) ? meta.category : DEFAULT_SKILL_CATEGORY;
  const icon =
    isSkillIconName(meta.icon) && meta.icon !== SKILL_CATEGORY_DEFAULT_ICON[category]
      ? meta.icon
      : null;

  const presentation: Record<string, unknown> = {};
  if (category !== DEFAULT_SKILL_CATEGORY) presentation.category = category;
  if (icon) presentation.icon = icon;
  // `category` is always written when anything else is present so the stored
  // record is self-describing even for the default bucket.
  if (Object.keys(presentation).length > 0 && !("category" in presentation)) {
    presentation.category = category;
  }

  return Object.keys(presentation).length > 0 ? { ...rest, presentation } : rest;
}
