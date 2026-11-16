// Command pentagentd 是 Snag 控制面守护进程（二进制名后续随项目改名为 snagd）。
//
// Day 01 仅占位骨架：真正的 Run 控制器、调度器、API 服务在后续开发天实现。
// 这里 import domain 只为证明 monorepo 模块路径与 domain 包可编入二进制。
package main

import (
	"fmt"

	"pentagent/internal/domain"
)

func main() {
	fmt.Printf("snagd (pentagentd) skeleton — domain ready, sample id %s\n", domain.NewID())
}
