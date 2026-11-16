package store_test

import (
	"context"
	"strings"
	"testing"

	"pentagent/internal/domain"
)

// store_full_test.go —— Day 03 §7 验收：§43.3 权威表清单全部建出 + 关键约束生效。
// 命令：go test -tags sqlite_modernc ./internal/store/...（无驱动自动 SKIP）。

// allTables 是 §43.3 权威清单（核心 13 + 其余 55，含 §4.2 要求但 §43.3 漏列的 identities）。
var allTables = []string{
	// 核心聚合（Day 02）
	"workspaces", "runs", "policy_revisions", "agent_sessions", "jobs", "attempts",
	"actions", "evidence", "hypotheses", "findings", "finding_evidence", "coverage_cells", "events",
	// 攻击面
	"assets", "endpoints", "endpoint_parameters", "modules", "endpoint_modules", "discoveries",
	"identities",
	// 调度执行
	"agent_seats", "job_dependencies", "approvals", "action_permits", "request_capabilities",
	"execution_receipts", "tool_recovery_attempts",
	// 证据验证
	"evidence_verifications", "evidence_citations", "coverage_revisions", "quarantine",
	// 规划
	"strategy_epochs", "strategy_snapshots", "plan_patches", "campaign_plan",
	// 预算资源
	"budget_ledger", "proxy_leases", "sandbox_instances", "resource_leases",
	// 模型
	"model_calls", "agent_context_revisions",
	// 命令事件
	"commands", "operations", "user_directives", "external_messages", "outbox",
	// 恢复
	"recovery_operations", "run_partial_reasons",
	// 报告产物
	"reports", "report_revisions", "artifacts",
	// 记忆
	"memory_sessions", "memory_outbox", "memory_links", "memory_gaps",
	// 知识技能
	"knowledge_docs", "knowledge_chunks", "knowledge_hits", "skills",
	// 工具扩展
	"tool_registry", "mcp_servers",
	// v2.4 新增
	"sequence_experiments", "sequence_steps", "race_experiments", "oast_correlations",
	"identity_auth_flows", "cross_run_finding_links", "audit_chain",
}

// 迁移(0001+0002)后，§43.3 清单里每张表都应存在，一张不缺。
func TestAllTablesExist(t *testing.T) {
	st := openTestStore(t) // 无驱动自动 SKIP；MigrateUp 会依次应用 0001、0002
	for _, tbl := range allTables {
		var name string
		err := st.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("§43.3 清单表缺失: %s (%v)", tbl, err)
		}
	}
	// 反向核对：实际表数应 ≥ 清单数（含 schema_migrations）。
	var count int
	if err := st.DB().QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count < len(allTables) {
		t.Errorf("实际表数 %d < 清单 %d", count, len(allTables))
	}
}

// commands: UNIQUE(actor_id, idempotency_key)——重复幂等键必须报错（§4.2 / Day 03 DoD）。
func TestCommandsIdempotencyUnique(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	ins := func() error {
		_, err := st.DB().ExecContext(ctx,
			`INSERT INTO commands (actor_id, idempotency_key, request_digest, state, resource_refs, created_at)
			 VALUES (?,?,?,?,?,?)`,
			"actor-1", "key-abc", "digest", "ACCEPTED", "{}", "2026-11-16T00:00:00Z",
		)
		return err
	}
	if err := ins(); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	err := ins()
	if err == nil {
		t.Fatal("重复 (actor_id, idempotency_key) 应违反唯一约束而报错，却成功了")
	}
	if !strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") &&
		!strings.Contains(strings.ToUpper(err.Error()), "CONSTRAINT") {
		t.Errorf("错误应为唯一约束冲突，得到: %v", err)
	}
}

// external_messages: UNIQUE(source_kind, external_message_id)——qitem 幂等（§4.2）。
func TestExternalMessagesUnique(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	ins := func() error {
		_, err := st.DB().ExecContext(ctx,
			`INSERT INTO external_messages (id, source_kind, external_message_id, content_type, payload_digest, received_at)
			 VALUES (?,?,?,?,?,?)`,
			string(domain.NewID()), "slack", "msg-1", "text", "d", "2026-11-16T00:00:00Z",
		)
		return err
	}
	if err := ins(); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if err := ins(); err == nil {
		t.Fatal("重复 (source_kind, external_message_id) 应报错，却成功了")
	}
}

// 复合外键：finding_evidence 跨 Run 引用证据应被拒（§4.3 #6），同 Run 应通过。
// 这里顺带验证 0002 不破坏 0001 既有的复合外键。
func TestFindingEvidenceCrossRunRejected(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	ws := domain.NewWorkspace("acme", "mpk-"+string(domain.NewID()))
	_ = st.InsertWorkspace(ctx, ws)
	r := domain.NewRun(ws.ID, "run-a", "tester", "d")
	_ = st.InsertRun(ctx, r)

	// 直接用 SQL 造一个 finding 和一个 evidence（本 Run），再造一条跨 Run 的 evidence 引用。
	fid := string(domain.NewID())
	eid := string(domain.NewID())
	now := "2026-11-16T00:00:00Z"
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO findings (id, run_id, state, created_at, updated_at) VALUES (?,?,?,?,?)`,
		fid, string(r.ID), "CANDIDATE", now, now); err != nil {
		t.Fatalf("insert finding: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO evidence (id, run_id, kind, plain_sha256, captured_at) VALUES (?,?,?,?,?)`,
		eid, string(r.ID), "response", "sha", now); err != nil {
		t.Fatalf("insert evidence: %v", err)
	}
	// 同 Run 引用：应成功。
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO finding_evidence (run_id, finding_id, evidence_id, role) VALUES (?,?,?,?)`,
		string(r.ID), fid, eid, "baseline"); err != nil {
		t.Fatalf("同 Run 引用应成功: %v", err)
	}
	// 跨 Run 引用：用一个不存在于本 Run 的 run_id 组合，复合外键应拒绝。
	otherRun := string(domain.NewID())
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO finding_evidence (run_id, finding_id, evidence_id, role) VALUES (?,?,?,?)`,
		otherRun, fid, eid, "variant"); err == nil {
		t.Fatal("跨 Run 的 finding_evidence 引用应被复合外键拒绝，却成功了")
	}
}
