// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { openRuntimeConfigEditorFromWindow } from "./runtime-config-editor";

describe("runtime-config editor handoff", () => {
  it("opens main's editor for main and registered issue windows", () => {
    const mainWindow = { isDestroyed: () => false };
    const issue = { isDestroyed: () => false };
    const openMainEditor = vi.fn();
    for (const sourceWindow of [mainWindow, issue]) {
      expect(openRuntimeConfigEditorFromWindow({
        sourceWindow, mainWindow, issueWindows: new Set([issue]), openMainEditor,
      })).toEqual({ ok: true });
    }
    expect(openMainEditor).toHaveBeenCalledTimes(2);
    expect(openMainEditor).toHaveBeenNthCalledWith(1);
  });

  it("allows an issue that outlived main to recreate the main editor", () => {
    const issue = { isDestroyed: () => false };
    const openMainEditor = vi.fn();
    expect(openRuntimeConfigEditorFromWindow({
      sourceWindow: issue, mainWindow: null, issueWindows: new Set([issue]), openMainEditor,
    })).toEqual({ ok: true });
    expect(openMainEditor).toHaveBeenCalledOnce();
  });

  it("rejects unknown or destroyed senders without any editor action", () => {
    const mainWindow = { isDestroyed: () => false };
    const deadIssue = { isDestroyed: () => true };
    const openMainEditor = vi.fn();
    for (const sourceWindow of [null, { isDestroyed: () => false }, deadIssue]) {
      expect(openRuntimeConfigEditorFromWindow({
        sourceWindow, mainWindow, issueWindows: new Set([deadIssue]), openMainEditor,
      })).toEqual({ ok: false });
    }
    expect(openMainEditor).not.toHaveBeenCalled();
  });
});
