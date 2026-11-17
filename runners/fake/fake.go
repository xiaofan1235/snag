// Package fake 是不碰真目标的 Fake Runner（§25）：收到动作后经 Policy Proxy 打 Mock Target、回回执。
// 用于无 AI 时测通「提案→Permit→Proxy→Runner→回执」执行链。
package fake

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"pentagent/internal/canon"
	"pentagent/internal/network"
	"pentagent/internal/sandbox"
	"pentagent/runners"
)

// Runner 是 Fake Runner。持沙箱管理器以做 attestation 门禁。
type Runner struct {
	sb *sandbox.Manager
}

// New 构造 Fake Runner。
func New(sb *sandbox.Manager) *Runner { return &Runner{sb: sb} }

// Execute 经 EgressLease 指向的 Proxy 发起目标请求，回回执。
func (r *Runner) Execute(ctx context.Context, a runners.Action, lease network.EgressLease) (runners.Receipt, error) {
	start := time.Now()
	rec := runners.Receipt{ActionID: a.ActionID}

	// attestation 门禁（§31 P5x）：NONE/UNKNOWN fail-closed 禁主动测试。
	if !r.sb.AllowActiveTesting() {
		rec.Blocked = true
		rec.BlockCode = "attestation_insufficient"
		rec.DurationMS = time.Since(start).Milliseconds()
		return rec, nil
	}

	// 唯一出口：经 EgressLease 的代理客户端（无直连）。
	client, err := lease.Client(time.Now())
	if err != nil {
		rec.Blocked = true
		rec.BlockCode = "egress_lease_invalid"
		rec.Error = err.Error()
		rec.DurationMS = time.Since(start).Milliseconds()
		return rec, nil
	}

	method := a.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, a.URL, nil)
	if err != nil {
		rec.Error = err.Error()
		rec.DurationMS = time.Since(start).Milliseconds()
		return rec, err
	}
	req.Header.Set("X-Snag-Capability", string(a.CapabilityID))

	resp, err := client.Do(req)
	if err != nil {
		rec.Error = err.Error()
		rec.DurationMS = time.Since(start).Milliseconds()
		return rec, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	rec.Status = resp.StatusCode
	rec.ResponseBytes = len(body)
	rec.ResponseDigest = canon.RawSHA256(body)
	rec.DurationMS = time.Since(start).Milliseconds()
	// Proxy 拦截以 403 + "blocked: <code>" 返回。
	if resp.StatusCode == http.StatusForbidden && strings.HasPrefix(string(body), "blocked:") {
		rec.Blocked = true
		rec.BlockCode = "proxy_blocked"
	}
	return rec, nil
}
