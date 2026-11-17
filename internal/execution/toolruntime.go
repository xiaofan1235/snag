package execution

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// toolruntime.go —— 工具/命令卡住处理（§51 卡住处理 / §53 长命令转后台）。
//
// 四条铁律：
//  1. 硬超时：每个工具/外部调用都带 hard timeout，到点必结束，绝不无限等待。
//  2. 看门狗：靠心跳判活；在 watchdog 间隔内无心跳（no-progress）即判定卡住并杀。
//  3. 长命令转后台：前台跑超过 fgLimit（默认 5s）不再阻塞 Executor，转为后台 Job（默认 30min 封顶）。
//  4. 诚实上报：被超时/看门狗杀掉如实标记 TimedOut/Killed，绝不把「被杀」伪装成「正常完成」。
//
// 说明：Go 无法强杀一个无视 ctx 的 goroutine；对真实外部命令用 ExecBounded（SIGTERM→宽限→SIGKILL
// 真杀进程），对进程内函数要求其尊重 ctx，到点 RunBounded 放弃等待并如实标记超时。

const (
	// DefaultForegroundLimit 前台最长驻留；超过即转后台（§53）。
	DefaultForegroundLimit = 5 * time.Second
	// DefaultBackgroundCap 后台 Job 封顶（§51）。
	DefaultBackgroundCap = 30 * time.Minute
	// DefaultKillGrace SIGTERM 到 SIGKILL 的宽限。
	DefaultKillGrace = 2 * time.Second
)

// ErrNoProgress 看门狗判定无进展（心跳超时）。
var ErrNoProgress = errors.New("tool watchdog: no heartbeat within interval")

// Outcome 是一次有界执行的结果，对「是否被杀」诚实。
type Outcome struct {
	Output       []byte
	Err          error
	TimedOut     bool // 命中硬超时
	Killed       bool // 被硬超时/看门狗强制结束
	NoProgress   bool // 被看门狗（心跳）判定卡住
	Backgrounded bool // 已转后台（本次为非阻塞返回）
	Duration     time.Duration
}

// RunBounded 在硬超时下运行 fn，fn 必须尊重 ctx。到点 fn 未返回 → TimedOut+Killed 并如实上报
// （fn 的 goroutine 被放弃，生产中应仅用于可被 ctx 取消的纯函数；真实子进程请用 ExecBounded）。
func RunBounded(parent context.Context, hard time.Duration, fn func(ctx context.Context) ([]byte, error)) Outcome {
	ctx, cancel := context.WithTimeout(parent, hard)
	defer cancel()
	start := time.Now()

	type res struct {
		out []byte
		err error
	}
	done := make(chan res, 1)
	go func() {
		out, err := fn(ctx)
		done <- res{out, err}
	}()

	select {
	case r := <-done:
		return Outcome{Output: r.out, Err: r.err, Duration: time.Since(start)}
	case <-ctx.Done():
		// 到点仍未返回：诚实标记超时/被杀，不等待可能永不返回的 fn。
		return Outcome{TimedOut: true, Killed: true, Err: ctx.Err(), Duration: time.Since(start)}
	}
}

// RunWatched 在硬超时之外再加看门狗：fn 每有进展就调一次 heartbeat()，
// 若 watchdog 间隔内无心跳即判定卡住（NoProgress）并结束等待。heartbeat<=0 关闭看门狗。
func RunWatched(parent context.Context, hard, watchdog time.Duration, fn func(ctx context.Context, heartbeat func()) ([]byte, error)) Outcome {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if hard > 0 {
		var hc context.CancelFunc
		ctx, hc = context.WithTimeout(ctx, hard)
		defer hc()
	}
	start := time.Now()

	var mu sync.Mutex
	last := time.Now()
	beat := func() { mu.Lock(); last = time.Now(); mu.Unlock() }

	type res struct {
		out []byte
		err error
	}
	done := make(chan res, 1)
	go func() {
		out, err := fn(ctx, beat)
		done <- res{out, err}
	}()

	var tick <-chan time.Time
	if watchdog > 0 {
		t := time.NewTicker(watchdog / 2)
		defer t.Stop()
		tick = t.C
	}

	for {
		select {
		case r := <-done:
			return Outcome{Output: r.out, Err: r.err, Duration: time.Since(start)}
		case <-ctx.Done():
			return Outcome{TimedOut: true, Killed: true, Err: ctx.Err(), Duration: time.Since(start)}
		case <-tick:
			mu.Lock()
			stalled := time.Since(last) >= watchdog
			mu.Unlock()
			if stalled {
				cancel() // 通知 fn 退出；下一轮由 ctx.Done 收束
				return Outcome{NoProgress: true, Killed: true, Err: ErrNoProgress, Duration: time.Since(start)}
			}
		}
	}
}

