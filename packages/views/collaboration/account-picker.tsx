import { useEffect, useId, useMemo, useState } from "react";
import { keepPreviousData, useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, LoaderCircle, Search } from "lucide-react";
import {
  collaborationCreationRecipientOptions,
  collaborationRecipientOptions,
  collaborationRecipientSearchOptions,
  collaborationRecipientSections,
  type CollaborationRecipient,
  type CollaborationRecipientAccount,
  type CollaborationRecipientSearchContext,
  type CollaborationScope,
} from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { PickerItem, PickerSection, PropertyPicker } from "../issues/components/pickers/property-picker";

export interface CollaborationAccountPickerProps {
  scope: CollaborationScope;
  context: CollaborationRecipientSearchContext;
  /** A single account for workflow/creation, or the current accounts for a multi-value field. */
  selected: CollaborationRecipientAccount | readonly CollaborationRecipientAccount[] | null;
  onSelect: (account: CollaborationRecipientAccount | null) => void;
  label: string;
  disabled?: boolean;
  allowSelf?: boolean;
  selfName?: string;
  membersOnly?: boolean;
  multiple?: boolean;
  maxSelected?: number;
}

function initials(name: string) {
  return Array.from(name.trim())[0] ?? "?";
}

function accountIdentity(account: CollaborationRecipientAccount) {
  return account.user_id.toLowerCase();
}

function selfAccount(scope: CollaborationScope, name: string): CollaborationRecipientAccount {
  return { user_id: scope.userId, name, avatar_url: "", external: false };
}

function toAccount(recipient: CollaborationRecipient): CollaborationRecipientAccount {
  return recipient.recipient;
}

