package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// mocktarget_test.go —— Day 05 §7 验收：对象归属的越权语义。

func do(t *testing.T, h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("X-Api-Key", token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestOrderOwnershipAuthz(t *testing.T) {
	h := newServer().handler()

	// 账号 A 读自己的订单 → 200。
	if rr := do(t, h, "GET", "/orders/order-1001", "token-a"); rr.Code != 200 {
		t.Errorf("A 读自己订单应 200, 得到 %d", rr.Code)
	}
	// 账号 A 读 B 的订单 → 403（越权被拒，这是 IDOR 靶点）。
	if rr := do(t, h, "GET", "/orders/order-2001", "token-a"); rr.Code != 403 {
		t.Errorf("A 读 B 订单应 403, 得到 %d", rr.Code)
	}
	// 账号 B 读自己的订单 → 200（对称）。
	if rr := do(t, h, "GET", "/orders/order-2001", "token-b"); rr.Code != 200 {
		t.Errorf("B 读自己订单应 200, 得到 %d", rr.Code)
	}
	// 无身份 → 401。
	if rr := do(t, h, "GET", "/orders/order-1001", ""); rr.Code != 401 {
		t.Errorf("无身份应 401, 得到 %d", rr.Code)
	}
	// 不存在的订单（本人视角）→ 404。
	if rr := do(t, h, "GET", "/orders/order-9999", "token-a"); rr.Code != 404 {
		t.Errorf("不存在订单应 404, 得到 %d", rr.Code)
	}
	// 无效令牌 → 401。
	if rr := do(t, h, "GET", "/orders/order-1001", "bogus"); rr.Code != 401 {
		t.Errorf("无效令牌应 401, 得到 %d", rr.Code)
	}
}

func TestWhoamiAndReset(t *testing.T) {
	s := newServer()
	h := s.handler()
	if rr := do(t, h, "GET", "/users/me", "token-a"); rr.Code != 200 {
		t.Errorf("whoami 应 200, 得到 %d", rr.Code)
	}
	// reset 后种子数据恢复（A 仍能读到 order-1001）。reset 不需要身份。
	if rr := do(t, h, "POST", "/admin/reset", ""); rr.Code != 200 {
		t.Errorf("reset 应 200, 得到 %d", rr.Code)
	}
	if rr := do(t, h, "GET", "/orders/order-1001", "token-a"); rr.Code != 200 {
		t.Errorf("reset 后 A 应仍能读 order-1001, 得到 %d", rr.Code)
	}
}
