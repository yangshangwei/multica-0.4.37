import { api } from "../api";
import type { Iteration, IterationDraft } from "../api/iteration-schemas";

export async function prepareIterationDraft(
  wsId: string,
  iteration: Iteration | null,
  settingsRevision: number,
  operation: IterationDraft["operation"],
  reason: string,
  targetId: string | null,
  startMode: "scheduled" | "today",
  retainTerminal: boolean,
): Promise<IterationDraft> {
  const draft: IterationDraft = {
    operation,
    iteration_id: iteration?.id ?? null,
    expected_iteration_revision: iteration?.revision ?? null,
    expected_scope_revision: iteration?.scope_revision ?? null,
    expected_settings_revision: settingsRevision,
    reason: reason || null,
    moves: [],
    start: null,
  };
  if (operation === "disable") return draft;
  if (!iteration) throw new Error("An iteration is required");
  if (
    operation === "delete" ||
    (operation === "cancel" && iteration.status === "planned")
  )
    return draft;

  if (operation === "start" || operation === "handoff") {
    const target = operation === "start" ? iteration.id : targetId;
    if (!target) throw new Error("Choose the next iteration");
    draft.start = { target_id: target, mode: startMode, terminal_choices: [] };
  }

  // An incomplete draft still yields the complete authorized source/target
  // facts and validation items. The final preview confirms the chosen moves;
  // neither pagination nor one HTTP request per task determines this set.
  const initial = await api.previewIteration(wsId, draft);
  if (initial.complete !== true)
    throw new Error("A complete iteration preview is required");
  const closesSource =
    ["end", "cancel", "handoff"].includes(operation) &&
    iteration.status === "active";
  const terminal = (category: string) =>
    ["done", "cancelled"].includes(category);
  return {
    ...draft,
    moves: closesSource
      ? initial.issues
          .filter(
            (issue) =>
              issue.source_id === iteration.id &&
              !terminal(issue.status_category),
          )
          .map((issue) => ({
            issue_id: issue.issue_id,
            expected_issue_revision: issue.revision,
            expected_source_id: iteration.id,
            target_id: targetId,
            allow_completed: false,
          }))
      : [],
    start: draft.start
      ? {
          ...draft.start,
          terminal_choices: initial.issues
            .filter(
              (issue) =>
                issue.source_id === draft.start!.target_id &&
                terminal(issue.status_category),
            )
            .map((issue) => ({
              issue_id: issue.issue_id,
              retain: retainTerminal,
            })),
        }
      : null,
  };
}
