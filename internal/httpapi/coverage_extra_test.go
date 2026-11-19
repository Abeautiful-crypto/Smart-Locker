package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"smart-locker/internal/store"
)

// TestMiscEndpoints 补齐零覆盖接口
func TestMiscEndpoints(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	tk := login(t, ts, "13900009001")
	oid := outletIDOf(t, ts)

	// 用户登出
	code, _ := call(t, ts, "POST", "/api/user/logout", tk, map[string]any{})
	if code != 200 {
		t.Fatalf("用户登出应200: %d", code)
	}
	if code, _ := call(t, ts, "GET", "/api/user/coupons", tk, nil); code != 401 {
		t.Fatal("登出后应401")
	}
	tk = login(t, ts, "13900009001")

	// 后台登出
	code, _ = call(t, ts, "POST", "/api/admin/logout", superTk, map[string]any{})
	if code != 200 {
		t.Fatalf("后台登出应200: %d", code)
	}
	if code, _ := call(t, ts, "GET", "/api/admin/dashboard", superTk, nil); code != 401 {
		t.Fatal("后台登出后应401")
	}
	superTk = adminLogin(t, ts, "super")

	// 公开收费规则
	code, rule := call(t, ts, "GET", fmt.Sprintf("/api/outlets/%d/rule", oid), "", nil)
	if code != 200 {
		t.Fatalf("公开规则应200: %v", rule)
	}
	if v, _ := rule["priceS"].(float64); v != 500 {
		t.Fatalf("规则单价不符: %v", rule)
	}
	// 不存在网点规则
	if code, _ := call(t, ts, "GET", "/api/outlets/999/rule", "", nil); code != 404 {
		t.Fatal("无规则网点应404")
	}
	// 网点详情
	code, detail := call(t, ts, "GET", fmt.Sprintf("/api/outlets/%d", oid), "", nil)
	if code != 200 {
		t.Fatalf("网点详情应200: %v", detail)
	}
	if _, ok := detail["groups"]; !ok {
		t.Fatal("详情应含柜组")
	}
	if code, _ := call(t, ts, "GET", "/api/outlets/999", "", nil); code != 404 {
		t.Fatal("不存在网点应404")
	}

	// 下单后查当前订单 + 管理订单列表
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	code, cur := call(t, ts, "GET", "/api/orders/current", tk, nil)
	if code != 200 {
		t.Fatalf("当前订单应200: %v", cur)
	}
	if arr, _ := cur["0"].([]any); arr == nil {
		// 裸数组响应：通过原始请求校验
		req, _ := http.NewRequest("GET", ts.URL+"/api/orders/current", nil)
		req.Header.Set("X-Token", tk)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal("当前订单原始请求应200")
		}
	}
	// 管理订单列表
	code, _ = call(t, ts, "GET", fmt.Sprintf("/api/admin/orders?outletId=%d&status=active", oid), superTk, nil)
	if code != 200 {
		t.Fatal("管理订单列表应200")
	}
	if code, _ := call(t, ts, "GET", "/api/admin/orders?status=finished", superTk, nil); code != 200 {
		t.Fatal("管理finished订单列表应200")
	}

	// 优惠券列表（管理端）+ 三种券类型的C端展示
	code, _ = call(t, ts, "GET", "/api/admin/coupons", superTk, nil)
	if code != 200 {
		t.Fatal("管理券列表应200")
	}
	// 新建1小时券与1元券，C端应出现 valueDesc
	code, out := call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "1小时券", "type": "hour", "value": 60, "scope": "all",
		"validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 5})
	if code != 200 {
		t.Fatalf("1小时券创建失败: %v", out)
	}
	hourDefID := int64(out["id"].(float64))
	code, out = call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "1元券", "type": "amount", "value": 100, "scope": "all",
		"validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 5})
	if code != 200 {
		t.Fatalf("1元券创建失败: %v", out)
	}
	amountDefID := int64(out["id"].(float64))
	uid := uidOf(t, ts, tk)
	_ = ms.CreateCouponInstance(ctx, &store.CouponInstance{DefID: hourDefID, UserID: uid, Status: "usable"})
	_ = ms.CreateCouponInstance(ctx, &store.CouponInstance{DefID: amountDefID, UserID: uid, Status: "usable"})
	code, list := call(t, ts, "GET", "/api/user/coupons", tk, nil)
	if code != 200 {
		t.Fatalf("C端券列表应200: %v", list)
	}
	usableArr, _ := list["usable"].([]any)
	if len(usableArr) < 2 {
		t.Fatalf("应至少2张可用券: %v", list)
	}
	names := map[string]bool{}
	for _, item := range usableArr {
		m := item.(map[string]any)
		names[m["type"].(string)] = true
	}
	if !names["时长券"] || !names["金额券"] {
		t.Fatalf("应包含时长券与金额券: %v", names)
	}

	// 客服工作台消息查询
	svcTk := adminLogin(t, ts, "svc")
	call(t, ts, "POST", "/api/chat/send", tk, map[string]any{"content": "寄存怎么收费"})
	code, msgs := call(t, ts, "GET", fmt.Sprintf("/api/admin/cs/messages?userId=%d", uid), svcTk, nil)
	if code != 200 {
		t.Fatalf("客服消息查询应200: %v", msgs)
	}
	// 工作台会话列表
	code, _ = call(t, ts, "GET", "/api/admin/cs/sessions", svcTk, nil)
	if code != 200 {
		t.Fatal("工作台会话应200")
	}

	// 订单详情（进行中）状态字段
	code, od := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID), tk, nil)
	if code != 200 {
		t.Fatalf("订单详情应200: %v", od)
	}

	// 历史订单（结算后）
	call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatal("结算应200")
	}
	code, hist := call(t, ts, "GET", "/api/orders/history", tk, nil)
	if code != 200 {
		t.Fatalf("历史订单应200: %v", hist)
	}
}

// TestSettleStatusFields 校验历史订单的结算状态分类
func TestSettleStatusFields(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	tk := login(t, ts, "13900009002")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	// 免费时长内结算 → free
	call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	code, _ := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatal("结算应200")
	}
	code, od := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID), tk, nil)
	if code != 200 {
		t.Fatal("详情应200")
	}
	if s, _ := od["settleStatus"].(string); s != "free" {
		t.Fatalf("免费订单应free: %v", od)
	}
	// 押金模式差额补交 → due
	order2 := createOrder(t, ts, tk, oid, "M", "deposit", 0)
	orderID2 := int64(order2["orderId"].(float64))
	ord2, _ := ms.GetOrder(ctx, orderID2)
	ord2.StartedAt = time.Now().Add(-5 * time.Hour)
	_ = ms.UpdateOrderSettlement(ctx, ord2)
	call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID2), tk, map[string]any{"method": "scan"})
	code, out2 := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID2), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算应200: %v", out2)
	}
	code, od2 := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID2), tk, nil)
	if code != 200 {
		t.Fatal("详情应200")
	}
	if s, _ := od2["settleStatus"].(string); s != "due" {
		t.Fatalf("需补交订单应due: %v", od2)
	}
	// 补交后 no_due
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pay-extra", orderID2), tk, map[string]any{})
	if code != 200 {
		t.Fatal("补交应200")
	}
	code, od3 := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID2), tk, nil)
	if code != 200 {
		t.Fatal("详情应200")
	}
	if s, _ := od3["settleStatus"].(string); s != "no_due" {
		t.Fatalf("补交后应no_due: %v", od3)
	}
}
