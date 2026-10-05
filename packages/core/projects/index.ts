export { projectKeys, projectListOptions, projectDetailOptions } from "./queries";
export { useCreateProject, useUpdateProject, useDeleteProject, useConfigureProjectSquad, useConfigureProjectSquads } from "./mutations";
export {
  DEFAULT_PROJECT_SQUAD_TEMPLATE_KEY,
  getProjectExecutionSquads,
  projectSquadSelection,
  replaceProjectSquadSelection,
  eligibleProjectRuntimes,
  selectProjectRuntime,
  projectLocalDaemonIds,
  getProjectSquadReadiness,
  type ProjectRuntimeSelectionOptions,
  type ProjectSquadReadiness,
} from "./execution-squad";
export type { ProjectExecutionSquad, ConfigureProjectSquadRequest } from "../types/project";
export { useProjectDraftStore } from "./draft-store";
export {
  useProjectViewStore,
  PROJECT_SORT_DEFAULT_DIRECTION,
  PROJECT_DEFAULT_HIDDEN_COLUMNS,
  EMPTY_PROJECT_FILTERS,
  type ProjectViewMode,
  type ProjectSortField,
  type ProjectSortDirection,
  type ProjectColumnKey,
  type ProjectListFilters,
} from "./stores/view-store";
export {
  projectResourceKeys,
  projectResourcesOptions,
  useCreateProjectResource,
  useUpdateProjectResource,
  useDeleteProjectResource,
} from "./resource-queries";

export { getProjectIssueMetrics } from "./metrics";

export * from "./p1-queries";
export * from "./p1-mutations";
export * from "./progress-draft-store";

export { ProjectDescriptionSave } from "./description-save";
export { projectGoalTemplateAppend, type ProjectGoalSection } from "./goal-template";

export { clearProtectedProjectContent, isProjectAccessLost, useProjectAccessStore, canAccessProject, markProjectDeleted, handleProjectAccessError, protectProjectRequest, registerProjectLocalTextFlush } from "./access";

export * from "./description-draft-store";
