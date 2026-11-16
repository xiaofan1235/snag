// Package schemas 嵌入四大契约的 JSON Schema（§25、Day 04 冻结）。
//
// 契约冻结于 M0：Policy / PlanPatch / Action / Experiment / Evidence(Finding)。
// 这些定了，控制面 / 工具 / AI / UI 才能并行开发（§25）；之后改契约代价很大。
package schemas

import "embed"

//go:embed *.json
var FS embed.FS

// 契约文件名常量（供 canon.ValidateAgainst 等按名加载）。
const (
	Policy     = "policy.schema.json"
	PlanPatch  = "planpatch.schema.json"
	Action     = "action.schema.json"
	Experiment = "experiment.schema.json"
	Evidence   = "evidence.schema.json"
)

// All 列出全部契约 Schema 文件名。
var All = []string{Policy, PlanPatch, Action, Experiment, Evidence}
