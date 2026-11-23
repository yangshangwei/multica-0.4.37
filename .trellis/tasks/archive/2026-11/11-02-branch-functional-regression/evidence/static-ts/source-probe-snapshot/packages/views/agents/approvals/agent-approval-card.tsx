"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { toast } from "sonner";
import {
  useCancelAgentApproval,
  useDecideAgentApproval,
} from "@multica/core/agent-approvals";
import type { AgentApproval } from "@multica/core/types";
import {
  isApprovalPending,
  isKnownApprovalRiskClass,
  isKnownApprovalStatus,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Label } from "@multica/ui/components/ui/label";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../../i18n";

/**
 * One approval request, and the only place in the product where a person can
 * authorize a high-risk agent action.
 *
 * Two rules shape this component:
 *
 *   - Approving is never one click. It opens a confirmation that restates the
 *     single action being authorized, because the approval is exactly as narrow
 *     as its wording and a mis-click here authorizes a production operation.
 *   - Status and risk class arrive as strings, not unions. An unrecognised value
 *     renders as itself rather than being hidden: "some high-risk action this
 *     build has no name for" is still something a person must read, and the
 *     buttons key off isApprovalPending, never off a client-side vocabulary.
 */

/** Which decision the dialog is confirming. `null` means it is closed. */
type PendingDecision = "approve" | "reject" | "withdraw" | null;

/** Risk classes rendered as a warning rather than plain: the irreversible ones. */
const severeRiskClasses = new Set([
  "production_release",
  "database_migration",
  "secret_access",
  "destructive_operation",
]);

export function AgentApprovalCard({
  approval,
  agentName,
}: {
  approval: AgentApproval;
  agentName: string | null;
}) {
  const { t } = useT("agents");
  const timeAgo = useTimeAgo();
  const [expanded, setExpanded] = useState(false);
  const [decision, setDecision] = useState<PendingDecision>(null);
  const [note, setNote] = useState("");

  const decide = useDecideAgentApproval();
  const withdraw = useCancelAgentApproval();
  const busy = decide.isPending || withdraw.isPending;

  const actor = agentName ?? t(($) => $.approvals.unknown_agent);
  const pending = isApprovalPending(approval.status);

  function closeDialog() {
    setDecision(null);
    setNote("");
  }

  async function confirm() {
    const mode = decision;
    if (!mode) return;
    try {
      if (mode === "withdraw") {
        await withdraw.mutateAsync(approval.id);
        toast.success(t(($) => $.approvals.withdrawn_toast));
      } else {
        await decide.mutateAsync({
          id: approval.id,
          decision: mode === "approve" ? "approve" : "reject",
          note: note.trim() || undefined,
        });
        toast.success(
          mode === "approve"
            ? t(($) => $.approvals.approved_toast, { agent: actor })
            : t(($) => $.approvals.rejected_toast),
        );
      }
      closeDialog();
    } catch (error) {
      // The 409 case is the one worth surfacing verbatim: someone decided first,
      // and the server's sentence says so better than a generic failure would.
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(($) => $.approvals.decide_failed),
      );
    }
  }

  return (
    <div className="border-b border-border px-4 py-3 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <RiskBadge riskClass={approval.risk_class} />
        <StatusBadge status={approval.status} />
        <span className="text-caption text-muted-foreground">{actor}</span>
        <span className="text-caption text-muted-foreground">
          {t(($) => $.approvals.filed, { when: timeAgo(approval.created_at) })}
        </span>
      </div>

      <p className="mt-2 text-body break-words">{approval.summary}</p>

      <div className="mt-2 flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-7 gap-1 px-1.5 text-caption text-muted-foreground"
          aria-expanded={expanded}
          onClick={() => setExpanded((open) => !open)}
        >
          {expanded ? (
            <ChevronDown aria-hidden="true" className="size-3.5" />
          ) : (
            <ChevronRight aria-hidden="true" className="size-3.5" />
          )}
          {expanded
            ? t(($) => $.approvals.hide_detail)
            : t(($) => $.approvals.show_detail)}
        </Button>
        {pending ? (
          <div className="ml-auto flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-7 text-caption text-muted-foreground"
              disabled={busy}
              onClick={() => setDecision("withdraw")}
            >
              {t(($) => $.approvals.withdraw)}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7"
              disabled={busy}
              onClick={() => setDecision("reject")}
            >
              {t(($) => $.approvals.reject)}
            </Button>
            <Button
              type="button"
              size="sm"
              className="h-7"
              disabled={busy}
              onClick={() => setDecision("approve")}
            >
              {t(($) => $.approvals.approve)}
            </Button>
          </div>
        ) : null}
      </div>

      {expanded ? (
        <ApprovalDetail approval={approval} />
      ) : null}

      <DecisionDialog
        mode={decision}
        actor={actor}
        summary={approval.summary}
        note={note}
        onNoteChange={setNote}
        busy={busy}
        onCancel={closeDialog}
        onConfirm={confirm}
      />
    </div>
  );
}

/**
 * The plan and whatever has been recorded against the request since.
 *
 * The plan is the thing being reviewed, so it is rendered verbatim in a
 * monospaced block that wraps rather than truncates — a reviewer who cannot see
 * the last line of a command is not reviewing it. An empty plan says so out loud
 * instead of rendering nothing, because "no plan attached" is a reason to reject.
 */
