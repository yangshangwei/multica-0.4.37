"use client";
import { useId, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, errorCode, isIterationAccessDenied } from "@multica/core/api";
import {
  useIterationCommand,
  iterationChoicesOptions,
  iterationDetailOptions,
  iterationDefaultDates,
  validateIterationName,
  type Iteration,
} from "@multica/core/iterations";
import { memberListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { IterationError } from "./iteration-error";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { useWorkspacePaths } from "@multica/core/paths";
import { RevisionConflictCompare } from "../issues/components/revision-conflict-compare";

type EditFields = Pick<
  Iteration,
  "name" | "description" | "start_date" | "end_date" | "coordinator_user_id"
>;

function editFields(iteration: Iteration): EditFields {
  return {
    name: iteration.name,
    description: iteration.description,
    start_date: iteration.start_date,
    end_date: iteration.end_date,
    coordinator_user_id: iteration.coordinator_user_id,
  };
}

function changedFields(
  baseline: EditFields,
  values: EditFields,
  status?: string,
): Partial<EditFields> {
  const editable: (keyof EditFields)[] = ["name", "description"];
  if (!status || status === "planned" || status === "active") {
    editable.push("coordinator_user_id", "end_date");
    if (!status || status === "planned") editable.push("start_date");
  }
  return Object.fromEntries(
    editable.filter((field) => values[field] !== baseline[field])
      .map((field) => [field, values[field]]),
  );
}
export function IterationForm({
  wsId,
  timezone,
  iteration,
}: {
  wsId: string;
  timezone: string;
  iteration?: Iteration;
}) {
  const { t } = useT("projects");
  const nav = useNavigation();
  const paths = useWorkspacePaths();
  const client = useQueryClient();
  const nameFieldId = useId();
  const nameInput = useRef<HTMLInputElement>(null);
  const [nameTouched, setNameTouched] = useState(false);
  const [edit, setEdit] = useState(() => {
    const dates = iterationDefaultDates(timezone);
    return {
      baseline: iteration,
      values: iteration ? editFields(iteration) : {
        name: "", description: null, coordinator_user_id: null,
        start_date: dates.start, end_date: dates.end,
      },
    };
  });
  const { name, description, start_date: start, end_date: end, coordinator_user_id: coordinator } = edit.values;
  const patch = (fields: Partial<EditFields>) => setEdit((old) => ({ ...old, values: { ...old.values, ...fields } }));
  const [reason, setReason] = useState("");
  const [resolution, setResolution] = useState<"conflict" | "saved" | null>(null);
  const [conflict, setConflict] = useState<Iteration | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<unknown>();
  const members = useQuery(memberListOptions(wsId));
  const choices = useQuery(iterationChoicesOptions(wsId));
  const overlaps = choices.data?.filter((item) => item.id !== iteration?.id && item.start_date <= end && item.end_date >= start) ?? [];
  const save = useIterationCommand(wsId, iteration?.id ?? "create");
  const pending = save.pending;
  const busy = save.isPending || refreshing;
  const current = edit.baseline && edit.baseline.revision > (iteration?.revision ?? 0) ? edit.baseline : iteration;
  const status = conflict && conflict.revision > (current?.revision ?? 0) ? conflict.status : current?.status;
  const fields = edit.baseline ? changedFields(edit.baseline, edit.values, status) : {};
  // Values and revision move together. Dirty input never acquires a remote
  // revision until the user has reviewed and explicitly rebased it.
  if (
    iteration && edit.baseline && iteration.revision > edit.baseline.revision &&
    !busy && !pending && !resolution &&
    Object.keys(changedFields(edit.baseline, edit.values)).length === 0
  ) {
    setEdit({ baseline: iteration, values: editFields(iteration) });
  }
  const nameError = nameTouched ? validateIterationName(name) : null;
  function adopt(current: Iteration, rebase = false) {
    setEdit((old) => ({
      baseline: current,
      values: {
        ...editFields(current),
        ...(rebase && old.baseline ? changedFields(old.baseline, old.values, current.status) : {}),
      },
    }));
    if (!rebase) setReason("");
    setNameTouched(false);
    setResolution(null);
    setConflict(null);
    setRefreshError(undefined);
    save.reset();
  }
  async function refresh(mode: "conflict" | "saved") {
    if (!iteration) return;
    const options = iterationDetailOptions(wsId, iteration.id);
    setRefreshing(true);
    setRefreshError(undefined);
    try {
      const current = await client.fetchQuery({ ...options, staleTime: 0 });
      if (mode === "saved") adopt(current.iteration);
      else setConflict(current.iteration);
    } catch (error) {
      setRefreshError(error);
    } finally {
      setRefreshing(false);
    }
  }
  async function submit() {
    if (busy || resolution) return;
    if (!pending && validateIterationName(name)) {
      setNameTouched(true);
      nameInput.current?.focus();
      return;
    }
    const body = {
      request_id: crypto.randomUUID(),
      name,
      description: description || null,
      coordinator_user_id: coordinator || null,
      start_date: start,
      end_date: end,
      confirmed_timezone: timezone,
    };
    const command =
      pending ??
      (iteration
        ? {
            kind: "edit" as const,
            id: iteration.id,
            body: {
              request_id: body.request_id,
              expected_revision: edit.baseline!.revision,
              fields,
              reason,
            },
          }
        : { kind: "create" as const, body });
    try {
      const result = await save.mutateAsync({
        command,
        recover: pending !== null,
      });
      if (!iteration && result.iteration_ids[0])
        nav.push(paths.iterationDetail(result.iteration_ids[0]));
      if (iteration) {
        setResolution("saved");
        await refresh("saved");
      }
    } catch (error) {
      /* The mutation retains uncertain commands and form input. */
      if (iteration && errorCode(error) === "iteration_revision_conflict") {
        setResolution("conflict");
        await refresh("conflict");
      }
    }
  }
  // An imperative refresh can reject as cancelled on revocation. The active
  // protected choices observer still receives the workspace's access error.
  const accessError = [save.error, refreshError, choices.error].find(isIterationAccessDenied);
  if (accessError || (refreshError instanceof ApiError && refreshError.status === 404)) {
    return <IterationError error={accessError ?? refreshError} />;
  }
  const compareValues = (values: EditFields) => [
    `${t(($) => $.iterations.name)}: ${values.name}`,
    `${t(($) => $.iterations.description)}: ${values.description ?? ""}`,
    `${t(($) => $.iterations.startDate)}: ${values.start_date}`,
    `${t(($) => $.iterations.endDate)}: ${values.end_date}`,
    `${t(($) => $.iterations.coordinator)}: ${values.coordinator_user_id ? members.data?.find((member) => member.user_id === values.coordinator_user_id)?.name ?? t(($) => $.iterations.coordinatorMissing) : t(($) => $.iterations.none)}`,
  ].join("\n");
  return (
    <details>
      <summary className="font-medium cursor-pointer">
        {t(($) => (iteration ? $.iterations.edit : $.iterations.create))}
      </summary>
      <form
        className="mt-4 max-w-2xl space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <fieldset
          disabled={busy || pending !== null || resolution === "saved"}
          className="space-y-4"
        >
          <label className="block">
            {t(($) => $.iterations.name)}
            <Input
              ref={nameInput}
              required
              aria-invalid={nameError ? true : undefined}
              aria-describedby={`${nameFieldId}-hint${nameError ? ` ${nameFieldId}-error` : ""}`}
              onInvalid={(event) => {
                event.preventDefault();
                setNameTouched(true);
                event.currentTarget.focus();
              }}
              onBlur={() => setNameTouched(true)}
              value={name}
              onChange={(e) => patch({ name: e.target.value })}
            />
          </label>
          <p
            id={`${nameFieldId}-hint`}
            className="text-caption text-muted-foreground"
          >
            {t(($) => $.iterations.nameHint)}
          </p>
          {nameError && (
            <p
              id={`${nameFieldId}-error`}
              role="alert"
              className="text-caption text-destructive"
            >
              {t(($) =>
                nameError === "required"
                  ? $.iterations.nameRequired
                  : $.iterations.nameTooLong,
              )}
            </p>
          )}
          <label className="block">
            {t(($) => $.iterations.description)}
            <textarea
              className="min-h-24 w-full rounded-md border bg-background p-3"
              value={description ?? ""}
              onChange={(e) => patch({ description: e.target.value || null })}
            />
          </label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label>
              {t(($) => $.iterations.startDate)}
              <Input
                type="date"
                required
                disabled={!!iteration && status !== "planned"}
                value={start}
                onChange={(e) => patch({ start_date: e.target.value })}
              />
            </label>
            <label>
              {t(($) => $.iterations.endDate)}
              <Input
                type="date"
                required
                disabled={
                  !!iteration &&
                  !["planned", "active"].includes(status ?? "")
                }
                min={start}
                value={end}
                onChange={(e) => patch({ end_date: e.target.value })}
              />
            </label>
          </div>
          {(!iteration || ["planned", "active"].includes(status ?? "")) && overlaps.length > 0 && <p role="status">{t(($) => $.iterations.overlapWarning)} {overlaps.map((item) => item.name).join(", ")}</p>}
          <label className="block">
            {t(($) => $.iterations.coordinator)}
            <select
              disabled={
                !!iteration && !["planned", "active"].includes(status ?? "")
              }
              value={coordinator ?? ""}
              onChange={(e) => patch({ coordinator_user_id: e.target.value || null })}
              className="block w-full rounded-md border bg-background p-2"
            >
              <option value="">{t(($) => $.iterations.none)}</option>
              {members.data?.map((member) => (
                <option key={member.user_id} value={member.user_id}>
                  {member.name ?? member.user_id}
                </option>
              ))}
            </select>
          </label>
          {iteration && (
            <label className="block">
              {t(($) => $.iterations.reason)}
              <Input
                required
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
          )}
        </fieldset>
        {conflict && <RevisionConflictCompare
          title={t(($) => $.iterations.editConflict)}
          serverLabel={t(($) => $.management.server_version)}
          localLabel={t(($) => $.iterations.rebasedVersion)}
          serverValue={compareValues(conflict)}
          localValue={compareValues({ ...editFields(conflict), ...fields })}
          footer={t(($) => $.iterations.rebaseHint)}
          serverAction={<Button type="button" className="h-auto min-h-8 w-full whitespace-normal break-words py-1.5" variant="outline" disabled={busy || !!pending} onClick={() => adopt(conflict)}>{t(($) => $.management.use_server)}</Button>}
          localAction={<Button type="button" className="h-auto min-h-8 w-full whitespace-normal break-words py-1.5" disabled={busy || !!pending} onClick={() => adopt(conflict, true)}>{t(($) => $.iterations.rebaseChanges)}</Button>}
        />}
        {resolution === "saved" && <p role="status">{t(($) => $.iterations.savedRefresh)}</p>}
        {resolution && !conflict && <Button type="button" disabled={busy || !!pending} onClick={() => void refresh(resolution)}>{t(($) => $.iterations.retryRefresh)}</Button>}
        {refreshError ? <IterationError error={refreshError} /> : save.error && !conflict && <IterationError error={save.error} />}
        <Button type="submit" disabled={busy || resolution !== null}>
          {t(($) => (iteration ? $.iterations.save : $.iterations.create))}
        </Button>
      </form>
    </details>
  );
}
