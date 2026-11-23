import type { IssueCreateDefaults } from "./types";

export function mergeIssueCreateDefaults(...layers: Array<IssueCreateDefaults | undefined>): IssueCreateDefaults {
  const merged: IssueCreateDefaults = {};
  for (const layer of layers) {
    if (!layer) continue;
    Object.assign(merged, layer);
    if (Object.hasOwn(layer, "assignee_type") || Object.hasOwn(layer, "assignee_id")) {
      // An override owns the complete identity. Clearing either field or
      // supplying an incomplete pair must never keep half of an older target.
      const complete = !!layer.assignee_type && !!layer.assignee_id;
      merged.assignee_type = complete ? layer.assignee_type : null;
      merged.assignee_id = complete ? layer.assignee_id : null;
    }
  }
  return merged;
}