function ApprovalDetail({ approval }: { approval: AgentApproval }) {
  const { t } = useT("agents");
  const timeAgo = useTimeAgo();

  return (
    <div className="mt-2 space-y-3 rounded-md border border-border bg-muted/30 p-3">
      <section>
        <h3 className="text-caption font-medium text-muted-foreground">
          {t(($) => $.approvals.plan_label)}
        </h3>
        {approval.plan.trim() ? (
          <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words font-mono text-caption leading-6 text-foreground">
            {approval.plan}
          </pre>
        ) : (
          <p className="mt-1 text-caption text-muted-foreground">
            {t(($) => $.approvals.plan_empty)}
          </p>
        )}
      </section>

      {approval.decided_at ? (
        <section>
          <h3 className="text-caption font-medium text-muted-foreground">
            {t(($) => $.approvals.decided, {
              when: timeAgo(approval.decided_at),
            })}
          </h3>
          {approval.decision_note ? (
            <p className="mt-1 whitespace-pre-wrap break-words text-caption text-foreground">
              {approval.decision_note}
            </p>
          ) : null}
        </section>
      ) : null}

      {approval.executed_at ? (
        <section>
          <h3 className="text-caption font-medium text-muted-foreground">
            {t(($) => $.approvals.executed_at, {
              when: timeAgo(approval.executed_at),
            })}
          </h3>
          {approval.execution_note ? (
            <p className="mt-1 whitespace-pre-wrap break-words text-caption text-foreground">
              {approval.execution_note}
            </p>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}

/**
 * The confirmation step. Approving restates the action and says out loud what the
 * approval does not cover, because the server enforces that narrowness and a
 * person who thought they were granting a standing permission would be wrong.
 *
 * Withdrawing takes no note: nothing is being decided, so there is no decision to
 * annotate.
 */
function DecisionDialog({
  mode,
  actor,
  summary,
  note,
  onNoteChange,
  busy,
  onCancel,
  onConfirm,
}: {
  mode: PendingDecision;
  actor: string;
  summary: string;
  note: string;
  onNoteChange: (value: string) => void;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const { t } = useT("agents");
  if (!mode) return null;

  const title =
    mode === "approve"
      ? t(($) => $.approvals.confirm_approve_title)
      : mode === "reject"
        ? t(($) => $.approvals.confirm_reject_title)
        : t(($) => $.approvals.confirm_withdraw_title);
  const body =
    mode === "approve"
      ? t(($) => $.approvals.confirm_approve_body, { agent: actor })
      : mode === "reject"
        ? t(($) => $.approvals.confirm_reject_body, { agent: actor })
        : t(($) => $.approvals.confirm_withdraw_body);
  const confirmLabel =
    mode === "approve"
      ? t(($) => $.approvals.confirm_approve_action)
      : mode === "reject"
        ? t(($) => $.approvals.reject)
        : t(($) => $.approvals.withdraw);

  return (
    <Dialog open onOpenChange={(open) => (open ? undefined : onCancel())}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{body}</DialogDescription>
        </DialogHeader>

        <p className="rounded-md border border-border bg-muted/40 px-3 py-2 text-body break-words">
          {summary}
        </p>

        {mode === "approve" ? (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.approvals.confirm_approve_warning)}
          </p>
        ) : null}

        {mode === "withdraw" ? null : (
          <div className="space-y-1.5">
            <Label htmlFor="approval-decision-note">
              {t(($) => $.approvals.note_label)}
            </Label>
            <Textarea
              id="approval-decision-note"
              value={note}
              onChange={(event) => onNoteChange(event.target.value)}
              placeholder={t(($) => $.approvals.note_placeholder)}
              rows={3}
              className="resize-y text-label"
            />
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" size="sm" onClick={onCancel}>
            {t(($) => $.approvals.keep_open)}
          </Button>
          <Button
            type="button"
            size="sm"
            variant={mode === "approve" ? "default" : "destructive"}
            disabled={busy}
            onClick={onConfirm}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The status, localized when this build knows the value and shown raw when it does
 * not. Never a source of truth about authorization — that is isApprovalActionable
 * on the server's own word.
 */
export function StatusBadge({ status }: { status: string }) {
  const { t } = useT("agents");
  const known = isKnownApprovalStatus(status);
  return (
    <Badge
      variant={
        status === "approved" || status === "executed"
          ? "secondary"
          : status === "rejected"
            ? "destructive"
            : "outline"
      }
    >
      {known ? t(($) => $.approvals.status[status]) : status}
    </Badge>
  );
}

/** The risk class, same fallback rule as the status. */
export function RiskBadge({
  riskClass,
  className,
}: {
  riskClass: string;
  className?: string;
}) {
  const { t } = useT("agents");
  const known = isKnownApprovalRiskClass(riskClass);
  return (
    <Badge
      variant={severeRiskClasses.has(riskClass) ? "destructive" : "outline"}
      className={cn(className)}
    >
      {known ? t(($) => $.approvals.risk[riskClass]) : riskClass}
    </Badge>
  );
}
