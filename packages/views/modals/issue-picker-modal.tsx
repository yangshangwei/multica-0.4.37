"use client";

import { issueStatusCategory } from "@multica/core/issues";
import { useState, useEffect, useCallback, useRef } from "react";
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
import { StatusIcon } from "../issues/components/status-icon";
import { useT } from "../i18n";

interface IssuePickerModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  excludeIds: string[];
  filterIssue?: (issue: Issue) => boolean;
  onSelect: (issue: Issue) => void | boolean | Promise<void | boolean>;
}

export function IssuePickerModal({
  open,
  onOpenChange,
  title,
  description,
  excludeIds,
  filterIssue,
  onSelect,
}: IssuePickerModalProps) {
  const { t } = useT("modals");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Issue[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSelecting, setIsSelecting] = useState(false);
  const [selectionFailed, setSelectionFailed] = useState(false);
  const selectionRef = useRef<symbol | null>(null);
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
      <Command shouldFilter={false}>
        <CommandInput
          placeholder={t(($) => $.issue_picker.search_placeholder)}
          value={query}
          disabled={isSelecting}
          onValueChange={(v) => {
            setQuery(v);
            setSelectionFailed(false);
            search(v);
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
        <CommandList aria-busy={isSelecting}>
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
          {filteredResults.length > 0 && (
            <CommandGroup>
              {filteredResults.map((issue) => (
                <CommandItem
                  key={issue.id}
                  value={issue.id}
                  disabled={isSelecting}
                  onSelect={() => void selectIssue(issue)}
                >
                  <StatusIcon
                    status={issue.status}
                    category={issueStatusCategory(issue) ?? undefined}
                    className="h-3.5 w-3.5 shrink-0"
                  />
                  <span className="text-muted-foreground shrink-0">{issue.identifier}</span>
                  <span className="truncate">{issue.title}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
        </CommandList>
      </Command>
    </CommandDialog>
  );
}
