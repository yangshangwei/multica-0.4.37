import type { EventEmitter } from "node:events";
import {
  CLOSE_BEHAVIOR_CHANNELS,
  isCloseBehavior,
  type CloseBehaviorPromptResult,
} from "../shared/close-behavior";

interface PromptWindow extends EventEmitter {
  isDestroyed(): boolean;
  webContents: EventEmitter & {
    isDestroyed(): boolean;
    send(channel: string, payload: { requestId: string }): void;
  };
}

interface PendingPrompt {
  sender: PromptWindow["webContents"];
  requestId: string;
  acknowledge(): void;
  finish(result: CloseBehaviorPromptResult): void;
}

const CANCEL: CloseBehaviorPromptResult = { action: "ask", remember: false };

/** Delivery has a deadline; a responsive user-facing dialog does not. */
export class CloseBehaviorPromptCoordinator {
  private pending: PendingPrompt | null = null;
  private sequence = 0;

  request(window: PromptWindow): Promise<CloseBehaviorPromptResult> {
    if (window.isDestroyed() || window.webContents.isDestroyed()) return Promise.resolve(CANCEL);
    if (this.pending) return Promise.resolve(CANCEL);
    const requestId = String(++this.sequence);

    return new Promise((resolve, reject) => {
      const cleanup = () => {
        clearTimeout(timeout);
        window.removeListener("unresponsive", onUnresponsive);
        window.webContents.removeListener("destroyed", onDestroyed);
        window.webContents.removeListener("render-process-gone", onDestroyed);
        window.webContents.removeListener("did-start-navigation", onNavigation);
        this.pending = null;
      };
      const finish = (result: CloseBehaviorPromptResult) => {
        cleanup();
        resolve(result);
      };
      const fail = (error: unknown) => {
        cleanup();
        reject(error);
      };
      const onUnresponsive = () => fail(new Error("close-behavior renderer unresponsive"));
      const onDestroyed = () => finish(CANCEL);
      const onNavigation = (_event: unknown, _url: string, sameDocument: boolean, mainFrame: boolean) => {
        if (mainFrame && !sameDocument) finish(CANCEL);
      };
      const timeout = setTimeout(
        () => fail(new Error("close-behavior prompt delivery timed out")),
        5000,
      );
      this.pending = {
        sender: window.webContents,
        requestId,
        acknowledge: () => clearTimeout(timeout),
        finish,
      };
      window.on("unresponsive", onUnresponsive);
      window.webContents.on("destroyed", onDestroyed);
      window.webContents.on("render-process-gone", onDestroyed);
      window.webContents.on("did-start-navigation", onNavigation);
      try {
        window.webContents.send(CLOSE_BEHAVIOR_CHANNELS.prompt, { requestId });
      } catch (error) {
        fail(error);
      }
    });
  }

  acknowledge(sender: object, requestId: unknown): void {
    if (this.pending?.sender === sender && this.pending.requestId === requestId) {
      this.pending.acknowledge();
    }
  }

  respond(sender: object, payload: unknown): void {
    if (!payload || typeof payload !== "object") return;
    const { requestId, action, remember } = payload as Record<string, unknown>;
    if (this.pending?.sender !== sender || this.pending.requestId !== requestId) return;
    if (!isCloseBehavior(action)) return;
    this.pending.finish({ action, remember: remember === true });
  }

  cancel(): void {
    this.pending?.finish(CANCEL);
  }
}
