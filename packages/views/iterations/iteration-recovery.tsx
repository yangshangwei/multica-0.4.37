"use client";
import { useState } from "react";
import { useIsMutating, useQueryClient } from "@tanstack/react-query";
import {
  useIterationCommand,
  usePendingIterationCommands,
  iterationChoicesOptions,
  iterationDetailOptions,
  type PendingIterationCommand,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { IterationError } from "./iteration-error";
import { iterationDisclosureClass } from "./iteration-presentation";
import { useT } from "../i18n";

/** Recovery survives closed periods, disabled settings and a disabled rollout. */
export function IterationRecovery({ wsId }: { wsId: string }) {
  const { t } = useT("projects");
  const pending = usePendingIterationCommands(wsId);
  const processing = useIsMutating({ mutationKey: ["iterations", wsId, "command"] }) > 0;
  const [failure, setFailure] = useState<unknown>(null);
  const [confirmed, setConfirmed] = useState(false);
  if (pending.length === 0 && !failure && !confirmed) return null;
  return (
    <section
      aria-label={t(($) => $.iterations.recoveryTitle)}
      className="space-y-3 px-6 pt-4"
    >
      {pending.length > 0 && (
        <p role="status">{t(($) => processing ? $.iterations.processing : $.iterations.unknownResult)}</p>
      )}
      {pending.map((entry) => (
        <RecoveryCommand
          key={entry.command.body.request_id}
          wsId={wsId}
          entry={entry}
          onFailure={(error) => {
            setConfirmed(false);
            setFailure(error);
          }}
          onConfirmed={() => {
            setFailure(null);
            setConfirmed(true);
          }}
        />
      ))}
      {failure !== null && <IterationError error={failure} />}
      {confirmed && (
        <p role="status">{t(($) => $.iterations.recoveryConfirmed)}</p>
      )}
    </section>
  );
}
function RecoveryCommand({
  wsId,
  entry,
  onFailure,
  onConfirmed,
}: {
  wsId: string;
  entry: PendingIterationCommand;
  onFailure: (error: unknown) => void;
  onConfirmed: () => void;
}) {
  const { t } = useT("projects");
  const client = useQueryClient();
  const mutation = useIterationCommand(wsId, entry.scope);
  const processing = useIsMutating({ mutationKey: ["iterations", wsId, "command", entry.scope] }) > 0;
  const { command } = entry;
  const operation = command.kind === "operation" ? command.body.draft.operation : command.kind;
  const operationLabel = t(($) => {
    switch (operation) {
      case "create": return $.iterations.create;
      case "edit": return $.iterations.edit;
      case "enable": return $.iterations.enable;
      case "disable": return $.iterations.disable;
      case "start": return $.iterations.start;
      case "end": return $.iterations.end;
      case "cancel": return $.iterations.cancel;
      case "delete": return $.iterations.delete;
      case "handoff": return $.iterations.handoff;
      default: return $.iterations.assign;
    }
  });
  const subjectId = command.kind === "edit" ? command.id
    : command.kind === "operation" ? command.body.draft.iteration_id ?? command.body.draft.start?.target_id ?? command.body.draft.moves[0]?.target_id
    : undefined;
  const catalogue = client.getQueryData(iterationChoicesOptions(wsId).queryKey);
  const knownName = subjectId
    ? client.getQueryData(iterationDetailOptions(wsId, subjectId).queryKey)?.iteration.name ?? catalogue?.find((item) => item.id === subjectId)?.name
    : undefined;
  const reason = command.kind === "edit" ? command.body.reason : command.kind === "operation" ? command.body.draft.reason : null;
  const taskId = command.kind === "operation" && operation === "move" ? command.body.draft.moves[0]?.issue_id : undefined;
  const taskIdentifier = taskId ? client.getQueryData(issueDetailOptions(wsId, taskId).queryKey)?.identifier : undefined;
  const fallbackContext = [reason?.trim(), taskId ?? subjectId].filter(Boolean).join(" · ") || t(($) => $.iterations.settings);
  const context = command.kind === "create" ? command.body.name
    : command.kind === "edit" ? command.body.fields.name ?? knownName ?? fallbackContext
    : taskId ? taskIdentifier ?? fallbackContext
    : knownName ?? fallbackContext;
  async function recover() {
    try {
      await mutation.mutateAsync({ command: entry.command, recover: true });
      onConfirmed();
    } catch (error) {
      onFailure(error);
    }
  }
  return (
    <div className="min-w-0 space-y-1">
      <Button variant="outline" className="h-auto min-h-8 max-w-full justify-start whitespace-normal text-left [overflow-wrap:anywhere] pointer-coarse:min-h-11" disabled={processing} onClick={() => void recover()}>
        {t(($) => $.iterations.audit.recoveryAction, { operation: operationLabel, context })}
      </Button>
      <details className="min-w-0">
        <summary className={iterationDisclosureClass}>{t(($) => $.iterations.audit.requestDetails)}</summary>
        <dl className="space-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.iterations.audit.requestId)}</dt><dd>{command.body.request_id}</dd></div>
          {subjectId && <div><dt>{t(($) => $.iterations.title)}</dt><dd>{subjectId}</dd></div>}
          {reason && <div><dt>{t(($) => $.iterations.reason)}</dt><dd>{reason}</dd></div>}
        </dl>
      </details>
    </div>
  );
}
