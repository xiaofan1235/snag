//go:build !unix

package execution

import "os/exec"

// proc_other.go —— 非 Unix 平台（Windows 等）的 ExecBounded 进程控制回退（BUG-002）。
//
// Setpgid/进程组收割为 Unix 专属优化；此处留空，保留 CommandContext 的默认取消语义
// （到点 Kill 进程），cmd.WaitDelay 仍提供 SIGKILL 前的宽限。缺进程组不影响超时/看门狗
// 的正确性，仅少了「整组收割」这一额外保障。
func configureProcGroup(cmd *exec.Cmd) {}
