import { useRef } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { collaborationKeys, executeCollaborationCommandCancel, executeCollaborationResource, parseCollaborationResourceDraft, parseCollaborationCommandCancelDraft, parseCollaborationControlDraft, prepareCollaborationControl, prepareCollaborationCommandCancel,
  recoverCollaborationControl, recoverCollaborationSwitch, sendCollaborationControl, sendCollaborationSwitch, useCollaborationPendingDrafts,
  type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationResourcePanel } from "./resource-panel";
import { CollaborationSwitchPanel } from "./switch-panel";
import { CollaborationRunDisclosure } from "./run-records";

export function CollaborationExecutionPanel({ scope, issue, selectedRunId }: { scope: CollaborationScope; issue: CollaborationIssue; selectedRunId?: string | null }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const pending = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const submitting = useRef(false);
  const control = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "execution-control"], retry: false, networkMode: "always", mutationFn: async ({ draft, recover }: { draft: CollaborationDraft; recover: boolean }) => {
    try { parseCollaborationControlDraft(draft); } catch {
      try { parseCollaborationCommandCancelDraft(draft); } catch {
        try { parseCollaborationResourceDraft(draft); } catch {
          return recover ? recoverCollaborationSwitch(client, scope, draft) : sendCollaborationSwitch(client, scope, draft);
        }
        return executeCollaborationResource(client, scope, draft, recover);
      }
      return executeCollaborationCommandCancel(client, scope, draft, recover);
    }
    return recover ? recoverCollaborationControl(client, scope, draft) : sendCollaborationControl(client, scope, draft);
  } });
  const receipt = control.data?.action === "stop" || control.data?.action === "resume" ? control.data : null;
  const execution = issue.execution;
  const canControl = issue.capabilities.control_ai === true && issue.write_context != null;
  const locked = control.isPending || pending.execution.length > 0 || pending.error;
  const laneName = (id: string) => t(($) => $.execution.lane, { number: (execution?.channels.findIndex((lane) => lane.id === id) ?? -1) + 1 });
  const targetName = (id: string) => {
    const target = execution?.targets.find((item) => item.id === id);
    return target ? `${target.kind === "squad" ? t(($) => $.execution.squad) : t(($) => $.execution.agent)} ${target.resource_id}` : id;
  };
  async function submit(draft: CollaborationDraft, recover = false) {
    if (submitting.current) return;
    submitting.current = true;
    try { return await control.mutateAsync({ draft, recover }); }
    catch { return undefined; }
    finally { submitting.current = false; }
  }
  function act(action: "stop" | "resume", targetId?: string) {
    const draft = prepareCollaborationControl(scope, issue, crypto.randomUUID(), action, targetId ? { scope: "target", targetId } : { scope: "issue" });
    submit(draft);
  }
  return <section aria-label={t(($) => $.execution.title)} className="min-w-0 space-y-4">
    <h3 className="text-body font-semibold">{t(($) => $.execution.title)}</h3>
    <p className="text-caption text-muted-foreground">{t(($) => $.execution.unavailable)}</p>
    <p className="text-caption text-muted-foreground">{t(($) => $.execution.stopScope)}</p>
    <p className="text-body">{labels.execution(issue.session.queue_state)}</p>
    {canControl && execution && <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={locked} onClick={() => act("stop")}>{t(($) => $.execution.stopAll)}</Button><Button variant="outline" disabled={locked || issue.session.queue_state === "closed"} onClick={() => act("resume")}>{t(($) => $.execution.resumeAll)}</Button></div>}
    {pending.error && <p role="alert" className="text-body">{t(($) => $.workflow.storageError)}</p>}
    {control.error && <CollaborationQueryState error={control.error} />}
    {control.isPending && <p role="status">{t(($) => $.workflow.pending)}</p>}
    {receipt && <div role="status" aria-label={t(($) => $.execution.controlResult)} className="space-y-2 text-body"><p>{receipt.status >= 400 ? labels.reason(receipt.code) : receipt.action === "resume"
      ? t(($) => $.execution.partial, { resumed: receipt.channels.filter((lane) => lane.queue_state === "active").length, paused: receipt.channels.filter((lane) => lane.queue_state === "paused").length }) : labels.reason(receipt.code)}</p>
      {receipt.action === "resume" && <p>{t(($) => $.execution.holdRemaining, { count: receipt.target_holds.filter((target) => target.hold_reason !== null).length })}</p>}
      <ul className="space-y-2 text-caption">{receipt.channels.map((lane) => <li key={lane.channel_id}>{laneName(lane.channel_id)} · {labels.reason(lane.code)}</li>)}
        {receipt.target_holds.map((target) => <li key={target.target_id} className="[overflow-wrap:anywhere]">{targetName(target.target_id)} · {labels.reason(target.code)}</li>)}</ul>
    </div>}
    {control.data?.action === "cancel_command" && <p role="status" className="text-body">{control.data.code === "command_cancelled" ? t(($) => $.execution.cancelledInput) : labels.reason(control.data.code)}</p>}
    {control.data && ["register_target", "consent_resource", "revoke_grant", "offer_resource", "adopt_resource_offer", "withdraw_resource_offer", "propose_target_switch", "consent_target_switch", "revoke_target_switch_consent", "commit_target_switch"].includes(control.data.action) && <p role="status" className="text-body">{labels.reason(control.data.code)}</p>}
    {control.data && "redacted" in control.data && control.data.redacted === true && <p role="status">{t(($) => $.execution.redacted)}</p>}
    {!control.isPending && pending.execution.map((draft) => <div key={draft.requestId} className="space-y-2"><p role="status" className="text-body">{t(($) => $.workflow.unknown)}</p><div className="flex flex-wrap gap-2"><Button variant="outline" disabled={pending.error} onClick={() => submit(draft, true)}>{t(($) => $.workflow.recover)}</Button><Button variant="outline" disabled={pending.error || !canControl} onClick={() => submit(draft)}>{t(($) => $.workflow.retryOriginal)}</Button></div></div>)}
    {canControl && execution && <CollaborationResourcePanel key={issue.id} scope={scope} issue={issue} locked={locked} submit={submit} />}
    {execution?.channels.map((lane, index) => {
      const targets = execution.targets.filter((target) => target.channel_id === lane.id);
      const groups = execution.groups.filter((group) => group.channel_id === lane.id);
      return <div key={lane.id} className="min-w-0 space-y-4 rounded-lg border border-border p-4">
        <div className="flex flex-wrap justify-between gap-2"><h4 className="text-body font-medium">{t(($) => $.execution.lane, { number: index + 1 })}</h4><span className="text-caption text-muted-foreground">{labels.execution(lane.queue_state)}</span></div>
        {targets.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.execution.history)}</p>}
        <ul className="space-y-4">{targets.map((target) => <li key={target.id} className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-baseline gap-2"><span className="text-body font-medium">{target.kind === "squad" ? t(($) => $.execution.squad) : t(($) => $.execution.agent)}</span><span className="break-all text-caption text-muted-foreground">{target.resource_id}</span></div>
          <p className="text-caption">{target.revoked_at !== null ? t(($) => $.execution.revoked) : target.hold_reason ? labels.execution(target.hold_reason) : t(($) => $.execution.released)}</p>
          {canControl && <div className="flex gap-2"><Button size="sm" variant="outline" disabled={locked} onClick={() => act("stop", target.id)}>{t(($) => $.execution.stop)}</Button><Button size="sm" variant="outline" disabled={locked || target.revoked_at !== null || issue.session.queue_state === "closed"} onClick={() => act("resume", target.id)}>{t(($) => $.execution.resume)}</Button></div>}
          {canControl && <CollaborationSwitchPanel scope={scope} issue={issue} targetId={target.id} locked={locked} submit={submit} />}
        </li>)}</ul>
        <ol className="space-y-4">{groups.map((group) => <li key={group.id} className="min-w-0 space-y-2">
          <p className="text-caption font-medium">{t(($) => $.execution.group)} · {labels.execution(group.state)}</p>
          <ul className="space-y-1 text-caption text-muted-foreground">{group.inputs.map((input) => <li key={input.command_id}>{t(($) => $.execution.consumed, { sequence: input.sequence })}</li>)}</ul>
          <ul className="space-y-4">{execution.runs.filter((run) => run.group_id === group.id).map((run) => <CollaborationRunDisclosure key={run.id} scope={scope} issue={issue} run={run} group={group} selectedRunId={selectedRunId} />)}</ul>
        </li>)}</ol>
        <ul className="space-y-3">{execution.commands.filter((command) => command.channel_id === lane.id && command.state === "waiting").map((command) => <li key={command.id} className="min-w-0 space-y-2 text-caption text-muted-foreground">
          <p>{t(($) => $.execution.input, { sequence: command.sequence })} · {command.group_id !== null ? t(($) => $.composer.joined) : t(($) => $.execution.waiting)}</p>
          <p className="[overflow-wrap:anywhere]">{targetName(command.target_id)}</p>
          <p className="[overflow-wrap:anywhere]">{t(($) => $.execution.author)}: {command.author_user_id === scope.userId ? t(($) => $.execution.you) : command.author_user_id}</p>
          {canControl && [command.author_user_id, issue.creator.user_id, issue.handler.user_id].includes(scope.userId) && <Button size="sm" variant="outline" disabled={locked} onClick={() => submit(prepareCollaborationCommandCancel(scope, issue, crypto.randomUUID(), command.id))}>{t(($) => $.execution.cancelInput, { sequence: command.sequence })}</Button>}
        </li>)}</ul>
      </div>;
    })}
    {execution?.targets.filter((target) => target.channel_id === null).map((target) => <p key={target.id} className="text-caption">{target.hold_reason ? labels.execution(target.hold_reason) : t(($) => $.execution.released)}</p>)}
    {selectedRunId && execution && !execution.runs.some((run) => run.id === selectedRunId) && <p role="alert" className="text-body text-muted-foreground">{t(($) => $.records.unavailable)}</p>}
  </section>;
}
