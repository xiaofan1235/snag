import { describe, it, expect } from "vitest";
import { ConsistentEventStream, type StreamState } from "./stream";
import { RESYNC_EVENT } from "./consistency";
import type { EventEnvelope } from "../api/schema.gen";

// buildStream 把若干文本块做成一个读完即关闭的 ReadableStream<Uint8Array>。
function buildStream(chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  let i = 0;
  return new ReadableStream<Uint8Array>({
    pull(controller) {
      if (i < chunks.length) {
        controller.enqueue(enc.encode(chunks[i++]));
      } else {
        controller.close();
      }
    },
  });
}

function frame(seq: number, type = "job.updated"): string {
  const env: EventEnvelope = { event_type: type, run_id: "run-1", sequence: seq };
  return `event: ${type}\ndata: ${JSON.stringify(env)}\n\n`;
}

const immediateSleep = () => Promise.resolve();

describe("ConsistentEventStream（§16.33 端到端）", () => {
  it("补发无空洞：快照 cursor=10 → 投递 11,12,13", async () => {
    const delivered: number[] = [];
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => 10,
      openStream: async () => buildStream([frame(11), frame(12) + frame(13)]),
      onEvent: (e) => {
        delivered.push(e.sequence);
        if (e.sequence === 13) stream.stop();
      },
      sleep: immediateSleep,
      backoff: () => 0,
    });
    await stream.run();
    expect(delivered).toEqual([11, 12, 13]);
  });

  it("旧事件丢弃：流里混入 seq=9（≤快照）不投递", async () => {
    const delivered: number[] = [];
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => 10,
      openStream: async () => buildStream([frame(9), frame(11)]),
      onEvent: (e) => {
        delivered.push(e.sequence);
        if (e.sequence === 11) stream.stop();
      },
      sleep: immediateSleep,
      backoff: () => 0,
    });
    await stream.run();
    expect(delivered).toEqual([11]); // 9 被丢弃
  });

  it("resync：收到 stream.resync_required → 重取快照（cursor 10→20）后从 21 继续", async () => {
    const snaps = [10, 20];
    let snapCall = 0;
    let openCall = 0;
    const states: StreamState[] = [];
    const delivered: number[] = [];
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => snaps[Math.min(snapCall++, snaps.length - 1)],
      openStream: async () => {
        openCall++;
        if (openCall === 1) return buildStream([frame(11), `event: ${RESYNC_EVENT}\ndata: {}\n\n`]);
        return buildStream([frame(21)]); // 21 > 新快照 20
      },
      onEvent: (e) => {
        delivered.push(e.sequence);
        if (e.sequence === 21) stream.stop();
      },
      onState: (s) => states.push(s),
      sleep: immediateSleep,
      backoff: () => 0,
    });
    await stream.run();
    expect(delivered).toEqual([11, 21]);
    expect(snapCall).toBe(2); // 重取了快照
    expect(states).toContain("RESYNCING");
  });

  it("断线重连：首次 openStream 抛错 → RECONNECTING → 重连成功投递", async () => {
    const states: StreamState[] = [];
    const delivered: number[] = [];
    let openCall = 0;
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => 0,
      openStream: async () => {
        openCall++;
        if (openCall === 1) throw new Error("boom");
        return buildStream([frame(1)]);
      },
      onEvent: (e) => {
        delivered.push(e.sequence);
        stream.stop();
      },
      onState: (s) => states.push(s),
      sleep: immediateSleep,
      backoff: () => 0,
    });
    await stream.run();
    expect(delivered).toEqual([1]);
    expect(states).toContain("RECONNECTING");
    expect(states).toContain("LIVE");
  });

  it("连续失败进 STALE（仍重试）后恢复 LIVE", async () => {
    const states: StreamState[] = [];
    let openCall = 0;
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => 0,
      openStream: async () => {
        openCall++;
        if (openCall <= 4) throw new Error("down");
        return buildStream([frame(1)]);
      },
      onEvent: () => stream.stop(),
      onState: (s) => states.push(s),
      sleep: immediateSleep,
      backoff: () => 0,
      staleAfter: 3,
    });
    await stream.run();
    expect(states).toContain("STALE");
    expect(states).toContain("LIVE");
  });

  it("stop() 后不再重连", async () => {
    let openCall = 0;
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => 0,
      openStream: async () => {
        openCall++;
        return buildStream([frame(1)]);
      },
      onEvent: () => stream.stop(),
      sleep: immediateSleep,
      backoff: () => 0,
    });
    await stream.run();
    const callsAfter = openCall;
    await immediateSleep();
    expect(openCall).toBe(callsAfter); // 未再 open
  });
});