export function CollaborationAccountPicker({
  scope,
  context,
  selected,
  onSelect,
  label,
  disabled = false,
  allowSelf = false,
  selfName,
  membersOnly = false,
  multiple = false,
  maxSelected,
}: CollaborationAccountPickerProps) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const exactId = useId();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [query, setQuery] = useState("");
  const [exactOpen, setExactOpen] = useState(false);
  const [exactText, setExactText] = useState("");
  const [exactRequest, setExactRequest] = useState("");
  const selectedAccounts = useMemo(() => {
    if (Array.isArray(selected)) return selected;
    return selected ? [selected] : [];
  }, [selected]);
  const self = allowSelf ? selfAccount(scope, selfName ?? t(($) => $.creation.self)) : null;
  const selectedIds = useMemo(() => new Set(selectedAccounts.map(accountIdentity)), [selectedAccounts]);
  const selfSelected = allowSelf && selected === null;

  useEffect(() => {
    const timer = window.setTimeout(() => setQuery(input.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [input]);

  const search = useInfiniteQuery({
    ...collaborationRecipientSearchOptions(client, scope, context, query, membersOnly),
    enabled: open && scope.userId !== "",
    placeholderData: keepPreviousData,
  });
  const exactOptions = context.kind === "issue"
    ? collaborationRecipientOptions(client, scope, context.issueId, exactRequest)
    : collaborationCreationRecipientOptions(client, scope, context.sourceWorkspaceId, exactRequest);
  const exact = useQuery({ ...exactOptions, enabled: open && exactOpen && exactRequest !== "" && exactRequest === exactText.trim() });
  const pages = search.data?.pages ?? [];
  const firstPage = pages[0];
  const recent = firstPage?.recent ?? [];
  const sections = collaborationRecipientSections({ selected: selectedAccounts, recent, pages, self });
  const allAccounts = sections.flatMap((section) => section.accounts);
  const display = selectedAccounts[0] ?? (selfSelected ? self : null);
  const displayName = display?.name ?? t(($) => $.recipientPicker.choose);
  const isRelated = firstPage?.scope === "related";
  const sourceMember = firstPage?.source_member;
  const exactAccount = exact.data ? toAccount(exact.data) : null;
  const exactAllowed = exactAccount !== null && (!membersOnly || exactAccount.external === false);
  const navigationResetKey = `${query}:${allAccounts.map(accountIdentity).join(",")}`;
  const canSelectMore = maxSelected === undefined || selectedAccounts.length < maxSelected;

  const pick = (account: CollaborationRecipientAccount) => {
    if (membersOnly && account.external !== false) return;
    if (allowSelf && accountIdentity(account) === accountIdentity(self!)) {
      onSelect(null);
    } else {
      onSelect(account);
    }
    if (!multiple) setOpen(false);
  };

  const renderAccount = (account: CollaborationRecipientAccount) => {
    const id = accountIdentity(account);
    const isSelf = allowSelf && id === accountIdentity(self!);
    const isSelected = isSelf ? selfSelected : selectedIds.has(id);
    const unavailable = membersOnly && account.external !== false;
    const blocked = unavailable || multiple && !isSelected && !canSelectMore;
    return <div key={id} data-recipient-row>
      <PickerItem selected={isSelected} disabled={blocked} onClick={() => pick(account)}>
        <ActorAvatar name={account.name} initials={initials(account.name)} avatarUrl={account.avatar_url} size="sm" />
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className="min-w-0 truncate">{account.name}</span>
            {account.external === true && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-caption text-muted-foreground">{t(($) => $.recipientPicker.external)}</span>}
          </span>
        </span>
      </PickerItem>
    </div>;
  };

  const list = query
    ? <div role="listbox" aria-label={label}>{allAccounts.map(renderAccount)}</div>
    : <div role="listbox" aria-label={label}>
      {sections.map((section) => section.accounts.length > 0 && <PickerSection key={section.key} label={t(($) => $.recipientPicker.sections[section.key])}>
        {section.accounts.map(renderAccount)}
      </PickerSection>)}
    </div>;

  const exactFooter = query && <div className="space-y-2">
    {!exactOpen && <button type="button" className="flex min-h-11 w-full items-center gap-2 rounded-md px-2 text-left text-caption text-muted-foreground hover:bg-accent hover:text-foreground" onClick={() => setExactOpen(true)}>
      <Search aria-hidden className="size-4 shrink-0" />{t(($) => $.recipientPicker.exactLookup)}
    </button>}
    {exactOpen && <div className="space-y-2 border-t p-2">
      <label className="sr-only" htmlFor={exactId}>{t(($) => $.recipientPicker.exactLookup)}</label>
      <div className="flex gap-2">
        <Input id={exactId} value={exactText} placeholder={t(($) => $.recipientPicker.exactPlaceholder)} className="min-h-11 min-w-0 flex-1" onChange={(event) => { setExactText(event.target.value); setExactRequest(""); }} />
        <Button type="button" variant="outline" className="min-h-11 shrink-0" disabled={!exactText.trim() || exact.isFetching} onClick={() => { const value = exactText.trim(); if (exactRequest === value) void exact.refetch(); else setExactRequest(value); }}>{t(($) => $.recipientPicker.findExact)}</Button>
      </div>
      {exact.isError && <CollaborationQueryState error={exact.error} retry={() => void exact.refetch()} />}
      {exact.data && !exactAllowed && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.fields.sourceAccountOnly)}</p>}
      {exactAccount && exactAllowed && <div className="flex items-center gap-2 rounded-md border border-border p-2">
        <ActorAvatar name={exactAccount.name} initials={initials(exactAccount.name)} avatarUrl={exactAccount.avatar_url} size="sm" />
        <span className="min-w-0 flex-1 truncate text-body">{exactAccount.name}</span>
        {exactAccount.external === true && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-caption text-muted-foreground">{t(($) => $.recipientPicker.external)}</span>}
        <Button type="button" variant="outline" className="min-h-11 shrink-0" disabled={multiple && !canSelectMore} onClick={() => pick(exactAccount)}>{t(($) => $.creation.useRecipient)}</Button>
      </div>}
    </div>}
  </div>;

  return <div data-recipient-picker className="min-w-0">
    <PropertyPicker open={open} onOpenChange={(next) => { setOpen(next); if (!next) { setExactOpen(false); setExactText(""); setExactRequest(""); } }}
      width="w-96 max-w-[calc(100vw-24px)]" align="start" searchable searchPlaceholder={t(($) => $.recipientPicker.search)}
      navigationResetKey={navigationResetKey} onSearchChange={setInput}
      triggerAriaLabel={label} triggerRender={<button type="button" aria-label={label} disabled={disabled} className="flex min-h-11 w-full min-w-0 items-center gap-2 rounded-md border border-input bg-background px-3 text-left text-body hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50" />} trigger={<span className="flex min-w-0 items-center gap-2">
        {display && <ActorAvatar name={display.name} initials={initials(display.name)} avatarUrl={display.avatar_url} size="sm" />}
        <span className="min-w-0 flex-1 truncate">{multiple && selectedAccounts.length > 1 ? t(($) => $.recipientPicker.selectedCount, { count: selectedAccounts.length }) : displayName}</span>
        {display?.external === true && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-caption text-muted-foreground">{t(($) => $.recipientPicker.external)}</span>}
        <ChevronDown aria-hidden className="size-4 shrink-0 text-muted-foreground" />
      </span>}
      footer={<>
        {search.isFetchingNextPage && <p role="status" className="px-2 py-2 text-caption text-muted-foreground"><LoaderCircle aria-hidden className="mr-1 inline size-3.5 animate-spin" />{t(($) => $.recipientPicker.loadingMore)}</p>}
        {search.hasNextPage && <button type="button" className="min-h-11 w-full rounded-md px-2 text-left text-caption text-muted-foreground hover:bg-accent hover:text-foreground" disabled={search.isFetchingNextPage} onClick={() => void search.fetchNextPage()}>{t(($) => $.recipientPicker.loadMore)}</button>}
        {exactFooter}
      </>}
    >
      {isRelated && <p role="note" className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.recipientPicker.relatedHint)}</p>}
      {membersOnly && sourceMember === false && <p role="note" className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.recipientPicker.membersOnlyHint)}</p>}
      {search.isPending && !search.data && <p role="status" className="px-2 py-3 text-caption text-muted-foreground"><LoaderCircle aria-hidden className="mr-1 inline size-3.5 animate-spin" />{t(($) => $.recipientPicker.loading)}</p>}
      {search.isError && !search.data && <CollaborationQueryState error={search.error} retry={() => void search.refetch()} />}
      {!search.isPending && !search.isError && allAccounts.length === 0 && <p className="px-2 py-3 text-center text-caption text-muted-foreground">{t(($) => $.recipientPicker.empty)}</p>}
      {list}
    </PropertyPicker>
  </div>;
}
