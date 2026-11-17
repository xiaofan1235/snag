// mockWorkbench.ts —— Run 工作台各页预览夹具（daemon 接线前）。真实数据随 daemon 接线替换。

import type {
  StrategyModel, JobsModel, HypothesesModel, SurfaceModel, TrafficModel,
  EvidenceModel, BrowserModel, ToolsModel, FilesModel, TimelineModel,
} from "../features/workbench";

export const mkStrategy: StrategyModel = {
  current: {
    id: "ep-3", epoch: 3, trigger: "高价值发现（IDOR）", createdAt: "11:58", snapshotRevision: 7,
    priorityDirection: "authz（对象归属）", pausedDirections: ["recon（已充分）"], noProgressCount: 0,
    lastReframe: "从 recon 转向 authz（发现跨身份读取线索）",
    patches: [
      { id: "patch-12", createdAt: "11:58", ops: [
        { kind: "add_hypothesis", summary: "账号A 可读账号B 订单", result: "ACCEPTED" },
        { kind: "create_job", summary: "EXPERIMENT /api/orders IDOR 变体", result: "ACCEPTED" },
        { kind: "create_job", summary: "重复的 recon Job", result: "DEDUPED", reason: "同指纹已存在" },
        { kind: "request_approval", summary: "DELETE /api/orders/42 写操作", result: "NEEDS_APPROVAL", reason: "写操作红线" },
        { kind: "reprioritize", summary: "降低 sqli 方向", result: "REJECTED", reason: "方向未停滞，不满足转向阶梯" },
      ] },
    ],
  },
  history: [
    { id: "ep-1", epoch: 1, trigger: "Run 启动" },
    { id: "ep-2", epoch: 2, trigger: "攻击面扩展（新端点）" },
  ],
};

export const mkJobs: JobsModel = {
  jobs: [
    { id: "job-1", kind: "Strategist", module: "-", state: "RUNNING", attempt: 1, priority: 100, budget: "∞", reason: "Run 核心", spec: "strategist.plan", progress: "Epoch 3 规划中", recentEvents: ["创建 patch-12"] },
    { id: "job-2", kind: "Experiment", module: "order", state: "RUNNING", attempt: 2, currentAction: "GET /api/orders/42 as A", priority: 80, budget: "120/200 req", lease: "02:31", reason: "验证 IDOR 假设", spec: "experiment.idor", inputEvidence: ["ev-1"], progress: "变体 2/3", subJobs: [] },
    { id: "job-3", kind: "Verifier", module: "order", state: "VERIFYING", attempt: 1, priority: 70, budget: "40 req", reason: "独立验证 f-1", spec: "verify.replay", inputEvidence: ["ev-1"], progress: "重放中" },
    { id: "job-4", kind: "Recon", module: "auth", state: "DONE", attempt: 1, priority: 30, budget: "50/50 req", reason: "攻击面发现", spec: "recon.crawl", progress: "完成，发现 8 端点" },
    { id: "job-5", kind: "Module", module: "payment", state: "BLOCKED", attempt: 1, currentAction: "等待审批", priority: 60, budget: "0 req", reason: "支付模块探测", failure: "待审批：写操作", spec: "module.probe" },
  ],
};

export const mkHyp: HypothesesModel = {
  items: [
    { id: "h-1", column: "CONFIRMED", statement: "账号A 可读账号B 订单（IDOR）", trustBoundary: "对象归属", expectedImpact: "跨租户数据泄漏", falsifier: "403 或返回空", currentEvidence: "ev-1（200 + B 的数据）", minimumExperiment: "A 请求 B 的 order id", stopCondition: "确认或 403" },
    { id: "h-2", column: "TESTING", statement: "订单枚举可遍历全量", trustBoundary: "速率/配额", expectedImpact: "批量泄漏", falsifier: "命中速率限制", currentEvidence: "部分 200", minimumExperiment: "小步枚举 10 个 id", stopCondition: "命中限流或 100 个" },
    { id: "h-3", column: "UNTESTED", statement: "登录接口存在用户枚举", trustBoundary: "认证", expectedImpact: "用户名泄漏", falsifier: "统一错误信息", currentEvidence: "无", minimumExperiment: "对比存在/不存在用户响应", stopCondition: "差异或无" },
    { id: "h-4", column: "REDLINE_BLOCKED", statement: "删除订单验证写入 IDOR", trustBoundary: "对象归属+写", expectedImpact: "数据破坏", falsifier: "403", currentEvidence: "无（待审批）", minimumExperiment: "DELETE B 的 order（需审批）", stopCondition: "审批结果" },
  ],
};

