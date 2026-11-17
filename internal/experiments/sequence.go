// Package experiments 实现多步序列实验与受控竞态实验（§44.1/§44.2）——SRC 最值钱的业务逻辑/竞态漏洞。
package experiments

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// sequence.go —— 多步序列实验（§44.1）：有状态、步骤间传值。传值靠**结构化提取**（JSONPath/正则，确定性），
// 不靠模型「记住」上一步响应；每步仍独立过 Policy/Permit、各自 Identity、各自留证。

// Extract 从一步的 JSON 响应体按结构化路径取值（JSONPath-lite）。
// 支持 "body.$.data.id" / "$.data.id" / "$.items.0.id"（数组下标）。确定性，不经模型。
func Extract(body []byte, path string) (string, error) {
	p := strings.TrimPrefix(path, "body.")
	p = strings.TrimPrefix(p, "$")
	p = strings.TrimPrefix(p, ".")
	var cur any
	if err := json.Unmarshal(body, &cur); err != nil {
		return "", domain.Errorf(domain.ErrInvalid, "bad json body")
	}
	if p == "" {
		return toStr(cur), nil
	}
	for _, seg := range strings.Split(p, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return "", domain.Errorf(domain.ErrNotFound, "path segment "+seg)
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return "", domain.Errorf(domain.ErrNotFound, "array index "+seg)
			}
			cur = node[idx]
		default:
			return "", domain.Errorf(domain.ErrNotFound, "cannot descend into "+seg)
		}
	}
	return toStr(cur), nil
}

func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// Substitute 把模板里的 {name} 用已提取变量替换（确定性传值）。
func Substitute(template string, vars map[string]string) string {
	out := template
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// Step 是序列实验的一步。
type Step struct {
	ID              string
	Identity        string
	RequestTemplate string            // 含 {var} 占位
	Extract         map[string]string // 变量名 -> 提取路径
	Mutate          map[string]string // 字段 -> 新值
}

// SequenceSpec 是序列实验规格（§44.1）。
type SequenceSpec struct {
	HypothesisID     string
	Steps            []Step
	Assertion        string
	Falsifier        string
	RequiredEvidence []string
}

// StepExecutor 执行一步的实际请求（经 Policy/Permit/留证，由执行面注入），返回响应体。
type StepExecutor func(ctx context.Context, stepID, identity, renderedRequest string) (body []byte, err error)

// StepResult 是一步的执行结果。
type StepResult struct {
	StepID    string
	Rendered  string
	Extracted map[string]string
}

// Store 持久化序列/竞态实验。
type Store struct{ w *store.Writer }

// NewStore 构造。
func NewStore(w *store.Writer) *Store { return &Store{w: w} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// CreateSequence 持久化序列实验 + 其步骤。
func (s *Store) CreateSequence(ctx context.Context, runID string, spec SequenceSpec, now time.Time) (string, error) {
	id := string(domain.NewID())
	reqEv, _ := json.Marshal(spec.RequiredEvidence)
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO sequence_experiments (id, run_id, hypothesis_id, assertion, falsifier, required_evidence_json, state, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			id, runID, nz(spec.HypothesisID), nz(spec.Assertion), nz(spec.Falsifier), string(reqEv), "PROPOSED", ts(now)); e != nil {
			return e
		}
		for i, st := range spec.Steps {
			exJSON, _ := json.Marshal(st.Extract)
			muJSON, _ := json.Marshal(st.Mutate)
			if _, e := tx.ExecContext(ctx, `
				INSERT INTO sequence_steps (id, sequence_experiment_id, ordinal, identity_id, request_template, extract_json, mutate_json, created_at)
				VALUES (?,?,?,?,?,?,?,?)`,
				string(domain.NewID()), id, i, nz(st.Identity), nz(st.RequestTemplate), string(exJSON), string(muJSON), ts(now)); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// RunSequence 按序执行各步：渲染模板（代入已提取变量）→ 执行（经注入的 StepExecutor，独立 Policy/Permit）
// → 结构化提取变量供下一步。返回每步结果。确定性传值，不靠模型记忆。
func (s *Store) RunSequence(ctx context.Context, spec SequenceSpec, exec StepExecutor) ([]StepResult, error) {
	vars := map[string]string{}
	var results []StepResult
	for _, st := range spec.Steps {
		rendered := Substitute(st.RequestTemplate, vars)
		body, err := exec(ctx, st.ID, st.Identity, rendered)
		if err != nil {
			return results, err // 某步失败即停（含被 Policy/Permit 拒）
		}
		extracted := map[string]string{}
		for name, path := range st.Extract {
			v, err := Extract(body, path)
			if err != nil {
				return results, domain.Errorf(domain.ErrInvalid, "step "+st.ID+" extract "+name+": "+err.Error())
			}
			vars[name] = v
			extracted[name] = v
		}
		results = append(results, StepResult{StepID: st.ID, Rendered: rendered, Extracted: extracted})
	}
	return results, nil
}
