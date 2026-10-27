// Event-stream client: one WebSocket per Project (operation `events` in the contract), resumed from a cursor.
// Behavior follows the contract text (first frame Hello, strictly increasing `seq`, `since=<seq>` replay,
// ResyncRequired). The earlier plugin's connection.ts was a behavioral reference only (ADR-0031); no code is shared.
import type { components } from "./generated/schema.ts";

export type StreamFrame = components["schemas"]["StreamFrame"];
export type Hello = components["schemas"]["Hello"];
export type ResyncRequired = components["schemas"]["ResyncRequired"];
export type PlatformEvent = components["schemas"]["Event"];

/** The part of WebSocket the client uses. globalThis.WebSocket satisfies it. */
export interface SocketLike {
  onopen: ((ev: unknown) => void) | null;
  onmessage: ((ev: { data: unknown }) => void) | null;
  onclose: ((ev: unknown) => void) | null;
  onerror: ((ev: unknown) => void) | null;
  close(code?: number, reason?: string): void;
}

export type StreamState =
  | { kind: "connecting" }
  | { kind: "connected" }
  /** The connection dropped; a new attempt follows the backoff. */
  | { kind: "reconnecting"; attempt: number }
  /** Too many failed attempts in a row; waits for retryNow(). */
  | { kind: "unavailable"; attempts: number }
  /** The Platform speaks another protocol major. Not retried. */
  | { kind: "incompatible"; protocolVersion: number }
  | { kind: "stopped" };

export interface EventStreamOptions {
  /** API root including the major version, for example http://127.0.0.1:7777/v1 (http becomes ws, https becomes wss). */
  baseUrl: string;
  projectId: string;
  /** Sent as ?token= because browsers cannot set WebSocket headers (contract). The URL is never passed to callbacks. */
  token?: string;
  /** Only events of this Execution. Its `seq` values are not gap-free, so gap detection is off when set. */
  executionId?: string;
  /** Last `seq` already processed; resume replays events with seq greater than this. */
  since?: number;
  /** Delivery of one event. A returned promise is awaited before the next event is delivered. */
  onEvent(event: PlatformEvent): void | Promise<void>;
  /**
   * The cursor cannot be served. Reload state through the HTTP API and resolve; the stream then resumes from
   * `resumeFrom`. Events are not delivered while this runs.
   */
  onResync(info: { reason: ResyncRequired["reason"]; resumeFrom: number }): void | Promise<void>;
  onState?(state: StreamState): void;
  /** Defaults to `new WebSocket(url)`. */
  webSocket?(url: string): SocketLike;
  /** Delay in ms before attempt n (0-based). Default 500 * 2^n, capped at 15 s. */
  backoff?(attempt: number): number;
  /** Consecutive failed attempts before the state becomes `unavailable`. Default 8. */
  maxAttempts?: number;
  /** Replaces setTimeout (tests). Resolves after `ms`. */
  sleep?(ms: number): Promise<void>;
}

const SUPPORTED_PROTOCOL = 1;

export const defaultBackoff = (attempt: number): number => Math.min(500 * 2 ** attempt, 15_000);

type End = "dropped" | "resync" | "gap" | "incompatible" | "stopped";

export class EventStream {
  private readonly opts: EventStreamOptions;
  private cursor: number;
  private stopped = false;
  private endCycle: (() => void) | null = null;
  private wake: (() => void) | null = null;
  private done: Promise<void> | null = null;

  constructor(options: EventStreamOptions) {
    this.opts = options;
    this.cursor = options.since ?? 0;
  }

  /** Highest `seq` delivered so far (or the initial `since`). */
  get lastSeq(): number {
    return this.cursor;
  }

  /** Starts the stream. Resolves when stop() is called or the protocol is incompatible. */
  start(): Promise<void> {
    this.done ??= this.run();
    return this.done;
  }

  stop(): void {
    this.stopped = true;
    this.endCycle?.();
    this.wake?.();
    this.state({ kind: "stopped" });
  }

  /** Skips the backoff wait, or leaves the `unavailable` state, and connects now. */
  retryNow(): void {
    this.wake?.();
  }

  private state(s: StreamState): void {
    this.opts.onState?.(s);
  }

  private async run(): Promise<void> {
    const max = this.opts.maxAttempts ?? 8;
    const backoff = this.opts.backoff ?? defaultBackoff;
    let failures = 0;
    while (!this.stopped) {
      this.state(failures === 0 && this.cursor === (this.opts.since ?? 0) ? { kind: "connecting" } : { kind: "reconnecting", attempt: failures });
      const { end, connected } = await this.cycle();
      if (this.stopped || end === "stopped") break;
      if (end === "incompatible") return;
      failures = connected ? 0 : failures + 1;
      if (end === "resync" || end === "gap") {
        failures = 0; // a deliberate reconnect, not a failure
        continue;
      }
      if (failures >= max) {
        this.state({ kind: "unavailable", attempts: failures });
        await this.park();
        failures = 0;
        continue;
      }
      this.state({ kind: "reconnecting", attempt: failures });
      await this.wait(backoff(Math.max(0, failures - 1)));
    }
    this.state({ kind: "stopped" });
  }

