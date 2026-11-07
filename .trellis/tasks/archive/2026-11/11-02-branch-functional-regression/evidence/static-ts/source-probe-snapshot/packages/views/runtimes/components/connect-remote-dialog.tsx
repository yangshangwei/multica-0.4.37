"use client";

import { useCallback, useEffect, useId, useRef, useState, useSyncExternalStore } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { Check, ChevronRight, Clock3, Copy, TriangleAlert, Terminal } from "lucide-react";
import { onlineManager, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { runtimeKeys, runtimeListOptions } from "@multica/core/runtimes/queries";
import { useWSEvent, useWSReconnect } from "@multica/core/realtime";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { useConfigStore } from "@multica/core/config";
import { DOCS_SLUGS } from "@multica/core/docs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { CODE_LIGATURE_CLASS } from "@multica/ui/lib/code-style";
import { copyText } from "@multica/ui/lib/clipboard";
import { UI_EASE_OUT, UI_MOTION_DURATION } from "@multica/ui/lib/motion";
import { cn } from "@multica/ui/lib/utils";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { connectionCommands, installCommand, type ComputerPlatform } from "./connect-remote-commands";
import { computerBaseline, newlyConnectedComputer } from "./connect-remote-machine";
import { runtimeRowLabel, type RuntimeMachine } from "./runtime-machines";

const PLATFORM_LABELS = { windows: "Windows", macos: "macOS", linux: "Linux" } as const;
const WAIT_DELAY_MS = 120_000;
const subscribeToOnline = (callback: () => void) => onlineManager.subscribe(callback);
const getOnline = () => onlineManager.isOnline();

export function ConnectRemoteDialog({ onClose }: { onClose: () => void }) {
  const wsId = useWorkspaceId();
  const slug = useWorkspaceSlug();
  const userId = useAuthStore((state) => state.user?.id);
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="flex max-h-[85dvh] flex-col gap-0 p-0 sm:max-w-[40rem]">
        <ConnectRemoteContent key={wsId + ":" + userId} wsId={wsId} slug={slug} userId={userId} onClose={onClose} />
      </DialogContent>
    </Dialog>
  );
}

function ConnectRemoteContent({ wsId, slug, userId, onClose }: {
  wsId: string;
  slug: string | null;
  userId: string | undefined;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const navigation = useNavigation();
  const shouldReduceMotion = useReducedMotion() ?? false;
  const online = useSyncExternalStore(subscribeToOnline, getOnline, () => true);
  const [baseline, setBaseline] = useState<Set<string> | null>(null);
  const [computer, setComputer] = useState<RuntimeMachine | null>(null);
  const { data, isSuccess, isError, isFetching, isFetchedAfterMount, refetch } = useQuery({
    ...runtimeListOptions(wsId, "me"),
    enabled: !!userId,
    refetchOnMount: "always",
    refetchInterval: computer ? false : 5000,
    retry: false,
  });

  // The first fresh, schema-validated inventory is the baseline. Cached or
  // failed inventories cannot prove which computers existed when we opened.
  useEffect(() => {
    if (!userId || !isSuccess || !isFetchedAfterMount || isFetching || !data || computer) return;
    if (!baseline) {
      setBaseline(computerBaseline(data, userId, wsId));
      return;
    }
    const connected = newlyConnectedComputer(data, baseline, userId, wsId);
    if (connected) setComputer(connected);
  }, [userId, wsId, data, isSuccess, isFetchedAfterMount, isFetching, baseline, computer]);

  // Notifications prompt a read; their untyped payloads are never success facts.
  const refreshRuntimes = useCallback(() => {
    void qc.invalidateQueries({ queryKey: runtimeKeys.all(wsId) });
  }, [qc, wsId]);
  useWSEvent("daemon:register", refreshRuntimes);
  useWSReconnect(refreshRuntimes);

  const goTo = (destination: string) => {
    onClose();
    navigation.push(destination);
  };

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={computer ? "success" : "instructions"}
        className="flex min-h-0 flex-1 flex-col"
        initial={{ opacity: 0, y: shouldReduceMotion ? 0 : 8 }}
        animate={{ opacity: 1, y: 0, transition: { duration: shouldReduceMotion ? UI_MOTION_DURATION.fast : UI_MOTION_DURATION.standard, ease: UI_EASE_OUT } }}
        exit={{ opacity: 0, y: shouldReduceMotion ? 0 : -8, transition: { duration: shouldReduceMotion ? UI_MOTION_DURATION.fast : UI_MOTION_DURATION.micro, ease: UI_EASE_OUT } }}
      >
        {computer ? (
          <SuccessStep
            computer={computer}
            onGoToAgents={slug ? () => goTo(paths.workspace(slug).newAgent()) : undefined}
            onGoToComputer={slug ? () => goTo(paths.workspace(slug).runtimeDetail(computer.id)) : undefined}
          />
        ) : (
          <InstructionsStep
            onClose={onClose}
            slug={slug}
            baselineReady={baseline !== null}
            interrupted={!online || isError || !userId}
            refreshing={isFetching}
            canRetry={online && !!userId}
            onRetry={() => { void refetch(); }}
          />
        )}
      </motion.div>
    </AnimatePresence>
  );
}

