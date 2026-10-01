"use client";

import { useId, useState } from "react";
import { ArrowRight, CircleCheck, Clock3, Download, Loader2, ShieldX } from "lucide-react";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { cn } from "@multica/ui/lib/utils";
import { DragStrip } from "@multica/views/platform";
import { ProviderLogo } from "../../runtimes/components/provider-logo";
import { useT } from "../../i18n";

/**
 * Step 0 — the one-shot product intro shown on every onboarding
 * entry, including new-workspace creation (the current step is not persisted).
 * New-workspace creation supplies onCancel instead of the existing-workspace
 * skip action so users can leave without completing onboarding.
 *
 * Layout: two-column editorial hero on lg+, single column below.
 * Left = wordmark + serif headline + lede + CTA; right = a stack of
 * mock issue cards that show what human/agent collaboration looks
 * like on the board — the thing the user is about to create. The
 * right column is an illustrative history, hidden below lg. Its
 * human-approval summary remains visible on narrow viewports.
 *
 * `onSkip`, when provided, renders a secondary ghost CTA that marks
 * onboarding complete server-side and sends the user straight to
 * their existing workspace. OnboardingFlow only passes it when the
 * user has ≥ 1 workspace — without that, skipping lands in limbo.
 *
 * `isWeb` flips two things when true: the subheading acknowledges
 * that web users have an extra runtime step (so "3 minutes" stops
 * being a lie), and a "Download Desktop" secondary CTA surfaces
 * before the user has invested in questionnaire / workspace. Desktop
 * bundles a daemon, so the same prompt would be noise there.
 */
export function StepWelcome({
  onNext,
  onSkip,
  onCancel,
  isWeb = false,
}: {
  onNext: () => void | Promise<void>;
  onSkip?: () => void | Promise<void>;
  onCancel?: () => void;
  isWeb?: boolean;
}) {
  const { t } = useT("onboarding");
  const illustrationTitleId = useId();
  // Tracks which button is mid-flight so we can show a per-button
  // spinner and disable both while one is in progress.
  const [pending, setPending] = useState<"next" | "skip" | null>(null);

  const handleNext = async () => {
    if (pending) return;
    setPending("next");
    try {
      await onNext();
    } finally {
      setPending(null);
    }
  };

  const handleSkip = async () => {
    if (pending || !onSkip) return;
    setPending("skip");
    try {
      await onSkip();
    } finally {
      setPending(null);
    }
  };

  return (
    <div className="animate-onboarding-enter flex min-h-svh shrink-0 flex-col lg:flex-row">
      {/* Intrinsic height keeps short windows scrollable; the example cannot push the CTA down. */}
      <div className="flex min-h-svh min-w-0 flex-col lg:sticky lg:top-0 lg:flex-1 lg:self-start">
        <DragStrip />
        <div className="flex flex-1 flex-col justify-center px-6 pb-12 pt-6 sm:px-10 xl:px-16 2xl:px-20">
          <div className="@container mx-auto flex w-full max-w-[560px] flex-col gap-7">
            <div className="flex items-center gap-2.5">
              <MulticaIcon className="size-5 text-foreground" noSpin />
              <span className="font-serif text-title-lg font-medium tracking-tight">
                {t(($) => $.welcome.wordmark)}
              </span>
            </div>

            <h1 className="text-balance font-serif text-[clamp(1.75rem,10cqi,3.75rem)] font-medium leading-[1.08] tracking-tight">
              {t(($) => $.welcome.headline_line1)}
              <br />
              {t(($) => $.welcome.headline_line2)}{" "}
              <em className="inline-block max-w-full align-bottom italic text-brand">
                {t(($) => $.welcome.headline_emphasis)}
              </em>
            </h1>

            <div className="flex flex-col gap-4">
              <p className="text-title leading-relaxed text-foreground">
                {t(($) => $.welcome.lede)}
              </p>
              <p className="text-body leading-relaxed text-muted-foreground">
                {isWeb
                  ? t(($) => $.welcome.lede_web)
                  : t(($) => $.welcome.lede_desktop)}
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              {isWeb ? (
                <>
                  {/* `<a>` rather than `<Button onClick={window.open}>`
                      so middle-click / cmd-click / "Copy link" all
                      behave and screen readers announce it as a link
                      (it navigates; `Continue on web` is the button
                      that mutates flow state). New tab preserves this
                      onboarding tab in case the desktop install
                      stalls and the user falls back here. */}
                  <a
                    href="/download"
                    target="_blank"
                    rel="noopener noreferrer"
                    className={buttonVariants({ size: "lg", className: "h-11 px-4" })}
                  >
                    <Download className="h-4 w-4" />
                    {t(($) => $.welcome.download_desktop)}
                  </a>
                  <Button
                    size="lg"
                    className="h-11 px-4"
                    variant="outline"
                    onClick={handleNext}
                    disabled={pending !== null}
                  >
                    {pending === "next" && (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    )}
                    {t(($) => $.welcome.continue_on_web)}
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </>
              ) : (
                <Button
                  size="lg"
                  className="h-11 px-4"
                  onClick={handleNext}
                  disabled={pending !== null}
                >
                  {pending === "next" && (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  )}
                  {t(($) => $.welcome.start_exploring)}
                  <ArrowRight className="h-4 w-4" />
                </Button>
              )}
              {onCancel && (
                <Button
                  size="lg"
                  className="h-11 px-4"
                  variant="ghost"
                  onClick={onCancel}
                  disabled={pending !== null}
                >
                  {t(($) => $.common.cancel)}
                </Button>
              )}
              {onSkip && (
                <Button
                  size="lg"
                  className="h-11 px-4"
                  variant="ghost"
                  onClick={handleSkip}
                  disabled={pending !== null}
                >
                  {pending === "skip" && (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  )}
                  {t(($) => $.welcome.skip_existing)}
                </Button>
              )}
            </div>
            <p className="text-body-lg leading-relaxed text-muted-foreground lg:hidden">
              {t(($) => $.welcome.illustration_caption)}
            </p>
          </div>
        </div>
      </div>

      {/* The example may grow beyond the viewport without moving the left CTA. */}
      <section
        aria-labelledby={illustrationTitleId}
        className="hidden min-w-0 border-l bg-muted/40 lg:flex lg:flex-1 lg:flex-col lg:overflow-hidden"
      >
        <DragStrip />
        <div className="flex flex-1 flex-col items-center justify-center gap-7 px-8 py-8">
          <div className="max-w-[460px] space-y-2 text-center">
            <h2 id={illustrationTitleId} className="text-balance font-serif text-title italic leading-snug">
              {t(($) => $.welcome.illustration_title)}
            </h2>
            <p className="text-pretty text-body leading-relaxed text-muted-foreground">
              {t(($) => $.welcome.illustration_context)}
            </p>
          </div>
          <WelcomeIllustration />
        </div>
      </section>
    </div>
  );
}


