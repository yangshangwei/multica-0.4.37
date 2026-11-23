"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { GitBranch, LoaderCircle, Plus, Search, Trash2 } from "lucide-react";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { toast } from "sonner";
import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useConfigStore } from "@multica/core/config";
import { useRepoProvider } from "@multica/core/vcs";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentWorkspace } from "@multica/core/paths";
import { memberListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import {
  githubInstallationRepositoriesOptions,
  githubInstallationsOptions,
} from "@multica/core/github";
import { api } from "@multica/core/api";
import type {
  GitHubRepository,
  Workspace,
  WorkspaceRepo,
} from "@multica/core/types";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import {
  SettingsCard,
  SettingsSaveState,
  SettingsSection,
  SettingsTab,
} from "./settings-layout";
import { useAutoSave } from "./use-auto-save";
import { GitHubMark } from "./github-mark";
import { getSettingsQueryState, SettingsQueryFeedback } from "./settings-query-feedback";

const EMPTY_REPOSITORIES: WorkspaceRepo[] = [];

function repositoriesValid(repositories: WorkspaceRepo[]) {
  return repositories.every((repo) => repo.url.trim().length > 0);
}

function repositoriesEqual(left: WorkspaceRepo[], right: WorkspaceRepo[]) {
  if (left.length !== right.length) return false;
  return left.every(
    (repo, index) =>
      repo.url === right[index]?.url &&
      (repo.description ?? "") === (right[index]?.description ?? ""),
  );
}

export function repositoryIdentity(rawURL: string): string | null {
  const value = rawURL.trim();
  if (!value) return null;

  let host = "";
  let path = "";
  if (!value.includes("://")) {
    const scpLike = value.match(/^(?:[^@\s/]+@)?([^:\s/]+):(.+)$/);
    if (scpLike) {
      host = scpLike[1] ?? "";
      path = scpLike[2] ?? "";
    }
  }
  if (!host) {
    try {
      const parsed = new URL(value);
      host = parsed.hostname;
      path = parsed.pathname;
    } catch {
      return null;
    }
  }

  const normalizedPath = path
    .replace(/^\/+|\/+$/g, "")
    .replace(/\.git$/i, "");
  if (!host || !normalizedPath) return null;
  return `${host.toLowerCase()}/${normalizedPath}`;
}

export function RepositoriesTab() {
  const { t } = useT("settings");
  const user = useAuthStore((state) => state.user);
  const workspace = useCurrentWorkspace();
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const navigation = useNavigation();
  const membersQuery = useQuery(memberListOptions(wsId));
  const membersState = getSettingsQueryState(membersQuery);
  const members = membersState.hasData ? membersQuery.data ?? [] : [];
  const [repositories, setRepositories] = useState<WorkspaceRepo[]>(
    workspace?.repos ?? EMPTY_REPOSITORIES,
  );
  const [pendingRemovalIndex, setPendingRemovalIndex] = useState<number | null>(null);
  const [removing, setRemoving] = useState(false);
  const removingRef = useRef(false);
  const [connectingGitHub, setConnectingGitHub] = useState(false);
  const connectingGitHubRef = useRef(false);
  const [githubPickerOpen, setGitHubPickerOpen] = useState(false);
  const [selectedInstallationID, setSelectedInstallationID] = useState("");
  const [selectedRepositories, setSelectedRepositories] = useState<
    Map<number, GitHubRepository>
  >(new Map());
  const [repositorySearch, setRepositorySearch] = useState("");

  const currentMember = members.find((member) => member.user_id === user?.id) ?? null;
  const canManageWorkspace =
    currentMember?.role === "owner" || currentMember?.role === "admin";
  const githubInstallationsQuery = useQuery({
    ...githubInstallationsOptions(wsId),
    enabled: !!wsId && canManageWorkspace,
  });
  const githubInstallationsState = getSettingsQueryState(githubInstallationsQuery);
  const githubData = githubInstallationsState.hasData ? githubInstallationsQuery.data : undefined;
  const githubInstallationsPending = githubInstallationsQuery.isPending;
  const githubInstallationsFetching = githubInstallationsQuery.isFetching;
  const githubInstallations = useMemo(
    () => githubData?.installations ?? [],
    [githubData?.installations],
  );
  const githubConnectConfigured = githubData?.configured === true;
  const githubBrowseConfigured =
    githubData?.repository_browse_configured === true;
  // Deployments without a GitHub App (intranet installs on a private GitLab)
  // get a connect button for the token-based Git providers instead. It leads
  // to the Integrations tab, where the connection form lives; repos are then
  // added here by URL. Once a provider is connected the button carries its
  // name; before that it defaults to GitLab, this deployment's target host.
  const vcsAvailable = useConfigStore((s) => s.vcsIntegrationAvailable);
  const repoProvider = useRepoProvider(wsId);
  const showVCSConnect =
    vcsAvailable && githubInstallationsState.hasData && !githubConnectConfigured && githubInstallations.length === 0;
  const vcsProviderName =
    repoProvider.kind === "git" || repoProvider.kind === "github"
      ? "GitLab"
      : repoProvider.name;
  const openIntegrations = () => {
    const params = new URLSearchParams(navigation.searchParams);
    params.set("tab", "integrations");
    navigation.replace(`${navigation.pathname}?${params.toString()}`);
  };
  const githubRepositoriesQuery = useInfiniteQuery({
    ...githubInstallationRepositoriesOptions(wsId, selectedInstallationID),
    enabled:
      githubPickerOpen &&
      canManageWorkspace &&
      githubBrowseConfigured &&
      !!selectedInstallationID,
  });
  const githubRepositoriesState = getSettingsQueryState(githubRepositoriesQuery);
  const githubRepositories = useMemo(
    () =>
      githubRepositoriesState.hasData ? githubRepositoriesQuery.data?.pages.flatMap(
        (page) => page.repositories,
      ) ?? [] : [],
    [githubRepositoriesQuery.data?.pages, githubRepositoriesState.hasData],
  );
  const existingRepositoryIdentities = useMemo(
    () =>
      new Set(
        repositories
          .map((repository) => repositoryIdentity(repository.url))
          .filter((identity): identity is string => !!identity),
      ),
    [repositories],
  );
  const filteredGitHubRepositories = useMemo(() => {
    const search = repositorySearch.trim().toLowerCase();
    if (!search) return githubRepositories;
    return githubRepositories.filter((repository) =>
      repository.full_name.toLowerCase().includes(search),
    );
  }, [githubRepositories, repositorySearch]);

  useEffect(() => {
    setRepositories(workspace?.repos ?? EMPTY_REPOSITORIES);
    setPendingRemovalIndex(null);
    // A cache update after auto-save replaces the Workspace object. Keying on
    // identity prevents that response from wiping a newer local keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentionally keyed on workspace identity
  }, [workspace?.id]);

  useEffect(() => {
    if (!canManageWorkspace || githubInstallationsState.accessDenied) {
      setGitHubPickerOpen(false);
      setSelectedRepositories(new Map());
      setRepositorySearch("");
    }
    if (!canManageWorkspace) setPendingRemovalIndex(null);
  }, [canManageWorkspace, githubInstallationsState.accessDenied]);

  useEffect(() => {
    if (
      selectedInstallationID &&
      githubInstallations.some(
        (installation) => installation.id === selectedInstallationID,
      )
    ) {
      return;
    }
    setSelectedInstallationID(githubInstallations[0]?.id ?? "");
  }, [githubInstallations, selectedInstallationID]);

  useEffect(() => {
    const connected = navigation.searchParams.get("github_connected") === "1";
    const githubError = navigation.searchParams.get("github_error");
    if ((!connected && !githubError) || !canManageWorkspace) return;
    if (
      !githubError &&
      (githubInstallationsPending || githubInstallationsFetching || !githubInstallationsState.hasData)
    ) {
      return;
    }

    if (githubError) {
      toast.error(t(($) => $.repositories.github_connect_failed));
    } else if (githubInstallations.length > 0 && githubBrowseConfigured) {
      setSelectedInstallationID(githubInstallations[0]!.id);
      setGitHubPickerOpen(true);
    } else if (githubInstallations.length > 0) {
      toast.error(t(($) => $.repositories.github_browse_not_configured));
    }

    const next = new URLSearchParams(navigation.searchParams);
    next.delete("github_connected");
    next.delete("github_error");
    const search = next.toString();
    navigation.replace(`${navigation.pathname}${search ? `?${search}` : ""}`);
  }, [
    canManageWorkspace,
    githubBrowseConfigured,
    githubInstallations,
    githubInstallationsFetching,
    githubInstallationsPending,
    githubInstallationsState.hasData,
    navigation,
    t,
  ]);

  const savedRepositories = workspace?.repos ?? EMPTY_REPOSITORIES;
  const draft = useMemo(() => repositories, [repositories]);
  const saveRepositories = useCallback(
    async (next: WorkspaceRepo[]) => {
      if (!workspace) return;
      const updated = await api.updateWorkspace(workspace.id, { repos: next });
      queryClient.setQueryData(
        workspaceKeys.list(),
        (old: Workspace[] | undefined) =>
          old?.map((item) => (item.id === updated.id ? updated : item)),
      );
    },
    [queryClient, workspace],
  );
  const allUrlsValid = repositoriesValid(repositories);
  const autoSave = useAutoSave({
    value: draft,
    savedValue: savedRepositories,
    onSave: saveRepositories,
    onSuccess: () =>
      toast.success(t(($) => $.repositories.toast_saved), {
        id: "settings-auto-save",
      }),
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t(($) => $.repositories.toast_save_failed),
      ),
    enabled: !!workspace && canManageWorkspace,
    isValid: repositoriesValid,
    isEqual: repositoriesEqual,
  });

  const updateRepository = (
    index: number,
    field: keyof WorkspaceRepo,
    value: string,
  ) => {
    if (!canManageWorkspace || removingRef.current) return;
    setRepositories((current) =>
      current.map((repo, repoIndex) =>
        repoIndex === index ? { ...repo, [field]: value } : repo,
      ),
    );
  };

  const addRepository = () => {
    if (!canManageWorkspace || removingRef.current) return;
    setRepositories((current) => [...current, { url: "" }]);
  };

  const openGitHubPicker = () => {
    if (!canManageWorkspace || !githubInstallationsState.hasData || removingRef.current) return;
    setSelectedInstallationID(
      selectedInstallationID || githubInstallations[0]?.id || "",
    );
    setGitHubPickerOpen(true);
  };

  const handleGitHubAction = async () => {
    if (!canManageWorkspace || !githubInstallationsState.hasData || connectingGitHubRef.current || removingRef.current) return;
    if (githubInstallations.length > 0) {
      openGitHubPicker();
      return;
    }
    connectingGitHubRef.current = true;
    setConnectingGitHub(true);
    try {
      const response = await api.getGitHubConnectURL(wsId, "repositories");
      if (!response.configured || !response.url) {
        toast.error(t(($) => $.repositories.github_not_configured));
        return;
      }
      window.open(response.url, "_blank", "noopener");
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t(($) => $.repositories.github_connect_failed),
      );
    } finally {
      connectingGitHubRef.current = false;
      setConnectingGitHub(false);
    }
  };

  const closeGitHubPicker = () => {
    setGitHubPickerOpen(false);
    setSelectedRepositories(new Map());
    setRepositorySearch("");
  };

  const toggleGitHubRepository = (
    repository: GitHubRepository,
    checked: boolean,
  ) => {
    setSelectedRepositories((current) => {
      const next = new Map(current);
      if (checked) next.set(repository.id, repository);
      else next.delete(repository.id);
      return next;
    });
  };

  const importGitHubRepositories = () => {
    if (!canManageWorkspace || !githubRepositoriesState.hasData || removingRef.current) return;
    if (!allUrlsValid) {
      toast.error(t(($) => $.repositories.complete_manual_entry_first));
      return;
    }
    const additions: WorkspaceRepo[] = [];
    const known = new Set(existingRepositoryIdentities);
    for (const repository of selectedRepositories.values()) {
      const identity = repositoryIdentity(repository.clone_url);
      if (!identity || known.has(identity) || repository.archived) continue;
      known.add(identity);
      additions.push({
        url: repository.clone_url,
        ...(repository.description?.trim()
          ? { description: repository.description.trim() }
          : {}),
      });
    }
    if (additions.length === 0) {
      closeGitHubPicker();
      return;
    }
    const next = [...repositories, ...additions];
    setRepositories(next);
    void autoSave.saveNow(next);
    closeGitHubPicker();
  };

  const removeRepository = async (index: number) => {
    if (!canManageWorkspace || removingRef.current) return;
    const next = repositories.filter((_, repoIndex) => repoIndex !== index);
    if (!repositoriesValid(next)) {
      toast.error(t(($) => $.repositories.complete_manual_entry_first));
      return;
    }
    removingRef.current = true;
    setRemoving(true);
    try {
      if (await autoSave.saveNow(next)) {
        setRepositories(next);
        setPendingRemovalIndex(null);
      }
    } finally {
      removingRef.current = false;
      setRemoving(false);
    }
  };

  if (!workspace) return null;

  return (
    <SettingsTab title={t(($) => $.page.tabs.repositories)}>
      <SettingsQueryFeedback
        query={membersQuery}
        loadingMessage={t(($) => $.query_feedback.permissions_loading)}
        errorMessage={t(($) => $.query_feedback.permissions_load_failed)}
        refreshMessage={t(($) => $.query_feedback.permissions_refresh_failed)}
      />
      {!membersState.accessDenied && <>
      <SettingsSection
        description={t(($) => $.repositories.description)}
        action={
          <SettingsSaveState
            status={autoSave.status}
            savingLabel={t(($) => $.auto_save.saving)}
            savedLabel={t(($) => $.auto_save.saved)}
            errorLabel={t(($) => $.auto_save.failed)}
          />
        }
      >
        {canManageWorkspace && (
          <SettingsQueryFeedback
            query={githubInstallationsQuery}
            loadingMessage={t(($) => $.repositories.github_installations_loading)}
            errorMessage={t(($) => $.repositories.github_installations_load_failed)}
            refreshMessage={t(($) => $.repositories.github_installations_refresh_failed)}
          />
        )}
        <SettingsCard>
          {repositories.length === 0 ? (
            <div className="px-4 py-8 text-center text-caption text-muted-foreground">
              {t(($) => $.repositories.empty)}
            </div>
          ) : null}

          {repositories.map((repository, index) => (
            <div
              key={index}
              className="grid gap-2 px-4 py-3.5 sm:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)_auto] sm:items-center"
            >
              <Input
                type="text"
                name={`repository-${index}-url`}
                autoComplete="off"
                spellCheck={false}
                aria-label={t(($) => $.repositories.url_placeholder)}
                value={repository.url}
                onChange={(event) =>
                  updateRepository(index, "url", event.target.value)
                }
                onBlur={autoSave.flush}
                disabled={!canManageWorkspace || removing}
                aria-invalid={!repository.url.trim()}
                placeholder={t(($) => $.repositories.url_placeholder)}
                className="font-mono text-caption"
              />
              <Input
                type="text"
                name={`repository-${index}-description`}
                autoComplete="off"
                aria-label={t(($) => $.repositories.description_placeholder)}
                value={repository.description ?? ""}
                onChange={(event) =>
                  updateRepository(index, "description", event.target.value)
                }
                onBlur={autoSave.flush}
                disabled={!canManageWorkspace || removing}
                placeholder={t(($) => $.repositories.description_placeholder)}
              />
              {canManageWorkspace ? (
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t(($) => $.repositories.delete_aria)}
                  className="justify-self-end text-muted-foreground hover:text-destructive"
                  disabled={removing}
                  onClick={() => {
                    if (!removingRef.current) setPendingRemovalIndex(index);
                  }}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              ) : null}
            </div>
          ))}

          {canManageWorkspace ? (
            <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5">
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="outline" size="sm" onClick={addRepository} disabled={removing}>
                  <Plus className="size-3.5" />
                  {t(($) => $.repositories.add)}
                </Button>
                {showVCSConnect ? (
                  <Button size="sm" onClick={openIntegrations} disabled={removing}>
                    <GitBranch className="size-3.5" />
                    {t(($) => $.repositories.connect_vcs, { provider: vcsProviderName })}
                  </Button>
                ) : githubInstallationsState.hasData ? (
                <Button
                  size="sm"
                  onClick={handleGitHubAction}
                  disabled={
                    connectingGitHub ||
                    removing ||
                    !githubBrowseConfigured ||
                    (!githubConnectConfigured &&
                      githubInstallations.length === 0)
                  }
                  title={
                    !githubBrowseConfigured
                      ? t(($) => $.repositories.github_browse_not_configured)
                      : undefined
                  }
                >
                  {connectingGitHub ? (
                    <LoaderCircle className="size-3.5 animate-spin" />
                  ) : (
                    <GitHubMark className="size-3.5" />
                  )}
                  {githubInstallations.length > 0
                    ? t(($) => $.repositories.choose_from_github)
                    : t(($) => $.repositories.connect_github)}
                </Button>
                ) : null}
              </div>
              {!allUrlsValid ? (
                <span className="text-caption text-muted-foreground">
                  {t(($) => $.repositories.url_empty)}
                </span>
              ) : null}
            </div>
          ) : currentMember?.role === "member" ? (
            <div className="px-4 py-3 text-caption text-muted-foreground">
              {t(($) => $.repositories.manage_hint)}
            </div>
          ) : null}
        </SettingsCard>
      </SettingsSection>
      </>}

      <Dialog
        open={githubPickerOpen && canManageWorkspace && githubInstallationsState.hasData}
        onOpenChange={(open) => {
          if (!open) closeGitHubPicker();
        }}
      >
        <DialogContent className="flex max-h-[85vh] flex-col gap-0 p-0 sm:max-w-2xl">
          <DialogHeader className="border-b px-6 py-5">
            <DialogTitle>
              {t(($) => $.repositories.github_picker_title)}
            </DialogTitle>
            <DialogDescription>
              {t(($) => $.repositories.github_picker_description)}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 px-6 py-4">
            {githubInstallations.length > 1 ? (
              <Select
                items={githubInstallations.map((installation) => ({
                  value: installation.id,
                  label: installation.account_login,
                }))}
                value={selectedInstallationID}
                onValueChange={(value) =>
                  setSelectedInstallationID(value ?? "")
                }
              >
                <SelectTrigger
                  aria-label={t(($) => $.repositories.github_account)}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {githubInstallations.map((installation) => (
                    <SelectItem
                      key={installation.id}
                      value={installation.id}
                    >
                      {installation.account_login}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : githubInstallations[0] ? (
              <p className="text-caption text-muted-foreground">
                {t(($) => $.repositories.github_account)}:{" "}
                <span className="font-medium text-foreground">
                  {githubInstallations[0].account_login}
                </span>
              </p>
            ) : null}

            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={repositorySearch}
                onChange={(event) => setRepositorySearch(event.target.value)}
                placeholder={t(
                  ($) => $.repositories.github_search_placeholder,
                )}
                aria-label={t(
                  ($) => $.repositories.github_search_placeholder,
                )}
                className="pl-8"
              />
            </div>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto border-y">
            <div className="px-6 py-4 empty:hidden">
              <SettingsQueryFeedback
                query={githubRepositoriesQuery}
                loadingMessage={t(($) => $.repositories.github_loading)}
                errorMessage={t(($) => $.repositories.github_load_failed)}
                refreshMessage={t(($) => $.repositories.github_refresh_failed)}
              />
            </div>
            {githubRepositoriesState.hasData && (filteredGitHubRepositories.length === 0 ? (
              <div className="px-6 py-12 text-center text-body text-muted-foreground">
                {repositorySearch
                  ? t(($) => $.repositories.github_no_search_results)
                  : t(($) => $.repositories.github_empty)}
              </div>
            ) : (
              <div className="divide-y">
                {filteredGitHubRepositories.map((repository) => {
                  const identity = repositoryIdentity(repository.clone_url);
                  const alreadyAdded =
                    !!identity && existingRepositoryIdentities.has(identity);
                  const disabled = alreadyAdded || repository.archived;
                  return (
                    <label
                      key={repository.id}
                      htmlFor={`github-repository-${repository.id}`}
                      className="flex items-start gap-3 px-6 py-3.5"
                    >
                      <Checkbox
                        id={`github-repository-${repository.id}`}
                        checked={
                          alreadyAdded ||
                          selectedRepositories.has(repository.id)
                        }
                        disabled={disabled}
                        onCheckedChange={(checked) =>
                          toggleGitHubRepository(
                            repository,
                            checked === true,
                          )
                        }
                        className="mt-0.5"
                      />
                      <span className="min-w-0 flex-1 space-y-1">
                        <span className="flex flex-wrap items-center gap-2">
                          <span className="truncate text-body font-medium">
                            {repository.full_name}
                          </span>
                          {repository.private ? (
                            <Badge variant="secondary">
                              {t(($) => $.repositories.github_private)}
                            </Badge>
                          ) : null}
                          {repository.archived ? (
                            <Badge variant="outline">
                              {t(($) => $.repositories.github_archived)}
                            </Badge>
                          ) : null}
                          {alreadyAdded ? (
                            <Badge variant="outline">
                              {t(($) => $.repositories.github_added)}
                            </Badge>
                          ) : null}
                        </span>
                        {repository.description ? (
                          <span className="block truncate text-caption text-muted-foreground">
                            {repository.description}
                          </span>
                        ) : null}
                      </span>
                    </label>
                  );
                })}
              </div>
            ))}

            {githubRepositoriesState.hasData && githubRepositoriesQuery.hasNextPage ? (
              <div className="flex justify-center border-t p-3">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    if (!githubRepositoriesQuery.isFetching) void githubRepositoriesQuery.fetchNextPage({ cancelRefetch: false });
                  }}
                  disabled={githubRepositoriesQuery.isFetching}
                >
                  {githubRepositoriesQuery.isFetchingNextPage
                    ? t(($) => $.repositories.github_loading)
                    : t(($) => $.repositories.github_load_more)}
                </Button>
              </div>
            ) : null}
          </div>

          <DialogFooter className="m-0 border-t bg-muted/30 px-6 py-4">
            <p className="mr-auto text-caption text-muted-foreground">
              {t(($) => $.repositories.github_selected_count, {
                count: selectedRepositories.size,
              })}
            </p>
            <Button variant="ghost" onClick={closeGitHubPicker}>
              {t(($) => $.repositories.github_cancel)}
            </Button>
            <Button
              onClick={importGitHubRepositories}
              disabled={selectedRepositories.size === 0 || !allUrlsValid || !githubRepositoriesState.hasData}
            >
              {t(($) => $.repositories.github_import)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={pendingRemovalIndex !== null && canManageWorkspace}
        onOpenChange={(open) => {
          if (!open && !removingRef.current) setPendingRemovalIndex(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.repositories.delete_confirm_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.repositories.delete_confirm_description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removing}>
              {t(($) => $.repositories.delete_confirm_cancel)}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={removing}
              aria-busy={removing}
              onClick={() => {
                if (pendingRemovalIndex !== null) {
                  void removeRepository(pendingRemovalIndex);
                }
              }}
            >
              {t(($) => $.repositories.delete_confirm_action)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsTab>
  );
}
