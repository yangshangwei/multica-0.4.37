"use client";

import { useEffect, useId, useState } from "react";
import { useTheme } from "next-themes";
import { useDocsLocale } from "./locale-link";

/**
 * Client-side Mermaid diagram renderer.
 *
 * Dynamic-imports the mermaid package so it's only loaded on pages that
 * actually use it (~400 KB). Re-renders when the page theme flips.
 *
 * Themed to pick up Multica design tokens at runtime via getComputedStyle,
 * so the diagram tracks both light / dark mode and any future token changes
 * without a rebuild.
 */
export function Mermaid({ chart }: { chart: string }) {
  const reactId = useId();
  const locale = useDocsLocale();
  const copy = locale === "zh"
    ? { loading: "正在绘制图示...", error: "图示暂时无法显示，可查看下方图源。", source: "查看图源", diagram: "图示" }
    : { loading: "Rendering diagram…", error: "This diagram could not be displayed. Its source is available below.", source: "View diagram source", diagram: "Diagram" };
  const { resolvedTheme } = useTheme();
  const [rendered, setRendered] = useState<{ svg: string; width: number } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function renderDiagram() {
      try {
        const { default: mermaid } = await import("mermaid");
        const css = getComputedStyle(document.documentElement);
        // Mermaid's khroma parser only understands legacy color syntax (hex /
        // rgb / hsl / named). Our tokens are authored in oklch(), which
        // getComputedStyle preserves verbatim, and a `color-mix(in srgb, ...)`
        // round-trip still serializes as `color(srgb r g b)` per CSS Color 4.
        // Rasterize each token through a 1x1 canvas: fillStyle accepts any CSS
        // <color>, getImageData returns concrete 8-bit sRGB bytes regardless
        // of the input's color space.
        const canvas = document.createElement("canvas");
        canvas.width = 1;
        canvas.height = 1;
        const ctx = canvas.getContext("2d", { willReadFrequently: true });

        const v = (name: string, fallback: string) => {
          const raw = css.getPropertyValue(name).trim();
          if (!raw || !ctx) return fallback;
          // fillStyle silently ignores unparseable input; prime with a known
          // baseline so a parse failure paints black, not whatever was last set.
          ctx.fillStyle = "#000";
          ctx.fillStyle = raw;
          ctx.fillRect(0, 0, 1, 1);
          const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
          return `rgb(${r}, ${g}, ${b})`;
        };

        const brand = v("--brand", "#3b82f6");
        const background = v("--background", "#ffffff");
        const foreground = v("--foreground", "#111111");
        const muted = v("--muted", "#f5f5f5");
        const mutedFg = v("--muted-foreground", "#6b7280");
        // Semantic diagram outlines need stronger contrast than the frame.
        const border = mutedFg;

        mermaid.initialize({
          startOnLoad: false,
          theme: "base",
          securityLevel: "strict",
          fontFamily: getComputedStyle(document.body).fontFamily,
          htmlLabels: false,
          suppressErrorRendering: true,
          themeVariables: {
            // Canvas
            background,
            mainBkg: background,
            // Nodes — soft muted fill with full-contrast text and a subtle border
            primaryColor: muted,
            primaryTextColor: foreground,
            primaryBorderColor: border,
            secondaryColor: muted,
            secondaryTextColor: foreground,
            secondaryBorderColor: border,
            tertiaryColor: background,
            tertiaryTextColor: foreground,
            tertiaryBorderColor: border,
            // Edges + labels
            lineColor: mutedFg,
            textColor: foreground,
            edgeLabelBackground: background,
            labelBackground: background,
            // Clusters (subgraph boxes)
            clusterBkg: muted,
            clusterBorder: border,
            titleColor: foreground,
            // Notes / callouts
            noteBkgColor: muted,
            noteTextColor: foreground,
            noteBorderColor: border,
            // Brand accent — used for active / start states in state diagrams,
            // user-decision diamonds in flowcharts, etc.
            activeTaskBkgColor: brand,
            activeTaskBorderColor: brand,
            altBackground: muted,
            // Sequence / git diagrams (harmless if unused)
            actorBkg: muted,
            actorBorder: border,
            actorTextColor: foreground,
            actorLineColor: mutedFg,
            signalColor: foreground,
            signalTextColor: foreground,
            labelBoxBkgColor: background,
            labelBoxBorderColor: border,
            labelTextColor: foreground,
            loopTextColor: foreground,
            // Fine print
            errorBkgColor: muted,
            errorTextColor: foreground,
          },
        });

        const domId = `mermaid-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`;
        const { svg } = await mermaid.render(domId, chart.trim());
        if (cancelled) return;
        const svgDocument = new DOMParser().parseFromString(svg, "image/svg+xml");
        const viewBox = svgDocument.documentElement.getAttribute("viewBox")?.split(/\s+/).map(Number);
        const width = viewBox?.[2];
        setRendered({ svg, width: width && Number.isFinite(width) && width > 0 ? width : 800 });
        setError(null);
      } catch (err: unknown) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
          setRendered(null);
        }
      }
    }
    void renderDiagram();

    return () => {
      cancelled = true;
    };
  }, [chart, reactId, resolvedTheme]);

  return (
    <div className="not-prose my-6 min-w-0 overflow-hidden rounded-lg border border-border bg-background">
      {error ? (
        <p role="status" className="m-0 px-4 pt-4 text-body text-muted-foreground">{copy.error}</p>
      ) : rendered ? (
        <div className="overflow-x-auto p-4 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring" tabIndex={0} role="region" aria-label={copy.diagram}>
          <div
            className="mx-auto shrink-0 [&_svg]:block [&_svg]:h-auto [&_svg]:w-full [&_svg]:max-w-none"
            style={{ width: rendered.width }}
            dangerouslySetInnerHTML={{ __html: rendered.svg }}
          />
        </div>
      ) : (
        <div role="status" className="flex min-h-56 items-center justify-center px-4 text-body text-muted-foreground">
          {copy.loading}
        </div>
      )}
      <details open={error ? true : undefined} className="border-t border-border px-4 py-3 text-caption">
        <summary className="w-fit cursor-pointer rounded text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring">
          {copy.source}
        </summary>
        <pre className="mt-3 overflow-x-auto whitespace-pre font-mono text-caption text-foreground"><code>{chart}</code></pre>
      </details>
    </div>
  );
}
