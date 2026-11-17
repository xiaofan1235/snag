package execution

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// 卡住的工具（永不自行返回）在硬超时命中后被杀，并如实标记 TimedOut/Killed。
func TestRunBoundedHungToolKilled(t *testing.T) {
	out := RunBounded(context.Background(), 50*time.Millisecond, func(ctx context.Context) ([]byte, error) {
		<-ctx.Done() // 阻塞直到被取消
		return nil, ctx.Err()
	})
	if !out.TimedOut || !out.Killed {
		t.Fatalf("卡住的工具未被硬超时杀掉：%+v", out)
	}
}

// 正常工具在超时前完成，不被误标。
func TestRunBoundedFastToolCompletes(t *testing.T) {
	out := RunBounded(context.Background(), time.Second, func(ctx context.Context) ([]byte, error) {
		return []byte("ok"), nil
	})
	if out.TimedOut || out.Killed || string(out.Output) != "ok" {
		t.Fatalf("正常工具被误处理：%+v", out)
	}
}

// DoD：长命令超过前台上限即转后台，Executor 不被阻塞；后台结果随后如实投递。
func TestForegroundOrBackgroundLongGoesBackground(t *testing.T) {
	start := time.Now()
	out, h := RunForegroundOrBackground(context.Background(), 50*time.Millisecond, time.Second,
		func(ctx context.Context) ([]byte, error) {
			select {
			case <-time.After(150 * time.Millisecond):
				return []byte("late"), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	if waited := time.Since(start); waited > 120*time.Millisecond {
		t.Fatalf("前台阻塞过久：%v（应在前台上限附近即返回）", waited)
	}
	if !out.Backgrounded || h == nil {
		t.Fatalf("长命令未转后台：out=%+v handle=%v", out, h)
	}
	final := <-h.Done
	if !final.Backgrounded || string(final.Output) != "late" {
		t.Fatalf("后台最终结果不符：%+v", final)
	}
}

// 快命令在前台上限内完成则留在前台（不转后台、无句柄）。
func TestForegroundOrBackgroundFastStaysForeground(t *testing.T) {
	out, h := RunForegroundOrBackground(context.Background(), 200*time.Millisecond, time.Second,
		func(ctx context.Context) ([]byte, error) {
			return []byte("quick"), nil
		})
	if out.Backgrounded || h != nil || string(out.Output) != "quick" {
		t.Fatalf("快命令不应转后台：out=%+v handle=%v", out, h)
	}
}

// 看门狗：无心跳（无进展）在远早于硬超时时即判卡住并杀。
func TestRunWatchedNoHeartbeatKilled(t *testing.T) {
	start := time.Now()
	out := RunWatched(context.Background(), 5*time.Second, 60*time.Millisecond,
		func(ctx context.Context, beat func()) ([]byte, error) {
			<-ctx.Done() // 从不心跳
			return nil, ctx.Err()
		})
	if !out.NoProgress || !out.Killed {
		t.Fatalf("无心跳工具未被看门狗杀掉：%+v", out)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("看门狗过慢：%v", waited)
	}
}

// 看门狗：规律心跳的工具不被误杀，正常完成。
func TestRunWatchedHeartbeatSurvives(t *testing.T) {
	out := RunWatched(context.Background(), 2*time.Second, 80*time.Millisecond,
		func(ctx context.Context, beat func()) ([]byte, error) {
			for i := 0; i < 5; i++ {
				beat()
				select {
				case <-time.After(20 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return []byte("done"), nil
		})
	if out.NoProgress || out.Killed || string(out.Output) != "done" {
		t.Fatalf("规律心跳的工具被误杀：%+v", out)
	}
}

// 真实外部进程：卡住的 sleep 被 SIGTERM→SIGKILL 真杀，且快速收束。
func TestExecBoundedKillsSlowProcess(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("无 sleep 命令，跳过")
	}
	out := ExecBounded(context.Background(), 100*time.Millisecond, 50*time.Millisecond, "sleep", "5")
	if !out.TimedOut || !out.Killed {
		t.Fatalf("慢进程未被杀：%+v", out)
	}
	if out.Duration > 2*time.Second {
		t.Fatalf("收割过慢：%v", out.Duration)
	}
}

// 真实外部进程：快命令正常完成，不被误标。
func TestExecBoundedFastProcess(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("无 true 命令，跳过")
	}
	out := ExecBounded(context.Background(), 2*time.Second, 50*time.Millisecond, "true")
	if out.TimedOut || out.Killed {
		t.Fatalf("快进程被误标：%+v", out)
	}
}
