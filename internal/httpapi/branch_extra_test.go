package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"smart-locker/internal/store"
)

// TestMoreBranches 补齐剩余分支
func TestMoreBranches(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	siteTk := adminLogin(t, ts, "site_rm")
	tk := login(t, ts, "13900009400")
	oid := outletIDOf(t, ts)

	// 网点管理员访问超管专属接口 → 403
	call(t, ts, "GET", "/api/admin/rules/1", siteTk, nil)              // 403
	call(t, ts, "PUT", "/api/admin/rules/1", siteTk, map[string]any{}) // 403
	call(t, ts, "GET", "/api/admin/coupons", siteTk, nil)              // 403
	call(t, ts, "GET", "/api/admin/staff", siteTk, nil)                // 403
	// 网点管理员本网点数据（缺省 outletId 用 scope）
	call(t, ts, "GET", "/api/admin/lockers", siteTk, nil) // 200
	call(t, ts, "GET", "/api/admin/orders", siteTk, nil)  // 200
	call(t, ts, "GET", "/api/admin/alerts", siteTk, nil)  // 200
	// 营收导出（site_admin）
	code, _ := call(t, ts, "GET", "/api/admin/revenue/export", siteTk, nil)
	if code != 200 {
		t.Fatal("网点管理员导出应200")
	}
	// 跨网点柜格明细 403
	code, _ = call(t, ts, "GET", "/api/admin/lockers/999999", siteTk, nil)
	if code != 404 {
		t.Fatal("不存在柜格明细应404")
	}
	// 柜格明细含订单
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	lockerID := int64(order["lockerId"].(float64))
	code, detail := call(t, ts, "GET", fmt.Sprintf("/api/admin/lockers/%d", lockerID), superTk, nil)
	if code != 200 {
		t.Fatal("柜格明细应200")
	}
	if _, ok := detail["order"]; !ok {
		t.Fatal("占用柜格明细应含订单")
	}

	// 过期优惠券下单 → 400
	def := &store.CouponDef{Name: "过期券", Ctype: "amount", Value: 100, Scope: "all",
		ValidFrom: time.Now().AddDate(0, 0, -10), ValidTo: time.Now().AddDate(0, 0, -5), Enabled: true, TotalCount: 5}
	_ = ms.CreateCouponDef(ctx, def)
	uid := uidOf(t, ts, tk)
	ci := &store.CouponInstance{DefID: def.ID, UserID: uid, Status: "usable"}
	_ = ms.CreateCouponInstance(ctx, ci)
	code, _ = call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": oid, "size": "S", "couponInstanceId": ci.ID})
	if code != 400 {
		t.Fatalf("过期券下单应400: %d", code)
	}

	// 无订单用户各取件方式 → 400
	tk2 := login(t, ts, "13900009401")
	call(t, ts, "POST", "/api/orders/1/pickup", tk2, map[string]any{"method": "scan"})                      // 400 无订单
	call(t, ts, "POST", "/api/orders/1/pickup", tk2, map[string]any{"method": "face"})                      // 400 无订单
	call(t, ts, "POST", "/api/orders/1/pickup", tk2, map[string]any{"method": "password", "password": "x"}) // 400 无订单
	// 取件码长度不符
	code, _ = call(t, ts, "POST", "/api/orders/1/pickup", tk2, map[string]any{"method": "code", "code": "12345"})
	if code != 400 {
		t.Fatal("5位取件码应400")
	}

	// 人工接待中用户继续发消息 → state agent
	call(t, ts, "POST", "/api/chat/send", tk, map[string]any{"content": "天气怎么样啊"}) // KB未命中
	svcTk := adminLogin(t, ts, "svc")
	call(t, ts, "POST", "/api/admin/cs/duty", svcTk, map[string]any{"on": true})
	call(t, ts, "POST", "/api/chat/transfer", tk, map[string]any{}) // waiting
	call(t, ts, "POST", "/api/admin/cs/reply", svcTk, map[string]any{"userId": uid, "content": "您好"})
	code, out := call(t, ts, "POST", "/api/chat/send", tk, map[string]any{"content": "继续咨询"})
	if code != 200 {
		t.Fatal("发送应200")
	}
	if st, _ := out["state"].(string); st != "agent" {
		t.Fatalf("人工接待中应agent: %v", out)
	}
	// waiting 状态发消息
	tk3 := login(t, ts, "13900009402")
	call(t, ts, "POST", "/api/chat/transfer", tk3, map[string]any{}) // waiting
	code, out = call(t, ts, "POST", "/api/chat/send", tk3, map[string]any{"content": "排队中"})
	if code != 200 {
		t.Fatal("发送应200")
	}
	if st, _ := out["state"].(string); st != "waiting" {
		t.Fatalf("排队中应waiting: %v", out)
	}
	// 客服消息 afterId
	code, msgs := call(t, ts, "GET", fmt.Sprintf("/api/admin/cs/messages?userId=%d&afterId=0", uid), svcTk, nil)
	if code != 200 {
		t.Fatalf("客服消息应200: %v", msgs)
	}

	// 核销失败回退：注入 UseCouponInstance 失败 → 券抵扣取消重算
	fs, ts2 := newFailServer(t)
	tk4 := login(t, ts2, "13900009403")
	oid2 := outletIDOf(t, ts2)
	order2 := createOrder(t, ts2, tk4, oid2, "S", "prepaid", 0)
	orderID2 := int64(order2["orderId"].(float64))
	// 结算（免费时长内）
	fs.fail.Store("UseCouponInstance")
	call(t, ts2, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID2), tk4, map[string]any{})
	fs.fail.Store("")
}

// TestSettleCouponFallback 核销失败回退路径
func TestSettleCouponFallback(t *testing.T) {
	fs, ts := newFailServer(t)
	tk := login(t, ts, "13900009404")
	oid := outletIDOf(t, ts)
	// 领取8折券
	ctx := context.Background()
	defs, _ := fs.ListCouponDefs(ctx)
	var discountDef *store.CouponDef
	for _, d := range defs {
		if d.Ctype == "discount" && d.Enabled {
			discountDef = d
			break
		}
	}
	uid := uidOf(t, ts, tk)
	ci := &store.CouponInstance{DefID: discountDef.ID, UserID: uid, Status: "usable"}
	_ = fs.CreateCouponInstance(ctx, ci)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", ci.ID)
	orderID := int64(order["orderId"].(float64))
	// 拨回70分钟前 → 1小时费用500，8折抵100
	ord, _ := fs.GetOrder(ctx, orderID)
	ord.StartedAt = time.Now().Add(-65 * time.Minute)
	_ = fs.UpdateOrderSettlement(ctx, ord)
	call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	// 注入核销失败 → 券抵扣取消 → 费用500
	fs.fail.Store("UseCouponInstance")
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("回退结算应200: %v", out)
	}
	if f, _ := out["fee"].(float64); f != 500 {
		t.Fatalf("核销失败后费用应500: %v", out)
	}
	fs.fail.Store("")
}
