// Package findings 管理攻击面之上的「战场地图」：模块聚类、技术栈指纹、Discovery 线索（§12.2/§12.3/§12.4）。
package findings

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pentagent/internal/domain"
	"pentagent/internal/knowledge/surface"
	"pentagent/internal/store"
)

// 业务模块键（§12.2）。
const (
	ModUser    = "user"
	ModAuth    = "auth"
	ModOrder   = "order"
	ModPayment = "payment"
	ModFile    = "file"
	ModAdmin   = "admin"
	ModMessage = "message"
	ModReport  = "report"
	ModOther   = "other"
)

type moduleRule struct {
	key      string
	keywords []string
}

// 聚类规则按优先级排序（auth 先于 user：登录路径归认证；payment 先于 order：账单归支付）。
var moduleRules = []moduleRule{
	{ModAuth, []string{"login", "logout", "signin", "signup", "oauth", "token", "session", "password", "register", "/auth", "sso", "mfa", "otp"}},
	{ModPayment, []string{"pay", "payment", "billing", "invoice", "charge", "wallet", "refund", "checkout/pay"}},
	{ModOrder, []string{"order", "cart", "checkout", "purchase"}},
	{ModFile, []string{"file", "upload", "download", "media", "attachment", "document", "/blob"}},
	{ModAdmin, []string{"admin", "manage", "dashboard", "backoffice", "console", "/sys"}},
	{ModMessage, []string{"message", "chat", "notification", "comment", "inbox", "mail", "/im"}},
	{ModReport, []string{"report", "stats", "analytics", "export", "metric", "/bi"}},
	{ModUser, []string{"user", "profile", "account", "member", "/me", "identity"}},
}

// ClassifyModule 按 URL 模式把接口归到业务模块（§12.2，URL 前缀 + 业务语义）。无命中归 other。
func ClassifyModule(urlPattern string) string {
	lp := strings.ToLower(urlPattern)
	for _, r := range moduleRules {
		for _, kw := range r.keywords {
			if strings.Contains(lp, kw) {
				return r.key
			}
		}
	}
	return ModOther
}

// ClusterResult 是一次聚类结果。
type ClusterResult struct {
	Modules map[string][]string // module_key -> url_patterns
}

// Store 持久化模块/指纹/Discovery。
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

// ClusterEndpoints 把一批接口聚成模块并持久化（modules 表，UNIQUE(run,module_key) 去重）。
// 返回 module_key → url_patterns 映射。
func (s *Store) ClusterEndpoints(ctx context.Context, runID string, eps []surface.Endpoint, now time.Time) (ClusterResult, error) {
	res := ClusterResult{Modules: map[string][]string{}}
	for _, e := range eps {
		mk := ClassifyModule(e.URLPattern)
		res.Modules[mk] = append(res.Modules[mk], e.URLPattern)
	}
	tstr := ts(now)
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		for mk := range res.Modules {
			if _, e := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO modules (id, run_id, module_key, title, created_at)
				VALUES (?,?,?,?,?)`,
				string(domain.NewID()), runID, mk, mk, tstr); e != nil {
				return e
			}
		}
		return nil
	})
	return res, err
}

// ModuleID 取某 Run 下某 module_key 的 id。
func (s *Store) ModuleID(ctx context.Context, runID, moduleKey string) (string, error) {
	var id string
	err := s.w.Durable(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx,
			`SELECT id FROM modules WHERE run_id=? AND module_key=?`, runID, moduleKey).Scan(&id)
		if e == sql.ErrNoRows {
			return domain.Errorf(domain.ErrNotFound, "module "+moduleKey)
		}
		return e
	})
	return id, err
}
