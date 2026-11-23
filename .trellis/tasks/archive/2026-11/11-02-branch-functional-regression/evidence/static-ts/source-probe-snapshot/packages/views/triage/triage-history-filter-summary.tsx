"use client";

import type { MemberWithUser } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import {
  TRIAGE_CONTROL,
  TRIAGE_HISTORY_ACTIONS,
  type TriageHistoryFilter,
} from "./triage-ui";

export function TriageHistoryFilterSummary({
  filters,
  members,
  onClear,
}: {
  filters: readonly TriageHistoryFilter[];
  members: readonly Pick<MemberWithUser, "user_id" | "name" | "email">[];
  onClear: () => void;
}) {
  const { t } = useT("triage");
  if (filters.length === 0) return null;

  const displayValue = ({ key, value }: TriageHistoryFilter) => {
    if (key === "source" && (value === "manual" || value === "csv"))
      return t(($) => $[value]);
    if (key === "result") {
      const action = TRIAGE_HISTORY_ACTIONS.find((name) => name === value);
      return action ? t(($) => $[action]) : value;
    }
    if (key === "processed_by") {
      const member = members.find((member) => member.user_id === value);
      return member?.name || member?.email || value;
    }
    // Date-only query values keep the exact calendar date entered in the filter.
    return value;
  };

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 px-4 pb-3">
      <ul
        aria-label={t(($) => $.history_filters.label)}
        className="flex min-w-0 flex-wrap gap-2"
      >
        {filters.map((filter) => {
          const key = filter.key;
          return (
            <li
              key={key}
              className="max-w-full rounded-md bg-muted px-2 py-1 text-caption text-foreground [overflow-wrap:anywhere]"
            >
              {t(($) => $.history_filters.condition, {
                label:
                  key === "q"
                    ? t(($) => $.history_filters.search)
                    : t(($) => $[key]),
                value: displayValue(filter),
              })}
            </li>
          );
        })}
      </ul>
      <Button variant="ghost" className={TRIAGE_CONTROL} onClick={onClear}>
        {t(($) => $.clear_filters)}
      </Button>
    </div>
  );
}
