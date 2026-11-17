// idempotency.ts —— 命令幂等核心（UI 文档 §16.33/§16.34 P0，不可后移）。纯逻辑可独立单测。
//
// 规则：
//  - 命令返回 202 + operation_id 只表示**已受理**，最终状态等事件或轮询（§16.33）；
//  - 网络超时用**同一 Idempotency-Key** 查询/重试，不生成第二份审批或 Job；
//  - 不 optimistic 把 Run 标 Paused / Finding 标 Confirmed——UI 等 operation 终态再反映。

export type CommandState = "IDLE" | "SUBMITTING" | "PENDING" | "DONE" | "FAILED";

// newIdempotencyKey 生成一次性幂等键（crypto.randomUUID 优先，回退时间+随机）。
export function newIdempotencyKey(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  return `idem-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export interface SubmitResult {
  operationId: string;
  // 后端可直接给终态，也可只给 202+pending（terminal 缺省=false）。
  terminal?: boolean;
  state?: string;
  resultRef?: string; // 终态时的结果对象引用（供跳转）
}
export interface OperationStatus {
  state: string; // PENDING/APPLIED/REJECTED/...
  terminal: boolean;
  resultRef?: string; // 结果对象引用（供跳转）
}

export interface CommandDeps {
  /** 提交命令，必须带 idempotencyKey（重试复用同一 key，后端据此去重）。 */
  submit: (idempotencyKey: string) => Promise<SubmitResult>;
  /** 轮询 operation 终态。 */
  poll: (operationId: string) => Promise<OperationStatus>;
  sleep?: (ms: number) => Promise<void>;
  pollInterval?: number;
  maxPolls?: number;
}

const defaultSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

// CommandRunner 驱动「提交→(202+operation_id)→轮询终态」，重试复用同一 Idempotency-Key。
export class CommandRunner {
  private key: string;
  private deps: Required<Pick<CommandDeps, "sleep" | "pollInterval" | "maxPolls">> & CommandDeps;
  state: CommandState = "IDLE";
  operationId: string | null = null;
  resultRef: string | null = null;
  error: string | null = null;

  constructor(deps: CommandDeps, key = newIdempotencyKey()) {
    this.key = key;
    this.deps = { sleep: deps.sleep ?? defaultSleep, pollInterval: deps.pollInterval ?? 500, maxPolls: deps.maxPolls ?? 20, ...deps };
  }

  get idempotencyKey(): string {
    return this.key;
  }

  // run 执行命令并等待终态。重试（再次调用 run）复用同一 key——后端据此不生成第二份。
  async run(): Promise<CommandState> {
    this.state = "SUBMITTING";
    this.error = null;
    let res: SubmitResult;
    try {
      res = await this.deps.submit(this.key);
    } catch (e) {
      this.state = "FAILED";
      this.error = "submit_failed";
      void e;
      return this.state;
    }
    this.operationId = res.operationId;
    if (res.terminal) {
      return this.settle(res.state ?? "APPLIED", res.resultRef);
    }
    // 202 + operation_id：轮询终态（不 optimistic）。
    this.state = "PENDING";
    for (let i = 0; i < this.deps.maxPolls; i++) {
      const st = await this.deps.poll(this.operationId);
      if (st.terminal) return this.settle(st.state, st.resultRef);
      await this.deps.sleep(this.deps.pollInterval);
    }
    this.state = "FAILED";
    this.error = "poll_timeout";
    return this.state;
  }

  private settle(state: string, resultRef?: string): CommandState {
    this.resultRef = resultRef ?? null;
    this.state = state === "REJECTED" || state === "FAILED" ? "FAILED" : "DONE";
    if (this.state === "FAILED") this.error = state;
    return this.state;
  }
}
