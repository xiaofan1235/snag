import { describe, it, expect, vi } from "vitest";
import { CommandRunner, newIdempotencyKey, type CommandDeps } from "./idempotency";

const immediateSleep = () => Promise.resolve();

describe("newIdempotencyKey", () => {
  it("生成非空且唯一的键", () => {
    const a = newIdempotencyKey();
    const b = newIdempotencyKey();
    expect(a).toBeTruthy();
    expect(a).not.toBe(b);
  });
});

describe("CommandRunner（§16.33/§16.34 命令幂等）", () => {
  it("202+operation_id → 轮询到 APPLIED 终态（不 optimistic）", async () => {
    const poll = vi
      .fn()
      .mockResolvedValueOnce({ state: "PENDING", terminal: false })
      .mockResolvedValueOnce({ state: "APPLIED", terminal: true, resultRef: "run-1" });
    const deps: CommandDeps = {
      submit: async () => ({ operationId: "op-1", terminal: false, state: "ACCEPTED" }),
      poll,
      sleep: immediateSleep,
    };
    const r = new CommandRunner(deps);
    const final = await r.run();
    expect(final).toBe("DONE");
    expect(r.resultRef).toBe("run-1");
    expect(poll).toHaveBeenCalledTimes(2);
  });

  it("重试复用同一 Idempotency-Key（后端据此不生成第二份）", async () => {
    const keys: string[] = [];
    const deps: CommandDeps = {
      submit: async (key) => {
        keys.push(key);
        // 第一次「超时」→FAILED；第二次成功。
        if (keys.length === 1) throw new Error("timeout");
        return { operationId: "op-1", terminal: true, state: "APPLIED", };
      },
      poll: async () => ({ state: "APPLIED", terminal: true }),
      sleep: immediateSleep,
    };
    const r = new CommandRunner(deps);
    const first = await r.run();
    expect(first).toBe("FAILED");
    const second = await r.run();
    expect(second).toBe("DONE");
    // 两次提交用的是同一个 key。
    expect(keys[0]).toBe(keys[1]);
  });

  it("后端直接给终态（无需轮询）", async () => {
    const poll = vi.fn();
    const deps: CommandDeps = {
      submit: async () => ({ operationId: "op-1", terminal: true, state: "APPLIED", resultRef: "x" }),
      poll,
      sleep: immediateSleep,
    };
    const r = new CommandRunner(deps);
    expect(await r.run()).toBe("DONE");
    expect(poll).not.toHaveBeenCalled();
  });

  it("REJECTED → FAILED 且记录原因", async () => {
    const deps: CommandDeps = {
      submit: async () => ({ operationId: "op-1", terminal: true, state: "REJECTED" }),
      poll: async () => ({ state: "REJECTED", terminal: true }),
      sleep: immediateSleep,
    };
    const r = new CommandRunner(deps);
    expect(await r.run()).toBe("FAILED");
    expect(r.error).toBe("REJECTED");
  });

  it("轮询超时 → FAILED(poll_timeout)", async () => {
    const deps: CommandDeps = {
      submit: async () => ({ operationId: "op-1", terminal: false }),
      poll: async () => ({ state: "PENDING", terminal: false }),
      sleep: immediateSleep,
      maxPolls: 3,
    };
    const r = new CommandRunner(deps);
    expect(await r.run()).toBe("FAILED");
    expect(r.error).toBe("poll_timeout");
  });
});
