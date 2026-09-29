"use client";

import {
  ArrowRight,
  BookOpen,
  Code2,
  GitPullRequest,
  ListChecks,
  Network,
  RotateCcw,
  ScanEye,
  Target,
} from "lucide-react";
import { useEffect, useRef } from "react";
import { useT } from "../../i18n";

export function DevelopmentLifecycle() {
  const { t } = useT("onboarding");
  const graphRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    const graph = graphRef.current;
    if (!graph) return;
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    graph.dataset.motion = preference.matches ? "complete" : "entering";
    const finishForReducedMotion = () => {
      if (preference.matches) graph.dataset.motion = "complete";
    };
    const finishEntrance = (event: AnimationEvent) => {
      if (event.animationName === "adlc-node-morph") {
        graph.dataset.motion = "complete";
      }
    };
    preference.addEventListener("change", finishForReducedMotion);
    graph.addEventListener("animationend", finishEntrance);
    return () => {
      preference.removeEventListener("change", finishForReducedMotion);
      graph.removeEventListener("animationend", finishEntrance);
    };
  }, []);

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
    <figure aria-label={t(($) => $.lifecycle.label)} className="adlc-lifecycle @container space-y-3">
      <div className="px-1">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <h2 className="text-body font-semibold text-muted-foreground">
            {t(($) => $.lifecycle.sdlc)}
          </h2>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.lifecycle.sdlc_description)}
          </p>
        </div>
        <ol className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
          {traditionalStages.map((stage, index) => (
            <li key={stage} className="flex items-center gap-3 text-caption text-muted-foreground">
              {stage}
              {index < traditionalStages.length - 1 && (
                <ArrowRight
                  aria-hidden
                  className="size-3 text-faint-foreground"
                />
              )}
            </li>
          ))}
        </ol>
      </div>

      <div className="adlc-surface rounded-xl border border-brand/20 bg-brand/5 px-4 pt-4 pb-3">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <h2 className="text-body font-semibold text-brand">
            {t(($) => $.lifecycle.adlc)}
          </h2>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.lifecycle.adlc_description)}
          </p>
        </div>
        <ul
          ref={graphRef}
          aria-label={t(($) => $.lifecycle.adlc)}
          className="adlc-graph"
        >
          <li aria-hidden role="presentation" className="adlc-connections">
            <svg viewBox="0 0 1000 260" preserveAspectRatio="none" className="adlc-wires">
              <path className="adlc-linear-wire" d="M83 137H917" />
              <g className="adlc-branch-wires">
                <path className="adlc-context-wire" d="M500 28V88" />
                <path d="M130 137H276Q290 137 290 123V102Q290 88 304 88H500" />
                <path d="M130 137H500" />
                <path d="M130 137H276Q290 137 290 151V172Q290 186 304 186H500" />
                <path d="M500 88H696Q710 88 710 102V123Q710 137 724 137H870" />
                <path d="M500 137H870" />
                <path d="M500 186H696Q710 186 710 172V151Q710 137 724 137H870" />
                <path className="adlc-feedback-wire" d="M870 137V218Q870 232 856 232H144Q130 232 130 218V137" />
                <path className="adlc-feedback-arrow" d="m122 205 8-5 8 5" />
              </g>
              <path className="adlc-signal" pathLength="1" d="M130 137H276Q290 137 290 123V102Q290 88 304 88H696Q710 88 710 102V123Q710 137 724 137H870V218Q870 232 856 232H144Q130 232 130 218V137" />
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
    </figure>
  );
}