// BackgroundHandle 是一个转入后台的长任务句柄。
type BackgroundHandle struct {
	Done   <-chan Outcome // 封顶时间内投递一次最终结果
	cancel context.CancelFunc
}

// Cancel 请求提前结束后台任务。
func (h *BackgroundHandle) Cancel() { h.cancel() }

// RunForegroundOrBackground 先前台等待 fgLimit；在此之内完成则直接返回前台 Outcome（bg=nil）。
// 超过 fgLimit 仍未完成 → 立即返回 Backgrounded 结果 + 句柄，fn 在 bgCap 封顶下继续，Executor 不被阻塞（§53）。
func RunForegroundOrBackground(parent context.Context, fgLimit, bgCap time.Duration, fn func(ctx context.Context) ([]byte, error)) (Outcome, *BackgroundHandle) {
	if fgLimit <= 0 {
		fgLimit = DefaultForegroundLimit
	}
	if bgCap <= 0 {
		bgCap = DefaultBackgroundCap
	}
	ctx, cancel := context.WithTimeout(parent, bgCap)
	start := time.Now()

	type res struct {
		out []byte
		err error
	}
	done := make(chan res, 1)
	go func() {
		out, err := fn(ctx)
		done <- res{out, err}
	}()

	fg := time.NewTimer(fgLimit)
	defer fg.Stop()

	select {
	case r := <-done:
		cancel()
		return Outcome{Output: r.out, Err: r.err, Duration: time.Since(start)}, nil
	case <-fg.C:
		// 超过前台上限：转后台，不阻塞 Executor。
		out := make(chan Outcome, 1)
		go func() {
			defer cancel()
			select {
			case r := <-done:
				out <- Outcome{Output: r.out, Err: r.err, Backgrounded: true, Duration: time.Since(start)}
			case <-ctx.Done():
				out <- Outcome{Backgrounded: true, TimedOut: true, Killed: true, Err: ctx.Err(), Duration: time.Since(start)}
			}
		}()
		return Outcome{Backgrounded: true, Duration: time.Since(start)}, &BackgroundHandle{Done: out, cancel: cancel}
	}
}

// ExecBounded 在硬超时下运行真实外部命令：到点先 SIGTERM，宽限后 SIGKILL（真杀进程，不留孤儿）。
// grace<=0 用 DefaultKillGrace。非 TTY、无交互，绝不挂起等待输入。
func ExecBounded(parent context.Context, hard, grace time.Duration, name string, args ...string) Outcome {
	if grace <= 0 {
		grace = DefaultKillGrace
	}
	start := time.Now()
	ctx, cancel := parent, context.CancelFunc(func() {})
	if hard > 0 {
		ctx, cancel = context.WithTimeout(parent, hard)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	// 进程组 + SIGTERM 优雅取消按平台拆分（BUG-002：Setpgid 为 Unix-only，见 proc_unix.go/proc_other.go）；
	// SIGTERM 后宽限，仍不退则 SIGKILL（真杀进程，不留孤儿）。
	configureProcGroup(cmd)
	cmd.WaitDelay = grace

	out, err := cmd.Output()
	o := Outcome{Output: out, Err: err, Duration: time.Since(start)}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		o.TimedOut, o.Killed = true, true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && !ee.Success() && ctx.Err() != nil {
		o.Killed = true
	}
	return o
}
