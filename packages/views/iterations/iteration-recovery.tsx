"use client";
import { useState } from "react";
import { useIsMutating } from "@tanstack/react-query";
import {
  useIterationCommand,
  usePendingIterationCommands,
  type PendingIterationCommand,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { IterationError } from "./iteration-error";
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
  const mutation = useIterationCommand(wsId, entry.scope);
  const processing = useIsMutating({ mutationKey: ["iterations", wsId, "command", entry.scope] }) > 0;
  async function recover() {
    try {
      await mutation.mutateAsync({ command: entry.command, recover: true });
      onConfirmed();
    } catch (error) {
      onFailure(error);
    }
  }
  return (
    <Button disabled={processing} onClick={() => void recover()}>
      {t(($) => $.iterations.recover)}
    </Button>
  );
}
