import { describe, it, expect } from "vitest";
import { affectedKeys, EventRouter, qk } from "./router";
import type { EventEnvelope } from "../api/schema.gen";

function ev(type: string): EventEnvelope {
  return { event_type: type, run_id: "run-1", sequence: 1 };
}

describe("event router", () => {
  it("所有事件都影响活动流", () => {
    const keys = affectedKeys(ev("job.updated"));
    expect(keys).toContainEqual([...qk.activity("run-1")]);
  });

  it("job.* 失效 jobs query", () => {
    const keys = affectedKeys(ev("job.updated"));
    expect(keys).toContainEqual([...qk.jobs("run-1")]);
  });

  it("finding.* 与 verification.* 都失效 findings query", () => {
    expect(affectedKeys(ev("finding.updated"))).toContainEqual([...qk.findings("run-1")]);
    expect(affectedKeys(ev("verification.updated"))).toContainEqual([...qk.findings("run-1")]);
  });

  it("无 run_id 事件不产生 key", () => {
    expect(affectedKeys({ event_type: "job.updated", sequence: 1 })).toEqual([]);
  });

  it("EventRouter.dispatch 对每个受影响 key 调 invalidate 一次（去重）", () => {
    const seen: unknown[][] = [];
    const r = new EventRouter((k) => seen.push([...k]));
    r.dispatch(ev("run.updated"));
    // run.* → run + runStatus，外加 activity，三个且不重复。
    expect(seen).toContainEqual([...qk.run("run-1")]);
    expect(seen).toContainEqual([...qk.runStatus("run-1")]);
    expect(seen).toContainEqual([...qk.activity("run-1")]);
    const uniq = new Set(seen.map((k) => JSON.stringify(k)));
    expect(uniq.size).toBe(seen.length);
  });
});
