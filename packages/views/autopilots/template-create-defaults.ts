import type { AutopilotAssigneeType } from "@multica/core/types";

export interface AutopilotTemplateDefaults {
  initialProjectId?: string | null;
  initialAssigneeType?: AutopilotAssigneeType | null;
  initialAssigneeId?: string | null;
}

export function autopilotTemplateDefaultsFromSearch(
  search: URLSearchParams,
): AutopilotTemplateDefaults {
  const type = search.get("assignee_type");
  return {
    initialProjectId: search.get("project_id"),
    initialAssigneeType: type === "agent" || type === "squad" ? type : null,
    initialAssigneeId: search.get("assignee_id"),
  };
}

export function autopilotTemplateHref(
  path: string,
  defaults: AutopilotTemplateDefaults = {},
  templateKey?: string | null,
): string {
  const search = new URLSearchParams();
  if (templateKey) search.set("template", templateKey);
  if (defaults.initialProjectId) search.set("project_id", defaults.initialProjectId);
  if (defaults.initialAssigneeType && defaults.initialAssigneeId) {
    search.set("assignee_type", defaults.initialAssigneeType);
    search.set("assignee_id", defaults.initialAssigneeId);
  }
  return search.size ? `${path}?${search}` : path;
}
