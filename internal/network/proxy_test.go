package network_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/network"
	"pentagent/internal/policy"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// proxy_test.go —— Day 18 §7：Policy Proxy 逐请求校验链 + 留证（§9.0.5/§23 M2）。
// 命令：go test -tags sqlite_modernc ./internal/network/...（无驱动自动 SKIP）。

func openProxyDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "proxy.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, context.Background()
}

// seedCapability 造 run/job/attempt/action(DISPATCHED)/action_permit(CONSUMED)/request_capability。
func seedCapability(t *testing.T, w *store.Writer, ctx context.Context, maxSub int) domain.ID {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	capID := domain.ID("cap-1")
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		stmts := [][]any{
			{`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-1", "a", "m", ts},
			{`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "d", ts},
			{`INSERT INTO jobs (id,run_id,kind,state,generation,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, "job-1", "run-1", "k", "RUNNING", 1, ts, ts},
			{`INSERT INTO attempts (id,job_id,generation,fencing_token,state,started_at) VALUES (?,?,?,?,?,?)`, "att-1", "job-1", 1, 1, "RUNNING", ts},
			{`INSERT INTO actions (id,run_id,job_id,attempt_id,tool_name,state,identity_id) VALUES (?,?,?,?,?,?,?)`, "act-1", "run-1", "job-1", "att-1", "http", "DISPATCHED", "idn-1"},
			{`INSERT INTO action_permits (id,run_id,action_id,attempt_id,fencing_token,policy_revision,identity_id,state,issued_at,consumed_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, "perm-1", "run-1", "act-1", "att-1", 1, 0, "idn-1", "CONSUMED", ts, ts},
			{`INSERT INTO request_capabilities (id,permit_id,action_id,run_id,attempt_id,fencing_token,max_subrequests,consumed_count) VALUES (?,?,?,?,?,?,?,0)`, string(capID), "perm-1", "act-1", "run-1", "att-1", 1, maxSub},
		}
		for _, s := range stmts {
			if _, e := tx.Exec(s[0].(string), s[1:]...); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return capID
}

func policyFor(hosts ...string) policy.Policy {
	var hr []policy.HostRule
	for _, h := range hosts {
		hr = append(hr, policy.HostRule{Value: h, Match: "exact"})
	}
	return policy.Policy{
		Version:       1,
		Target:        policy.Target{Hosts: hr},
		Authorization: policy.AuthWindow{ValidFrom: "2026-01-01T00:00:00Z", ValidUntil: "2026-12-31T23:59:59Z"},
		Methods:       policy.Methods{Default: []string{"GET", "HEAD", "POST"}},
	}
}

func hostOf(rawurl string) string {
	u, _ := url.Parse(rawurl)
	return u.Hostname()
}

func TestProxyFailClosedBeforeRebuild(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 5)
	pm := execution.NewPermits(w)
	cache := network.NewEpochCache() // 未 Rebuild
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor("127.0.0.1"), nil }
	px := network.NewProxy(pm, resolver, network.NewMemEvidence(), cache)

	res := px.CheckRequest(ctx, capID, "http://127.0.0.1/x", "GET", true, time.Now())
	if res.Allowed || res.Code != "proxy_not_ready" {
		t.Fatalf("重建前应 fail-closed，得 %+v", res)
	}
}

func TestProxyCompliantAndCapture(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 5)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok-from-target"))
	}))
	defer mock.Close()

	pm := execution.NewPermits(w)
	cache := network.NewEpochCache()
	cache.Rebuild()
	ev := network.NewMemEvidence()
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor(hostOf(mock.URL)), nil }
	px := network.NewProxy(pm, resolver, ev, cache)

	// CheckRequest 放行。
	if res := px.CheckRequest(ctx, capID, mock.URL+"/p", "GET", true, time.Now()); !res.Allowed {
		t.Fatalf("合规请求应放行，得 %+v", res)
	}
	// ServeHTTP 转发 + 留证。
	req, _ := http.NewRequest("GET", mock.URL+"/p", nil)
	req.Header.Set("X-Snag-Capability", string(capID))
	rec := httptest.NewRecorder()
	px.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "ok-from-target" {
		t.Fatalf("转发应得目标响应，得 code=%d body=%q", rec.Code, rec.Body.String())
	}
	// 证据已留（CheckRequest 消耗 1 + ServeHTTP 消耗 1 + 转发留证）。
	recs := ev.Records()
	found := false
	for _, r := range recs {
		if r.Allowed && r.Status == 200 && r.RespBytes > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("应留下一条成功响应证据，得 %+v", recs)
	}
}

