import type { ProjectStatus, ProjectPriority, ConfigureProjectSquadRequest } from "../types";
import { createDraftStore } from "../drafts/create-draft-store";

interface ProjectDraft {
  title: string;
  description: string;
  status: ProjectStatus;
  priority: ProjectPriority;
  leadType?: "member" | "agent";
  leadId?: string;
  icon?: string;
  // Calendar days ("YYYY-MM-DD"); empty/undefined means unset.
  startDate?: string;
  dueDate?: string;
  // Undefined uses the recommendation; null or an empty object opts out.
  executionSquad?: ConfigureProjectSquadRequest | null;
  // Undefined restores the legacy choice or the recommendation; [] opts out.
  executionSquads?: ConfigureProjectSquadRequest[];
}

const EMPTY_DRAFT: ProjectDraft = {
  title: "",
  description: "",
  status: "planned",
  priority: "none",
  leadType: undefined,
  leadId: undefined,
  icon: undefined,
  startDate: undefined,
  dueDate: undefined,
  executionSquad: undefined,
  executionSquads: undefined,
};

export const useProjectDraftStore = createDraftStore<ProjectDraft>({
  storageKey: "multica_project_draft",
  emptyData: EMPTY_DRAFT,
  hasMeaningful: (d) => !!(d.title || d.description),
});
