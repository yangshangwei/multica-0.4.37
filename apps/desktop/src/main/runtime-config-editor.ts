interface EditorSourceWindow {
  isDestroyed(): boolean;
}

/** Opening the editor does not validate, save, or mutate a server/session. */
export function openRuntimeConfigEditorFromWindow<T extends EditorSourceWindow>({
  sourceWindow,
  mainWindow,
  issueWindows,
  openMainEditor,
}: {
  sourceWindow: T | null;
  mainWindow: T | null;
  issueWindows: ReadonlySet<T>;
  openMainEditor: () => void;
}): { ok: boolean } {
  if (!sourceWindow || sourceWindow.isDestroyed()) return { ok: false };
  if (sourceWindow !== mainWindow && !issueWindows.has(sourceWindow)) return { ok: false };
  openMainEditor();
  return { ok: true };
}