export const mkSurface: SurfaceModel = {
  surfaceRevision: 7,
  endpoints: [
    { id: "e1", module: "order", method: "GET", url: "/api/orders/{id}", authType: "Bearer", statusCode: 200, source: "crawl", hasParams: true, hasFinding: true, covered: true, inScope: true },
    { id: "e2", module: "order", method: "DELETE", url: "/api/orders/{id}", authType: "Bearer", statusCode: 200, source: "openapi", hasParams: true, hasFinding: false, covered: false, inScope: true },
    { id: "e3", module: "auth", method: "POST", url: "/api/login", authType: "none", statusCode: 200, source: "crawl", hasParams: true, hasFinding: false, covered: false, inScope: true },
    { id: "e4", module: "payment", method: "POST", url: "/api/pay/charge", authType: "Bearer", statusCode: 200, source: "js", hasParams: true, hasFinding: false, covered: false, inScope: true },
    { id: "e5", module: "external", method: "GET", url: "https://cdn.other.com/asset", authType: "none", statusCode: 200, source: "js", hasParams: false, hasFinding: false, covered: false, inScope: false },
  ],
  discoveries: [
    { id: "d1", kind: "身份差异", detail: "账号A 对 /api/orders/42 返回 200（预期 403）", source: "ev-1", confidence: "high", stale: false, followState: "open" },
    { id: "d2", kind: "JS 引用", detail: "发现 /api/v2/orders 未在 OpenAPI", source: "runtime.js", confidence: "medium", stale: false, followState: "open" },
    { id: "d3", kind: "被策略阻止", detail: "DELETE /api/orders/42 方向被红线阻止，待审批", source: "policy", confidence: "high", stale: false, followState: "following" },
  ],
};

export const mkTraffic: TrafficModel = {
  exchanges: [
    {
      id: "x1", method: "GET", host: "acme.com", path: "/api/orders/42", status: 200, identity: "账号A", jobId: "job-2",
      hasEvidence: true, hasSecretMark: true,
      request: { startLine: "GET /api/orders/42 HTTP/1.1", headers: [["Host", "acme.com"], ["Authorization", "Bearer ***REDACTED***"]] },
      response: { startLine: "HTTP/1.1 200 OK", headers: [["Content-Type", "application/json"], ["X-Owner", "B"]], body: '{"id":42,"owner":"B","total":199}' },
      relatedAction: "act-1", relatedPermit: "perm-1", relatedEvidence: "ev-1", relatedFinding: "f-1",
    },
    {
      id: "x2", method: "GET", host: "acme.com", path: "/api/orders/7", status: 403, identity: "账号A", jobId: "job-2",
      hasEvidence: true, hasSecretMark: false,
      request: { startLine: "GET /api/orders/7 HTTP/1.1", headers: [["Host", "acme.com"]] },
      response: { startLine: "HTTP/1.1 403 Forbidden", headers: [["Content-Type", "application/json"]], body: '{"error":"forbidden"}' },
      relatedAction: "act-2", relatedEvidence: "ev-2",
    },
  ],
};

export const mkEvidence: EvidenceModel = {
  items: [
    {
      id: "ev-1", kind: "http_response", capturedAt: "11:57", sha256: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
      identity: "账号A", policyRevision: 3, linkedFinding: "f-1",
      redacted: {
        request: "GET /api/orders/42\nAuthorization: Bearer ***REDACTED***",
        response: 'HTTP/1.1 200 OK\nX-Owner: B\n\n{"id":42,"owner":"B","total":199}',
        metadata: "proxy_lease=lease-1 captured_at=11:57",
      },
      meta: { proxy_lease: "lease-1", size: "412 B" },
    },
    {
      id: "ev-2", kind: "http_response", capturedAt: "11:58", sha256: "ffee...", identity: "账号A", policyRevision: 3,
      redacted: { response: "HTTP/1.1 403 Forbidden" }, meta: { size: "88 B" },
    },
  ],
};

