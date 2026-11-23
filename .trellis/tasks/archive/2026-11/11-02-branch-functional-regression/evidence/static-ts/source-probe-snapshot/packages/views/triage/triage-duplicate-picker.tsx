"use client";

import { useMemo, useRef, useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { isFormalAdmission, triageDuplicateTargetsOptions } from "@multica/core/triage";
import {
  Command,
  CommandInput,
  CommandItem,
  CommandList,
} from "@multica/ui/components/ui/command";
import { Button } from "@multica/ui/components/ui/button";
import { useDebouncedValue } from "../common/use-debounced-value";
import { useStatusLabel } from "../issues/utils/status-label";
import { useT } from "../i18n";
import { duplicateReference, TRIAGE_CONTROL } from "./triage-ui";

export function TriageDuplicatePicker({
  wsId,
  workspaceSlug,
  excludeId,
  value,
  onChange,
}: {
  wsId: string;
  workspaceSlug: string;
  excludeId: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useT("triage");
  const statusLabel = useStatusLabel(wsId);
  const [open, setOpen] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  let search = value.trim();
  let invalidLink = false;
  if (/^(?:[a-z][a-z\d+.-]*:\/\/|\/\/|\/?[^/\s]+\/issues(?:\/|$))/i.test(search)) {
    try {
      search = duplicateReference(search, workspaceSlug);
    } catch {
      invalidLink = true;
    }
  }
  const debounced = useDebouncedValue(search, 250);
  const effectiveSearch = search ? debounced : "";
  const waiting = effectiveSearch !== search;
  const query = useInfiniteQuery({
    ...triageDuplicateTargetsOptions(wsId, effectiveSearch),
    enabled: open && !!wsId && !invalidLink && !waiting,
    retry: false,
  });
  const issues = useMemo(() => {
    const candidates = query.data?.pages.flatMap((page) => page.issues) ?? [];
    return [...new Map(candidates
      .filter((issue) => issue.workspace_id === wsId && issue.id !== excludeId &&
        isFormalAdmission(issue.admission_status))
      .map((issue) => [issue.id, issue])).values()];
  }, [query.data, wsId, excludeId]);
  const loading = waiting || query.isPending;
  const loadMore = () => {
    if (query.hasNextPage && !query.isFetching && !waiting)
      void query.fetchNextPage();
  };

  return (
    <Command
      label={t(($) => $.duplicate_target)}
      shouldFilter={false}
      className="h-auto border border-input bg-background p-0 [&_[data-slot=input-group]]:h-auto!"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
      onKeyDownCapture={(event) => {
        if (event.key === "Escape" && open) {
          event.preventDefault();
          event.stopPropagation();
          input.current?.focus();
          setOpen(false);
        } else if (event.target !== input.current) {
          // cmdk handles every bubbled Enter as an option selection.
          event.stopPropagation();
        }
      }}
    >
      <CommandInput
        asChild
        aria-label={t(($) => $.duplicate_target)}
        placeholder={t(($) => $.duplicate_placeholder)}
        className={TRIAGE_CONTROL}
        value={value}
        onValueChange={(next) => {
          onChange(next);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onClick={() => setOpen(true)}
        onKeyDown={(event) => {
          if (!open && ["ArrowDown", "ArrowUp"].includes(event.key)) {
            event.preventDefault();
            setOpen(true);
          } else if (!open && event.key === "Enter") {
            event.preventDefault();
            event.stopPropagation();
          }
        }}
      >
        {/* cmdk hardcodes expanded=true; the slotted input owns popup state. */}
        <input ref={input} aria-expanded={open}
          {...(!open ? { "aria-controls": undefined, "aria-activedescendant": undefined } : {})} />
      </CommandInput>
      {open && (
        <>
          <CommandList
            label={t(($) => $.duplicate_candidates)}
            aria-busy={loading || query.isFetchingNextPage}
            className="max-h-56 overscroll-contain p-1"
            onScroll={(event) => {
              const list = event.currentTarget;
              if (list.scrollHeight - list.scrollTop - list.clientHeight < 48 &&
                  !query.isError) loadMore();
            }}
          >
            {invalidLink ? (
              <p role="status" className="p-3 text-body text-muted-foreground">
                {t(($) => $.invalid_duplicate_link)}
              </p>
            ) : loading ? (
              <p role="status" className="p-3 text-body text-muted-foreground">
                {t(($) => $.loading)}
              </p>
            ) : (
              <>
                {issues.map((issue) => (
                  <CommandItem
                    key={issue.id}
                    value={issue.id}
                    className={`${TRIAGE_CONTROL} gap-2 py-2`}
                    onSelect={() => {
                      onChange(issue.identifier);
                      setOpen(false);
                    }}
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-caption text-muted-foreground">
                        {issue.identifier}
                      </span>
                      <span className="block break-words">{issue.title}</span>
                    </span>
                    <span className="max-w-24 shrink-0 break-words text-caption text-muted-foreground">
                      {statusLabel(issue.status)}
                    </span>
                  </CommandItem>
                ))}
                {issues.length === 0 && !query.isError && !query.hasNextPage && (
                  <p role="status" className="p-3 text-body text-muted-foreground">
                    {search ? t(($) => $.duplicate_no_results) : t(($) => $.duplicate_empty)}
                  </p>
                )}
              </>
            )}
          </CommandList>
          {!invalidLink && !loading && query.isError && (
            <div className="space-y-2 px-3 pb-3">
              <p role="alert" className="text-body text-destructive">
                {query.isFetchNextPageError
                  ? t(($) => $.duplicate_more_failed)
                  : t(($) => $.duplicate_failed)}
              </p>
              <Button variant="outline" className={TRIAGE_CONTROL}
                disabled={query.isFetching}
                onClick={() => {
                  input.current?.focus();
                  void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch());
                }}>
                {t(($) => $.retry)}
              </Button>
            </div>
          )}
          {!invalidLink && !loading && !query.isError && query.hasNextPage && (
            <Button variant="ghost" className={`${TRIAGE_CONTROL} m-1`}
              disabled={query.isFetchingNextPage} onClick={() => {
                input.current?.focus();
                loadMore();
              }}>
              {query.isFetchingNextPage ? t(($) => $.loading) : t(($) => $.duplicate_more)}
            </Button>
          )}
        </>
      )}
    </Command>
  );
}
