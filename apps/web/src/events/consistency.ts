// consistency.ts —— 快照 + SSE 一致性核心（UI 文档 §16.33）。纯决策，无网络/无 React，可独立单测。
//
// 规则（§16.33）：
//  - 页面先取快照，记 event_cursor；建流从 cursor+1 补发；
//  - 收到比当前快照旧的事件（sequence ≤ snapshotCursor）必须丢弃，不回滚 UI；
//  - 重复/乱序（sequence ≤ lastDelivered）丢弃，保证不丢不重；
//  - 控制事件 `stream.resync_required` → 停止应用增量，要求重取快照；
//  - 顺序推进 lastDelivered；补发无空洞由服务端从 cursor+1 连续发保证，客户端校验连续性可选。

import type { EventEnvelope } from "../api/schema.gen";

export const RESYNC_EVENT = "stream.resync_required";

export type IngestAction =
  | { kind: "deliver"; event: EventEnvelope }
  | { kind: "discard"; reason: "stale" | "duplicate" | "control" }
  | { kind: "resync" };

export interface IngestFrame {
  event?: string;
  sequence?: number;
  envelope?: EventEnvelope;
}

// StreamConsistency 跟踪快照游标与已投递序号，对每个进入的帧给出 deliver/discard/resync 决策。
export class StreamConsistency {
  private snapshotCursor: number;
  private lastDelivered: number;

  constructor(snapshotCursor: number) {
    this.snapshotCursor = snapshotCursor;
    this.lastDelivered = snapshotCursor;
  }

  // 重取快照后调用：把游标前移到新快照（新快照 ≥ 旧值）。resync 完成的收尾。
  resetSnapshot(cursor: number): void {
    // 快照只会前进；若后端给了更小值（异常），取较大者，避免回滚。
    this.snapshotCursor = Math.max(this.snapshotCursor, cursor);
    this.lastDelivered = Math.max(this.lastDelivered, this.snapshotCursor);
  }

  get cursor(): number {
    return this.lastDelivered;
  }

  // ingest 处理一个帧。控制事件 resync 优先；其后按序号判定 stale/duplicate/deliver。
  ingest(frame: IngestFrame): IngestAction {
    if (frame.event === RESYNC_EVENT) {
      return { kind: "resync" };
    }
    const env = frame.envelope;
    const seq = frame.sequence ?? env?.sequence;
    if (seq == null) {
      // 无序号的普通事件无法定位，保守丢弃（不回滚 UI）。
      return { kind: "discard", reason: "control" };
    }
    if (seq <= this.snapshotCursor) {
      return { kind: "discard", reason: "stale" }; // 比当前快照旧
    }
    if (seq <= this.lastDelivered) {
      return { kind: "discard", reason: "duplicate" }; // 重复/乱序回退
    }
    this.lastDelivered = seq;
    if (!env) {
      // 有序号但无 envelope（理论不该发生）；推进游标但不投递。
      return { kind: "discard", reason: "control" };
    }
    return { kind: "deliver", event: env };
  }
}