/** Six snapshots of a shared task, including review gates and human decisions. */
function WelcomeIllustration() {
  const { t } = useT("onboarding");
  return (
    <ol role="list" className="flex w-full max-w-[460px] flex-col gap-5">
      <MockActivityCard
        actor={{
          kind: "user",
          name: t(($) => $.welcome.illustration.card1_actor_name),
          initial: t(($) => $.welcome.illustration.card1_actor_initial),
        }}
        issueId="MCA-42"
        content={
          <>
            <Mention>{t(($) => $.welcome.illustration.card1_mention_assignee)}</Mention>
            {t(($) => $.welcome.illustration.card1_body)}
          </>
        }
      />
      <MockActivityCard
        className="-translate-x-6 -rotate-[0.5deg]"
        actor={{
          kind: "agent",
          name: t(($) => $.welcome.illustration.card2_actor_name),
          provider: "kimi",
        }}
        issueId="MCA-42"
        content={
          <>
            {t(($) => $.welcome.illustration.card2_body_prefix)}
            <Mention>{t(($) => $.welcome.illustration.card2_mention)}</Mention>
            {t(($) => $.welcome.illustration.card2_body_suffix)}
          </>
        }
        status="waiting"
        statusLabel={t(($) => $.welcome.illustration.card2_status)}
      />
      <MockActivityCard
        className="translate-x-6 rotate-[0.5deg]"
        actor={{
          kind: "agent",
          name: t(($) => $.welcome.illustration.card3_actor_name),
          provider: "hermes",
        }}
        issueId="MCA-42"
        content={t(($) => $.welcome.illustration.card3_body)}
        status="done"
        statusLabel={t(($) => $.welcome.illustration.card3_status)}
      />
      <MockActivityCard
        className="-translate-x-6 -rotate-[0.5deg]"
        actor={{
          kind: "agent",
          name: t(($) => $.welcome.illustration.card4_actor_name),
          provider: "codex",
        }}
        issueId="MCA-42"
        content={t(($) => $.welcome.illustration.card4_body)}
        status="waiting"
        statusLabel={t(($) => $.welcome.illustration.card4_status)}
      />
      <MockActivityCard
        className="translate-x-6 rotate-[0.5deg]"
        actor={{
          kind: "agent",
          name: t(($) => $.welcome.illustration.card5_actor_name),
          provider: "openclaw",
        }}
        issueId="MCA-42"
        content={
          <>
            {t(($) => $.welcome.illustration.card5_body_prefix)}
            <Mention>{t(($) => $.welcome.illustration.card5_mention)}</Mention>
            {t(($) => $.welcome.illustration.card5_body_suffix)}
          </>
        }
        status="blocked"
        statusLabel={t(($) => $.welcome.illustration.card5_status)}
      />
      <MockActivityCard
        className="-translate-x-6 -rotate-[0.5deg]"
        actor={{
          kind: "agent",
          name: t(($) => $.welcome.illustration.card6_actor_name),
          provider: "claude",
        }}
        issueId="MCA-42"
        content={
          <>
            {t(($) => $.welcome.illustration.card6_body_prefix)}
            <Mention>{t(($) => $.welcome.illustration.card6_mention)}</Mention>
            {t(($) => $.welcome.illustration.card6_body_suffix)}
          </>
        }
        status="waiting"
        statusLabel={t(($) => $.welcome.illustration.card6_status)}
      />
    </ol>
  );
}

