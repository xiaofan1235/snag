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
	"flag"
	"fmt"
	"os"

	"pentagent/internal/control"
	"pentagent/internal/outbox"
	"pentagent/internal/store"
	"pentagent/internal/store/sqlite"
)

func main() {
	dbPath := flag.String("db", "snag.db", "SQLite 数据库路径")
	flag.Parse()

	if !sqlite.Registered() {
		fmt.Fprintln(os.Stderr, "snagd: 未接入 SQLite 驱动，请以 -tags sqlite_modernc 构建")
		os.Exit(2)
	}
	if err := run(*dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "snagd: %v\n", err)
		os.Exit(1)
	}
}

func run(dbPath string) error {
	ctx := context.Background()

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

	// §19 ⑧⑨：启动 Scheduler + 开放新目标动作 —— 后续开发天实现（API 服务、调度循环、
	// scheduler.ExpireLeases / Reconciler.SweepTimeouts 周期任务）。此处先按次序占位。
	fmt.Println("snagd: 调度器与 API 服务将在后续开发天接入（恢复已先行，次序符合 §19）")
	return nil
}
