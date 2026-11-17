import { describe, it, expect } from "vitest";
import { StreamConsistency, RESYNC_EVENT } from "./consistency";
import type { EventEnvelope } from "../api/schema.gen";

function env(seq: number, type = "job.updated"): EventEnvelope {
  return { event_type: type, run_id: "run-1", sequence: seq };
}

describe("StreamConsistency（§16.33 快照+SSE 一致性）", () => {
  it("从快照 cursor+1 起按序投递，不丢不重", () => {
    const c = new StreamConsistency(10);
    const delivered: number[] = [];
    for (const s of [11, 12, 13]) {
      const a = c.ingest({ envelope: env(s), sequence: s });
      if (a.kind === "deliver") delivered.push(a.event.sequence);
    }
    expect(delivered).toEqual([11, 12, 13]);
    expect(c.cursor).toBe(13);
  });

  it("比当前快照旧的事件丢弃（stale），不回滚", () => {
    const c = new StreamConsistency(10);
    const a = c.ingest({ envelope: env(9), sequence: 9 });
    expect(a).toEqual({ kind: "discard", reason: "stale" });
    expect(c.cursor).toBe(10); // 未回退
  });

  it("边界：等于快照 cursor 的事件也丢弃（已含在快照内）", () => {
    const c = new StreamConsistency(10);
    expect(c.ingest({ envelope: env(10), sequence: 10 }).kind).toBe("discard");
  });

  it("重复/乱序回退事件丢弃（duplicate）", () => {
    const c = new StreamConsistency(0);
    expect(c.ingest({ envelope: env(1), sequence: 1 }).kind).toBe("deliver");
    expect(c.ingest({ envelope: env(2), sequence: 2 }).kind).toBe("deliver");
    const dup = c.ingest({ envelope: env(2), sequence: 2 });
    expect(dup).toEqual({ kind: "discard", reason: "duplicate" });
  });

  it("stream.resync_required → resync 决策", () => {
    const c = new StreamConsistency(5);
    expect(c.ingest({ event: RESYNC_EVENT }).kind).toBe("resync");
  });

  it("resetSnapshot 前移游标且不回滚（重取快照后）", () => {
    const c = new StreamConsistency(5);
    c.ingest({ envelope: env(6), sequence: 6 });
    c.resetSnapshot(20); // 新快照跳到 20
    expect(c.cursor).toBe(20);
    // 20 以内的旧事件此后都丢弃。
    expect(c.ingest({ envelope: env(15), sequence: 15 }).kind).toBe("discard");
    expect(c.ingest({ envelope: env(21), sequence: 21 }).kind).toBe("deliver");
  });

  it("resetSnapshot 给更小值时不回滚（取较大者）", () => {
    const c = new StreamConsistency(10);
    c.resetSnapshot(3);
    expect(c.cursor).toBe(10);
  });
});
