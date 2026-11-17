// stream.ts —— 一致性事件流编排（UI 文档 §16.33）。把「取快照→建流→补发→丢旧→断线退避→resync 重取」
// 串成一个可取消的循环。依赖全部注入，便于在无真实服务端下单测。
//
// 状态（§16.33）：CONNECTING → LIVE；断线 RECONNECTING（指数退避）；长时间连不上 STALE；
// 收到 stream.resync_required → RESYNCING 重取快照。浏览器离线只影响本流，不代表 Run 已停止。

import type { EventEnvelope } from "../api/schema.gen";
import { createSSEParser } from "./sse";
import { StreamConsistency, RESYNC_EVENT } from "./consistency";

export type StreamState = "CONNECTING" | "LIVE" | "RECONNECTING" | "STALE" | "RESYNCING";

export interface StreamDeps {
  /** 取快照，返回其 event_cursor。 */
  fetchSnapshotCursor: () => Promise<number>;
  /** 从 fromCursor+1 建流（fetch-based），返回字节流。 */
  openStream: (fromCursor: number, signal: AbortSignal) => Promise<ReadableStream<Uint8Array>>;
  onEvent: (ev: EventEnvelope) => void;
  onState?: (s: StreamState) => void;
  /** 退避毫秒（attempt 从 1 起）。默认指数 + 上限 30s。 */
  backoff?: (attempt: number) => number;
  sleep?: (ms: number) => Promise<void>;
  /** 连续失败多少次后进 STALE（仍继续重试）。默认 4。 */
  staleAfter?: number;
}

const defaultBackoff = (attempt: number) => Math.min(1000 * 2 ** (attempt - 1), 30_000);
const defaultSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export class ConsistentEventStream {
  private deps: Required<Pick<StreamDeps, "backoff" | "sleep" | "staleAfter">> & StreamDeps;
  private ctrl: AbortController | null = null;
  private stopped = false;
  private state: StreamState = "CONNECTING";
  private consistency: StreamConsistency | null = null;

  constructor(deps: StreamDeps) {
    this.deps = {
      backoff: deps.backoff ?? defaultBackoff,
      sleep: deps.sleep ?? defaultSleep,
      staleAfter: deps.staleAfter ?? 4,
      ...deps,
    };
  }

  getState(): StreamState {
    return this.state;
  }

  cursor(): number {
    return this.consistency?.cursor ?? 0;
  }

  private setState(s: StreamState) {
    if (this.state !== s) {
      this.state = s;
      this.deps.onState?.(s);
    }
  }

  // run 启动主循环（await 到 stop）。先取快照建立一致性游标，再进入连接循环。
  async run(): Promise<void> {
    this.setState("CONNECTING");
    const cursor = await this.deps.fetchSnapshotCursor();
    this.consistency = new StreamConsistency(cursor);
    await this.loop();
  }

  stop(): void {
    this.stopped = true;
    this.ctrl?.abort();
  }

  private async loop(): Promise<void> {
    let failures = 0;
    while (!this.stopped) {
      this.ctrl = new AbortController();
      try {
        const stream = await this.deps.openStream(this.cursor(), this.ctrl.signal);
        this.setState("LIVE");
        failures = 0;
        const resyncNeeded = await this.consume(stream);
        if (this.stopped) return;
        if (resyncNeeded) {
          await this.resync();
          continue;
        }
        // 流正常结束（服务端关闭）——视为断线，退避重连。
      } catch (err) {
        if (this.stopped) return;
        void err; // 错误不回显（可能含不可信内容，§16.33）
      }
      if (this.stopped) return;
      failures++;
      this.setState(failures >= this.deps.staleAfter ? "STALE" : "RECONNECTING");
      await this.deps.sleep(this.deps.backoff(failures));
    }
  }

  // consume 读流、解析 SSE、喂一致性核心；返回是否需要 resync。
  private async consume(stream: ReadableStream<Uint8Array>): Promise<boolean> {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    const parser = createSSEParser();
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) return false;
        const frames = parser.feed(decoder.decode(value, { stream: true }));
        for (const f of frames) {
          if (f.event === RESYNC_EVENT) return true;
          let env: EventEnvelope | undefined;
          try {
            env = JSON.parse(f.data) as EventEnvelope;
          } catch {
            continue; // 坏帧丢弃，不回显
          }
          const action = this.consistency!.ingest({ event: f.event, envelope: env, sequence: env.sequence });
          if (action.kind === "resync") return true;
          if (action.kind === "deliver") this.deps.onEvent(action.event);
          // discard: 静默丢弃（stale/duplicate）
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  private async resync(): Promise<void> {
    this.setState("RESYNCING");
    const cursor = await this.deps.fetchSnapshotCursor();
    this.consistency!.resetSnapshot(cursor);
  }
}
