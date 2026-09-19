"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Check, Search, Tag } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  labelListOptions,
  resourceLabelsOptions,
  useAttachResourceLabel,
  useDetachResourceLabel,
} from "@multica/core/labels";
import type { Label } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { useT } from "../i18n";
import { LabelChip } from "./label-chip";

interface ResourceLabelPickerProps {
  resourceType: "agent" | "skill";
  /**
   * The resource whose labels are edited. Omit for **draft mode** (e.g. the
   * create-skill dialog, where the resource doesn't exist yet): pass
   * `selectedIds` + `onSelectedIdsChange` instead and submit the ids with the
   * create request. Mirrors the issue `LabelPicker` draft-mode convention.
   */
  resourceId?: string;
  /** Draft-mode selection. Ignored when `resourceId` is set. */
  selectedIds?: string[];
  /** Draft-mode change handler. Ignored when `resourceId` is set. */
  onSelectedIdsChange?: (ids: string[]) => void;
  canEdit: boolean;
}

/**
 * Two modes:
 * - **Attached mode** (`resourceId` set): selection comes from the resource's
 *   own labels query; toggling hits attach/detach.
 * - **Draft mode** (`resourceId` omitted): selection is held by the caller as
 *   ids and resolved against the workspace catalog; nothing is persisted here.
 */
export function ResourceLabelPicker({
  resourceType,
  resourceId,
  selectedIds = [],
  onSelectedIdsChange,
  canEdit,
}: ResourceLabelPickerProps) {
  const { t } = useT("labels");
  const wsId = useWorkspaceId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const isDraft = resourceId === undefined;
  const { data: catalog = [] } = useQuery(labelListOptions(wsId, resourceType));
  // `resourceLabelsOptions` disables itself for an empty id, so the draft
  // path never fires the by-resource read.
  const { data: attached = [] } = useQuery(
    resourceLabelsOptions(wsId, resourceType, resourceId ?? ""),
  );
  // Hooks must run unconditionally; in draft mode the empty id is never used
  // because toggling routes through onSelectedIdsChange instead.
  const attach = useAttachResourceLabel(resourceType, resourceId ?? "");
  const detach = useDetachResourceLabel(resourceType, resourceId ?? "");

  // Draft mode resolves ids against the catalog (dropping any id whose label
  // was deleted meanwhile) and preserves the user's selection order.
  const selected = useMemo<Label[]>(() => {
    if (!isDraft) return attached;
    return selectedIds
      .map((id) => catalog.find((label) => label.id === id))
      .filter((label): label is Label => Boolean(label));
  }, [isDraft, attached, selectedIds, catalog]);
  const selectedIdSet = useMemo(() => new Set(selected.map((label) => label.id)), [selected]);
  const filtered = catalog.filter((label) =>
    label.name.toLowerCase().includes(query.trim().toLowerCase()),
  );

  const toggle = (labelId: string) => {
    if (isDraft) {
      onSelectedIdsChange?.(
        selectedIdSet.has(labelId)
          ? selectedIds.filter((id) => id !== labelId)
          : [...selectedIds, labelId],
      );
    } else if (selectedIdSet.has(labelId)) {
      detach.mutate(labelId);
    } else {
      attach.mutate(labelId);
    }
  };

  const content = selected.length > 0 ? (
    <div className="flex flex-wrap justify-start gap-1 sm:justify-end">
      {selected.map((label) => (
        <LabelChip key={label.id} label={label} />
      ))}
    </div>
  ) : (
    <span className="text-body text-muted-foreground">{t(($) => $.resource_picker.empty)}</span>
  );

  if (!canEdit) return content;

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <PopoverTrigger
        render={
          <Button
            type="button"
            variant="outline"
            className="h-auto min-h-9 w-full justify-start px-2.5 py-1.5 sm:justify-end"
          >
            {selected.length > 0 ? content : (
              <>
                <Tag className="size-3.5 text-muted-foreground" />
                {t(($) => $.resource_picker.add)}
              </>
            )}
          </Button>
        }
      />
      <PopoverContent align="end" className="w-72 p-2">
        <div className="relative mb-2">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t(($) => $.resource_picker.search)}
            className="h-8 pl-8 text-body"
          />
        </div>
        <div className="max-h-64 space-y-0.5 overflow-y-auto">
          {filtered.map((label) => {
            const isSelected = selectedIdSet.has(label.id);
            return (
              <button
                key={label.id}
                type="button"
                onClick={() => toggle(label.id)}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-body hover:bg-accent"
              >
                <span
                  className="size-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: label.color }}
                />
                <span className="min-w-0 flex-1 truncate">{label.name}</span>
                {isSelected ? <Check className="size-3.5 text-primary" /> : null}
              </button>
            );
          })}
          {filtered.length === 0 ? (
            <p className="px-2 py-6 text-center text-caption text-muted-foreground">
              {catalog.length === 0
                ? t(($) => $.resource_picker.no_labels)
                : t(($) => $.resource_picker.no_results)}
            </p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
