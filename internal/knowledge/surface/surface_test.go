package surface

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

var t0 = time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)

func TestTemplatizePath(t *testing.T) {
	cases := map[string]string{
		"/api/users/123":                              "/api/users/{id}",
		"/api/users/123/orders/456":                   "/api/users/{id}/orders/{id}",
		"/api/u/550e8400-e29b-41d4-a716-446655440000": "/api/u/{uuid}",
		"/api/blob/deadbeefdeadbeef":                  "/api/blob/{hash}",
		"/api/list?page=2":                            "/api/list",
		"/api/users":                                  "/api/users",
	}
	for in, want := range cases {
		if got := TemplatizePath(in); got != want {
			t.Errorf("TemplatizePath(%q)=%q，应 %q", in, got, want)
		}
	}
}

// DoD：OpenAPI 规格确定性生成端点×参数矩阵（§31 E3）。
func TestFromOpenAPI(t *testing.T) {
	doc := []byte(`{"paths":{
		"/api/users":{
			"get":{"parameters":[{"name":"page","in":"query","required":false,"schema":{"type":"integer"}}]},
			"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"name":{"type":"string"}},"required":["name"]}}}}}
		},
		"/api/users/{id}":{"get":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}]}}
	}}`)
	eps, err := FromOpenAPI(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("应生成 3 个端点，实得 %d", len(eps))
	}
	// 确定性顺序：/api/users GET, /api/users POST, /api/users/{id} GET。
	byKey := map[string][]Param{}
	for _, e := range eps {
		if e.Source != SrcSpec {
			t.Fatalf("来源应 spec")
		}
		byKey[e.Method+" "+e.URLPattern] = e.Params
	}
	if ps := byKey["GET /api/users"]; len(ps) != 1 || ps[0].Name != "page" || ps[0].Location != InQuery {
		t.Fatalf("GET /api/users 参数矩阵错误：%+v", ps)
	}
	if ps := byKey["POST /api/users"]; len(ps) != 1 || ps[0].Name != "name" || ps[0].Location != InBody || !ps[0].Required {
		t.Fatalf("POST /api/users body 参数错误：%+v", ps)
	}
	if ps := byKey["GET /api/users/{id}"]; len(ps) != 1 || ps[0].Location != InPath {
		t.Fatalf("GET /api/users/{id} path 参数错误：%+v", ps)
	}
}

func TestFromGraphQL(t *testing.T) {
	schema := `
	type Query { user(id: ID!): User  users: [User] }
	type Mutation { createUser(name: String!, age: Int): User }
	`
	eps := FromGraphQL(schema)
	got := map[string]bool{}
	for _, e := range eps {
		if e.Method != "POST" || e.Source != SrcSpec {
			t.Fatalf("GraphQL 端点应 POST/spec：%+v", e)
		}
		got[e.URLPattern] = true
	}
	for _, want := range []string{"/graphql/query.user", "/graphql/query.users", "/graphql/mutation.createUser"} {
		if !got[want] {
			t.Fatalf("应生成 %q，实得 %v", want, got)
		}
	}
}

func openDB(t *testing.T) (*store.Writer, context.Context) {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	w, err := store.OpenWriter(filepath.Join(t.TempDir(), "surface.db"))
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

func seedRun(t *testing.T, w *store.Writer, ctx context.Context) {
	t.Helper()
	const tss = "2026-11-17T00:00:00Z"
	if err := w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.Exec(`INSERT OR IGNORE INTO workspaces (id,name,memory_project_key,created_at) VALUES (?,?,?,?)`, "ws-s", "a", "m", tss); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO runs (id,workspace_id,name,state,authorization_scope_digest,created_at) VALUES (?,?,?,?,?,?)`, "run-1", "ws-s", "r", "RUNNING", "d", tss)
		return e
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// DoD：新接口入库提升 SurfaceRevision；重复接口去重不虚增（§12.1）。
func TestStoreAddBumpsSurfaceRevision(t *testing.T) {
	w, ctx := openDB(t)
	seedRun(t, w, ctx)
	s := NewStore(w)

	// 首批：2 个新接口 → SurfaceRevision 0→1。
	eps := []Endpoint{
		{Method: "get", Path: "/api/users", Source: SrcStatic, Params: []Param{{Name: "page", Location: InQuery}}},
		{Method: "post", Path: "/api/login", Source: SrcStatic},
	}
	r1, err := s.AddEndpoints(ctx, "run-1", "", eps, "js_discovery", t0)
	if err != nil {
		t.Fatal(err)
	}
	if r1.AddedEndpoints != 2 || r1.SurfaceRevision != 1 {
		t.Fatalf("首批应新增 2、SurfaceRevision=1，实得 %+v", r1)
	}

	// 重复同一批（路径模板化后同键）→ 去重、不新增、SurfaceRevision 不动。
	r2, err := s.AddEndpoints(ctx, "run-1", "", eps, "js_discovery", t0)
	if err != nil {
		t.Fatal(err)
	}
	if r2.AddedEndpoints != 0 || r2.SurfaceRevision != 1 {
		t.Fatalf("重复接口应去重、SurfaceRevision 不变，实得 %+v", r2)
	}

	// /api/users/42 与 /api/users/99 模板化后同为 /api/users/{id}，只算 1 个新接口 → SurfaceRevision 1→2。
	eps2 := []Endpoint{
		{Method: "get", Path: "/api/users/42", Source: SrcRuntime},
		{Method: "get", Path: "/api/users/99", Source: SrcRuntime},
	}
	r3, err := s.AddEndpoints(ctx, "run-1", "", eps2, "runtime_capture", t0)
	if err != nil {
		t.Fatal(err)
	}
	if r3.AddedEndpoints != 1 || r3.SurfaceRevision != 2 {
		t.Fatalf("模板化去重后应新增 1、SurfaceRevision=2，实得 %+v", r3)
	}
	if n, _ := s.CountEndpoints(ctx, "run-1"); n != 3 {
		t.Fatalf("接口总数应 3，实得 %d", n)
	}
}
