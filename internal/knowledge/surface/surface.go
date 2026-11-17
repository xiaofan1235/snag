// Package surface 是攻击面发现的领域模型与持久化（§12.1 资产与接口发现）。
//
// 攻击面 = 可测的接口集合。本包负责：接口归一化、路径参数模板化、来源标记（static/runtime/spec/
// inferred/user）、去重入库（endpoints/endpoint_parameters），以及「新接口提升 SurfaceRevision」——
// 旧覆盖率不能继续冒充完整（§12.1）。
//
// 网络关闭下的有据偏离：JS AST 本应用 tree-sitter（CGO + 需下载语法），浏览器运行时捕获本应用 CDP/
// Playwright/rod（需浏览器与网络），本环境均不可得。故 JS 提取退化为 stdlib 正则启发式（runners/js），
// 运行时捕获退化为「摄取已捕获请求」的接口（runners/browser）；提取器都做成可插拔，具备环境后可换真实
// AST/CDP 实现而不动本模型。
package surface

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/store"
)

// Source 是接口/参数的来源标记（§12.1）。
type Source string

const (
	SrcStatic   Source = "static"   // JS/HTML 静态提取
	SrcRuntime  Source = "runtime"  // 浏览器运行时 XHR/Fetch 捕获
	SrcSpec     Source = "spec"     // OpenAPI/GraphQL 规格驱动
	SrcInferred Source = "inferred" // 推测（仍需 Policy 判定）
	SrcUser     Source = "user"     // 用户提供
)

// ParamLocation 是参数位置。
type ParamLocation string

const (
	InQuery  ParamLocation = "query"
	InBody   ParamLocation = "body"
	InHeader ParamLocation = "header"
	InPath   ParamLocation = "path"
	InCookie ParamLocation = "cookie"
)

// Param 是一个接口参数。
type Param struct {
	Name      string
	Location  ParamLocation
	ValueType string
	Required  bool
}

// Endpoint 是一个归一化后的接口。
type Endpoint struct {
	Method     string // 大写
	URLPattern string // 归一化 + 路径参数模板化后的模式（去重键的一部分）
	Path       string // 原始路径（未模板化，留痕）
	Source     Source
	Params     []Param
}

var (
	reDigits = regexp.MustCompile(`^\d+$`)
	reUUID   = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reHex    = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
)

// TemplatizePath 把路径里的 ID 段模板化（§12.1 路径参数模板化）：纯数字→{id}、UUID→{uuid}、长 hex→{hash}。
// 查询串剥离（归一化按 path 去重）。
func TemplatizePath(rawPath string) string {
	if i := strings.IndexByte(rawPath, '?'); i >= 0 {
		rawPath = rawPath[:i]
	}
	if i := strings.IndexByte(rawPath, '#'); i >= 0 {
		rawPath = rawPath[:i]
	}
	segs := strings.Split(rawPath, "/")
	for i, s := range segs {
		switch {
		case s == "":
			continue
		case reDigits.MatchString(s):
			segs[i] = "{id}"
		case reUUID.MatchString(s):
			segs[i] = "{uuid}"
		case reHex.MatchString(s):
			segs[i] = "{hash}"
		}
	}
	out := strings.Join(segs, "/")
	if out == "" {
		return "/"
	}
	return out
}

// Normalize 归一化一个 Endpoint：方法大写、路径模板化成 URLPattern。
func Normalize(e Endpoint) Endpoint {
	e.Method = strings.ToUpper(strings.TrimSpace(e.Method))
	if e.Method == "" {
		e.Method = "GET"
	}
	if e.Path == "" {
		e.Path = e.URLPattern
	}
	e.URLPattern = TemplatizePath(e.Path)
	return e
}

// Store 持久化攻击面（endpoints/endpoint_parameters）并维护 SurfaceRevision。
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

// AddResult 报告一次入库结果。
type AddResult struct {
	AddedEndpoints  int
	AddedParams     int
	SurfaceRevision int // 入库后当前 SurfaceRevision（有新增则已 +1）
}

// AddEndpoints 归一化去重入库一批接口；有新接口或新参数则提升 SurfaceRevision（§12.1）。
// 同一事务原子：去重插入 + 新增则 bump。返回新增计数与当前 SurfaceRevision。
func (s *Store) AddEndpoints(ctx context.Context, runID, assetID string, eps []Endpoint, reason string, now time.Time) (AddResult, error) {
	var res AddResult
	tstr := ts(now)
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		for _, raw := range eps {
			e := Normalize(raw)
			// 去重插入接口（唯一索引 run_id+method+url_pattern）。
			id := string(domain.NewID())
			r, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO endpoints (id, run_id, asset_id, method, url_pattern, path, discovered_via, first_seen_at, metadata_json)
				VALUES (?,?,?,?,?,?,?,?,?)`,
				id, runID, nz(assetID), e.Method, e.URLPattern, e.Path, string(e.Source), tstr, nil)
			if err != nil {
				return err
			}
			inserted, _ := r.RowsAffected()
			if inserted > 0 {
				res.AddedEndpoints++
			}
			// 取该接口 id（新插入用 id，否则查既有）。
			epID := id
			if inserted == 0 {
				if e2 := tx.QueryRowContext(ctx,
					`SELECT id FROM endpoints WHERE run_id=? AND method=? AND url_pattern=?`,
					runID, e.Method, e.URLPattern).Scan(&epID); e2 != nil {
					return e2
				}
			}
			// 参数去重插入（UNIQUE(endpoint_id,name,location)）。
			for _, p := range e.Params {
				pr, err := tx.ExecContext(ctx, `
					INSERT OR IGNORE INTO endpoint_parameters (id, endpoint_id, name, location, value_type, required, metadata_json)
					VALUES (?,?,?,?,?,?,?)`,
					string(domain.NewID()), epID, p.Name, string(p.Location), nz(p.ValueType), b2i(p.Required), nil)
				if err != nil {
					return err
				}
				if n, _ := pr.RowsAffected(); n > 0 {
					res.AddedParams++
				}
			}
		}
		// 当前 SurfaceRevision。
		cur := 0
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(surface_revision),0) FROM coverage_revisions WHERE run_id=?`, runID).Scan(&cur)
		// 有新增 → 提升（新接口会提升 SurfaceRevision，§12.1）。
		if res.AddedEndpoints > 0 || res.AddedParams > 0 {
			cur++
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO coverage_revisions (id, run_id, surface_revision, reason, created_at)
				VALUES (?,?,?,?,?)`,
				string(domain.NewID()), runID, cur, nz(reason), tstr); err != nil {
				return err
			}
		}
		res.SurfaceRevision = cur
		return nil
	})
	return res, err
}

// CurrentSurfaceRevision 返回某 Run 当前 SurfaceRevision（无则 0）。
func (s *Store) CurrentSurfaceRevision(ctx context.Context, runID string) (int, error) {
	var cur int
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(surface_revision),0) FROM coverage_revisions WHERE run_id=?`, runID).Scan(&cur)
	})
	return cur, err
}

// CountEndpoints 返回某 Run 的接口总数（测试/覆盖率用）。
func (s *Store) CountEndpoints(ctx context.Context, runID string) (int, error) {
	var n int
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM endpoints WHERE run_id=?`, runID).Scan(&n)
	})
	return n, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
