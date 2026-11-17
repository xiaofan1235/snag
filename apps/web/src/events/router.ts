// router.ts —— SSE 事件 → 失效哪些 Query（UI 文档 §16.33：SSE 只更新其明确覆盖的 Query，
// 复杂聚合直接标记失效并重新查询）。保守策略：按事件类型映射到受影响的 queryKey 前缀。

import type { EventEnvelope } from "../api/schema.gen";

// queryKey 约定（与 queries.ts 对齐）。
export const qk = {
  run: (runId: string) => ["run", runId] as const,
  runStatus: (runId: string) => ["run", runId, "status"] as const,
  jobs: (runId: string) => ["run", runId, "jobs"] as const,
  evidence: (runId: string) => ["run", runId, "evidence"] as const,
  reports: (runId: string) => ["run", runId, "reports"] as const,
  findings: (runId: string) => ["run", runId, "findings"] as const,
  coverage: (runId: string) => ["run", runId, "coverage"] as const,
  activity: (runId: string) => ["run", runId, "activity"] as const,
};

// 事件类型前缀 → 受影响的 queryKey 工厂（给定 runId）。前缀匹配，覆盖同族事件。
const RULES: Array<{ prefix: string; keys: (runId: string) => ReadonlyArray<readonly unknown[]> }> = [
  { prefix: "run.", keys: (r) => [qk.run(r), qk.runStatus(r)] },
  { prefix: "job.", keys: (r) => [qk.jobs(r)] },
  { prefix: "attempt.", keys: (r) => [qk.jobs(r)] },
  { prefix: "finding.", keys: (r) => [qk.findings(r)] },
  { prefix: "verification.", keys: (r) => [qk.findings(r)] },
  { prefix: "evidence.", keys: (r) => [qk.evidence(r)] },
  { prefix: "coverage.", keys: (r) => [qk.coverage(r)] },
  { prefix: "report.", keys: (r) => [qk.reports(r)] },
  { prefix: "activity.", keys: (r) => [qk.activity(r)] },
  { prefix: "budget.", keys: (r) => [qk.runStatus(r)] },
];

// affectedKeys 返回某事件应失效的 queryKey 列表（活动流事件总是影响 activity）。
export function affectedKeys(ev: EventEnvelope): readonly unknown[][] {
  const runId = ev.run_id ?? "";
  if (!runId) return [];
  const keys: unknown[][] = [[...qk.activity(runId)]]; // 所有事件都进活动流
  for (const rule of RULES) {
    if (ev.event_type.startsWith(rule.prefix)) {
      for (const k of rule.keys(runId)) keys.push([...k]);
    }
  }
  // 去重（同一 key 只失效一次）。
  const seen = new Set<string>();
  return keys.filter((k) => {
    const s = JSON.stringify(k);
    if (seen.has(s)) return false;
    seen.add(s);
    return true;
  });
}

// EventRouter 把事件派发给 invalidate 回调（由 React 层接 queryClient.invalidateQueries）。
export class EventRouter {
  constructor(private invalidate: (key: readonly unknown[]) => void) {}
  dispatch(ev: EventEnvelope): void {
    for (const key of affectedKeys(ev)) this.invalidate(key);
  }
}
