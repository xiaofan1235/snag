package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"pentagent/internal/api"
	"pentagent/internal/control"
	"pentagent/internal/faultinject"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

// cli_lifecycle_test.go —— Day 13 §7：CLI 全生命周期走通（create→start→pause→resume→stop），
// 并验证故障注入 Hook 可在生命周期路径触发模拟崩溃。
// 命令：go test -tags sqlite_modernc ./cmd/pentagent/...（无驱动自动 SKIP）。

func startTestAPI(t *testing.T) *httptest.Server {
	t.Helper()
	if !sqlite.Registered() {
		t.Skip("未接入 SQLite 驱动：-tags sqlite_modernc 下运行")
	}
	path := filepath.Join(t.TempDir(), "cli.db")
	w, err := store.OpenWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Migrate(); err != nil {
		t.Fatal(err)
	}
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	rd, err := store.OpenReader(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rd.Close() })

	srv := api.NewServer(control.NewController(w), rd)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	t.Setenv("PENTAGENT_API", ts.URL)
	return ts
}

// runCLI 执行一条 CLI 命令，返回 (退出码, stdout, stderr)。
func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCLILifecycle(t *testing.T) {
	startTestAPI(t)

	// create
	code, out, errs := runCLI("run", "create", "--json")
	if code != 0 {
		t.Fatalf("create 失败: code=%d err=%s", code, errs)
	}
	var created runResp
	if e := json.Unmarshal([]byte(out), &created); e != nil || created.ID == "" {
		t.Fatalf("create 输出解析失败: %q (%v)", out, e)
	}
	if created.State != "DRAFT" {
		t.Errorf("新建应 DRAFT，得 %s", created.State)
	}
	id := created.ID

	// start → RUNNING（CLI 内部 preflight+start）
	if st := mustState(t, "start", id); st != "RUNNING" {
		t.Fatalf("start 后应 RUNNING，得 %s", st)
	}

	// status → RUNNING
	code, out, _ = runCLI("run", "status", "--id", id, "--json")
	if code != 0 {
		t.Fatal("status 失败")
	}
	var statusResp runResp
	_ = json.Unmarshal([]byte(out), &statusResp)
	if statusResp.State != "RUNNING" {
		t.Errorf("status 应 RUNNING，得 %s", statusResp.State)
	}

	// pause --mode now → PAUSED
	code, out, errs = runCLI("run", "pause", "--id", id, "--mode", "now", "--json")
	if code != 0 {
		t.Fatalf("pause 失败: %s", errs)
	}
	var paused runResp
	_ = json.Unmarshal([]byte(out), &paused)
	if paused.State != "PAUSED" {
		t.Errorf("pause 后应 PAUSED，得 %s", paused.State)
	}
	if paused.Mode != "now" {
		t.Errorf("mode 应回显 now，得 %s", paused.Mode)
	}

	// resume → RUNNING
	if st := mustState(t, "resume", id); st != "RUNNING" {
		t.Fatalf("resume 后应 RUNNING，得 %s", st)
	}

	// stop → STOPPED
	if st := mustState(t, "stop", id); st != "STOPPED" {
		t.Fatalf("stop 后应 STOPPED，得 %s", st)
	}
}

// 非法迁移：对已 STOPPED 的 Run 再 start → CLI 返回错误码（API 409）。
func TestCLIIllegalTransition(t *testing.T) {
	startTestAPI(t)
	code, out, _ := runCLI("run", "create", "--json")
	if code != 0 {
		t.Fatal("create 失败")
	}
	var r runResp
	_ = json.Unmarshal([]byte(out), &r)
	mustState(t, "stop", r.ID) // → STOPPED（终态）

	if code, _, _ := runCLI("run", "start", "--id", r.ID); code == 0 {
		t.Errorf("对终态 Run start 应失败，却 code=0")
	}
}

// 故障注入：武装 BeforeRunStateApply → 生命周期调用模拟崩溃，CLI 返回错误码。
func TestCLIFaultInjection(t *testing.T) {
	startTestAPI(t)
	t.Cleanup(faultinject.Disable)

	code, out, _ := runCLI("run", "create", "--json")
	if code != 0 {
		t.Fatal("create 失败")
	}
	var r runResp
	_ = json.Unmarshal([]byte(out), &r)

	disarm := faultinject.Arm(faultinject.BeforeRunStateApply, errors.New("simulated crash"))
	if code, _, errs := runCLI("run", "start", "--id", r.ID); code == 0 {
		t.Errorf("注入故障后 start 应失败，却 code=0 (err=%s)", errs)
	}
	disarm()

	// 解除后恢复正常。
	if st := mustState(t, "start", r.ID); st != "RUNNING" {
		t.Errorf("解除注入后 start 应 RUNNING，得 %s", st)
	}
}

func mustState(t *testing.T, action, id string) string {
	t.Helper()
	code, out, errs := runCLI("run", action, "--id", id, "--json")
	if code != 0 {
		t.Fatalf("%s 失败: code=%d err=%s", action, code, errs)
	}
	var r runResp
	if e := json.Unmarshal([]byte(out), &r); e != nil {
		t.Fatalf("%s 输出解析失败: %q", action, out)
	}
	return r.State
}
