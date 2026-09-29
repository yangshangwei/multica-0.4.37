"use client";

import {
  ArrowRight,
  BookOpen,
  Code2,
  GitPullRequest,
  ListChecks,
  Network,
  RotateCcw,
  Target,
} from "lucide-react";
import { useT } from "../../i18n";

export function DevelopmentLifecycle() {
  const { t } = useT("onboarding");
  const traditionalStages = [
    t(($) => $.lifecycle.requirements),
    t(($) => $.lifecycle.design),
    t(($) => $.lifecycle.implementation),
    t(($) => $.lifecycle.testing),
    t(($) => $.lifecycle.deployment),
    t(($) => $.lifecycle.maintenance),
  ];
  const agentStages = [
    { icon: Target, label: t(($) => $.lifecycle.goal) },
    { icon: BookOpen, label: t(($) => $.lifecycle.context) },
    { icon: Network, label: t(($) => $.lifecycle.orchestration) },
    { icon: Code2, label: t(($) => $.lifecycle.coding) },
    { icon: ListChecks, label: t(($) => $.lifecycle.validation) },
    { icon: GitPullRequest, label: t(($) => $.lifecycle.review) },
  ];

  return (
    <figure aria-label={t(($) => $.lifecycle.label)} className="@container space-y-4">
      <div className="px-4">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <h2 className="text-body font-semibold text-muted-foreground">
            {t(($) => $.lifecycle.sdlc)}
          </h2>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.lifecycle.sdlc_description)}
          </p>
        </div>
        <ol className="mt-3 grid grid-cols-3 gap-x-3 gap-y-2 @xl:grid-cols-6">
          {traditionalStages.map((stage, index) => (
            <li key={stage} className="relative text-center text-caption text-muted-foreground">
              {stage}
              {index < traditionalStages.length - 1 && (
                <ArrowRight
                  aria-hidden
                  className="absolute -right-3 top-0.5 hidden size-3 text-faint-foreground @xl:block"
                />
              )}
            </li>
          ))}
        </ol>
      </div>

      <div className="rounded-xl border border-brand/20 bg-brand/5 px-4 py-4">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <h2 className="text-body font-semibold text-brand">
            {t(($) => $.lifecycle.adlc)}
          </h2>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.lifecycle.adlc_description)}
          </p>
        </div>
        <ol className="mt-4 grid grid-cols-3 gap-x-3 gap-y-4 @xl:grid-cols-6">
          {agentStages.map(({ icon: Icon, label }, index) => (
            <li key={label} className="relative flex min-w-0 flex-col items-center gap-2 text-center">
              <Icon aria-hidden className="size-4 text-brand" />
              <span className="text-balance text-caption font-medium text-foreground">
                {label}
              </span>
              {index < agentStages.length - 1 && (
                <ArrowRight
                  aria-hidden
                  className="absolute -right-3 top-0.5 hidden size-3 text-brand @xl:block"
                />
              )}
            </li>
          ))}
        </ol>
        <p className="mt-4 flex items-center justify-center gap-2 border-t border-dashed border-brand/20 pt-3 text-caption text-muted-foreground">
          <RotateCcw aria-hidden className="size-3.5 shrink-0 text-brand" />
          {t(($) => $.lifecycle.feedback)}
        </p>
      </div>
    </figure>
  );
}
