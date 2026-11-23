import { z } from "zod";

// The answer of `GET /api/issue-budgets/{rootId}` (design 12.9, "HTTP v1").
//
// Amounts and revisions are 64-bit values. The server writes them as decimal
// strings because a JSON number cannot carry them, and they stay strings
// here. A number in their place fails the parse: it may already have lost
// digits on its way through a JSON reader.
const decimal = z.string().regex(/^(0|[1-9][0-9]*)$/);

// How many of something. Small enough to be a JSON number.
const count = z.number().int().nonnegative();

// States, modes, reasons and names are strings and not closed lists: a later
// server may add one, and a value this version does not know must not cost
// the rest of the answer. A field that may be null is still required: an
// absent key is a malformed answer, not "no value".
export const IssueBudgetSchema = z.object({
  budget_root_id: z.string(),
  root_issue_id: z.string(),
  admission_state: z.string(),
  mode: z.string(),
  policy_revision: decimal,
  policy_epoch: decimal,
  ledger_revision: decimal,
  activated_at: z.string(),
  settled_as_of_revision: decimal.nullable(),
  limit_ticks: decimal.nullable(),
  attempt_reserve_ticks: decimal.nullable(),
  ledger_state: z.string(),
  amounts: z.object({
    actual_known_ticks: decimal,
    estimated_ticks: decimal,
    reserved_ticks: decimal,
    unknown_hold_ticks: decimal,
    overrun_ticks: decimal,
  }).nullable(),
  unknown: z.object({
    by_reason: z.array(z.object({ reason: z.string(), count })),
    pre_activation: count,
  }),
  total_known: z.boolean(),
  remaining_ticks: decimal.nullable(),
  coverage: z.object({
    included_attempts: count.nullable(),
    not_applicable: z.array(z.string()),
  }),
});

export type IssueBudget = z.infer<typeof IssueBudgetSchema>;
