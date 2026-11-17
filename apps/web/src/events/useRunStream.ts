// useRunStream —— 把 ConsistentEventStream 接到 React + TanStack Query（UI 文档 §16.33）。
// 取快照用 getRunStatus 的 event_cursor；建流用 fetch（res.body 流）；事件经 EventRouter 失效 Query。
// 离开页面取消订阅、释放流（§16.33 有界 Buffer）。

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import { ConsistentEventStream, type StreamState } from "./stream";
import { EventRouter } from "./router";
import type { EventEnvelope } from "../api/schema.gen";

export interface UseRunStreamResult {
  state: StreamState;
  /** 有界实时 Buffer（最近 N 条事件，供活动流即时呈现）。 */
  recent: EventEnvelope[];
}

// fetch-based openStream：原生 EventSource 无法设首帧头，故用 fetch 读 res.body（§15.6）。
async function openFetchStream(runId: string, fromCursor: number, signal: AbortSignal): Promise<ReadableStream<Uint8Array>> {
  const res = await fetch(api.eventStreamUrl(runId, fromCursor), {
    credentials: "same-origin",
    headers: { accept: "text/event-stream" },
    signal,
  });
  if (!res.ok || !res.body) {
    throw new Error(`stream open failed: ${res.status}`);
  }
  return res.body;
}

export function useRunStream(runId: string, bufferSize = 200): UseRunStreamResult {
  const qc = useQueryClient();
  const [state, setState] = useState<StreamState>("CONNECTING");
  const [recent, setRecent] = useState<EventEnvelope[]>([]);
  const bufRef = useRef<EventEnvelope[]>([]);

  useEffect(() => {
    if (!runId) return;
    const router = new EventRouter((key) => qc.invalidateQueries({ queryKey: key }));
    const stream = new ConsistentEventStream({
      fetchSnapshotCursor: async () => (await api.getRunStatus(runId)).event_cursor,
      openStream: (from, signal) => openFetchStream(runId, from, signal),
      onState: setState,
      onEvent: (ev) => {
        router.dispatch(ev);
        // 有界 Buffer：只留最近 bufferSize 条。
        const next = [...bufRef.current, ev].slice(-bufferSize);
        bufRef.current = next;
        setRecent(next);
      },
    });
    void stream.run();
    return () => {
      stream.stop();
      bufRef.current = [];
    };
  }, [runId, qc, bufferSize]);

  return { state, recent };
}
