"use client";
import { useId, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  useIterationCommand,
  iterationChoicesOptions,
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
  const nameFieldId = useId();
  const nameInput = useRef<HTMLInputElement>(null);
  const [nameTouched, setNameTouched] = useState(false);
  const [name, setName] = useState(iteration?.name ?? "");
  const [description, setDescription] = useState(iteration?.description ?? "");
  const [start, setStart] = useState(
    () => iteration?.start_date ?? iterationDefaultDates(timezone).start,
  );
  const [end, setEnd] = useState(
    () => iteration?.end_date ?? iterationDefaultDates(timezone).end,
  );
  const [coordinator, setCoordinator] = useState(
    iteration?.coordinator_user_id ?? "",
  );
  const [reason, setReason] = useState("");
  const members = useQuery(memberListOptions(wsId));
  const choices = useQuery(iterationChoicesOptions(wsId));
  const overlaps = choices.data?.filter((item) => item.id !== iteration?.id && item.start_date <= end && item.end_date >= start) ?? [];
  const save = useIterationCommand(wsId, iteration?.id ?? "create");
  const pending = save.pending;
  const nameError = nameTouched ? validateIterationName(name) : null;
  async function submit() {
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
              expected_revision: iteration.revision,
              fields: {
                name,
                description: body.description,
                ...(["planned", "active"].includes(iteration.status)
                  ? {
                      coordinator_user_id: body.coordinator_user_id,
                      end_date: end,
                      ...(iteration.status === "planned"
                        ? { start_date: start }
                        : {}),
                    }
                  : {}),
              },
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
    } catch {
      /* The mutation retains uncertain commands and form input. */
    }
  }
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
          disabled={save.isPending || pending !== null}
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
              onChange={(e) => setName(e.target.value)}
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
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label>
              {t(($) => $.iterations.startDate)}
              <Input
                type="date"
                required
                disabled={!!iteration && iteration.status !== "planned"}
                value={start}
                onChange={(e) => setStart(e.target.value)}
              />
            </label>
            <label>
              {t(($) => $.iterations.endDate)}
              <Input
                type="date"
                required
                disabled={
                  !!iteration &&
                  !["planned", "active"].includes(iteration.status)
                }
                min={start}
                value={end}
                onChange={(e) => setEnd(e.target.value)}
              />
            </label>
          </div>
          {(!iteration || ["planned", "active"].includes(iteration.status)) && overlaps.length > 0 && <p role="status">{t(($) => $.iterations.overlapWarning)} {overlaps.map((item) => item.name).join(", ")}</p>}
          <label className="block">
            {t(($) => $.iterations.coordinator)}
            <select
              disabled={
                !!iteration && !["planned", "active"].includes(iteration.status)
              }
              value={coordinator}
              onChange={(e) => setCoordinator(e.target.value)}
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
        {save.error && <IterationError error={save.error} />}
        <Button type="submit" disabled={save.isPending}>
          {t(($) => (iteration ? $.iterations.save : $.iterations.create))}
        </Button>
      </form>
    </details>
  );
}
