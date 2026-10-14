"use client";

import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Trash2, Copy, Check, Info } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import type { PersonalAccessToken } from "@multica/core/types";
import { Alert, AlertDescription } from "@multica/ui/components/ui/alert";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@multica/ui/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
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
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { copyText } from "@multica/ui/lib/clipboard";
import { toast } from "sonner";
import { errorCode } from "@multica/core/api";
import {
  personalAccessTokensOptions,
  useAuthStore,
  useCreatePersonalAccessToken,
  useRevokePersonalAccessToken,
} from "@multica/core/auth";
import { useT } from "../../i18n";
import { SettingsSection, SettingsTab } from "./settings-layout";
import { getSettingsQueryState, SettingsQueryFeedback } from "./settings-query-feedback";

const EXPIRY_KEYS = ["30", "90", "365", "never"] as const;

export function TokensTab() {
  const userId = useAuthStore((state) => state.user?.id ?? null);
  return <TokenSettings key={userId} userId={userId} />;
}

function TokenSettings({ userId }: { userId: string | null }) {
  const { t } = useT("settings");
  const queryClient = useQueryClient();
  const tokenOptions = personalAccessTokensOptions(userId);
  const tokensQuery = useQuery(tokenOptions);
  const tokenState = getSettingsQueryState(tokensQuery);
  const tokens = tokenState.hasData ? tokensQuery.data ?? [] : [];
  const canManage = !!userId && tokenState.hasData;
  const expiryItems = EXPIRY_KEYS.map((value) => ({
    value,
    label: t(($) => $.tokens.expiry[value]),
  }));
  const [tokenName, setTokenName] = useState("");
  const [tokenExpiry, setTokenExpiry] = useState("90");
  const [tokenCreating, setTokenCreating] = useState(false);
  const [newToken, setNewToken] = useState<string | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [commandCopied, setCommandCopied] = useState(false);
  const [storedConfirmed, setStoredConfirmed] = useState(false);
  const [tokenRevoking, setTokenRevoking] = useState<string | null>(null);
  const [revokeConfirmId, setRevokeConfirmId] = useState<string | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);
  const writingRef = useRef(false);
  const writing = tokenCreating || tokenRevoking !== null;
  const createTokenMutation = useCreatePersonalAccessToken((token) => {
    if (useAuthStore.getState().user?.id !== userId) return;
    setNewToken(token);
    setTokenName("");
    setTokenExpiry("90");
  });
  const revokeTokenMutation = useRevokePersonalAccessToken();

  const handleCreateToken = async () => {
    if (!canManage || !tokenName.trim() || writingRef.current || newToken) return;
    writingRef.current = true;
    setTokenCreating(true);
    setCreateError(null);
    try {
      const expiresInDays = tokenExpiry === "never" ? undefined : Number(tokenExpiry);
      await createTokenMutation.mutateAsync({ name: tokenName.trim(), expires_in_days: expiresInDays });
      if (useAuthStore.getState().user?.id !== userId) return;
      await tokensQuery.refetch();
    } catch (e) {
      if (useAuthStore.getState().user?.id !== userId) return;
      const invalidResponse = errorCode(e) === "invalid_personal_access_token";
      const message = invalidResponse ? t(($) => $.tokens.create_response_invalid)
        : e instanceof Error && e.message ? e.message : t(($) => $.tokens.toast_create_failed);
      setCreateError(message);
      toast.error(message);
      if (invalidResponse) await tokensQuery.refetch();
    } finally {
      writingRef.current = false;
      setTokenCreating(false);
    }
  };

  const handleRevokeToken = async (id: string) => {
    if (!canManage || writingRef.current) return;
    writingRef.current = true;
    setTokenRevoking(id);
    setRevokeError(null);
    try {
      await revokeTokenMutation.mutateAsync(id);
      if (useAuthStore.getState().user?.id !== userId) return;
      queryClient.setQueryData<PersonalAccessToken[]>(tokenOptions.queryKey, (previous) =>
        previous?.filter((token) => token.id !== id),
      );
      await queryClient.invalidateQueries({ queryKey: tokenOptions.queryKey });
      if (useAuthStore.getState().user?.id !== userId) return;
      toast.success(t(($) => $.tokens.toast_revoked));
      setRevokeConfirmId(null);
    } catch (e) {
      if (useAuthStore.getState().user?.id !== userId) return;
      const message = e instanceof Error && e.message ? e.message : t(($) => $.tokens.toast_revoke_failed);
      setRevokeError(message);
      toast.error(message);
    } finally {
      writingRef.current = false;
      setTokenRevoking(null);
    }
  };

  const handleCopyToken = async () => {
    if (!newToken) return;
    if (await copyText(newToken)) {
      setTokenCopied(true);
      setTimeout(() => setTokenCopied(false), 2000);
    }
  };

  const handleCopyCommand = async () => {
    if (!newToken) return;
    if (await copyText(`multica login --token ${newToken}`)) {
      setCommandCopied(true);
      setTimeout(() => setCommandCopied(false), 2000);
    }
  };

  const closeCreatedDialog = () => {
    if (!storedConfirmed) return;
    setNewToken(null);
    setTokenCopied(false);
    setCommandCopied(false);
    setStoredConfirmed(false);
  };

  return (
    <SettingsTab title={t(($) => $.tokens.title)}>
      <SettingsSection
        description={
          <>
            {t(($) => $.tokens.description)}
            <br />
            {t(($) => $.tokens.security_note)}
          </>
        }
      >
        <SettingsQueryFeedback
          query={tokensQuery}
          loadingMessage={t(($) => $.tokens.loading)}
          errorMessage={t(($) => $.tokens.load_failed)}
          refreshMessage={t(($) => $.tokens.refresh_failed)}
        />
        <Card>
          <CardContent className="space-y-3">
            <div className="grid gap-3 sm:grid-cols-[1fr_120px_auto]">
              <Input
                type="text"
                name="token-name"
                autoComplete="off"
                disabled={!canManage || writing}
                aria-label={t(($) => $.tokens.name_placeholder)}
                value={tokenName}
                onChange={(e) => setTokenName(e.target.value)}
                placeholder={t(($) => $.tokens.name_placeholder)}
              />
              <Select
                items={expiryItems}
                value={tokenExpiry}
                disabled={!canManage || writing}
                onValueChange={(v) => { if (v) setTokenExpiry(v); }}
              >
                <SelectTrigger
                  size="sm"
                  aria-label={t(($) => $.tokens.expiry_label)}
                ><SelectValue /></SelectTrigger>
                <SelectContent>
                  {expiryItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button onClick={handleCreateToken} disabled={!canManage || writing || !tokenName.trim()}>
                {tokenCreating ? t(($) => $.tokens.creating) : t(($) => $.tokens.create)}
              </Button>
            </div>
            {createError && <p role="alert" className="text-caption text-destructive">{createError}</p>}
          </CardContent>
        </Card>

        {!tokenState.hasData && !tokenState.failed ? (
          <div className="space-y-2">
            {Array.from({ length: 2 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="flex items-center gap-3">
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                  <Skeleton className="h-8 w-8 rounded" />
                </CardContent>
              </Card>
            ))}
          </div>
        ) : !tokenState.hasData ? null : tokens.length === 0 ? (
          <Card>
            <CardContent>
              <p className="text-caption text-muted-foreground">
                {t(($) => $.tokens.empty)}
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {tokens.map((token) => (
              <Card key={token.id}>
                <CardContent className="flex items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="text-body font-medium truncate">{token.name}</div>
                    <div className="text-caption text-muted-foreground">
                      {t(($) => $.tokens.metadata_prefix, {
                        prefix: token.token_prefix,
                        created: new Date(token.created_at).toLocaleDateString(),
                        lastUsed: token.last_used_at
                          ? t(($) => $.tokens.last_used_with_date, {
                              date: new Date(token.last_used_at!).toLocaleDateString(),
                            })
                          : t(($) => $.tokens.last_used_never),
                      })}
                      {token.expires_at && t(($) => $.tokens.expires_with_date, {
                        date: new Date(token.expires_at!).toLocaleDateString(),
                      })}
                    </div>
                  </div>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => { setRevokeError(null); setRevokeConfirmId(token.id); }}
                          disabled={writing}
                          aria-label={t(($) => $.tokens.revoke_aria, { name: token.name })}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      }
                    />
                    <TooltipContent>{t(($) => $.tokens.revoke_tooltip)}</TooltipContent>
                  </Tooltip>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </SettingsSection>

      <AlertDialog open={canManage && !!revokeConfirmId} onOpenChange={(v) => { if (!v && !writingRef.current) setRevokeConfirmId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.tokens.revoke_dialog.title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.tokens.revoke_dialog.description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {revokeError && <p role="alert" className="text-caption text-destructive">{revokeError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={writing}>{t(($) => $.tokens.revoke_dialog.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={writing}
              onClick={async () => {
                if (revokeConfirmId) await handleRevokeToken(revokeConfirmId);
              }}
            >
              {tokenRevoking ? t(($) => $.tokens.revoking) : t(($) => $.tokens.revoke_dialog.confirm)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={canManage && !!newToken} onOpenChange={(v, details) => {
        if (!v && !storedConfirmed) { details.cancel(); return; }
        if (!v) closeCreatedDialog();
      }}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{t(($) => $.tokens.created_dialog.title)}</DialogTitle>
          </DialogHeader>
          <Alert>
            <Info />
            <AlertDescription>
              {t(($) => $.tokens.created_dialog.warning_prefix)}
              <span className="font-medium text-foreground">{t(($) => $.tokens.created_dialog.warning_emphasis)}</span>
              {t(($) => $.tokens.created_dialog.warning_suffix)}
            </AlertDescription>
          </Alert>
          <div className="flex min-w-0 items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-md border bg-muted/50 px-3 py-2 text-body select-all">
              {newToken}
            </code>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={handleCopyToken}
                    aria-label={t(($) => $.tokens.created_dialog.copy_tooltip)}
                  >
                    {tokenCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                }
              />
              <TooltipContent>{t(($) => $.tokens.created_dialog.copy_tooltip)}</TooltipContent>
            </Tooltip>
          </div>
          <div className="min-w-0 space-y-1.5">
            <p className="text-caption text-muted-foreground">{t(($) => $.tokens.created_dialog.cli_hint)}</p>
            <div className="flex min-w-0 items-center gap-2">
              <code className="min-w-0 flex-1 truncate rounded-md border bg-muted/50 px-3 py-2 text-body select-all">
                {`multica login --token ${newToken}`}
              </code>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={handleCopyCommand}
                      aria-label={t(($) => $.tokens.created_dialog.copy_command_tooltip)}
                    >
                      {commandCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                  }
                />
                <TooltipContent>{t(($) => $.tokens.created_dialog.copy_command_tooltip)}</TooltipContent>
              </Tooltip>
            </div>
          </div>
          <DialogFooter className="items-center sm:justify-between">
            <label className="flex items-center gap-2 text-body">
              <Checkbox
                checked={storedConfirmed}
                onCheckedChange={(v) => setStoredConfirmed(v === true)}
              />
              {t(($) => $.tokens.created_dialog.confirm_stored)}
            </label>
            <Button disabled={!storedConfirmed} onClick={closeCreatedDialog}>
              {t(($) => $.tokens.created_dialog.done)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsTab>
  );
}
