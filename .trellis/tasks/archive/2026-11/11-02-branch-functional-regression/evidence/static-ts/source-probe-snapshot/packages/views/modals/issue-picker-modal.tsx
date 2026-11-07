"use client";

import { issueStatusCategory } from "@multica/core/issues";
import { useState, useEffect, useCallback, useRef } from "react";
import { useInfiniteQuery, type InfiniteData, type QueryKey, type UseInfiniteQueryOptions } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { api } from "@multica/core/api";
import {
  Command,
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from "@multica/ui/components/ui/command";
import { Button } from "@multica/ui/components/ui/button";
import { StatusIcon } from "../issues/components/status-icon";
import { useDebouncedValue } from "../common/use-debounced-value";
import { useT } from "../i18n";

interface IssuePickerPage {
  issues: Issue[];
  nextOffset: number | undefined;
}

type BrowseOptions = (search: string) => UseInfiniteQueryOptions<
  IssuePickerPage, Error, InfiniteData<IssuePickerPage>, QueryKey, number
>;

interface IssuePickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  excludeIds: string[];
  filterIssue?: (issue: Issue) => boolean;
  browseOptions?: BrowseOptions;
  getStatusLabel?: (status: string) => string;
  onSelect: (issue: Issue) => void | boolean | Promise<void | boolean>;
}

export function IssuePickerModal({
  open,
  onOpenChange,
  title,
  description,
  excludeIds,
  filterIssue,
  browseOptions,
  getStatusLabel,
  onSelect,
}: IssuePickerModalProps) {
  const { t } = useT("modals");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Issue[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSelecting, setIsSelecting] = useState(false);
  const [selectionFailed, setSelectionFailed] = useState(false);
  const selectionRef = useRef<symbol | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const abortRef = useRef<AbortController>(undefined);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setResults([]);
      setIsLoading(false);
      setIsSelecting(false);
      setSelectionFailed(false);
    }

    return () => {
      selectionRef.current = null;
      if (debounceRef.current) clearTimeout(debounceRef.current);
      abortRef.current?.abort();
    };
  }, [open]);

  const search = useCallback(
    (q: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      if (abortRef.current) abortRef.current.abort();

      if (!q.trim()) {
        setResults([]);
        setIsLoading(false);
        return;
      }

      setIsLoading(true);
      debounceRef.current = setTimeout(async () => {
        const controller = new AbortController();
        abortRef.current = controller;
        try {
          const res = await api.searchIssues({
            q: q.trim(),
            limit: 20,
            include_closed: true,
            signal: controller.signal,
          });
          if (!controller.signal.aborted) {
            setResults(res.issues);
            setIsLoading(false);
          }
        } catch {
          if (!controller.signal.aborted) {
            setIsLoading(false);
          }
        }
      }, 300);
    },
    [],
  );

  const filteredResults = results.filter(
    (issue) => !excludeIds.includes(issue.id) && (!filterIssue || filterIssue(issue)),
  );

  const selectIssue = async (issue: Issue) => {
    if (selectionRef.current) return;
    const selection = Symbol();
    selectionRef.current = selection;
    setIsSelecting(true);
    setSelectionFailed(false);
    try {
      const result = await onSelect(issue);
      if (selectionRef.current === selection && result !== false) {
        onOpenChange(false);
      }
    } catch {
      if (selectionRef.current === selection) setSelectionFailed(true);
    } finally {
      if (selectionRef.current === selection) {
        selectionRef.current = null;
        setIsSelecting(false);
      }
    }
  };

  return (
    <CommandDialog
      open={open}
      onOpenChange={(nextOpen, eventDetails) => {
        if (selectionRef.current) {
          eventDetails.cancel();
          return;
        }
        onOpenChange(nextOpen);
      }}
      title={title}
      description={description}
    >
      <Command shouldFilter={false} label={title}>
        <CommandInput
          ref={inputRef}
          placeholder={t(($) => $.issue_picker.search_placeholder)}
          value={query}
          disabled={isSelecting}
          onValueChange={(v) => {
            setQuery(v);
            setSelectionFailed(false);
            if (!browseOptions) search(v);
          }}
        />
        <p aria-hidden="true" className="px-3 pt-2 text-caption text-muted-foreground">
          {description}
        </p>
        {selectionFailed && (
          <p role="alert" className="px-3 pt-2 text-body text-destructive">
            {t(($) => $.issue_picker.select_failed)}
          </p>
        )}
        {browseOptions ? open && <BrowseIssueResults
          query={query}
          browseOptions={browseOptions}
          excludeIds={excludeIds}
          filterIssue={filterIssue}
          isSelecting={isSelecting}
          onSelect={selectIssue}
          getStatusLabel={getStatusLabel}
          focusInput={() => inputRef.current?.focus()}
        /> : <CommandList aria-busy={isSelecting}>
          {isLoading && (
            <div className="py-6 text-center text-body text-muted-foreground">
              {t(($) => $.issue_picker.searching)}
            </div>
          )}
          {!isLoading && query.trim() && filteredResults.length === 0 && (
            <CommandEmpty>{t(($) => $.issue_picker.no_results)}</CommandEmpty>
          )}
          {!isLoading && !query.trim() && (
            <div className="py-6 text-center text-body text-muted-foreground">
              {t(($) => $.issue_picker.prompt_to_search)}
            </div>
          )}
          <IssuePickerItems issues={filteredResults} isSelecting={isSelecting} onSelect={selectIssue} />
        </CommandList>}
      </Command>
    </CommandDialog>
  );
}

