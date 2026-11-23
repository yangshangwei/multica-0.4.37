import { useT } from "../i18n";
import { useStatusCategoryLabel } from "../issues/utils/status-label";
export function useIterationLabels() {
  const { t } = useT("projects");
  const category = useStatusCategoryLabel(
    t(($) => $.iterations.unknownTaskStatus),
  );
  const status = (value: string) => {
    switch (value) {
      case "planned":
        return t(($) => $.iterations.planned);
      case "active":
        return t(($) => $.iterations.active);
      case "completed":
        return t(($) => $.iterations.completed);
      case "cancelled":
        return t(($) => $.iterations.cancelled);
      default:
        return t(($) => $.iterations.unknown);
    }
  };
  const validation = (code: string) => {
    switch (code) {
      case "iteration_revision_conflict":
      case "iteration_preview_stale":
        return t(($) => $.iterations.conflict);
      case "iteration_reason_required":
        return t(($) => $.iterations.validationReason);
      case "iteration_terminal_choice_required":
      case "iteration_terminal_choice_invalid":
        return t(($) => $.iterations.validationTerminal);
      case "iteration_active_conflict":
        return t(($) => $.iterations.validationActive);
      case "iteration_move_set_invalid":
        return t(($) => $.iterations.validationMoves);
      case "iteration_terminal_target_invalid":
      case "iteration_history_move_unsupported":
        return t(($) => $.iterations.validationTarget);
      case "triage_review_required":
        return t(($) => $.iterations.validationTriage);
      case "issue_not_found":
        return t(($) => $.iterations.validationMissing);
      case "iteration_rollover_exhausted":
        return t(($) => $.iterations.validationRollover);
      case "iteration_delete_requires_unused_plan":
        return t(($) => $.iterations.validationDelete);
      case "iteration_start_dates_invalid":
        return t(($) => $.iterations.validationDates);
      default:
        return t(($) => $.iterations.validationUnknown);
    }
  };
  return { status, category, validation };
}
