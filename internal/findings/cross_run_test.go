package findings

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// seedRunWithScope 造一个带指定 workspace + scope digest 的 Run 及一个 Finding。
func seedRunWithScope(t *testing.T, w interface {
	Durable(context.Context, func(*sql.Tx) error) error
}, ctx context.Context, runID, wsID, scope, findingID string) {
	t.Helper()
	const ts = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, wsID, "a", "m-"+wsID, ts); e != nil {
			return e
		}
		if _, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`,
			runID, wsID, "r", "RUNNING", scope, ts); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO findings (id,run_id,title,state,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			findingID, runID, "t", FindReportable, ts, ts)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// DoD：同 Workspace 同 Scope 的两 Run，Finding 可关联为 REOPENED（§47.3）。
func TestLinkCrossRunSameScope(t *testing.T) {
	w, ctx := openDB(t)
	seedRunWithScope(t, w, ctx, "run-a", "ws-1", "scope-1", "f-a")
	seedRunWithScope(t, w, ctx, "run-b", "ws-1", "scope-1", "f-b")
	s := NewStore(w)
	id, err := s.LinkCrossRun(ctx, CrossRunLink{
		FindingID: "f-b", RunID: "run-b", LinkedFindingID: "f-a", LinkedRunID: "run-a",
		Relation: RelReopened, Basis: "endpoint=/orders param=id attack=IDOR", Confidence: "high",
	}, t0)
	if err != nil || id == "" {
		t.Fatalf("同 scope 应可关联：%v", err)
	}
	links, err := s.CrossRunLinks(ctx, "f-b")
	if err != nil || len(links) != 1 || links[0].Relation != RelReopened {
		t.Fatalf("应有一条 REOPENED 关联，实得 %+v (%v)", links, err)
	}
}

// 红线：跨 Workspace 关联被拒（§47.3/§4.0）。
func TestLinkCrossRunRejectsCrossWorkspace(t *testing.T) {
	w, ctx := openDB(t)
	seedRunWithScope(t, w, ctx, "run-a", "ws-1", "scope-1", "f-a")
	seedRunWithScope(t, w, ctx, "run-b", "ws-2", "scope-1", "f-b") // 不同 workspace
	s := NewStore(w)
	_, err := s.LinkCrossRun(ctx, CrossRunLink{
		FindingID: "f-b", RunID: "run-b", LinkedFindingID: "f-a", LinkedRunID: "run-a", Relation: RelReopened,
	}, t0)
	if !errors.Is(err, ErrCrossWorkspaceLink) {
		t.Fatalf("跨 Workspace 应拒（ErrCrossWorkspaceLink），实得 %v", err)
	}
}

// 红线：同 Workspace 但不同 Scope Digest（跨目标）被拒（§47.3/§4.0）。
func TestLinkCrossRunRejectsCrossScope(t *testing.T) {
	w, ctx := openDB(t)
	seedRunWithScope(t, w, ctx, "run-a", "ws-1", "scope-1", "f-a")
	seedRunWithScope(t, w, ctx, "run-b", "ws-1", "scope-2", "f-b") // 同 ws 不同 scope
	s := NewStore(w)
	_, err := s.LinkCrossRun(ctx, CrossRunLink{
		FindingID: "f-b", RunID: "run-b", LinkedFindingID: "f-a", LinkedRunID: "run-a", Relation: RelReopened,
	}, t0)
	if !errors.Is(err, ErrCrossScopeLink) {
		t.Fatalf("跨 Scope（跨目标）应拒（ErrCrossScopeLink），实得 %v", err)
	}
}

// 非法关系被拒。
func TestLinkCrossRunBadRelation(t *testing.T) {
	w, ctx := openDB(t)
	seedRunWithScope(t, w, ctx, "run-a", "ws-1", "scope-1", "f-a")
	seedRunWithScope(t, w, ctx, "run-b", "ws-1", "scope-1", "f-b")
	s := NewStore(w)
	_, err := s.LinkCrossRun(ctx, CrossRunLink{
		FindingID: "f-b", RunID: "run-b", LinkedFindingID: "f-a", LinkedRunID: "run-a", Relation: "BOGUS",
	}, t0)
	if !errors.Is(err, ErrBadRelation) {
		t.Fatalf("非法关系应拒（ErrBadRelation），实得 %v", err)
	}
}

// RelationForRetest 纯函数：复现→REOPENED；不复现→FIXED。
func TestRelationForRetest(t *testing.T) {
	if RelationForRetest(true) != RelReopened {
		t.Fatalf("复现应 REOPENED")
	}
	if RelationForRetest(false) != RelFixed {
		t.Fatalf("不复现应 FIXED")
	}
}
