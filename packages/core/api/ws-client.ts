import type { WSMessage, WSEventType } from "../types/events";
import { type Logger, noopLogger } from "../logger";

type EventHandler = (payload: unknown, actorId?: string, actorType?: string) => void;

// Cap how much of an unparseable frame we put into the log. A malformed or
// rogue server can stream arbitrarily large garbage, and the warn handler may
// be a console / IPC bridge whose buffers we don't want to blow.
const UNPARSEABLE_LOG_MAX_CHARS = 200;

// Reconnect backoff parameters. A flat delay causes a thundering herd when many
// clients reconnect after a server restart; exponential backoff with jitter
// spreads the reconnection attempts over time. The client retries indefinitely
// (capped at RECONNECT_MAX_DELAY_MS) because the web/desktop UI does not yet
// expose a visible disconnected state or manual retry action.
const RECONNECT_BASE_DELAY_MS = 1_000;
const RECONNECT_MAX_DELAY_MS = 30_000;

function summarizeUnparseable(data: unknown): string {
  const text = typeof data === "string" ? data : String(data);
  if (text.length <= UNPARSEABLE_LOG_MAX_CHARS) return text;
  return `${text.slice(0, UNPARSEABLE_LOG_MAX_CHARS)}… (truncated, ${text.length} chars total)`;
}

/** Identifies the WS client to the server. Sent as `client_platform`,
 *  `client_version`, and `client_os` query parameters on the upgrade URL —
 *  browsers cannot set custom headers on WebSocket handshakes, so query
 *  params are the only portable channel. */
export interface WSClientIdentity {
  platform?: string;
  version?: string;
  os?: string;
}

export class WSClient {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private token: string | null = null;
  private workspaceSlug: string | null = null;
  private cookieAuth = false;
  private identity: WSClientIdentity | undefined;
  private handlers = new Map<WSEventType, Set<EventHandler>>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private hasConnectedBefore = false;
  // One-shot per connection. A non-conforming frame can repeat hundreds of
  // times per session, so we log the first drop and suppress the rest. Reset
  // on each connect() so a fresh connection logs once again.
  private badFrameLogged = false;
  private onReconnectCallbacks = new Set<() => void>();
  private authenticated = false;
  private onAuthenticatedCallbacks = new Set<() => void>();
  private scopes = new Map<string, { scope: "account" | "issue"; id: string; count: number }>();
  private anyHandlers = new Set<(msg: WSMessage) => void>();
  private logger: Logger;

  constructor(
    url: string,
    options?: {
      logger?: Logger;
      cookieAuth?: boolean;
      identity?: WSClientIdentity;
    },
  ) {
    this.baseUrl = url;
    this.logger = options?.logger ?? noopLogger;
    this.cookieAuth = options?.cookieAuth ?? false;
    this.identity = options?.identity;
  }

  setAuth(token: string | null, workspaceSlug: string | null) {
    this.token = token;
    this.workspaceSlug = workspaceSlug;
  }

  connect() {
    this.badFrameLogged = false;
    this.authenticated = false;
    const url = new URL(this.baseUrl);
    // Token is never sent as a URL query parameter — it would be logged by
    // proxies, CDNs, and browser history.  In cookie mode the HttpOnly cookie
    // is sent automatically with the upgrade request.  In token mode the token
    // is delivered as the first WebSocket message after the connection opens.
    url.searchParams.delete("workspace_slug");
    url.searchParams.delete("workspace_id");
    url.searchParams.delete("scope");
    url.searchParams.delete("token");
    if (this.workspaceSlug) {
      url.searchParams.set("workspace_slug", this.workspaceSlug);
    } else {
      url.searchParams.set("scope", "account");
    }
    if (this.identity?.platform)
      url.searchParams.set("client_platform", this.identity.platform);
    if (this.identity?.version)
      url.searchParams.set("client_version", this.identity.version);
    if (this.identity?.os)
      url.searchParams.set("client_os", this.identity.os);

    const socket = new WebSocket(url.toString());
    this.ws = socket;

    socket.onopen = () => {
      if (this.ws !== socket) return;
      if (!this.cookieAuth && this.token) {
        socket.send(
          JSON.stringify({ type: "auth", payload: { token: this.token } }),
        );
        return;
      }

      this.markAuthenticated();
    };

    socket.onmessage = (event) => {
      if (this.ws !== socket) return;
      let msg: WSMessage;
      try {
        msg = JSON.parse(event.data as string) as WSMessage;
      } catch {
        this.logger.warn(
          "ws: received unparseable message",
          this.protectedConnection() ? "[redacted]" : summarizeUnparseable(event.data),
        );
        return;
      }
      // Trust boundary: a frame must be an object carrying a string `type`.
      // The server protocol guarantees this for every frame, but a
      // non-conforming frame — an out-of-protocol frame injected by a proxy /
      // browser extension, or a bare JSON primitive — must degrade to a no-op
      // here. Without this guard every downstream consumer (the onAny
      // dispatcher and every ws.on subscriber) runs against a bad shape;
      // `msg.type.split(...)` in the realtime sync threw an uncaught TypeError
      // out of onmessage and surfaced as a flood of global `$exception` events
      // (MUL-3418). Validate once at the boundary, trust the shape downstream.
      if (!msg || typeof (msg as { type?: unknown }).type !== "string") {
        if (!this.badFrameLogged) {
          this.badFrameLogged = true;
          this.logger.warn(
            "ws: dropping frame without a string type",
            this.protectedConnection() ? "[redacted]" : summarizeUnparseable(event.data),
          );
        }
        return;
      }
      if ((msg as any).type === "auth_ack") {
        this.markAuthenticated();
        return;
      }
      this.logger.debug("received", this.protectedConnection() ? "protected event" : msg.type);
      const eventHandlers = this.handlers.get(msg.type);
      if (eventHandlers) {
        for (const handler of eventHandlers) {
          handler(msg.payload, msg.actor_id, msg.actor_type);
        }
      }
      for (const handler of this.anyHandlers) {
        handler(msg);
      }
    };

    socket.onclose = () => {
      if (this.ws !== socket) return;
      this.authenticated = false;
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      // Suppress — onclose handles reconnect; errors during StrictMode
      // double-fire are expected in dev and harmless.
    };
  }