export const mkBrowser: BrowserModel = {
  sessions: [
    {
      id: "bs-1", identity: "账号A", container: "sbx-12", state: "RUNNING", currentPage: "https://acme.com/orders", createdAt: "11:50", lastAction: "点击「我的订单」",
      screenshots: [{ actionId: "act-10", caption: "订单列表" }, { actionId: "act-11", caption: "订单详情 42" }],
      domSnapshot: "<main>\n  <h1>我的订单</h1>\n  <ul>…订单 42（owner B）…</ul>\n</main>",
      network: [{ method: "GET", url: "/api/orders/42", status: 200 }, { method: "GET", url: "/api/profile", status: 200 }],
      console: ["[warn] mixed content blocked", "[info] app loaded"],
      cookies: [{ name: "session", scope: "acme.com", expires: "12:50", redactedSummary: "sess_***（HttpOnly）" }],
    },
  ],
};

export const mkTools: ToolsModel = {
  actions: [
    { id: "act-1", tool: "http", policyDecision: "allow", exitCode: 0, stdout: "200 OK · 412 bytes · X-Owner: B", stderr: "" },
    { id: "act-2", tool: "http", policyDecision: "allow", exitCode: 0, stdout: "403 Forbidden", stderr: "" },
  ],
  timeline: [
    { seq: 1, tool: "http", durationMs: 120, retries: 0, degraded: false },
    { seq: 2, tool: "http", durationMs: 95, retries: 0, degraded: false },
    { seq: 3, tool: "diff", durationMs: 8, retries: 0, degraded: false },
  ],
  terminal: "$ probe /api/orders/42 as A\n200 OK\n{\"id\":42,\"owner\":\"B\"}\n$ probe /api/orders/7 as A\n403 Forbidden",
  errors: [
    { code: "TIMEOUT", count: 1, lastDetail: "provider 调用超时（已重试成功）" },
    { code: "OUTCOME_UNKNOWN", count: 0, lastDetail: "—" },
  ],
};

export const mkFiles: FilesModel = {
  files: [
    { path: "runs/run-1/poc/idor.http", type: "http", sizeBytes: 412, sha256: "a1b2…", sourceJob: "job-2", createdAt: "11:57", retention: "run", sensitive: true },
    { path: "runs/run-1/screenshots/order-42.png", type: "image", sizeBytes: 83920, sha256: "c3d4…", sourceJob: "job-2", createdAt: "11:57", retention: "run", sensitive: true },
    { path: "runs/run-1/report/report.md", type: "markdown", sizeBytes: 2048, sha256: "e5f6…", sourceJob: "job-6", createdAt: "12:10", retention: "run", sensitive: false },
  ],
  resources: [
    { id: "res-1", name: "授权书.pdf", scope: "acme.com", tags: ["授权"], sha256: "11aa…", uploader: "owner", expires: "2026-12-31", isSecret: false },
    { id: "res-2", name: "测试账号A cookie", scope: "acme.com", tags: ["identity"], sha256: "22bb…", uploader: "owner", expires: "12:50", isSecret: true },
  ],
};

export const mkTimeline: TimelineModel = {
  timeline: [
    { at: "11:50", kind: "strategy", summary: "Epoch 1 启动", key: true },
    { at: "11:55", kind: "job", summary: "Recon Job 完成，发现 8 端点" },
    { at: "11:57", kind: "action", summary: "GET /api/orders/42 as A → 200" },
    { at: "11:57", kind: "evidence", summary: "ev-1 捕获（200 + B 的数据）", key: true },
    { at: "11:58", kind: "finding", summary: "f-1 候选：IDOR on /orders", key: true },
    { at: "11:59", kind: "approval", summary: "DELETE /api/orders/42 待审批", key: true },
    { at: "12:10", kind: "finding", summary: "f-1 升 REPORTABLE（Verifier 确认）", key: true },
  ],
  audit: [
    { at: "11:50", actor: "system", command: "run.create", ref: "run-1", policyRevision: 1, idempotencyKey: "k1", result: "OK", auditHash: "0000anchor…" },
    { at: "11:57", actor: "agent", command: "action.execute", ref: "act-1", policyRevision: 3, idempotencyKey: "k2", result: "OK", auditHash: "a1b2c3d4e5f6…" },
    { at: "11:59", actor: "user", command: "approval.request", ref: "ap-1", policyRevision: 3, idempotencyKey: "k3", result: "PENDING", auditHash: "b2c3d4e5f6…", flagged: true },
    { at: "12:10", actor: "verifier", command: "finding.promote", ref: "f-1", policyRevision: 3, idempotencyKey: "k4", result: "REPORTABLE", auditHash: "c3d4e5f6…", flagged: true },
  ],
};
