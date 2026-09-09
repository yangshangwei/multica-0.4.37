"use client";

import { useState } from "react";
import { Play } from "lucide-react";
import { useT } from "../i18n";

/**
 * VideoEmbed — the in-app rendering of the docs site's `<VideoEmbed>`, which the
 * generator rewrites to `::video-embed{provider="…" id="…" title="…"}`.
 *
 * Same click-to-load facade as apps/docs/components/video-embed.tsx: no
 * third-party iframe is mounted until the reader asks for it, so opening a docs
 * page never reaches out to a player or its trackers. Kept as a separate
 * implementation rather than a shared component because that one lives in a
 * Next.js app and carries the docs site's own hardcoded Chinese copy.
 *
 * The player itself is remote, so on an intranet deployment it will not load.
 * That is what the fallback link below is for — it names the site rather than
 * pretending the embed works.
 */

type Provider = "bilibili" | "youtube";

interface ProviderConfig {
  embedUrl: (id: string) => string;
  watchUrl: (id: string) => string;
  siteName: string;
  isValidId: (id: string) => boolean;
}

const PROVIDERS: Record<Provider, ProviderConfig> = {
  bilibili: {
    embedUrl: (id) =>
      `https://player.bilibili.com/player.html?bvid=${id}&autoplay=1&high_quality=1&danmaku=0`,
    watchUrl: (id) => `https://www.bilibili.com/video/${id}/`,
    siteName: "Bilibili",
    isValidId: (id) => /^BV[0-9A-Za-z]+$/.test(id),
  },
  youtube: {
    embedUrl: (id) =>
      `https://www.youtube-nocookie.com/embed/${id}?autoplay=1&rel=0`,
    watchUrl: (id) => `https://www.youtube.com/watch?v=${id}`,
    siteName: "YouTube",
    isValidId: (id) => /^[0-9A-Za-z_-]{11}$/.test(id),
  },
};

function providerConfig(provider: string): ProviderConfig | null {
  // Server-driven value, so an unknown provider must not index blindly into the
  // record — a newer bundle may name one this client does not implement.
  if (provider === "bilibili" || provider === "youtube") {
    return PROVIDERS[provider];
  }
  return null;
}

export function DocsVideoEmbed({
  provider = "bilibili",
  id,
  title,
}: {
  provider?: string;
  id?: string;
  title?: string;
}) {
  const { t } = useT("docs");
  const [active, setActive] = useState(false);
  const config = providerConfig(provider);

  if (!config || !id || !config.isValidId(id)) {
    return (
      <p className="my-4 rounded-lg border border-border bg-muted/30 p-3.5 text-body text-muted-foreground">
        {t(($) => $.video.unavailable)}
      </p>
    );
  }

  const label = title || t(($) => $.video.fallback_label);
  const watchUrl = config.watchUrl(id);

  return (
    <figure className="my-5">
      <div className="relative aspect-video w-full overflow-hidden rounded-lg border border-border bg-muted/40">
        {active ? (
          <iframe
            src={config.embedUrl(id)}
            title={label}
            loading="lazy"
            allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
            allowFullScreen
            className="absolute inset-0 size-full"
          />
        ) : (
          <button
            type="button"
            onClick={() => setActive(true)}
            aria-label={t(($) => $.video.play, { title: label })}
            className="group absolute inset-0 flex size-full cursor-pointer flex-col items-center justify-center gap-3 bg-gradient-to-b from-muted/20 to-muted/60 transition-colors hover:from-muted/30 hover:to-muted/70"
          >
            <span className="flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg transition-transform group-hover:scale-105">
              <Play aria-hidden="true" className="size-6 translate-x-0.5 fill-current" />
            </span>
            <span className="px-6 text-center text-body font-medium text-foreground">
              {label}
            </span>
          </button>
        )}
      </div>
      <figcaption className="mt-2 text-caption text-muted-foreground">
        {t(($) => $.video.slow_prompt)}{" "}
        <a
          href={watchUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-2 transition-colors hover:text-foreground"
        >
          {t(($) => $.video.watch_on, { site: config.siteName })}
        </a>
      </figcaption>
    </figure>
  );
}
