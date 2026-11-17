package network

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/policy"
)

// PolicyResolver 按 run 载入其当前 Policy（Proxy 逐请求回查用）。
type PolicyResolver func(ctx context.Context, runID domain.ID) (policy.Policy, error)

// EvidenceRecord 是 Proxy 在咽喉处留下的原始请求/响应证据（§13.2）。Day 20 接内容寻址 Blob。
type EvidenceRecord struct {
	CapabilityID domain.ID
	URL          string
	Method       string
	Status       int
	RespBytes    int
	Allowed      bool
	BlockCode    string
	CapturedAt   time.Time
}

// EvidenceSink 收取证据。
type EvidenceSink interface {
	Capture(rec EvidenceRecord)
}

// MemEvidence 是内存证据汇（Day 18 默认，Day 20 换内容寻址加密存储）。
type MemEvidence struct {
	mu   sync.Mutex
	recs []EvidenceRecord
}

func NewMemEvidence() *MemEvidence { return &MemEvidence{} }
func (m *MemEvidence) Capture(rec EvidenceRecord) {
	m.mu.Lock()
	m.recs = append(m.recs, rec)
	m.mu.Unlock()
}
func (m *MemEvidence) Records() []EvidenceRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EvidenceRecord, len(m.recs))
	copy(out, m.recs)
	return out
}

// EpochCache 是进程内权威缓存的骨架（§31 P1x）：撤销代际 + ready 门。重启必从 DB 重建、
// 重建前 fail-closed（§9.0.3）。Day 18 先立骨架与 fail-closed 语义；O(1) permit 快照随热路径接入。
type EpochCache struct {
	ready atomic.Bool
	epoch atomic.Int64
}

func NewEpochCache() *EpochCache { return &EpochCache{} }

// Rebuild 标记缓存已从 DB 重建完成，放开 fail-closed。
func (c *EpochCache) Rebuild() { c.ready.Store(true) }

// Ready 报告是否已重建（false=fail-closed 拦截一切）。
func (c *EpochCache) Ready() bool { return c.ready.Load() }

// BumpEpoch 撤销发生时递增代际（使相关缓存项作废，回落 DB 复查）。
func (c *EpochCache) BumpEpoch() int64 { return c.epoch.Add(1) }
func (c *EpochCache) Epoch() int64     { return c.epoch.Load() }

// Proxy 是逐请求策略执法点 + 证据产生点。
type Proxy struct {
	permits     *execution.Permits
	policies    PolicyResolver
	evidence    EvidenceSink
	cache       *EpochCache
	client      *http.Client
	egressGuard bool        // 开启则私网/元数据地址一律拒出网（云部署防 SSRF/DNS 重绑定，默认关）
	killed      atomic.Bool // Kill Switch：熔断后 revoke_all_target_egress，逐请求一律 fail-closed（§9.7）
}

// RevokeAllTargetEgress 拉下 Kill Switch（§9.7）：之后所有出网请求一律拒绝（SYSTEM_DEGRADED），
// 并 bump 代际使在途缓存全部作废。由安全熔断器在绊线超阈时触发，宁可停机不可带病放行。幂等。
func (p *Proxy) RevokeAllTargetEgress() {
	p.killed.Store(true)
	p.cache.BumpEpoch()
}

// Degraded 报告是否已被 Kill Switch 置为降级态。
func (p *Proxy) Degraded() bool { return p.killed.Load() }

// Option 配置 Proxy。
type Option func(*Proxy)

// WithEgressGuard 开启私网 egress 守卫：即便在 Scope 内，私有/环回/元数据地址也拒出网。
// 默认关——授权内网测试的目标可能本就是私网，由 Scope 把关；开启适合云部署的纵深防御。
func WithEgressGuard() Option { return func(p *Proxy) { p.egressGuard = true } }