  /**
   * Schedule a reconnection attempt with exponential backoff and jitter.
   * Retries indefinitely with a capped delay because the web/desktop UI
   * does not yet expose a visible disconnected state or manual retry action.
   */
  private scheduleReconnect() {
    const base = Math.min(
      RECONNECT_BASE_DELAY_MS * 2 ** this.reconnectAttempt,
      RECONNECT_MAX_DELAY_MS,
    );
    // ±20 % jitter so clients that disconnected at the same time don't
    // reconnect in lockstep.
    const jitter = base * 0.2 * (Math.random() * 2 - 1);
    const delay = Math.round(
      Math.min(base + jitter, RECONNECT_MAX_DELAY_MS),
    );

    this.reconnectAttempt++;
    this.logger.warn(
      `ws: disconnected, reconnecting in ${delay}ms (attempt ${this.reconnectAttempt})`,
    );
    this.reconnectTimer = setTimeout(() => this.connect(), delay);
  }

  private markAuthenticated() {
    if (this.authenticated) return;
    this.authenticated = true;
    this.logger.info("connected");
    const recoveredConnection = this.hasConnectedBefore || this.reconnectAttempt > 0;
    this.reconnectAttempt = 0;
    for (const { scope, id } of this.scopes.values()) this.sendScope("subscribe", scope, id);
    for (const callback of this.onAuthenticatedCallbacks) callback();
    if (recoveredConnection) {
      for (const cb of this.onReconnectCallbacks) {
        try {
          cb();
        } catch {
          // ignore reconnect callback errors
        }
      }
    }
    this.hasConnectedBefore = true;
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      // Remove handlers before close to prevent onclose from scheduling a reconnect
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.close();
      this.ws = null;
    }
    this.hasConnectedBefore = false;
    this.authenticated = false;
    this.reconnectAttempt = 0;
    this.handlers.clear();
    this.anyHandlers.clear();
    this.onReconnectCallbacks.clear();
    this.onAuthenticatedCallbacks.clear();
    this.scopes.clear();
  }

  private protectedConnection() {
    return !this.workspaceSlug || this.scopes.size > 0;
  }

  private sendScope(type: "subscribe" | "unsubscribe", scope: "account" | "issue", id: string) {
    if (this.authenticated && this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload: { scope, id } }));
    }
  }

  /** Reference counts keep one wire subscription across multiple consumers. */
  subscribeScope(scope: "account" | "issue", id: string) {
    const key = JSON.stringify([scope, id]);
    const prior = this.scopes.get(key);
    if (prior) prior.count += 1;
    else {
      this.scopes.set(key, { scope, id, count: 1 });
      this.sendScope("subscribe", scope, id);
    }
    let disposed = false;
    return () => {
      if (disposed) return;
      disposed = true;
      const current = this.scopes.get(key);
      if (!current || --current.count > 0) return;
      this.scopes.delete(key);
      this.sendScope("unsubscribe", scope, id);
    };
  }

  /** Recheck a desired scope after a server-side revocation removed its room. */
  revalidateScope(scope: "account" | "issue", id: string) {
    if (this.scopes.has(JSON.stringify([scope, id]))) this.sendScope("subscribe", scope, id);
  }

  onAuthenticated(callback: () => void) {
    this.onAuthenticatedCallbacks.add(callback);
    if (this.authenticated) callback();
    return () => { this.onAuthenticatedCallbacks.delete(callback); };
  }

  on(event: WSEventType, handler: EventHandler) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);
    return () => {
      this.handlers.get(event)?.delete(handler);
    };
  }

  onAny(handler: (msg: WSMessage) => void) {
    this.anyHandlers.add(handler);
    return () => {
      this.anyHandlers.delete(handler);
    };
  }

  onReconnect(callback: () => void) {
    this.onReconnectCallbacks.add(callback);
    return () => {
      this.onReconnectCallbacks.delete(callback);
    };
  }

  send(message: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }
}