func TestProxyOutOfScopeBlocked(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 5)
	pm := execution.NewPermits(w)
	cache := network.NewEpochCache()
	cache.Rebuild()
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor("api.target.test"), nil }
	px := network.NewProxy(pm, resolver, network.NewMemEvidence(), cache)

	if res := px.CheckRequest(ctx, capID, "https://evil.test/x", "GET", true, time.Now()); res.Allowed || res.Code != "scope_violation" {
		t.Fatalf("越域应拦截 scope_violation，得 %+v", res)
	}
}

func TestProxyCapabilityPerRequestConsume(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 2) // max 2
	pm := execution.NewPermits(w)
	cache := network.NewEpochCache()
	cache.Rebuild()
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor("127.0.0.1"), nil }
	px := network.NewProxy(pm, resolver, network.NewMemEvidence(), cache)

	for i := 0; i < 2; i++ {
		if res := px.CheckRequest(ctx, capID, "http://127.0.0.1/x", "GET", true, time.Now()); !res.Allowed {
			t.Fatalf("第 %d 个子请求应放行，得 %+v", i+1, res)
		}
	}
	if res := px.CheckRequest(ctx, capID, "http://127.0.0.1/x", "GET", true, time.Now()); res.Allowed || res.Code != "capability_exhausted" {
		t.Fatalf("超 max_subrequests 应中断，得 %+v", res)
	}
}

func TestProxyRevokedPermitBlocked(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 5)
	pm := execution.NewPermits(w)
	// 撤销该 attempt 的 permit（但它已 CONSUMED，不受 RevokeAttemptPermits 影响，故直接改状态模拟撤销）。
	_ = w.Durable(ctx, func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE action_permits SET state='REVOKED' WHERE id='perm-1'`)
		return e
	})
	cache := network.NewEpochCache()
	cache.Rebuild()
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor("127.0.0.1"), nil }
	px := network.NewProxy(pm, resolver, network.NewMemEvidence(), cache)

	if res := px.CheckRequest(ctx, capID, "http://127.0.0.1/x", "GET", true, time.Now()); res.Allowed || res.Code != "capability_not_live" {
		t.Fatalf("Permit 撤销应拦截 capability_not_live，得 %+v", res)
	}
}

func TestProxyEgressGuard(t *testing.T) {
	w, ctx := openProxyDB(t)
	capID := seedCapability(t, w, ctx, 5)
	pm := execution.NewPermits(w)
	cache := network.NewEpochCache()
	cache.Rebuild()
	// 策略放行私网，但开启 egress 守卫 → 仍拒。
	resolver := func(context.Context, domain.ID) (policy.Policy, error) { return policyFor("10.0.0.1"), nil }
	px := network.NewProxy(pm, resolver, network.NewMemEvidence(), cache, network.WithEgressGuard())

	if res := px.CheckRequest(ctx, capID, "http://10.0.0.1/x", "GET", true, time.Now()); res.Allowed || res.Code != "egress_private_ip" {
		t.Fatalf("egress 守卫应拦私网，得 %+v", res)
	}
}

func TestIsPrivateHostAndRedirect(t *testing.T) {
	// isPrivateHost 经 egress 守卫路径间接验证已在上；此处验重定向越域判定。
	tg := policy.Target{Hosts: []policy.HostRule{{Value: "api.target.test", Match: "exact"}}}
	if in, _ := network.RedirectInScope("https://api.target.test/a", "https://evil.test/b", tg); in {
		t.Error("越域重定向应判 out-of-scope")
	}
	if in, _ := network.RedirectInScope("https://api.target.test/a", "/relative/path", tg); !in {
		t.Error("同域相对重定向应 in-scope")
	}
}

var _ = errors.Is
