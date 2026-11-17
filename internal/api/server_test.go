package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"pentagent/internal/api"
	"pentagent/internal/control"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// server_test.go —— internal/api 直测：创建、状态、错误码映射（CLI 测试之外的补充覆盖）。
// 命令：go test -tags sqlite_modernc ./internal/api/...（无驱动自动 SKIP）。

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "api.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })
	ts := httptest.NewServer(api.NewServer(control.NewController(w), rd).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestCreateAndStatus(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Post(ts.URL+"/v1/runs", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create 应 201，得 %d", resp.StatusCode)
	}
	var rr struct {
		ID, State string
	}
	_ = json.NewDecoder(resp.Body).Decode(&rr)
	resp.Body.Close()
	if rr.ID == "" || rr.State != "DRAFT" {
		t.Fatalf("create 应返回 DRAFT Run，得 %+v", rr)
	}

	r2, err := http.Get(ts.URL + "/v1/runs/" + rr.ID + "/status")
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("status 应 200，得 %d", r2.StatusCode)
	}
	r2.Body.Close()
}

func TestStatusNotFound(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/v1/runs/nonexistent/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("不存在 Run 的 status 应 404，得 %d", resp.StatusCode)
	}
	var er struct{ Code string }
	_ = json.NewDecoder(resp.Body).Decode(&er)
	if er.Code != "not_found" {
		t.Errorf("错误码应 not_found，得 %s", er.Code)
	}
}

func TestStartNonexistent(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/v1/runs/nope/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("start 不存在 Run 应 404，得 %d", resp.StatusCode)
	}
}
