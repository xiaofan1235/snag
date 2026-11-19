// Package api 是 Snag 控制面的最小 REST 服务（Day 13：Run 生命周期路由）。
//
// CLI 只调本服务、不碰库（§17/§39.1）。本服务把 HTTP 请求翻译成 control.Controller 的状态机
// 迁移。生命周期路径上挂了故障注入 Hook（§22.3），测试可在此「模拟崩溃」。
//
// 后续开发天在此扩展：鉴权/Origin 校验（§17）、Idempotency-Key（Day 11 账本接入）、
// SSE watch（Day 40）、Job/Action/审批等路由。
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"pentagent/internal/control"
	"pentagent/internal/domain"
	"pentagent/internal/faultinject"
	"pentagent/internal/store"
)

const defaultWorkspaceID = "ws-cli"

// Server 持有控制器（写/迁移）与只读池（status 读），提供 http.Handler。
type Server struct {
	ctrl *control.Controller
	rd   *store.Reader
}

// NewServer 构造服务。
func NewServer(ctrl *control.Controller, rd *store.Reader) *Server {
	return &Server{ctrl: ctrl, rd: rd}
}

// Handler 返回带全部路由的 http.Handler（Go 1.22+ 方法+路径模式）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/runs", s.listRuns)
	mux.HandleFunc("POST /v1/runs", s.createRun)
	mux.HandleFunc("GET /v1/runs/{id}/status", s.status)
	mux.HandleFunc("POST /v1/runs/{id}/start", s.lifecycle("start"))
	mux.HandleFunc("POST /v1/runs/{id}/pause", s.lifecycle("pause"))
	mux.HandleFunc("POST /v1/runs/{id}/resume", s.lifecycle("resume"))
	mux.HandleFunc("POST /v1/runs/{id}/stop", s.lifecycle("stop"))
	return mux
}

type runResp struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Mode  string `json:"mode,omitempty"`
}

// runItem 对应前端 Run 资源（§16.33）：列表与创建返回它，字段与 apps/web schema.gen.ts Run 对齐。
type runItem struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
	CreatedBy   string `json:"created_by,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// runListResp 对应前端 RunList。M1 暂无游标分页（next_cursor 省略）。
type runListResp struct {
	Items []runItem `json:"items"`
}

type errResp struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// httpStatus 把领域错误映射到稳定 HTTP 码 + 机读 code。
func httpStatus(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusConflict, "invalid_state"
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "conflict"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

func writeErr(w http.ResponseWriter, err error) {
	st, code := httpStatus(err)
	writeJSON(w, st, errResp{Error: err.Error(), Code: code})
}

// listRuns 列出 Run（§16.33 GET /v1/runs）。M1 无游标分页：按创建时间倒序取最近 200 条，
// 供前端「运行」列表页消费。只读走 Reader 池（§39.1 CLI/前端不碰写连接）。
func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	rows, err := s.rd.DB().QueryContext(r.Context(),
		`SELECT id, workspace_id, name, state, revision, COALESCE(created_by,''), created_at
		 FROM runs ORDER BY created_at DESC, id DESC LIMIT 200`)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer rows.Close()
	items := make([]runItem, 0, 16)
	for rows.Next() {
		var it runItem
		if e := rows.Scan(&it.ID, &it.WorkspaceID, &it.Name, &it.State, &it.Revision, &it.CreatedBy, &it.CreatedAt); e != nil {
			writeErr(w, e)
			return
		}
		items = append(items, it)
	}
	if e := rows.Err(); e != nil {
		writeErr(w, e)
		return
	}
	writeJSON(w, http.StatusOK, runListResp{Items: items})
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		WorkspaceID string `json:"workspace_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // 允许空体
	if body.Name == "" {
		body.Name = "cli-run"
	}
	wsID := body.WorkspaceID
	if wsID == "" {
		wsID = defaultWorkspaceID
		if e := s.ctrl.EnsureWorkspace(r.Context(), wsID, "cli-default"); e != nil {
			writeErr(w, e)
			return
		}
	}
	id, err := s.ctrl.CreateRun(r.Context(), wsID, body.Name, "cli")
	if err != nil {
		writeErr(w, err)
		return
	}
	// 返回完整 Run 资源（新建恒为 DRAFT/revision=1，见 domain.NewRun），前端据此跳转/刷新列表。
	writeJSON(w, http.StatusCreated, runItem{
		ID:          string(id),
		WorkspaceID: wsID,
		Name:        body.Name,
		State:       string(domain.RunDraft),
		Revision:    1,
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	id := domain.ID(r.PathValue("id"))
	var st string
	e := s.rd.DB().QueryRowContext(r.Context(), `SELECT state FROM runs WHERE id=?`, string(id)).Scan(&st)
	if e == sql.ErrNoRows {
		writeErr(w, domain.ErrNotFound)
		return
	}
	if e != nil {
		writeErr(w, e)
		return
	}
	writeJSON(w, http.StatusOK, runResp{ID: string(id), State: st})
}

// lifecycle 返回一个把 start/pause/resume/stop 翻成状态机事件序列的处理器。
// 每个动作可能是多步迁移（如 start = preflight→start，pause = pause→pause_drained）。
func (s *Server) lifecycle(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := domain.ID(r.PathValue("id"))

		// §22.3 故障注入点：测试可在此模拟「应用状态迁移前崩溃」。
		if e := faultinject.Hook(faultinject.BeforeRunStateApply); e != nil {
			writeJSON(w, http.StatusInternalServerError, errResp{Error: e.Error(), Code: "fault_injected"})
			return
		}

		mode := ""
		var events []control.Event
		switch action {
		case "start":
			events = []control.Event{control.EventPreflight, control.EventStart}
		case "pause":
			// §9.7 两档：now / after_current_action。M1 无真实动作，排空即时完成 → 都到 PAUSED。
			mode = r.URL.Query().Get("mode")
			if mode == "" {
				mode = "after_current_action"
			}
			if mode != "now" && mode != "after_current_action" {
				writeJSON(w, http.StatusBadRequest, errResp{Error: "mode 仅支持 now|after_current_action", Code: "bad_mode"})
				return
			}
			events = []control.Event{control.EventPause, control.EventPauseDrained}
		case "resume":
			events = []control.Event{control.EventResume, control.EventResumeReady}
		case "stop":
			events = []control.Event{control.EventStop, control.EventStopDrained}
		default:
			writeJSON(w, http.StatusNotFound, errResp{Error: "未知动作", Code: "not_found"})
			return
		}

		st, err := s.applyAll(r.Context(), id, events)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, runResp{ID: string(id), State: string(st), Mode: mode})
	}
}

// applyAll 依次施加事件，返回最终状态；任一步失败即返回该错误。
func (s *Server) applyAll(ctx context.Context, id domain.ID, events []control.Event) (domain.RunState, error) {
	var st domain.RunState
	for _, ev := range events {
		var err error
		st, err = s.ctrl.Apply(ctx, id, ev)
		if err != nil {
			return "", err
		}
	}
	return st, nil
}
