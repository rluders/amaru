import assert from "node:assert/strict";
import { test } from "node:test";
import { EventStream, type EventStreamOptions, type PlatformEvent, type SocketLike, type StreamState } from "../src/index.ts";

class FakeSocket implements SocketLike {
  onopen: SocketLike["onopen"] = null;
  onmessage: SocketLike["onmessage"] = null;
  onclose: SocketLike["onclose"] = null;
  onerror: SocketLike["onerror"] = null;
  closed = false;
  readonly url: URL;
  constructor(url: URL) {
    this.url = url;
  }
  close(): void {
    this.closed = true;
  }
  send(frame: unknown): void {
    this.onmessage?.({ data: typeof frame === "string" ? frame : JSON.stringify(frame) });
  }
  drop(): void {
    this.onclose?.({});
  }
}

const hello = (lastSeq: number, protocolVersion = 1) => ({ type: "hello", protocolVersion, platformVersion: "0.0.0-test", lastSeq });
const event = (seq: number, extra: Record<string, unknown> = {}) => ({
  id: `01J0000000000000000000${String(seq).padStart(4, "0")}`,
  seq,
  type: "work.changed",
  time: "2026-10-26T10:00:00Z",
  projectId: "p1",
  resource: { type: "work_package", id: "wp1" },
  data: {},
  ...extra,
});

function harness(over: Partial<EventStreamOptions> = {}) {
  const sockets: FakeSocket[] = [];
  const waiters: Array<(s: FakeSocket) => void> = [];
  const events: number[] = [];
  const states: StreamState[] = [];
  const resyncs: Array<{ reason: string; resumeFrom: number }> = [];
  const sleeps: number[] = [];
  const stream = new EventStream({
    baseUrl: "https://platform.test/v1/",
    projectId: "p1",
    token: "tok-fake",
    onEvent: (e: PlatformEvent) => void events.push(e.seq),
    onResync: (i) => void resyncs.push(i),
    onState: (s) => void states.push(s),
    webSocket: (url) => {
      const s = new FakeSocket(new URL(url));
      sockets.push(s);
      waiters.shift()?.(s);
      return s;
    },
    sleep: async (ms) => void sleeps.push(ms),
    ...over,
  });
  // Resolves with the nth socket once it exists.
  const socket = (n: number): Promise<FakeSocket> =>
    new Promise((resolve) => {
      const tick = () => (sockets[n] ? resolve(sockets[n]) : setImmediate(tick));
      tick();
    });
  const settle = () => new Promise<void>((r) => setImmediate(r));
  return { stream, sockets, socket, events, states, resyncs, sleeps, settle };
}

test("connects to the Project stream with the token and no cursor on the first connect", async () => {
  const h = harness();
  const run = h.stream.start();
  const s = await h.socket(0);
  assert.equal(s.url.protocol, "wss:");
  assert.equal(s.url.pathname, "/v1/projects/p1/events");
  assert.equal(s.url.searchParams.get("token"), "tok-fake");
  assert.equal(s.url.searchParams.get("since"), null);
  h.stream.stop();
  await run;
});

test("delivers events in order and ignores a frame that arrives before Hello", async () => {
  const h = harness();
  const run = h.stream.start();
  const s = await h.socket(0);
  s.send(event(1));
  s.send(hello(0));
  s.send(event(1));
  s.send(event(2));
  await h.settle();
  assert.deepEqual(h.events, [1, 2]);
  assert.equal(h.stream.lastSeq, 2);
  assert.ok(h.states.some((x) => x.kind === "connected"));
  h.stream.stop();
  await run;
});

test("after a drop it reconnects with since=<last delivered seq> and does not redeliver", async () => {
  const h = harness();
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(0));
  a.send(event(1));
  a.send(event(2));
  await h.settle();
  a.drop();
  const b = await h.socket(1);
  assert.equal(b.url.searchParams.get("since"), "2");
  b.send(hello(3));
  b.send(event(2)); // replayed overlap: dropped by seq
  b.send(event(3));
  await h.settle();
  assert.deepEqual(h.events, [1, 2, 3]);
  h.stream.stop();
  await run;
});

test("a gap in a Project stream reconnects to replay instead of skipping events", async () => {
  const h = harness();
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(0));
  a.send(event(1));
  a.send(event(3)); // 2 is missing
  await h.settle();
  assert.deepEqual(h.events, [1]);
  const b = await h.socket(1);
  assert.equal(b.url.searchParams.get("since"), "1");
  assert.ok(a.closed);
  b.send(hello(3));
  b.send(event(2));
  b.send(event(3));
  await h.settle();
  assert.deepEqual(h.events, [1, 2, 3]);
  h.stream.stop();
  await run;
});

