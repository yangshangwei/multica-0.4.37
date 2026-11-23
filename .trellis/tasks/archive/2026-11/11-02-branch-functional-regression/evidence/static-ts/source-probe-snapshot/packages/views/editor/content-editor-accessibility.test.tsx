import { createRef } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import { expect, it } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider } from "../navigation/context";
import { RESOURCES } from "../locales";
import { ContentEditor, type ContentEditorRef } from "./content-editor";

// Keep the actual Tiptap root: mocking EditorContent would hide missing ARIA forwarding.
it("names the editable textbox and updates its description without replacing the draft", async () => {
  const client = new QueryClient({ defaultOptions: { queries: { enabled: false, retry: false, gcTime: 0 } } });
  const ref = createRef<ContentEditorRef>();
  function host(described: boolean, invalid = false) {
    return <QueryClientProvider client={client}>
      <I18nProvider locale="en" resources={RESOURCES}>
        <WorkspaceSlugProvider slug="test">
          <NavigationProvider value={{ push: () => {}, replace: () => {}, back: () => {}, pathname: "/test/issues", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path }}>
            <p id="prompt-label">Task description</p>
            <p id="prompt-help">Describe the expected result.</p>
            <ContentEditor ref={ref} defaultValue="Original draft" showBubbleMenu={false}
              aria-label="Fallback name" aria-labelledby="prompt-label"
              aria-describedby={described ? "prompt-help" : undefined}
              aria-invalid={invalid} />
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>
    </QueryClientProvider>;
  }
  const view = render(host(true));
  const editor = await screen.findByRole("textbox", { name: "Task description" });
  expect(editor).toHaveAttribute("contenteditable", "true");
  expect(editor).toHaveAttribute("aria-multiline", "true");
  expect(editor).toHaveAttribute("aria-invalid", "false");
  expect(editor).toHaveAccessibleDescription("Describe the expected result.");
  await act(async () => { ref.current?.adoptContent("Unfinished user draft"); });
  view.rerender(host(false, true));
  await waitFor(() => expect(editor).not.toHaveAttribute("aria-describedby"));
  expect(editor).toHaveAttribute("aria-invalid", "true");
  expect(screen.getByRole("textbox", { name: "Task description" })).toBe(editor);
  expect(ref.current?.getMarkdown()).toBe("Unfinished user draft");
  view.unmount();
  client.clear();
});
