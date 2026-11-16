// Command mocktarget 是一个本地假目标服务（§22.5），让没有真实靶场时也能测越权/IDOR 等。
//
// 核心能力（Day 05）：匿名/双账号、对象归属、跨身份读取差异(401/403/200)、限流、重定向、
// 时间响应、上传测试对象、清理接口。OAST/故障注入留待后续里程碑。
//
// 默认只监听 127.0.0.1（与 §15.7 本地安全一致）。它是**测试靶子**，不是产品的一部分。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8099", "监听地址")
	flag.Parse()
	srv := newServer()
	log.Printf("mocktarget 监听 http://%s （账号: token-a / token-b）", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.handler()))
}

// order 是一个有归属的测试对象。
type order struct {
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Item   string `json:"item"`
	Amount int64  `json:"amount_micro"` // 整数子单位，避免浮点（与仓库 Q-3 口径一致）
}

type server struct {
	mu     sync.Mutex
	orders map[string]*order
	seq    int
	// 简单固定窗口限流：每账号每窗口计数。
	rlWindow time.Time
	rlCount  map[string]int
	rlLimit  int
	rlDur    time.Duration
}

// tokens 把 Bearer/X-Api-Key 令牌映射到账号。匿名则空账号。
var tokens = map[string]string{
	"token-a": "acct_a",
	"token-b": "acct_b",
}

func newServer() *server {
	s := &server{
		rlLimit: 120,
		rlDur:   time.Minute,
		rlCount: map[string]int{},
	}
	s.reset()
	return s
}

// reset 恢复初始种子数据：acct_a 拥有 order-1001，acct_b 拥有 order-2001。
func (s *server) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders = map[string]*order{
		"order-1001": {ID: "order-1001", Owner: "acct_a", Item: "widget", Amount: 1500000},
		"order-2001": {ID: "order-2001", Owner: "acct_b", Item: "gadget", Amount: 2990000},
	}
	s.seq = 2001
	s.rlCount = map[string]int{}
	s.rlWindow = time.Now()
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me", s.auth(s.whoami))
	mux.HandleFunc("GET /orders", s.auth(s.listOrders))
	mux.HandleFunc("POST /orders", s.auth(s.createOrder))
	mux.HandleFunc("GET /orders/{id}", s.auth(s.getOrder))
	mux.HandleFunc("GET /redirect", s.redirect)
	mux.HandleFunc("GET /slow", s.slow)
	mux.HandleFunc("POST /admin/reset", s.adminReset)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	return mux
}

// account 从请求解析账号；匿名返回 ""。
func account(r *http.Request) string {
	tok := r.Header.Get("X-Api-Key")
	if tok == "" {
		if a := r.Header.Get("Authorization"); len(a) > 7 && a[:7] == "Bearer " {
			tok = a[7:]
		}
	}
	return tokens[tok]
}

// auth 包装需要身份的处理器：匿名 → 401；并做每账号限流 → 429。
func (s *server) auth(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acct := account(r)
		if acct == "" {
			writeErr(w, 401, "unauthorized", "缺少或无效的身份令牌")
			return
		}
		if !s.allow(acct) {
			writeErr(w, 429, "rate_limited", "超出速率限制")
			return
		}
		next(w, r, acct)
	}
}

func (s *server) allow(acct string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.rlWindow) > s.rlDur {
		s.rlWindow = time.Now()
		s.rlCount = map[string]int{}
	}
	s.rlCount[acct]++
	return s.rlCount[acct] <= s.rlLimit
}

func (s *server) whoami(w http.ResponseWriter, _ *http.Request, acct string) {
	writeJSON(w, 200, map[string]string{"account": acct})
}

func (s *server) listOrders(w http.ResponseWriter, _ *http.Request, acct string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*order
	for _, o := range s.orders {
		if o.Owner == acct {
			out = append(out, o)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *server) createOrder(w http.ResponseWriter, r *http.Request, acct string) {
	var in struct {
		Item   string `json:"item"`
		Amount int64  `json:"amount_micro"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	s.mu.Lock()
	s.seq++
	o := &order{ID: fmt.Sprintf("order-%d", s.seq), Owner: acct, Item: in.Item, Amount: in.Amount}
	s.orders[o.ID] = o
	s.mu.Unlock()
	writeJSON(w, 201, o)
}

// getOrder 实现对象归属的越权语义：非本人对象 → 403（而非 404，便于测 IDOR 的差异）。
func (s *server) getOrder(w http.ResponseWriter, r *http.Request, acct string) {
	id := r.PathValue("id")
	s.mu.Lock()
	o, ok := s.orders[id]
	s.mu.Unlock()
	if !ok {
		writeErr(w, 404, "not_found", "对象不存在")
		return
	}
	if o.Owner != acct {
		writeErr(w, 403, "forbidden", "该对象不属于当前身份")
		return
	}
	writeJSON(w, 200, o)
}

func (s *server) redirect(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		to = "/healthz"
	}
	http.Redirect(w, r, to, http.StatusFound)
}

func (s *server) slow(w http.ResponseWriter, r *http.Request) {
	ms := 0
	fmt.Sscanf(r.URL.Query().Get("ms"), "%d", &ms)
	if ms > 5000 {
		ms = 5000
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	writeJSON(w, 200, map[string]int{"slept_ms": ms})
}

func (s *server) adminReset(w http.ResponseWriter, _ *http.Request) {
	s.reset()
	writeJSON(w, 200, map[string]string{"status": "reset"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"code": errCode, "message": msg})
}
