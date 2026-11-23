"use client";

import { MermaidDiagram, MERMAID_SKELETON_HEIGHT_PX } from "../editor/mermaid-diagram";
import { LazyRichBlock } from "../rich-content/lazy-rich-block";
import "../editor/styles/mermaid.css";
import "./docs-diagram.css";

/** Only the diagram inherits diagram styles; the surrounding prose stays in the reader. */
export function DocsDiagram({ chart }: { chart: string }) {
  return (
    <div className="docs-diagram my-6 min-w-0">
      <LazyRichBlock reservedHeightPx={MERMAID_SKELETON_HEIGHT_PX} sourceKey={chart}>
        <MermaidDiagram chart={chart} />
      </LazyRichBlock>
    </div>
  );
}