test("an Execution-filtered stream accepts non-contiguous seq", async () => {
  const h = harness({ executionId: "x1" });
  const run = h.stream.start();
  const a = await h.socket(0);
  assert.equal(a.url.searchParams.get("executionId"), "x1");
  a.send(hello(9));
  a.send(event(2));
  a.send(event(7));
  await h.settle();
  assert.deepEqual(h.events, [2, 7]);
  h.stream.stop();
  await run;
});

test("ResyncRequired: onResync runs before any further event, then the stream resumes from resumeFrom", async () => {
  const order: string[] = [];
  const h = harness({
    since: 5,
    onEvent: (e) => void order.push(`event:${e.seq}`),
    onResync: async (i) => {
      order.push(`resync:${i.reason}:${i.resumeFrom}`);
      await new Promise((r) => setImmediate(r)); // reloading state takes time
      order.push("reloaded");
    },
  });
  const run = h.stream.start();
  const a = await h.socket(0);
  assert.equal(a.url.searchParams.get("since"), "5");
  a.send(hello(100));
  a.send({ type: "resync_required", reason: "cursor_too_old", resumeFrom: 80 });
  a.send(event(81)); // arrives while resync is running on the old connection: must not be delivered
  const b = await h.socket(1);
  assert.equal(b.url.searchParams.get("since"), "80");
  b.send(hello(100));
  b.send(event(81));
  await h.settle();
  assert.deepEqual(order, ["resync:cursor_too_old:80", "reloaded", "event:81"]);
  h.stream.stop();
  await run;
});

test("a Platform with fewer events than the cursor (restart) triggers a resync even without the frame", async () => {
  const h = harness({ since: 50 });
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(3));
  const b = await h.socket(1);
  assert.deepEqual(h.resyncs, [{ reason: "cursor_ahead", resumeFrom: 3 }]);
  assert.equal(b.url.searchParams.get("since"), "3");
  h.stream.stop();
  await run;
});

test("an incompatible protocol major stops the stream and is not retried", async () => {
  const h = harness();
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(0, 2));
  await run; // resolves by itself
  assert.deepEqual(h.states.at(-1), { kind: "incompatible", protocolVersion: 2 });
  assert.equal(h.sockets.length, 1);
});

test("connection failures back off, give up after maxAttempts, and retryNow starts over", async () => {
  const h = harness({ maxAttempts: 3, backoff: (n) => 100 * (n + 1) });
  const run = h.stream.start();
  for (let i = 0; i < 3; i++) (await h.socket(i)).drop();
  await h.settle();
  assert.deepEqual(h.sleeps, [100, 200]); // third failure does not sleep: it gives up
  assert.deepEqual(h.states.at(-1), { kind: "unavailable", attempts: 3 });
  h.stream.retryNow();
  const s = await h.socket(3);
  s.send(hello(0));
  await h.settle();
  assert.deepEqual(h.states.at(-1), { kind: "connected" });
  h.stream.stop();
  await run;
});

test("a good connection resets the failure count", async () => {
  const h = harness({ maxAttempts: 2, backoff: () => 1 });
  const run = h.stream.start();
  (await h.socket(0)).drop(); // failure 1
  const b = await h.socket(1);
  b.send(hello(0));
  await h.settle();
  b.drop(); // after a good session: failure count restarts, so this is not the 2nd failure
  const c = await h.socket(2);
  c.send(hello(0));
  await h.settle();
  assert.notEqual(h.states.at(-1)?.kind, "unavailable");
  h.stream.stop();
  await run;
});

test("a malformed frame reconnects; unknown frame types are ignored", async () => {
  const h = harness({ backoff: () => 1 });
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(0));
  a.send({ type: "future.frame", seq: undefined, x: 1 });
  a.send(event(1));
  await h.settle();
  assert.deepEqual(h.events, [1]);
  a.send("not json");
  const b = await h.socket(1);
  assert.equal(b.url.searchParams.get("since"), "1");
  h.stream.stop();
  await run;
});

test("an event handler that throws is retried: the cursor does not move past it", async () => {
  let fail = true;
  const seen: number[] = [];
  const h = harness({
    backoff: () => 1,
    onEvent: (e) => {
      if (e.seq === 2 && fail) {
        fail = false;
        throw new Error("boom");
      }
      seen.push(e.seq);
    },
  });
  const run = h.stream.start();
  const a = await h.socket(0);
  a.send(hello(0));
  a.send(event(1));
  a.send(event(2));
  const b = await h.socket(1);
  assert.equal(b.url.searchParams.get("since"), "1");
  b.send(hello(2));
  b.send(event(2));
  await h.settle();
  assert.deepEqual(seen, [1, 2]);
  h.stream.stop();
  await run;
});

test("stop() closes the socket and ends start()", async () => {
  const h = harness();
  const run = h.stream.start();
  const a = await h.socket(0);
  h.stream.stop();
  await run;
  assert.ok(a.closed);
  assert.deepEqual(h.states.at(-1), { kind: "stopped" });
});
