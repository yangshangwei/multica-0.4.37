"use client";

import {
  BookOpen,
  Code2,
  GitPullRequest,
  ListChecks,
  Network,
  RotateCcw,
  ScanEye,
  Target,
} from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

const collaborationPaths = [
  "M130 137H276Q290 137 290 123V102Q290 88 304 88H696Q710 88 710 102V123Q710 137 724 137H870",
  "M130 137H870",
  "M130 137H276Q290 137 290 151V172Q290 186 304 186H696Q710 186 710 172V151Q710 137 724 137H870",
];
const feedbackPath = "M870 137V218Q870 232 856 232H144Q130 232 130 218V137";

export function DevelopmentLifecycle() {
  const { t } = useT("onboarding");
  const [playing, setPlaying] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [replay, setReplay] = useState(0);

  useEffect(() => {
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReducedMotion(preference.matches);
    setPlaying(!preference.matches);
    const syncPreference = () => {
      setReducedMotion(preference.matches);
      if (preference.matches) setPlaying(false);
    };
    preference.addEventListener("change", syncPreference);
    return () => preference.removeEventListener("change", syncPreference);
  }, []);

  function replayTransition() {
    if (playing || reducedMotion) return;
    setReplay((value) => value + 1);
    setPlaying(true);
  }

  const traditionalStages = [
    t(($) => $.lifecycle.requirements),
    t(($) => $.lifecycle.design),
    t(($) => $.lifecycle.implementation),
    t(($) => $.lifecycle.testing),
    t(($) => $.lifecycle.deployment),
    t(($) => $.lifecycle.maintenance),
  ];
  const agentStages = [
    {
      id: "goal",
      icon: Target,
      label: t(($) => $.lifecycle.goal),
      detail: t(($) => $.lifecycle.direction),
    },
    {
      id: "context",
      icon: BookOpen,
      label: t(($) => $.lifecycle.context),
      detail: t(($) => $.lifecycle.shared),
    },
    { id: "coding", icon: Code2, label: t(($) => $.lifecycle.coding) },
    { id: "testing", icon: ListChecks, label: t(($) => $.lifecycle.validation) },
    { id: "review", icon: ScanEye, label: t(($) => $.lifecycle.agent_review) },
    {
      id: "delivery",
      icon: GitPullRequest,
      label: t(($) => $.lifecycle.review),
      detail: t(($) => $.lifecycle.decision),
    },
  ];

  return (
    <figure
      aria-label={t(($) => $.lifecycle.label)}
      data-motion={playing ? "entering" : "complete"}
      className="adlc-lifecycle @container"
    >
      <div className="relative">
        {!reducedMotion && (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            className="adlc-replay absolute top-3 right-3 z-10 text-muted-foreground"
            disabled={playing}
            focusableWhenDisabled
            onClick={replayTransition}
          >
            <RotateCcw aria-hidden />
            {t(($) => $.lifecycle.replay)}
          </Button>
        )}
        <div key={replay} className="adlc-surface rounded-xl border border-brand/20 bg-brand/5 px-4 pt-4 pb-3">
          <div className="adlc-heading flex flex-wrap items-baseline gap-x-2 gap-y-1 pr-28">
            <h2 className="text-body font-semibold text-brand">
              <span className="sr-only">{t(($) => $.lifecycle.adlc)}</span>
              <span aria-hidden className="adlc-wordmark">
                <span className="adlc-letter">
                  <span className="adlc-letter-before">{t(($) => $.lifecycle.sdlc).slice(0, 1)}</span>
                  <span className="adlc-letter-after">{t(($) => $.lifecycle.adlc).slice(0, 1)}</span>
                </span>
                <span>{t(($) => $.lifecycle.adlc).slice(1)}</span>
              </span>
            </h2>
            <p className="adlc-description text-caption text-muted-foreground">
              {t(($) => $.lifecycle.adlc_description)}
            </p>
          </div>
          <ul
            aria-label={t(($) => $.lifecycle.adlc)}
            className="adlc-graph"
          >
            <li aria-hidden role="presentation" className="adlc-connections">
              <svg viewBox="0 0 1000 260" preserveAspectRatio="none" className="adlc-wires">
                <path className="adlc-linear-wire" d="M83 137H917" />
                <path className="adlc-linear-signal" pathLength="1" d="M83 137H917" />
                <g className="adlc-branch-wires">
                  <path className="adlc-context-wire" d="M500 28V88" />
                  {collaborationPaths.map((path) => <path key={path} pathLength="1" d={path} />)}
                </g>
                <path className="adlc-feedback-wire" pathLength="1" d={feedbackPath} />
                <path className="adlc-feedback-arrow" d="m122 205 8-5 8 5" />
                {collaborationPaths.map((path) => <path key={path} className="adlc-signal" pathLength="1" d={path} />)}
                <path
                  className="adlc-feedback-signal"
                  pathLength="1"
                  d={feedbackPath}
                  onAnimationEnd={(event) => {
                    if (event.animationName === "adlc-feedback-travel") setPlaying(false);
                  }}
                />
              </svg>
            </li>
            {agentStages.map(({ id, icon: Icon, label, detail }, index) => (
              <li key={id} className={`adlc-node adlc-node-${id} text-caption`}>
                <span aria-hidden className="adlc-before text-muted-foreground">
                  {traditionalStages[index]}
                </span>
                <span className="adlc-node-content">
                  <Icon aria-hidden className="size-4 shrink-0 text-brand" />
                  <span className="adlc-node-label font-medium text-foreground">{label}</span>
                  {detail && <span className="adlc-node-detail text-muted-foreground">{detail}</span>}
                </span>
              </li>
            ))}
          </ul>
          <div className="adlc-captions text-caption text-muted-foreground">
            <p className="adlc-feedback">
              <RotateCcw aria-hidden className="size-3 shrink-0 text-brand" />
              {t(($) => $.lifecycle.feedback)}
            </p>
            <p className="flex items-center justify-center gap-1.5">
              <Network aria-hidden className="size-3.5 text-brand" />
              {t(($) => $.lifecycle.orchestration)}
            </p>
          </div>
        </div>
      </div>
    </figure>
  );
}
