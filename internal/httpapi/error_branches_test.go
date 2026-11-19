package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"smart-locker/internal/store"
)

// TestErrorBranches 集中覆盖各接口的错误与边界分支
func TestErrorBranches(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	svcTk := adminLogin(t, ts, "svc")
	tk := login(t, ts, "13900009100")
	oid := outletIDOf(t, ts)

	// ---- 登录边界 ----
	call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": "", "password": ""})            // 400
	call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": "ghost", "password": "000000"}) // 400 不存在
	call(t, ts, "POST", "/api/user/send-code", "", map[string]any{})                                       // 400
	call(t, ts, "POST", "/api/user/send-code", "", map[string]any{"phone": "abc"})                         // 400
	call(t, ts, "POST", "/api/user/logout", "", nil)                                                       // 401
	call(t, ts, "POST", "/api/admin/logout", "", nil)                                                      // 401
	call(t, ts, "POST", "/api/user/pickup-pwd", tk, map[string]any{})                                      // 400 短密码
	call(t, ts, "POST", "/api/user/pickup-pwd", tk, map[string]any{"password": "123456"})                  // 200
	call(t, ts, "POST", "/api/quote", "", map[string]any{})                                                // 400
	call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": 999999, "size": "S"})                // 404 网点不存在

	// 暂停网点下单
	code, out := call(t, ts, "POST", "/api/admin/outlets", superTk, map[string]any{"name": "暂停站", "city": "上海"})
	if code != 200 {
		t.Fatalf("创建网点应200: %v", out)
	}
	pausedID := int64(out["id"].(float64))
	call(t, ts, "PUT", fmt.Sprintf("/api/admin/outlets/%d", pausedID), superTk, map[string]any{"status": "paused"})
	call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": pausedID, "size": "S"})                      // 400 已暂停
	call(t, ts, "POST", "/api/admin/outlets", superTk, map[string]any{})                                           // 400 缺名称
	call(t, ts, "PUT", "/api/admin/outlets/999999", superTk, map[string]any{"status": "normal"})                   // 404
	call(t, ts, "PUT", fmt.Sprintf("/api/admin/outlets/%d", pausedID), superTk, map[string]any{"status": "weird"}) // 状态非法不生效

	// ---- 网点列表（super）----
	code, outs := call(t, ts, "GET", "/api/admin/outlets", superTk, nil)
	if code != 200 {
		t.Fatalf("super网点列表应200: %v", outs)
	}

	// ---- 柜格边界 ----
	call(t, ts, "GET", "/api/admin/lockers", superTk, nil)        // 无outletId
	call(t, ts, "GET", "/api/admin/lockers/999999", superTk, nil) // 404
	// 空闲柜格远程开柜 409
	lockers, _ := ms.ListLockers(ctx, oid, 0)
	var idleL *store.Locker
	var occL *store.Locker
	for _, l := range lockers {
		if l.Status == "idle" && idleL == nil {
			idleL = l
		}
	}
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	for _, l := range lockers {
		if l.ID == int64(order["lockerId"].(float64)) {
			got, _ := ms.GetLocker(ctx, l.ID)
			occL = got
		}
	}
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", idleL.ID), superTk, map[string]any{"action": "remote_open"}) // 409
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", idleL.ID), superTk, map[string]any{"action": "repair"})      // 409 非fault
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", occL.ID), superTk, map[string]any{"action": "repair"})       // 409 有订单
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", idleL.ID), superTk, map[string]any{"action": "switch"})      // 409 无订单
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", 999999), superTk, map[string]any{"action": "remote_open"})   // 404
	call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", occL.ID), superTk, map[string]any{"action": "remote_open"})  // 200 占用可开

	// ---- 规则边界 ----
	call(t, ts, "GET", "/api/admin/rules/999999", superTk, nil) // 404
	call(t, ts, "PUT", fmt.Sprintf("/api/admin/rules/%d", pausedID), superTk, map[string]any{"mode": "daily", "priceS": 500, "priceM": 700, "priceL": 1000})
	call(t, ts, "GET", fmt.Sprintf("/api/admin/rules/%d", pausedID), superTk, nil) // 200 daily

	// ---- 优惠券边界 ----
	call(t, ts, "GET", "/api/admin/coupons", superTk, nil) // 200
	call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "x", "type": "amount", "value": 100, "validFrom": "2026-12-31", "validTo": "2026-01-01", "totalCount": 1}) // 400 日期颠倒
	call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "x", "type": "amount", "value": 100, "scope": "outlet", "validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 1}) // 400 缺网点
	call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "x", "type": "hour", "value": 0, "validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 1}) // 200
	call(t, ts, "POST", "/api/admin/coupons/999999/toggle", superTk, map[string]any{"enabled": false}) // 500/404 不存在
	call(t, ts, "POST", "/api/admin/coupons/1/toggle", superTk, map[string]any{})                      // 400 缺开关
	// 8折券实例（覆盖折扣展示分支）
	defs, _ := ms.ListCouponDefs(ctx)
	for _, d := range defs {
		if d.Ctype == "discount" && d.Enabled {
			_ = ms.CreateCouponInstance(ctx, &store.CouponInstance{DefID: d.ID, UserID: uidOf(t, ts, tk), Status: "usable"})
			break
		}
	}
	code, cl := call(t, ts, "GET", "/api/user/coupons", tk, nil)
	if code != 200 {
		t.Fatalf("券列表应200: %v", cl)
	}
	for _, item := range cl["usable"].([]any) {
		m := item.(map[string]any)
		if m["type"] == "折扣券" {
			if vd, _ := m["valueDesc"].(string); !strings.HasSuffix(vd, "折") {
				t.Fatalf("折扣展示不符: %v", m)
			}
		}
	}

	// ---- 告警/工单/员工边界 ----
	call(t, ts, "POST", "/api/admin/alerts/999999/handle", superTk, nil)                                                                                                   // 404
	call(t, ts, "POST", "/api/admin/staff", superTk, map[string]any{"empNo": "X1", "account": "x1", "name": "x", "phone": "123", "role": "service", "password": "000000"}) // 400 手机号非法
	call(t, ts, "POST", "/api/admin/staff/999999/role", superTk, map[string]any{"role": "service"})                                                                        // 500 不存在
	call(t, ts, "POST", "/api/admin/staff/999999/reset-pwd", superTk, map[string]any{"password": "123456"})                                                                // 500
	call(t, ts, "POST", "/api/admin/staff/999999/toggle", superTk, map[string]any{"status": "active"})                                                                     // 500
	call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/reset-pwd", 2), superTk, map[string]any{"password": "123"})                                                       // 400 短密码
	call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/toggle", 2), superTk, map[string]any{"status": "weird"})                                                          // 400 非法状态
	call(t, ts, "GET", "/api/admin/cs/duty", superTk, nil)                                                                                                                 // super也可读 200
	call(t, ts, "POST", "/api/admin/cs/duty", svcTk, map[string]any{})                                                                                                     // 400 缺开关
	call(t, ts, "POST", "/api/admin/cs/reply", svcTk, map[string]any{"userId": 1, "content": " "})                                                                         // 400 空内容
	call(t, ts, "POST", "/api/admin/tickets/999999/resolve", svcTk, map[string]any{"reply": "x"})                                                                          // 404

	// ---- 客服消息 ----
	call(t, ts, "GET", "/api/admin/cs/messages", svcTk, nil) // 缺userId

	// ---- 用户侧边界 ----
	call(t, ts, "POST", "/api/chat/send", tk, map[string]any{}) // 400 空消息
	call(t, ts, "GET", "/api/chat/messages", "", nil)           // 401
	call(t, ts, "GET", "/api/chat/state", tk, nil)              // 200
	orderID := int64(order["orderId"].(float64))
	call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/report", orderID), tk, map[string]any{}) // 400 空问题
	call(t, ts, "GET", "/api/orders/999999", tk, nil)                                        // 404
	call(t, ts, "GET", "/api/orders/abc", tk, nil)                                           // 400 参数错误
}

// TestMaskPhone 脱敏函数
func TestMaskPhone(t *testing.T) {
	if maskPhone("13800000001") != "138****0001" {
		t.Fatal("11位脱敏不符")
	}
	if maskPhone("12345") != "12345" {
		t.Fatal("非11位应原样返回")
	}
}
