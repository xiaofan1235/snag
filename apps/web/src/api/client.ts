// API client（UI 文档 §15/§16.33）——基于 fetch 的类型化客户端。
// 安全（§16.33/§15.7）：同源、HttpOnly+SameSite=Strict 会话 Cookie（credentials:same-origin）；
// Token 不放 URL、不进 LocalStorage；错误不含秘密/原始响应。默认连本地 127.0.0.1:8787（§15.7）。

import type {
  Run,
  RunList,
  RunStatus,
  Job,
  JobList,
  EvidenceList,
  ReportList,
  CreateRunRequest,
  StartRunRequest,
  PauseRunRequest,
  Operation,
} from "./schema.gen";

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export interface ClientOptions {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
}

// 默认同源（生产 UI 与 daemon 同源）；开发可覆写 baseUrl。
const DEFAULT_BASE = "";

export class ApiClient {
  private base: string;
  private f: typeof fetch;

  constructor(opts: ClientOptions = {}) {
    this.base = opts.baseUrl ?? DEFAULT_BASE;
    this.f = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
  }

  private async req<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await this.f(this.base + path, {
      method,
      credentials: "same-origin",
      headers: body ? { "content-type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      let code = "http_error";
      let message = `HTTP ${res.status}`;
      try {
        const e = (await res.json()) as { code?: string; message?: string };
        if (e.code) code = e.code;
        if (e.message) message = e.message; // 服务端已脱敏；前端不再拼接原始响应
      } catch {
        /* 非 JSON 错误体忽略，不回显原文（防泄漏） */
      }
      throw new ApiError(res.status, code, message);
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  // ---- Run ----
  listRuns = () => this.req<RunList>("GET", "/v1/runs");
  getRun = (runId: string) => this.req<Run>("GET", `/v1/runs/${enc(runId)}`);
  getRunStatus = (runId: string) => this.req<RunStatus>("GET", `/v1/runs/${enc(runId)}/status`);
  createRun = (b: CreateRunRequest) => this.req<Run>("POST", "/v1/runs", b);
  startRun = (runId: string, b: StartRunRequest) => this.req<Run>("POST", `/v1/runs/${enc(runId)}/start`, b);
  pauseRun = (runId: string, b: PauseRunRequest) => this.req<Run>("POST", `/v1/runs/${enc(runId)}/pause`, b);
  resumeRun = (runId: string) => this.req<Run>("POST", `/v1/runs/${enc(runId)}/resume`);
  stopRun = (runId: string) => this.req<Run>("POST", `/v1/runs/${enc(runId)}/stop`);

  // ---- Job ----
  listRunJobs = (runId: string) => this.req<JobList>("GET", `/v1/runs/${enc(runId)}/jobs`);
  getJob = (jobId: string) => this.req<Job>("GET", `/v1/jobs/${enc(jobId)}`);

  // ---- Evidence / Report ----
  listRunEvidence = (runId: string, cursor?: string) =>
    this.req<EvidenceList>("GET", `/v1/runs/${enc(runId)}/evidence${cursor ? `?cursor=${enc(cursor)}` : ""}`);
  listRunReports = (runId: string) => this.req<ReportList>("GET", `/v1/runs/${enc(runId)}/reports`);

  // ---- Operation（幂等命令结果查询） ----
  getOperation = (operationId: string) => this.req<Operation>("GET", `/v1/operations/${enc(operationId)}`);

  // SSE 事件流 URL（供 fetch-based SSE 使用；原生 EventSource 无法设首帧头，§15.6）。
  eventStreamUrl = (runId: string, fromCursor: number) =>
    `${this.base}/v1/runs/${enc(runId)}/events?cursor=${fromCursor}`;
}

function enc(s: string): string {
  return encodeURIComponent(s);
}

// 进程内单例（生产同源）。测试可 new ApiClient({fetchImpl}) 注入。
export const api = new ApiClient();