function CommandBlock({ cmd, shell, copyAria }: { cmd: string; shell: string; copyAria: string }) {
  const { t } = useT("runtimes");
  const [feedback, setFeedback] = useState<"idle" | "copied" | "failed">("idle");
  const copyAttempt = useRef(0);
  useEffect(() => () => { copyAttempt.current++; }, []);
  useEffect(() => {
    if (feedback !== "copied") return;
    const timeout = setTimeout(() => setFeedback("idle"), 1500);
    return () => clearTimeout(timeout);
  }, [feedback]);

  const handleCopy = async () => {
    const attempt = ++copyAttempt.current;
    const ok = await copyText(cmd).catch(() => false);
    if (attempt === copyAttempt.current) setFeedback(ok ? "copied" : "failed");
  };
  return (
    <div className="rounded-lg bg-muted px-3 py-3">
      <div className="mb-2 flex min-w-0 items-center justify-between gap-3">
        <span className="flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
          <Terminal className="size-3.5 shrink-0" aria-hidden />
          {shell}
        </span>
        <Button
          type="button" variant="outline" size="sm" onClick={() => { void handleCopy(); }} aria-label={copyAria}
          className="shrink-0 text-caption pointer-coarse:min-h-11"
        >
          {feedback === "copied" ? <Check className="size-3.5 text-success" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
          {feedback === "copied" ? t(($) => $.connect.copied) : t(($) => $.connect.copy)}
        </Button>
      </div>
      <pre className="min-w-0 overflow-x-auto font-mono text-body leading-relaxed">
        <code className={cn("select-text whitespace-pre-wrap [overflow-wrap:anywhere]", CODE_LIGATURE_CLASS)}>{cmd}</code>
      </pre>
      <p role="status" aria-live="polite" className={feedback === "failed" ? "mt-2 text-caption text-destructive" : "sr-only"}>
        {feedback === "failed" ? t(($) => $.connect.copy_failed) : feedback === "copied" ? t(($) => $.connect.copied) : ""}
      </p>
    </div>
  );
}

function InstructionsStep({ onClose, slug, baselineReady, interrupted, refreshing, canRetry, onRetry }: {
  onClose: () => void;
  slug: string | null;
  baselineReady: boolean;
  interrupted: boolean;
  refreshing: boolean;
  canRetry: boolean;
  onRetry: () => void;
}) {
  const { t } = useT("runtimes");
  const [platform, setPlatform] = useState<ComputerPlatform | null>(null);
  const [delayed, setDelayed] = useState(false);
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const platformLabelId = useId();
  const daemonServerUrl = useConfigStore((s) => s.daemonServerUrl);
  const daemonAppUrl = useConfigStore((s) => s.daemonAppUrl);
  const cliInstallCommand = useConfigStore((s) => s.cliInstallCommand);
  const cliInstallCommands = useConfigStore((s) => s.cliInstallCommands);
  const commands = platform ? connectionCommands(platform, daemonServerUrl, daemonAppUrl) : null;
  const installer = platform ? installCommand(platform, cliInstallCommands, cliInstallCommand) : null;
  const waiting = !!commands;
  useEffect(() => {
    if (!waiting) return;
    const timeout = setTimeout(() => setDelayed(true), WAIT_DELAY_MS);
    return () => clearTimeout(timeout);
  }, [waiting]);

  const shell = platform === "windows" ? "PowerShell" : t(($) => $.connect.shell_terminal);
  const environmentHint = platform === "windows" ? t(($) => $.connect.terminal_windows) :
    platform === "macos" ? t(($) => $.connect.terminal_macos) : t(($) => $.connect.terminal_linux);
  const showTroubleshooting = () => {
    const details = detailsRef.current;
    if (!details) return;
    details.open = true;
    details.querySelector("summary")?.focus();
  };

  return (
    <>
      <DialogHeader className="shrink-0 px-6 pt-6 pb-2 pr-12">
        <DialogTitle className="text-title-sm text-balance">{t(($) => $.connect.title)}</DialogTitle>
        <DialogDescription className="text-caption text-balance">{t(($) => $.connect.description)}</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
        <p id={platformLabelId} className="mb-2 text-caption font-medium">{t(($) => $.connect.target_system)}</p>
        <Tabs value={platform} onValueChange={(value) => {
          if (value === "windows" || value === "macos" || value === "linux") setPlatform(value);
        }} className="gap-4">
          <TabsList aria-labelledby={platformLabelId} className="h-auto min-h-11 w-full">
            {(["windows", "macos", "linux"] as const).map((value) => (
              <TabsTrigger key={value} value={value} className="min-h-9 data-active:font-semibold pointer-coarse:min-h-11">
                {PLATFORM_LABELS[value]}
              </TabsTrigger>
            ))}
          </TabsList>
          {platform ? (
            <TabsContent value={platform} className="min-w-0 space-y-5">
              <div className="space-y-1 text-caption text-muted-foreground">
                <p>{environmentHint}</p>
                <p>{platform === "windows" ? t(($) => $.connect.wsl_hint) : t(($) => $.connect.account_hint)}</p>
              </div>
              <section className="space-y-2">
                <h3 className="text-body font-medium">1. {t(($) => $.connect.step1_label)}</h3>
                {installer ? (
                  <CommandBlock key={platform + ":" + installer} cmd={installer} shell={shell}
                    copyAria={t(($) => $.connect.copy_install_aria, { platform: PLATFORM_LABELS[platform] })} />
                ) : (
                  <div className="space-y-1 rounded-lg bg-muted p-3 text-caption">
                    <p className="font-medium">{t(($) => $.connect.installation_missing_title)}</p>
                    <p className="text-muted-foreground">{t(($) => $.connect.installation_missing_hint)}</p>
                    {slug && <GuideLink href={paths.workspace(slug).docsPage(DOCS_SLUGS.cli)} onClose={onClose}>{t(($) => $.connect.installation_guide)}</GuideLink>}
                  </div>
                )}
              </section>
              <section className="space-y-2">
                <h3 className="text-body font-medium">2. {t(($) => $.connect.step2_label)}</h3>
                {commands ? (
                  <>
                    <CommandBlock key={platform + ":" + commands.setup} cmd={commands.setup} shell={shell} copyAria={t(($) => $.connect.copy_setup_aria)} />
                    <p className="text-caption leading-relaxed text-muted-foreground">{t(($) => $.connect.step2_hint)}</p>
                  </>
                ) : (
                  <AddressWarning serverUrl={daemonServerUrl} appUrl={daemonAppUrl} slug={slug} onClose={onClose} />
                )}
              </section>
              {commands ? (
                <ConnectionStatus
                  baselineReady={baselineReady} interrupted={interrupted} delayed={delayed} refreshing={refreshing}
                  canRetry={canRetry} onRetry={onRetry} onTroubleshoot={showTroubleshooting}
                />
              ) : <p role="status" className="text-caption text-muted-foreground">{t(($) => $.connect.address_waiting)}</p>}
              <TroubleshootingDetails ref={detailsRef} platform={platform} shell={shell} commands={commands} slug={slug} onClose={onClose} />
            </TabsContent>
          ) : (
            <p className="py-6 text-caption text-muted-foreground">{t(($) => $.connect.select_system_hint)}</p>
          )}
        </Tabs>
      </div>
      <DialogFooter className="m-0 shrink-0 items-start rounded-b-xl px-6 py-3 sm:items-center">
        <p className="flex-1 text-caption text-muted-foreground">{t(($) => $.connect.close_hint)}</p>
        <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" onClick={onClose}>{t(($) => $.connect.close)}</Button>
      </DialogFooter>
    </>
  );
}

function GuideLink({ href, onClose, children }: { href: string; onClose: () => void; children: React.ReactNode }) {
  return <AppLink href={href} onClick={onClose} className="inline-flex min-h-8 items-center text-caption text-foreground underline underline-offset-4 pointer-coarse:min-h-11">{children}</AppLink>;
}

function AddressWarning({ serverUrl, appUrl, slug, onClose }: { serverUrl: string; appUrl: string; slug: string | null; onClose: () => void }) {
  const { t } = useT("runtimes");
  return (
    <div className="rounded-lg bg-warning/10 p-3 text-caption">
      <p className="mb-1 flex items-start gap-2 font-medium text-warning">
        <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
        {t(($) => $.connect.address_title)}
      </p>
      <p className="leading-relaxed text-foreground">{t(($) => $.connect.address_hint)}</p>
      <dl className="mt-2 space-y-1">
        {[[t(($) => $.connect.api_address), serverUrl], [t(($) => $.connect.web_address), appUrl]].map(([label, value]) => (
          <div key={label} className="flex min-w-0 flex-wrap gap-x-2">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="min-w-0 select-text break-all font-mono">{value?.trim() || t(($) => $.connect.address_unset)}</dd>
          </div>
        ))}
      </dl>
      {slug && <GuideLink href={paths.workspace(slug).docsPage(DOCS_SLUGS.selfHostQuickstart)} onClose={onClose}>{t(($) => $.connect.address_guide)}</GuideLink>}
    </div>
  );
}

function TroubleshootingDetails({ ref, platform, shell, commands, slug, onClose }: {
  ref: React.Ref<HTMLDetailsElement>;
  platform: ComputerPlatform;
  shell: string;
  commands: ReturnType<typeof connectionCommands>;
  slug: string | null;
  onClose: () => void;
}) {
  const { t } = useT("runtimes");
  return (
    <details ref={ref} className="group">
      <summary className="flex min-h-9 cursor-pointer list-none items-center gap-1.5 rounded text-caption font-medium text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
        <ChevronRight className="size-3.5 shrink-0 transition-transform group-open:rotate-90 motion-reduce:transition-none" aria-hidden />
        {t(($) => $.connect.troubleshooting)}
      </summary>
      <div className="space-y-4 pt-3 text-caption leading-relaxed text-muted-foreground">
        {commands ? (
          <>
            <p>
              {t(($) => $.connect.trouble_intro)}{" "}
              {slug ? <GuideLink href={paths.workspace(slug).settings() + "?tab=tokens"} onClose={onClose}>{t(($) => $.connect.token_destination)}</GuideLink> : t(($) => $.connect.token_destination)}
            </p>
            <div className="space-y-2">
              <p className="font-medium text-foreground">{t(($) => $.connect.headless_config_label)}</p>
              <CommandBlock key={platform + ":" + commands.configure} cmd={commands.configure} shell={shell} copyAria={t(($) => $.connect.copy_config_aria)} />
            </div>
            <div className="space-y-2">
              <p className="font-medium text-foreground">{t(($) => $.connect.headless_login_label)}</p>
              <p>{t(($) => $.connect.headless_login_hint)}</p>
              <CommandBlock key={platform + ":login"} cmd={commands.login} shell={shell} copyAria={t(($) => $.connect.copy_login_aria)} />
            </div>
            <div className="space-y-2">
              <p className="font-medium text-foreground">{t(($) => $.connect.headless_start_label)}</p>
              <p>{t(($) => $.connect.headless_start_hint)}</p>
              <CommandBlock key={platform + ":start"} cmd={commands.start} shell={shell} copyAria={t(($) => $.connect.copy_start_aria)} />
            </div>
          </>
        ) : <p>{t(($) => $.connect.headless_address_hint)}</p>}
        <div className="space-y-2">
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
            {t(($) => $.connect.trouble_check_status)}
            <code className={cn("select-text rounded bg-muted px-1.5 py-0.5 font-mono text-foreground", CODE_LIGATURE_CLASS)}>{"multica daemon status"}</code>
          </p>
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
            {t(($) => $.connect.trouble_view_logs)}
            <code className={cn("select-text rounded bg-muted px-1.5 py-0.5 font-mono text-foreground", CODE_LIGATURE_CLASS)}>{"multica daemon logs -f"}</code>
          </p>
        </div>
      </div>
    </details>
  );
}

function ConnectionStatus({ baselineReady, interrupted, delayed, refreshing, canRetry, onRetry, onTroubleshoot }: {
  baselineReady: boolean;
  interrupted: boolean;
  delayed: boolean;
  refreshing: boolean;
  canRetry: boolean;
  onRetry: () => void;
  onTroubleshoot: () => void;
}) {
  const { t } = useT("runtimes");
  return (
    <div className="flex items-start gap-2.5 text-caption">
      <Clock3 className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
      <div className="min-w-0 space-y-1">
        <div role="status" aria-live="polite">
          <p className="font-medium text-foreground">{interrupted ? t(($) => $.connect.interrupted) : delayed ? t(($) => $.connect.delayed) : t(($) => $.connect.waiting)}</p>
          <p className="mt-1 leading-relaxed text-muted-foreground">
            {interrupted ? t(($) => $.connect.interrupted_hint) : !baselineReady ? t(($) => $.connect.checking_hint) : delayed ? t(($) => $.connect.delayed_hint) : t(($) => $.connect.waiting_hint)}
          </p>
        </div>
        {interrupted ? (
          <Button variant="outline" size="sm" disabled={!canRetry || refreshing} onClick={onRetry}
            aria-label={t(($) => $.connect.retry)} className="mt-1 pointer-coarse:min-h-11">
            {refreshing ? t(($) => $.connect.retrying) : t(($) => $.connect.retry)}
          </Button>
        ) : delayed && (
          <Button variant="ghost" size="sm" onClick={onTroubleshoot} className="mt-1 pointer-coarse:min-h-11">
            {t(($) => $.connect.troubleshoot_action)}
          </Button>
        )}
      </div>
    </div>
  );
}

function SuccessStep({ computer, onGoToAgents, onGoToComputer }: {
  computer: RuntimeMachine;
  onGoToAgents?: () => void;
  onGoToComputer?: () => void;
}) {
  const { t } = useT("runtimes");
  const primaryRef = useRef<HTMLButtonElement>(null);
  useEffect(() => { primaryRef.current?.focus({ preventScroll: true }); }, []);
  const system = computer.deviceInfo?.split(" · ").slice(1).join(" · ") || t(($) => $.connect.system_unknown);
  return (
    <>
      <DialogHeader className="shrink-0 px-6 pt-6 pb-2 pr-12">
        <DialogTitle className="text-title-sm text-balance">{t(($) => $.connect.success_title)}</DialogTitle>
        <DialogDescription className="text-caption text-balance">{t(($) => $.connect.success_description)}</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-6">
        <div className="flex items-center gap-3" role="status">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-success/10" aria-hidden>
            <Check className="size-5 text-success" />
          </div>
          <div className="min-w-0">
            <p className="break-words text-title-sm font-medium [overflow-wrap:anywhere]">{computer.title}</p>
            <p className="mt-1 break-words text-caption text-muted-foreground">{system} · {t(($) => $.connect.online)}</p>
          </div>
        </div>
        <section className="space-y-2">
          <h3 className="text-caption text-muted-foreground">{t(($) => $.connect.available_runtimes)}</h3>
          <ul className="space-y-2 text-body">
            {computer.runtimes.map((runtime) => <li key={runtime.id} className="break-words [overflow-wrap:anywhere]">{runtimeRowLabel(runtime, computer.title)}</li>)}
          </ul>
        </section>
      </div>
      <DialogFooter className="m-0 shrink-0 rounded-b-xl px-6 py-3">
        {onGoToComputer && <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" onClick={onGoToComputer}>{t(($) => $.connect.view_computer)}</Button>}
        <Button ref={primaryRef} size="sm" className="pointer-coarse:min-h-11" disabled={!onGoToAgents} onClick={onGoToAgents}>
          {t(($) => $.connect.create_agent)}
          <ChevronRight className="size-3.5" aria-hidden />
        </Button>
      </DialogFooter>
    </>
  );
}
