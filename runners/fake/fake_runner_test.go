package fake_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/canon"
	"pentagent/internal/domain"
	"pentagent/internal/execution"
	"pentagent/internal/network"
	"pentagent/internal/policy"
	"pentagent/internal/sandbox"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
	"pentagent/runners"
	"pentagent/runners/fake"
)

// fake_runner_test.go —— Day 19 §7：无 AI 跑通执行链（§23 M2「CLI 发起受控只读动作」）。
// 命令：go test -tags sqlite_modernc ./runners/...（无驱动自动 SKIP）。

// noneSandbox 报告 attestation=NONE（测门禁用）。
type noneSandbox struct{}

func (noneSandbox) Attestation() sandbox.Attestation { return sandbox.AttestNone }
func (noneSandbox) Profile() sandbox.Profile         { return sandbox.Profile{} }

func TestSandboxAttestationGate(t *testing.T) {
	deg := sandbox.NewManager(sandbox.NewDegradedSandbox())
	if deg.Attestation() != sandbox.AttestPartial || !deg.AllowActiveTesting() {
		t.Errorf("降级沙箱应 PARTIAL 且允许主动测试")
	}
	none := sandbox.NewManager(noneSandbox{})
	if none.AllowActiveTesting() {
		t.Errorf("NONE attestation 应 fail-closed 禁主动测试")
	}
}

func seedCap(t *testing.T, w *store.Writer, ctx context.Context) domain.ID {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		for _, s := range [][]any{
			{`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-1", "a", "m", ts},
			{`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-1", "r", "RUNNING", "d", ts},
			{`INSERT INTO jobs (id,run_id,kind,state,generation,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, "job-1", "run-1", "k", "RUNNING", 1, ts, ts},
			{`INSERT INTO attempts (id,job_id,generation,fencing_token,state,started_at) VALUES (?,?,?,?,?,?)`, "att-1", "job-1", 1, 1, "RUNNING", ts},
			{`INSERT INTO actions (id,run_id,job_id,attempt_id,tool_name,state,identity_id) VALUES (?,?,?,?,?,?,?)`, "act-1", "run-1", "job-1", "att-1", "http", "DISPATCHED", "idn-1"},
			{`INSERT INTO action_permits (id,run_id,action_id,attempt_id,fencing_token,policy_revision,identity_id,state,issued_at,consumed_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, "perm-1", "run-1", "act-1", "att-1", 1, 0, "idn-1", "CONSUMED", ts, ts},
			{`INSERT INTO request_capabilities (id,permit_id,action_id,run_id,attempt_id,fencing_token,max_subrequests,consumed_count) VALUES (?,?,?,?,?,?,?,0)`, "cap-1", "perm-1", "act-1", "run-1", "att-1", 1, 5},
		} {
			if _, e := tx.Exec(s[0].(string), s[1:]...); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return "cap-1"
}

// 全链路：Fake Runner 收只读动作→经 Proxy→打 Mock Target→回 Receipt；流量留证。
func TestFakeRunnerFullChain(t *testing.T) {
	if !sqlite.Registered() {
		t.Skip("需 -tags sqlite_modernc")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "fr.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	defer w.Close()
	ctx := context.Background()
	capID := seedCap(t, w, ctx)

	// Mock Target。
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello-target"))
	}))
	defer target.Close()
	targetHost := func() string { u, _ := url.Parse(target.URL); return u.Hostname() }()

	// Policy Proxy（内存证据 + 已重建缓存）。
	pm := execution.NewPermits(w)
	cache := network.NewEpochCache()
	cache.Rebuild()
	ev := network.NewMemEvidence()
	resolver := func(context.Context, domain.ID) (policy.Policy, error) {
		return policy.Policy{
			Version:       1,
			Target:        policy.Target{Hosts: []policy.HostRule{{Value: targetHost, Match: "exact"}}},
			Authorization: policy.AuthWindow{ValidFrom: "2026-01-01T00:00:00Z", ValidUntil: "2026-12-31T23:59:59Z"},
			Methods:       policy.Methods{Default: []string{"GET", "HEAD"}},
		}, nil
	}
	proxy := network.NewProxy(pm, resolver, ev, cache)
	proxySrv := httptest.NewServer(proxy)
	defer proxySrv.Close()

	// Fake Runner（降级沙箱）+ Egress 租约指向 Proxy。
	runner := fake.New(sandbox.NewManager(sandbox.NewDegradedSandbox()))
	lease := network.EgressLease{ProxyURL: proxySrv.URL, ValidUntil: time.Now().Add(time.Hour)}

	rec, err := runner.Execute(ctx, runners.Action{
		ActionID: "act-1", CapabilityID: capID, URL: target.URL + "/data", Method: "GET",
		IdempotencyClass: domain.IdempotencyReadOnly,
	}, lease)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if rec.Blocked || rec.Status != 200 {
		t.Fatalf("只读动作应 200 不被拦，得 %+v", rec)
	}
	if rec.ResponseDigest != canon.RawSHA256([]byte("hello-target")) {
		t.Errorf("响应摘要不符")
	}
	// 流量经 Proxy 留证。
	found := false
	for _, r := range ev.Records() {
		if r.Allowed && r.Status == 200 && r.RespBytes > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("流量应经 Proxy 留证，得 %+v", ev.Records())
	}
}

// attestation=NONE → 门禁拦截，不发出请求。
func TestFakeRunnerAttestationBlocked(t *testing.T) {
	runner := fake.New(sandbox.NewManager(noneSandbox{}))
	lease := network.EgressLease{ProxyURL: "http://127.0.0.1:1", ValidUntil: time.Now().Add(time.Hour)}
	rec, err := runner.Execute(context.Background(), runners.Action{ActionID: "act-1", CapabilityID: "cap-1", URL: "http://x/y", Method: "GET"}, lease)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Blocked || rec.BlockCode != "attestation_insufficient" || rec.Status != 0 {
		t.Errorf("NONE attestation 应门禁拦截且不发请求，得 %+v", rec)
	}
}

// 无效 Egress 租约 → 拦截。
func TestFakeRunnerEgressLeaseInvalid(t *testing.T) {
	runner := fake.New(sandbox.NewManager(sandbox.NewDegradedSandbox()))
	lease := network.EgressLease{ProxyURL: "http://127.0.0.1:1", ValidUntil: time.Now().Add(-time.Hour)} // 已过期
	rec, _ := runner.Execute(context.Background(), runners.Action{ActionID: "act-1", CapabilityID: "cap-1", URL: "http://x/y"}, lease)
	if !rec.Blocked || rec.BlockCode != "egress_lease_invalid" {
		t.Errorf("过期租约应拦截，得 %+v", rec)
	}
}
