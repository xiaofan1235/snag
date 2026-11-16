// Package api 持有 Snag 的 REST/事件契约源文件并内嵌，供契约测试与工具按需读取。
//
// 契约以这两个 YAML 为唯一来源；Go/TS 类型由 cmd/gen-contracts 生成（见各 *_gen.go）。
package api

import _ "embed"

//go:embed events.yaml
var EventsYAML []byte

//go:embed openapi.yaml
var OpenAPIYAML []byte
