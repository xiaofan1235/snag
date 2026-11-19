// Command pentagentd 是 Snag 控制面守护进程（二进制名后续随项目改名为 snagd）。
//
// Day 12：落实 §19 启动顺序的前半——①打开库 + 迁移 → 启动恢复器 Reconcile（恢复孤儿 Attempt /
// 未决动作对账 / 增代 / 补发 Outbox）→ 之后才开 Scheduler 与 API（后续天实现）。
// "先恢复、后开调度、最后才接新目标动作"的次序由此 main 的调用顺序保证。
//
// 需接入 SQLite 驱动方可真正开库：go run/build -tags sqlite_modernc。未接驱动时打印提示并退出。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pentagent/internal/api"
	"pentagent/internal/control"
	"pentagent/internal/outbox"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

func main() {
	dbPath := flag.String("db", "snag.db", "SQLite 数据库路径")
	addr := flag.String("addr", "127.0.0.1:8787", "API 监听地址（CLI/前端经此访问控制面，§17）")
	flag.Parse()

	if !sqlite.Registered() {
		fmt.Fprintln(os.Stderr, "snagd: 未接入 SQLite 驱动，请以 -tags sqlite_modernc 构建")
		os.Exit(2)
	}
	if err := run(context.Background(), *dbPath, *addr); err != nil {
		fmt.Fprintf(os.Stderr, "snagd: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dbPath, addr string) error {

	// §19 ①：打开库 + 迁移。
	w, err := store.OpenWriter(dbPath)
	if err != nil {
		return fmt.Errorf("打开写连接: %w", err)
	}
	defer w.Close()
	if err := w.Migrate(); err != nil {
		return fmt.Errorf("迁移: %w", err)
	}
	w.Start()
	rd, err := store.OpenReader(dbPath, 4)
	if err != nil {
		return fmt.Errorf("打开读连接: %w", err)
	}
	defer rd.Close()

	// §19 ②–⑦：启动恢复。
	jobs := control.NewJobController(w)
	pub := outbox.NewPublisher(w, rd, outbox.NewMemBus())
	rec := control.NewReconciler(w, rd, jobs, pub)
	rep, err := rec.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("启动恢复: %w", err)
	}
	fmt.Printf("snagd: 启动恢复完成 — 未决动作=%d 孤儿Attempt=%d 进RECONCILING=%d 重新上架=%d 增代=%d 补发Outbox=%d\n",
		rep.UnknownActions, rep.OrphanedAttempts, rep.JobsReconciling, rep.JobsReoffered, rep.GenerationsBumped, rep.OutboxDrained)

	// §19 ⑧：恢复完成后才开 API 服务（CLI/前端经此访问控制面，§17/§39.1）。
	// 偏离开发文档记录：原 main 在此占位、不监听；本次接入 HTTP 服务以打通前后端最小闭环
	// （GET/POST /v1/runs + 生命周期）。调度循环（scheduler.ExpireLeases /
	// Reconciler.SweepTimeouts 周期任务，§19 ⑨）仍待后续开发天，不在本次范围。
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(control.NewController(w), rd).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("snagd: API 监听 http://%s （§16.33 Run 路由已接入）\n", addr)
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			errCh <- e
		}
	}()

	// 等待停止信号或监听错误；收到信号后给在途请求 5s 优雅关闭窗口。
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case e := <-errCh:
		return fmt.Errorf("API 监听: %w", e)
	case <-sigCtx.Done():
		fmt.Println("\nsnagd: 收到停止信号，优雅关闭…")
	}
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := srv.Shutdown(shutCtx); e != nil {
		return fmt.Errorf("优雅关闭: %w", e)
	}
	return nil
}