  private url(): string {
    const u = new URL(`${this.opts.baseUrl.replace(/\/+$/, "")}/projects/${encodeURIComponent(this.opts.projectId)}/events`);
    u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
    if (this.cursor > 0) u.searchParams.set("since", String(this.cursor));
    if (this.opts.executionId) u.searchParams.set("executionId", this.opts.executionId);
    if (this.opts.token) u.searchParams.set("token", this.opts.token);
    return u.toString();
  }

  /** One connection from open to close. `connected` is true once Hello was accepted. */
  private cycle(): Promise<{ end: End; connected: boolean }> {
    return new Promise((resolve) => {
      let connected = false;
      let settled = false;
      let sawHello = false;
      // Frames are handled one at a time so that onEvent/onResync can be async without reordering.
      let chain: Promise<void> = Promise.resolve();

      let socket: SocketLike;
      try {
        socket = this.opts.webSocket ? this.opts.webSocket(this.url()) : (new WebSocket(this.url()) as unknown as SocketLike);
      } catch {
        resolve({ end: "dropped", connected: false });
        return;
      }
      const finish = (end: End): void => {
        if (settled) return;
        settled = true;
        socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null;
        if (this.endCycle === stopCycle) this.endCycle = null;
        try {
          socket.close(1000);
        } catch {
          // already closed
        }
        resolve({ end: this.stopped ? "stopped" : end, connected });
      };

      const stopCycle = (): void => finish("stopped");
      this.endCycle = stopCycle;
      socket.onerror = () => finish("dropped");
      socket.onclose = () => finish("dropped");
      socket.onmessage = (ev) => {
        chain = chain.then(async () => {
          if (settled) return;
          const frame = parse(ev.data);
          if (!frame) return finish("dropped"); // malformed: reconnect and replay from the cursor
          if (isHello(frame)) {
            if (sawHello) return;
            sawHello = true;
            if (frame.protocolVersion !== SUPPORTED_PROTOCOL) {
              this.state({ kind: "incompatible", protocolVersion: frame.protocolVersion });
              return finish("incompatible");
            }
            connected = true;
            if (frame.lastSeq < this.cursor) {
              // The Platform has fewer events than we have seen (restart, restore): the cursor is meaningless.
              await this.resync("cursor_ahead", frame.lastSeq);
              return finish("resync");
            }
            this.state({ kind: "connected" });
            return;
          }
          if (isResync(frame)) {
            await this.resync(frame.reason, frame.resumeFrom);
            return finish("resync");
          }
          if (!sawHello || !isEvent(frame)) return; // unknown frame types are ignored (additive evolution)
          if (frame.seq <= this.cursor) return; // already delivered (redelivery after reconnect)
          if (!this.opts.executionId && this.cursor > 0 && frame.seq !== this.cursor + 1) return finish("gap");
          await this.opts.onEvent(frame);
          this.cursor = frame.seq;
                }).catch(() => finish("dropped"));
      };
    });
  }

  private async resync(reason: ResyncRequired["reason"], resumeFrom: number): Promise<void> {
    await this.opts.onResync({ reason, resumeFrom });
    this.cursor = resumeFrom;
  }

  private wait(ms: number): Promise<void> {
    return new Promise((resolve) => {
      this.wake = () => {
        this.wake = null;
        resolve();
      };
      const sleep = this.opts.sleep ?? ((n: number) => new Promise<void>((r) => setTimeout(r, n)));
      void sleep(ms).then(() => this.wake?.());
    });
  }

  private park(): Promise<void> {
    return new Promise((resolve) => {
      this.wake = () => {
        this.wake = null;
        resolve();
      };
    });
  }
}

type Raw = { type: string } & Record<string, unknown>;

function parse(data: unknown): Raw | null {
  if (typeof data !== "string") return null;
  try {
    const v: unknown = JSON.parse(data);
    if (typeof v === "object" && v !== null && typeof (v as { type?: unknown }).type === "string") return v as Raw;
  } catch {
    // fall through
  }
  return null;
}

const isInt = (v: unknown): v is number => typeof v === "number" && Number.isInteger(v) && v >= 0;

function isHello(f: Raw): f is Raw & { protocolVersion: number; lastSeq: number } {
  return f.type === "hello" && isInt(f["protocolVersion"]) && isInt(f["lastSeq"]);
}

function isResync(f: Raw): f is Raw & { reason: ResyncRequired["reason"]; resumeFrom: number } {
  return f.type === "resync_required" && typeof f["reason"] === "string" && isInt(f["resumeFrom"]);
}

// Frames other than Hello and ResyncRequired are Events; the server's schema guarantees the rest of the shape.
function isEvent(f: Raw): f is Raw & PlatformEvent {
  return f.type !== "hello" && f.type !== "resync_required" && isInt(f["seq"]) && (f["seq"] as number) >= 1 && typeof f["id"] === "string";
}
