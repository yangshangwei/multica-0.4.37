import { useMemo } from "react";
import type { Agent, Label, MemberWithUser } from "@multica/core/types";
import {
  SKILL_CATEGORIES,
  type SkillCategory,
  type SkillPresentationMeta,
} from "@multica/core/skills";
import type { SkillOriginType } from "@multica/core/skills/stores";

/** The slice of a page row the facet counts need. */
export interface SkillFacetRow {
  agents: Agent[];
  creator: MemberWithUser | null;
  originType: SkillOriginType;
  meta: SkillPresentationMeta;
  /** Workspace labels embedded on the skill summary (empty on older servers). */
  labels: Label[];
}

export interface SkillListFacets {
  total: number;
  usedCount: number;
  unusedCount: number;
  categoryCounts: Record<SkillCategory, number>;
  originCounts: Map<SkillOriginType, number>;
  agentOptions: Map<string, { agent: Agent; count: number }>;
  creatorOptions: Map<string, { member: MemberWithUser; count: number }>;
  /** Label id -> the label and how many skills carry it, in first-seen order. */
  labelOptions: Map<string, { label: Label; count: number }>;
}

/**
 * Option lists and counts for the toolbar, the category sidebar and the
 * chips, derived from the UNFILTERED rows so toggling one dimension never
 * makes another dimension's options vanish. One implementation so the
 * sidebar and the filter dropdown can never disagree.
 */
export function computeSkillListFacets(
  rows: readonly SkillFacetRow[],
): SkillListFacets {
  const categoryCounts = Object.fromEntries(
    SKILL_CATEGORIES.map((c) => [c, 0]),
  ) as Record<SkillCategory, number>;
  const originCounts = new Map<SkillOriginType, number>();
  const agentOptions = new Map<string, { agent: Agent; count: number }>();
  const creatorOptions = new Map<
    string,
    { member: MemberWithUser; count: number }
  >();
  const labelOptions = new Map<string, { label: Label; count: number }>();
  let usedCount = 0;

  for (const row of rows) {
    categoryCounts[row.meta.category] += 1;
    originCounts.set(row.originType, (originCounts.get(row.originType) ?? 0) + 1);
    if (row.agents.length > 0) usedCount += 1;
    for (const agent of row.agents) {
      const entry = agentOptions.get(agent.id);
      if (entry) entry.count += 1;
      else agentOptions.set(agent.id, { agent, count: 1 });
    }
    if (row.creator) {
      const entry = creatorOptions.get(row.creator.user_id);
      if (entry) entry.count += 1;
      else creatorOptions.set(row.creator.user_id, { member: row.creator, count: 1 });
    }
    for (const label of row.labels) {
      const entry = labelOptions.get(label.id);
      if (entry) entry.count += 1;
      else labelOptions.set(label.id, { label, count: 1 });
    }
  }

  return {
    total: rows.length,
    usedCount,
    unusedCount: rows.length - usedCount,
    categoryCounts,
    originCounts,
    agentOptions,
    creatorOptions,
    labelOptions,
  };
}

export function useSkillListFacets(rows: readonly SkillFacetRow[]): SkillListFacets {
  return useMemo(() => computeSkillListFacets(rows), [rows]);
}
