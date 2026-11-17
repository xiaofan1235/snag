//go:build unix

package execution

import (
	"os/exec"
	"syscall"
)

// proc_unix.go —— ExecBounded 的 Unix 专属进程控制（BUG-002：Setpgid 为 Unix-only 字段，
// 需按平台拆分以免 Windows 构建失败）。
//
// 设独立进程组并以 SIGTERM 优雅取消：子进程若自己再 fork，整组可一并收割、不留孤儿；
// 配合 cmd.WaitDelay 宽限后由 os/exec 兜底 SIGKILL（§51 卡住处理）。
func configureProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
}