type ProviderName =
  | "claude"
  | "codex"
  | "opencode"
  | "openclaw"
  | "hermes"
  | "kimi"
  | "kiro"
  | "qoder"
  | "pi"
  | "copilot"
  | "cursor";

type ActivityActor =
  | { kind: "user"; name: string; initial: string }
  | { kind: "agent"; name: string; provider: ProviderName };

function MockActivityCard({
  actor,
  issueId,
  content,
  status,
  statusLabel,
  className,
}: {
  actor: ActivityActor;
  issueId: string;
  content: React.ReactNode;
  status?: "waiting" | "done" | "blocked";
  statusLabel?: string;
  className?: string;
}) {
  return (
    <li
      className={cn(
        "rounded-lg border bg-card px-4 py-3.5 shadow-sm",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <MockAvatar actor={actor} />
          <span className="truncate text-body font-medium text-foreground">
            {actor.name}
          </span>
        </div>
        <span className="shrink-0 font-mono text-micro text-muted-foreground">
          {issueId}
        </span>
      </div>

      <p className="mt-2.5 text-body leading-snug text-foreground">
        {content}
      </p>

      {status && statusLabel && <StatusFooter status={status} label={statusLabel} />}
    </li>
  );
}

function MockAvatar({ actor }: { actor: ActivityActor }) {
  if (actor.kind === "user") {
    return (
      <div
        aria-hidden
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-foreground text-micro font-semibold text-background"
      >
        {actor.initial}
      </div>
    );
  }
  return (
    <div
      aria-hidden
      className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border bg-muted/40 text-foreground"
    >
      <ProviderLogo provider={actor.provider} className="h-3.5 w-3.5" />
    </div>
  );
}

// Illustration stages describe approvals, rather than live issue statuses.
function StatusFooter({
  status,
  label,
}: {
  status: "waiting" | "done" | "blocked";
  label: string;
}) {
  const Icon = status === "blocked" ? ShieldX : status === "done" ? CircleCheck : Clock3;
  return (
    <div
      className={cn(
        "mt-3 flex items-center gap-1.5 text-caption font-medium",
        status === "blocked" ? "text-destructive" : status === "done" ? "text-brand" : "text-muted-foreground",
      )}
    >
      <Icon aria-hidden className="h-3.5 w-3.5 shrink-0" />
      <span>{label}</span>
    </div>
  );
}

function Mention({ children }: { children: React.ReactNode }) {
  return <span className="font-medium text-brand">{children}</span>;
}
