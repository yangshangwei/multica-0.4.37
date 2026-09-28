import { createRef, useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { workspaceListOptions } from "@multica/core/workspace/queries";
import { ContentEditor, type ContentEditorRef } from "../editor/content-editor";
import { NavigationProvider } from "../navigation/context";
import { RESOURCES } from "../locales";
import { IssueDescriptionAssist } from "./issue-description-assist";

const optimize = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/api")>();
  return { ...actual, api: { optimizeIssueDescription: optimize } };
});

// jsdom supplies Range but no layout geometry; native ProseMirror focus uses it.
const rangeGeometry = ["getClientRects", "getBoundingClientRect"].map((name) => ({
  name, descriptor: Object.getOwnPropertyDescriptor(Range.prototype, name),
}));
beforeAll(() => {
  Object.defineProperty(Range.prototype, "getClientRects", { configurable: true, value: () => [] });
  Object.defineProperty(Range.prototype, "getBoundingClientRect", { configurable: true, value: () => new DOMRect() });
});
afterAll(() => {
  for (const { name, descriptor } of rangeGeometry) {
    if (descriptor) Object.defineProperty(Range.prototype, name, descriptor);
    else Reflect.deleteProperty(Range.prototype, name);
  }
});

// Keep the real editor, Markdown parser, node views, readonly preview and mutation.
// Only the AI API is mocked; unrelated queries are disabled in this isolated host.
afterEach(() => vi.restoreAllMocks());

it("preserves file cards, mentions and code through AI apply/undo and returns focus to the editor", async () => {
  const protectedContent = [
    "!file[report.pdf](/uploads/report.pdf)",
    "[@Alice](mention://member/alice)",
    "```ts\nconst result = compute(1, 2);\n```",
  ].join("\n\n");
  const original = `Review this report.\n\n${protectedContent}`;
  const optimized = `Review the report and explain the result.\n\n${protectedContent}`;
  optimize.mockResolvedValue({ text: optimized, questions: ["Which milestone?"] });
  const editorRef = createRef<ContentEditorRef>();
  const queryClient = new QueryClient({ defaultOptions: { queries: { enabled: false, retry: false } } });
  queryClient.setQueryData(workspaceListOptions().queryKey, [{
    id: "workspace-1", slug: "test", name: "Test", description: null, context: null,
    settings: {}, repos: [], issue_prefix: "TES", avatar_url: null,
    created_at: "2026-09-29T00:00:00Z", updated_at: "2026-09-29T00:00:00Z",
  }]);

  function Host() {
    const [value, setValue] = useState(original);
    return (
      <>
        <ContentEditor ref={editorRef} defaultValue={original} onUpdate={setValue} debounceMs={500} />
        <IssueDescriptionAssist wsId="workspace-1" mode="manual" editorRef={editorRef} value={value}
          onChange={setValue} uploading={false} isBlocked={() => false} submitting={false} />
        <output aria-label="Persisted draft">{value}</output>
      </>
    );
  }

  render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider locale="en" resources={RESOURCES}>
        <WorkspaceSlugProvider slug="test">
          <NavigationProvider value={{ push: () => {}, replace: () => {}, back: () => {}, pathname: "/test/issues", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => `https://example.test${path}` }}>
            <Host />
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(editorRef.current?.getMarkdown()).toContain("!file[report.pdf](/uploads/report.pdf)"));
  const originalSerialized = editorRef.current!.getMarkdown().trim();
  const editor = document.querySelector<HTMLElement>(".ProseMirror")!;
  expect(editor.querySelector('[data-type="fileCard"]')).not.toBeNull();
  expect(editor.querySelector(".mention")).toHaveTextContent("@Alice");
  expect(editor.querySelector("pre")).toHaveTextContent("const result = compute(1, 2);");

  await userEvent.click(screen.getByRole("button", { name: "AI optimize description" }));
  await screen.findByRole("button", { name: "Apply and replace" });
  expect(optimize).toHaveBeenCalledWith(expect.objectContaining({ text: originalSerialized, mode: "manual" }), { workspaceId: "workspace-1", signal: expect.any(AbortSignal), onText: expect.any(Function) });
  expect(editorRef.current!.getMarkdown().trim()).toBe(originalSerialized);
  await userEvent.click(screen.getByRole("button", { name: "Apply and replace" }));
  expect(editorRef.current!.getMarkdown()).toContain(protectedContent);
  expect(screen.getByLabelText("Persisted draft")).toHaveTextContent("Review the report and explain the result.");
  expect(editorRef.current!.getMarkdown()).not.toContain("Which milestone?");
  await waitFor(() => expect(editor).toHaveFocus());

  await userEvent.click(screen.getByRole("button", { name: "Undo" }));
  expect(editorRef.current!.getMarkdown().trim()).toBe(originalSerialized);
  expect(screen.getByLabelText("Persisted draft").textContent).toBe(originalSerialized);
  await waitFor(() => expect(editor).toHaveFocus());
});
