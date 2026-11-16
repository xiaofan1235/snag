package api_test

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"pentagent/api"
	apigen "pentagent/api/gen"
	"pentagent/internal/events"
)

// contracts_test.go —— Day 05 §7 契约测试（§23 M0：文档注册的每个事件/路由都有 Schema）。
// 命令覆盖：go test ./api/...

// expectedEvents 是 §15.6 列出的全部事件（49 个）。events.yaml 必须不多不少地覆盖它们。
var expectedEvents = []string{
	"run.updated", "run.exit_blocked", "preflight.updated",
	"job.created", "job.updated", "attempt.updated", "agent.progress", "activity.created",
	"plan.updated", "action.updated", "command.completed", "operation.updated", "directive.updated", "exchange.created",
	"evidence.created", "asset.updated", "discovery.updated", "artifact.created", "resource.updated",
	"hypothesis.updated", "finding.updated", "verification.updated", "retest.updated", "coverage.updated",
	"approval.requested", "approval.resolved", "budget.updated", "provider.updated", "identity.updated", "egress.updated", "sandbox.updated",
	"tool.updated", "browser.updated", "report.updated", "notification.updated",
	"knowledge.updated", "skill.updated", "template.updated",
	"auth.required", "diagnostics.updated",
	"memory.health_updated", "memory.gap_created", "memory.observation_created", "memory.rebuild_updated",
	"seat.updated", "recovery.updated", "policy.denied", "system.warning", "stream.resync_required",
}

func TestEventsRegistryComplete(t *testing.T) {
	var doc struct {
		Events map[string]struct {
			Description string            `yaml:"description"`
			Payload     map[string]string `yaml:"payload"`
		} `yaml:"events"`
	}
	if err := yaml.Unmarshal(api.EventsYAML, &doc); err != nil {
		t.Fatal(err)
	}
	// §15.6 每个事件都在，且每个事件有非空 payload schema。
	for _, name := range expectedEvents {
		ev, ok := doc.Events[name]
		if !ok {
			t.Errorf("events.yaml 缺 §15.6 事件: %s", name)
			continue
		}
		if len(ev.Payload) == 0 {
			t.Errorf("事件 %s 缺 payload schema", name)
		}
	}
	// 不多不少。
	if len(doc.Events) != len(expectedEvents) {
		t.Errorf("events.yaml 事件数 %d != §15.6 的 %d（多出或缺失）", len(doc.Events), len(expectedEvents))
	}
	// 生成物与 YAML 同步（否则需重跑 gen-contracts）。
	if len(events.All) != len(doc.Events) {
		t.Errorf("生成的 events.All(%d) 与 events.yaml(%d) 不同步，请重跑 go run ./cmd/gen-contracts",
			len(events.All), len(doc.Events))
	}
}

type opT struct {
	OperationID string `yaml:"operationId"`
	Responses   map[string]struct {
		Content map[string]struct {
			Schema map[string]any `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"responses"`
}

type pathItemT struct {
	Get    *opT `yaml:"get"`
	Post   *opT `yaml:"post"`
	Put    *opT `yaml:"put"`
	Delete *opT `yaml:"delete"`
	Patch  *opT `yaml:"patch"`
}

func (p pathItemT) ops() []*opT {
	var out []*opT
	for _, o := range []*opT{p.Get, p.Post, p.Put, p.Delete, p.Patch} {
		if o != nil {
			out = append(out, o)
		}
	}
	return out
}

func TestOpenAPIRoutesHaveSchema(t *testing.T) {
	var doc struct {
		Paths map[string]pathItemT `yaml:"paths"`
	}
	if err := yaml.Unmarshal(api.OpenAPIYAML, &doc); err != nil {
		t.Fatal(err)
	}

	count := 0
	seen := map[string]bool{}
	for path, item := range doc.Paths {
		for _, op := range item.ops() {
			count++
			if op.OperationID == "" {
				t.Errorf("路由 %s 缺 operationId", path)
				continue
			}
			seen[op.OperationID] = true
			// 至少一个响应带 content + schema（§23：每路由有 Schema）。
			hasSchema := false
			for _, resp := range op.Responses {
				for _, c := range resp.Content {
					if len(c.Schema) > 0 {
						hasSchema = true
					}
				}
			}
			if !hasSchema {
				t.Errorf("路由 %s (%s) 无带 Schema 的响应", path, op.OperationID)
			}
		}
	}

	// 生成的 Routes 与 openapi.yaml 路由数一致，且每个 operationId 都在生成物里。
	if len(apigen.Routes) != count {
		t.Errorf("生成的 Routes(%d) 与 openapi.yaml 路由数(%d) 不同步，请重跑 gen-contracts",
			len(apigen.Routes), count)
	}
	for _, r := range apigen.Routes {
		if !seen[r.OperationID] {
			t.Errorf("生成的 Route %s 不在 openapi.yaml 中", r.OperationID)
		}
	}
}
