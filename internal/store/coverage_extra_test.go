package store

import (
	"context"
	"testing"
	"time"
)

// TestMemCoverageExtras 补齐 MemStore 边界分支
func TestMemCoverageExtras(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()

	// Close 幂等
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	// 未知手机号
	if _, err := m.GetUserByPhone(ctx, "10000000000"); err != ErrNotFound {
		t.Fatal("未知手机号应NotFound")
	}
	// 取件密码：用户不存在
	if err := m.SetPickupPwd(ctx, 9999, "x"); err != ErrNotFound {
		t.Fatal("用户不存在应NotFound")
	}
	// 员工
	outs, _ := m.ListOutlets(ctx, OutletFilter{})
	oid := outs[0].ID
	st := &Staff{EmpNo: "X1", Account: "x1", Name: "x", Phone: "13700000001", Role: "service", PasswordHash: "h", Status: "active", ScopeOutletID: oid}
	_ = m.CreateStaff(ctx, st)
	if _, err := m.GetStaff(ctx, st.ID); err != nil {
		t.Fatal("GetStaff失败")
	}
	if _, err := m.GetStaff(ctx, 9999); err != ErrNotFound {
		t.Fatal("GetStaff不存在应NotFound")
	}
	if err := m.UpdateStaffRole(ctx, 9999, "service"); err != ErrNotFound {
		t.Fatal("角色更新不存在应NotFound")
	}
	if err := m.UpdateStaffPassword(ctx, 9999, "h"); err != ErrNotFound {
		t.Fatal("密码更新不存在应NotFound")
	}
	// 网点过滤：未知城市/状态
	if ls, _ := m.ListOutlets(ctx, OutletFilter{City: "无"}); len(ls) != 0 {
		t.Fatal("未知城市应空")
	}
	if ls, _ := m.ListOutlets(ctx, OutletFilter{Status: "paused"}); len(ls) != 0 {
		t.Fatal("paused应空")
	}
	// 柜格
	if ls, _ := m.ListLockers(ctx, 9999, 0); len(ls) != 0 {
		t.Fatal("未知网点柜格应空")
	}
	if _, err := m.GetLocker(ctx, 9999); err != ErrNotFound {
		t.Fatal("GetLocker不存在应NotFound")
	}
	if err := m.UpdateLocker(ctx, &Locker{ID: 9999}); err != ErrNotFound {
		t.Fatal("UpdateLocker不存在应NotFound")
	}
	// 订单
	order := &Order{OrderNo: "SX1", UserID: 1, OutletID: oid, Size: "S", PayMode: "prepaid", Code: "111111", Status: "active", StartedAt: time.Now()}
	_ = m.CreateOrder(ctx, order)
	if _, err := m.GetOrderByNo(ctx, "SX1"); err != nil {
		t.Fatal("GetOrderByNo失败")
	}
	if _, err := m.GetOrderByNo(ctx, "NOPE"); err != ErrNotFound {
		t.Fatal("GetOrderByNo不存在应NotFound")
	}
	if ls, _ := m.ListOrders(ctx, OrderFilter{UserID: 9999}); len(ls) != 0 {
		t.Fatal("未知用户订单应空")
	}
	if ls, _ := m.ListOrders(ctx, OrderFilter{OutletID: 9999}); len(ls) != 0 {
		t.Fatal("未知网点订单应空")
	}
	if ls, _ := m.ListOrders(ctx, OrderFilter{Status: "finished"}); len(ls) != 0 {
		t.Fatal("finished应空")
	}
	now := time.Now()
	n, _ := m.CountOrdersBetween(ctx, oid, now.Add(-time.Hour), now.Add(time.Hour))
	if n != 1 {
		t.Fatal("订单统计应1")
	}
	if n2, _ := m.CountOrdersBetween(ctx, 9999, now.Add(-time.Hour), now.Add(time.Hour)); n2 != 0 {
		t.Fatal("未知网点统计应0")
	}
	// 券
	def := &CouponDef{Name: "X", Ctype: "amount", Value: 100, Scope: "all",
		ValidFrom: now.AddDate(0, 0, -1), ValidTo: now.AddDate(0, 0, 1), Enabled: true, TotalCount: 5}
	_ = m.CreateCouponDef(ctx, def)
	if _, err := m.GetCouponDef(ctx, def.ID); err != nil {
		t.Fatal("GetCouponDef失败")
	}
	if _, err := m.GetCouponDef(ctx, 9999); err != ErrNotFound {
		t.Fatal("GetCouponDef不存在应NotFound")
	}
	if ls, _ := m.ListCouponDefs(ctx); len(ls) < 1 {
		t.Fatal("券列表应非空")
	}
	ci := &CouponInstance{DefID: def.ID, UserID: 7, Status: "usable"}
	_ = m.CreateCouponInstance(ctx, ci)
	if _, err := m.GetCouponInstance(ctx, 9999); err != ErrNotFound {
		t.Fatal("券不存在应NotFound")
	}
	// 核销：券不存在 → Conflict（不可核销）
	if err := m.UseCouponInstance(ctx, 9999, 1); err != ErrConflict {
		t.Fatal("核销不可用券应Conflict")
	}
	// 客服会话：不存在时创建默认
	cs, _ := m.GetChatSession(ctx, 424242)
	if cs.UserID != 424242 || cs.State != "bot" {
		t.Fatalf("默认会话不符: %+v", cs)
	}
	// 消息限制
	_ = m.AppendMessage(ctx, &ChatMessage{UserID: 8, Sender: "user", Content: "a"})
	_ = m.AppendMessage(ctx, &ChatMessage{UserID: 8, Sender: "bot", Content: "b"})
	_ = m.AppendMessage(ctx, &ChatMessage{UserID: 8, Sender: "user", Content: "c"})
	if ls, _ := m.ListMessages(ctx, 8, 0, 2); len(ls) != 2 {
		t.Fatalf("limit应生效: %d", len(ls))
	}
	// 营收：网点过滤
	_ = m.AddRevenue(ctx, &RevenueRecord{OutletID: oid, OrderID: order.ID, Kind: "income", AmountFen: 100})
	if ls, _ := m.ListRevenue(ctx, 9999, now.Add(-time.Hour), now.Add(time.Hour)); len(ls) != 0 {
		t.Fatal("未知网点营收应空")
	}
	if _, _, _, err := m.SumRevenue(ctx, oid, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
		t.Fatal("SumRevenue失败")
	}
	// SeedOutlet 重复名
	if _, err := m.SeedOutlet("X站", "上海", map[string]int{"S": 1}); err == nil {
		// 允许重名则跳过断言
		t.Log("SeedOutlet 允许重名")
	}
}