// NewProxy 构造 Proxy。client 关闭自动重定向（重定向由逐请求 Scope 回查把关）。
func NewProxy(permits *execution.Permits, resolver PolicyResolver, evidence EvidenceSink, cache *EpochCache, opts ...Option) *Proxy {
	p := &Proxy{
		permits:  permits,
		policies: resolver,
		evidence: evidence,
		cache:    cache,
		client: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// CheckResult 是一次逐请求判定。
type CheckResult struct {
	Allowed bool
	Code    string
	Reason  string
}

func blockR(code, reason string) CheckResult { return CheckResult{Code: code, Reason: reason} }
func allowR() CheckResult                    { return CheckResult{Allowed: true} }

// CheckRequest 对一个在线请求做完整逐请求回查（§9.5，不信任 Runner 快照）：
// ① 缓存未重建 → fail-closed；② Capability 整条链存活（Permit/Attempt 当代/Run 可执行/Policy 未取代/未撤销）；
// ③ 载入 Run 的 Policy；④ 网络层 Scope+授权窗+Method（与 PDP 同源）；⑤ 私网 egress 守卫；
// ⑥ 原子消耗一个子请求配额（放最后，被拦的请求不耗预算）。
func (p *Proxy) CheckRequest(ctx context.Context, capID domain.ID, rawURL, method string, endpointVerified bool, now time.Time) CheckResult {
	if p.killed.Load() {
		return blockR("system_degraded", "Kill Switch 已触发，全部出网已撤销（§9.7）")
	}
	if !p.cache.Ready() {
		return blockR("proxy_not_ready", "缓存重建前 fail-closed（§9.0.3）")
	}
	ch, err := p.permits.ValidateCapabilityLive(ctx, capID)
	if err != nil {
		return blockR("capability_not_found", "capability 不存在或读取失败")
	}
	if !ch.Live() {
		return blockR("capability_not_live", "Permit 撤销/越代/过期/Run 不可执行/策略被取代/配额用尽")
	}
	pol, err := p.policies(ctx, ch.RunID)
	if err != nil {
		return blockR("policy_load_failed", "载入 Run 策略失败")
	}
	if d := policy.CheckNetwork(pol, rawURL, method, endpointVerified, now); !d.IsAllowed() {
		return blockR(d.Code, d.Reason)
	}
	if p.egressGuard {
		if c, cerr := policy.Canonicalize(rawURL); cerr == nil && isPrivateHost(c.Host) {
			return blockR("egress_private_ip", "目标为私有/环回/元数据地址，拒绝出网")
		}
	}
	if err := p.permits.ConsumeSubrequest(ctx, capID); err != nil {
		return blockR("capability_exhausted", "子请求超 max_subrequests 或 capability 已撤销")
	}
	return allowR()
}

// ServeHTTP 是 HTTP 转发代理：逐请求回查 → 放行则转发到目标、原始留证、按 Scope 回查重定向。
// Runner 用 X-Snag-Capability 头标明本请求所属 Capability。HTTPS CONNECT/MITM 与 CA 安装随 Day 19
// 沙箱、TLS 指纹对齐随 Day 31；本版先把 HTTP 咽喉与校验链打通。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	capID := domain.ID(r.Header.Get("X-Snag-Capability"))
	target := r.URL.String()

	res := p.CheckRequest(ctx, capID, target, r.Method, true, time.Now())
	if !res.Allowed {
		p.evidence.Capture(EvidenceRecord{CapabilityID: capID, URL: target, Method: r.Method, Allowed: false, BlockCode: res.Code, CapturedAt: time.Now()})
		http.Error(w, "blocked: "+res.Code+": "+res.Reason, http.StatusForbidden)
		return
	}

	outReq, err := http.NewRequestWithContext(ctx, r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "bad target", http.StatusBadGateway)
		return
	}
	copyHeaders(outReq.Header, r.Header)
	outReq.Header.Del("X-Snag-Capability")

	resp, err := p.client.Do(outReq)
	if err != nil {
		p.evidence.Capture(EvidenceRecord{CapabilityID: capID, URL: target, Method: r.Method, Allowed: true, BlockCode: "upstream_error", CapturedAt: time.Now()})
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 越域重定向：目标回 3xx 且 Location 出界 → 拦截，要求重新授权（§9.5）。
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			if pol, perr := p.policies(ctx, resolveRun(p, ctx, capID)); perr == nil {
				if in, _ := RedirectInScope(target, loc, pol.Target); !in {
					p.evidence.Capture(EvidenceRecord{CapabilityID: capID, URL: target, Method: r.Method, Status: resp.StatusCode, Allowed: false, BlockCode: "redirect_out_of_scope", CapturedAt: time.Now()})
					http.Error(w, "blocked: redirect_out_of_scope: 重定向目标越域，须重新授权", http.StatusForbidden)
					return
				}
			}
		}
	}

	body, _ := io.ReadAll(resp.Body)
	p.evidence.Capture(EvidenceRecord{CapabilityID: capID, URL: target, Method: r.Method, Status: resp.StatusCode, RespBytes: len(body), Allowed: true, CapturedAt: time.Now()})

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func resolveRun(p *Proxy, ctx context.Context, capID domain.ID) domain.ID {
	ch, err := p.permits.ValidateCapabilityLive(ctx, capID)
	if err != nil {
		return ""
	}
	return ch.RunID
}

var hopHeaders = map[string]bool{
	"Connection": true, "Proxy-Connection": true, "Keep-Alive": true,
	"Proxy-Authenticate": true, "Proxy-Authorization": true, "Te": true,
	"Trailer": true, "Transfer-Encoding": true, "Upgrade": true,
}

func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		if hopHeaders[k] {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