function IssuePickerItems({ issues, isSelecting, onSelect, browse = false, getStatusLabel }: {
  issues: Issue[];
  isSelecting: boolean;
  onSelect: (issue: Issue) => void;
  browse?: boolean;
  getStatusLabel?: (status: string) => string;
}) {
  if (issues.length === 0) return null;
  return <CommandGroup>
    {issues.map((issue) => <CommandItem
      key={issue.id}
      value={issue.id}
      disabled={isSelecting}
      onSelect={() => onSelect(issue)}
    >
      <StatusIcon
        status={issue.status}
        category={issueStatusCategory(issue) ?? undefined}
        className="h-3.5 w-3.5 shrink-0"
      />
      <span className="text-muted-foreground shrink-0">{issue.identifier}</span>
      <span className={browse ? "min-w-0 flex-1 break-words" : "truncate"}>{issue.title}</span>
      {browse && <span className="max-w-24 shrink-0 break-words text-caption text-muted-foreground">
        {getStatusLabel?.(issue.status) ?? issue.status}
      </span>}
    </CommandItem>)}
  </CommandGroup>;
}

function BrowseIssueResults({ query, browseOptions, excludeIds, filterIssue, isSelecting, onSelect, getStatusLabel, focusInput }: {
  query: string;
  browseOptions: BrowseOptions;
  excludeIds: string[];
  filterIssue?: (issue: Issue) => boolean;
  isSelecting: boolean;
  onSelect: (issue: Issue) => void;
  getStatusLabel?: (status: string) => string;
  focusInput: () => void;
}) {
  const { t } = useT("modals");
  const search = query.trim();
  const debounced = useDebouncedValue(search, 300);
  const effectiveSearch = search ? debounced : "";
  const waiting = search !== effectiveSearch;
  const options = browseOptions(effectiveSearch);
  const candidates = useInfiniteQuery({ ...options, enabled: waiting ? false : options.enabled });
  const loading = waiting || candidates.isPending ||
    (candidates.isFetching && !candidates.isFetchingNextPage);
  const issues = [...new Map((candidates.data?.pages.flatMap((page) => page.issues) ?? [])
    .filter((issue) => !excludeIds.includes(issue.id) && (!filterIssue || filterIssue(issue)))
    .map((issue) => [issue.id, issue])).values()];
  const loadMore = () => {
    if (candidates.hasNextPage && !candidates.isFetching && !waiting && !isSelecting)
      void candidates.fetchNextPage();
  };

  return <>
    <CommandList
      label={t(($) => $.issue_picker.candidates_label)}
      aria-busy={loading || candidates.isFetchingNextPage || isSelecting}
      onScroll={(event) => {
        const list = event.currentTarget;
        if (list.scrollHeight - list.scrollTop - list.clientHeight < 48 && !candidates.isError)
          loadMore();
      }}
    >
      {loading ? <p role="status" className="py-6 text-center text-body text-muted-foreground">
        {t(($) => $.issue_picker.loading)}
      </p> : <>
        <IssuePickerItems issues={issues} isSelecting={isSelecting} onSelect={onSelect} getStatusLabel={getStatusLabel} browse />
        {issues.length === 0 && !candidates.isError && !candidates.hasNextPage &&
          <p role="status" className="py-6 text-center text-body text-muted-foreground">
            {search ? t(($) => $.issue_picker.no_results) : t(($) => $.issue_picker.no_candidates)}
          </p>}
      </>}
    </CommandList>
    {!loading && candidates.isError && <div className="space-y-2 px-3 pb-3">
      <p role="alert" className="text-body text-destructive">
        {candidates.isFetchNextPageError
          ? t(($) => $.issue_picker.load_more_failed)
          : t(($) => $.issue_picker.load_failed)}
      </p>
      <Button variant="outline" size="sm" disabled={isSelecting || candidates.isFetching}
        onKeyDown={(event) => event.stopPropagation()}
        onClick={() => {
          focusInput();
          void (candidates.isFetchNextPageError ? candidates.fetchNextPage() : candidates.refetch());
        }}>
        {t(($) => $.issue_picker.retry)}
      </Button>
    </div>}
    {!loading && !candidates.isError && candidates.hasNextPage && <Button
      variant="ghost" size="sm" className="m-1"
      disabled={isSelecting || candidates.isFetchingNextPage}
      onKeyDown={(event) => event.stopPropagation()}
      onClick={() => {
        focusInput();
        loadMore();
      }}
    >
      {candidates.isFetchingNextPage ? t(($) => $.issue_picker.loading) : t(($) => $.issue_picker.load_more)}
    </Button>}
  </>;
}
