package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func newSeeded(t *testing.T) *MemStore {
	t.Helper()
	m := NewMem()
	ctx := context.Background()
	o, err := m.SeedOutlet("人民广场站", "上海", map[string]int{"S": 3, "M": 2, "L": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.UpsertRule(ctx, &Rule{OutletID: o.ID, Mode: "hourly", PriceS: 500, PriceM: 700, PriceL: 1000,
		DepositS: 1000, DepositM: 2000, DepositL: 3000, CapS: 5000, CapM: 7000, CapL: 10000, FreeMinutes: 10}); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestUsersAndSessions(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	// 用户注册
	u, err := m.CreateUser(ctx, "13900000001")
	if err != nil || u.ID == 0 {
		t.Fatalf("注册失败: %v", err)
	}
	// 重复注册
	if _, err := m.CreateUser(ctx, "13900000001"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复注册应冲突: %v", err)
	}
	// 查询
	u2, err := m.GetUserByPhone(ctx, "13900000001")
	if err != nil || u2.ID != u.ID {
		t.Fatalf("按手机号查询失败: %v", err)
	}
	u3, err := m.GetUserByID(ctx, u.ID)
	if err != nil || u3.ID != u.ID {
		t.Fatalf("按ID查询失败: %v", err)
	}
	if _, err := m.GetUserByID(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在用户应NotFound: %v", err)
	}
	// 取件密码
	if err := m.SetPickupPwd(ctx, u.ID, "abc"); err != nil {
		t.Fatal(err)
	}
	u4, _ := m.GetUserByID(ctx, u.ID)
	if u4.PickupPwd != "abc" {
		t.Fatalf("密码未保存: %s", u4.PickupPwd)
	}
	// 会话
	tk := "tok-1"
	if err := m.PutSession(ctx, &SessionRow{Token: tk, Kind: "user", RefID: u.ID, ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	sess, err := m.GetSession(ctx, tk)
	if err != nil || sess.Kind != "user" {
		t.Fatalf("会话查询失败: %v", err)
	}
	// 过期会话
	if err := m.PutSession(ctx, &SessionRow{Token: "tok-2", Kind: "user", RefID: u.ID, ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSession(ctx, "tok-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("过期会话应NotFound: %v", err)
	}
	if err := m.DeleteSession(ctx, tk); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSession(ctx, tk); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后会话应不存在: %v", err)
	}
	// 批量删除
	if err := m.PutSession(ctx, &SessionRow{Token: "tok-3", Kind: "staff", RefID: 9, ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := m.PutSession(ctx, &SessionRow{Token: "tok-4", Kind: "staff", RefID: 9, ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteSessionsFor(ctx, "staff", 9); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSession(ctx, "tok-3"); !errors.Is(err, ErrNotFound) {
		t.Fatal("批量删除失败")
	}
}

func TestStaffOps(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	st := &Staff{EmpNo: "E1", Account: "super", Name: "超管", Phone: "13800000001", Role: "super", PasswordHash: "h1", Status: "active"}
	if err := m.CreateStaff(ctx, st); err != nil {
		t.Fatal(err)
	}
	// 工号/账号/手机号冲突
	if err := m.CreateStaff(ctx, &Staff{EmpNo: "E1", Account: "x", Name: "x", Phone: "13800000009", Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("工号冲突应报错")
	}
	if err := m.CreateStaff(ctx, &Staff{EmpNo: "E2", Account: "super", Name: "x", Phone: "13800000009", Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("账号冲突应报错")
	}
	if err := m.CreateStaff(ctx, &Staff{EmpNo: "E3", Account: "y", Name: "x", Phone: "13800000001", Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("手机号冲突应报错")
	}
	got, err := m.GetStaffByIdentity(ctx, "super")
	if err != nil || got.Role != "super" {
		t.Fatalf("按账号查询失败: %v", err)
	}
	got2, err := m.GetStaffByIdentity(ctx, "13800000001")
	if err != nil || got2.ID != got.ID {
		t.Fatalf("按手机号查询失败: %v", err)
	}
	if _, err := m.GetStaffByIdentity(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在应NotFound")
	}
	list, err := m.ListStaff(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("员工列表失败: %v", err)
	}
	if err := m.UpdateStaffRole(ctx, got.ID, "site_admin"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateStaffPassword(ctx, got.ID, "newhash"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateStaffStatus(ctx, got.ID, "disabled"); err != nil {
		t.Fatal(err)
	}
	got3, _ := m.GetStaffByIdentity(ctx, "super")
	if got3.Role != "site_admin" || got3.PasswordHash != "newhash" || got3.Status != "disabled" {
		t.Fatalf("员工更新未生效: %+v", got3)
	}
	if err := m.UpdateStaffStatus(ctx, 999, "active"); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在员工应NotFound")
	}
}

func TestOutletsAndLockers(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	outs, err := m.ListOutlets(ctx, OutletFilter{City: "上海"})
	if err != nil || len(outs) != 1 {
		t.Fatalf("网点列表失败: %v", err)
	}
	oid := outs[0].ID
	o, err := m.GetOutlet(ctx, oid)
	if err != nil || o.Name != "人民广场站" {
		t.Fatalf("网点查询失败: %v", err)
	}
	if _, err := m.GetOutlet(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在网点应NotFound")
	}
	// 创建与更新
	o2 := &Outlet{Name: "郑州东站", Address: "郑州", City: "郑州", Status: "normal"}
	if err := m.CreateOutlet(ctx, o2); err != nil {
		t.Fatal(err)
	}
	o2.Status = "paused"
	if err := m.UpdateOutlet(ctx, o2); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateOutlet(ctx, &Outlet{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在网点应NotFound")
	}
	// 柜组
	groups, err := m.ListGroups(ctx, oid)
	if err != nil || len(groups) != 1 {
		t.Fatalf("柜组列表失败: %v", err)
	}
	// 柜格
	lockers, err := m.ListLockers(ctx, oid, 0)
	if err != nil || len(lockers) != 6 {
		t.Fatalf("柜格数量不符: %d %v", len(lockers), err)
	}
	lockersByGroup, _ := m.ListLockers(ctx, oid, groups[0].ID)
	if len(lockersByGroup) != 6 {
		t.Fatalf("按柜组过滤失败: %d", len(lockersByGroup))
	}
	lk := lockers[0]
	got, err := m.GetLocker(ctx, lk.ID)
	if err != nil || got.Code != lk.Code {
		t.Fatalf("柜格查询失败: %v", err)
	}
	lk.Status = "fault"
	if err := m.UpdateLocker(ctx, lk); err != nil {
		t.Fatal(err)
	}
	got2, _ := m.GetLocker(ctx, lk.ID)
	if got2.Status != "fault" {
		t.Fatal("柜格状态未更新")
	}
	// 空闲统计
	idle, err := m.CountIdleByOutlet(ctx, oid)
	if err != nil || idle != 5 {
		t.Fatalf("空闲数应5: %d %v", idle, err)
	}
	cityIdle, err := m.IdleCountByCity(ctx, "上海")
	if err != nil || cityIdle != 5 {
		t.Fatalf("城市空闲数应5: %d %v", cityIdle, err)
	}
	if cityIdle2, _ := m.IdleCountByCity(ctx, "深圳"); cityIdle2 != 0 {
		t.Fatalf("深圳空闲应为0: %d", cityIdle2)
	}
}

func TestAllocateLockerAndExhaustion(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	outs, _ := m.ListOutlets(ctx, OutletFilter{City: "上海"})
	oid := outs[0].ID
	// L格只有1个
	l1, err := m.AllocateLocker(ctx, oid, "L")
	if err != nil || l1.Status != "occupied" {
		t.Fatalf("分配L失败: %v", err)
	}
	if _, err := m.AllocateLocker(ctx, oid, "L"); !errors.Is(err, ErrNoFreeLocker) {
		t.Fatalf("L耗尽应报错: %v", err)
	}
	// 释放后可再分配
	l1.Status = "idle"
	_ = m.UpdateLocker(ctx, l1)
	if _, err := m.AllocateLocker(ctx, oid, "L"); err != nil {
		t.Fatalf("释放后再分配失败: %v", err)
	}
	// 不存在的规格
	if _, err := m.AllocateLocker(ctx, oid, "X"); !errors.Is(err, ErrNoFreeLocker) {
		t.Fatal("未知规格应无空闲")
	}
}

func TestAllocateLockerConcurrent(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	outs, _ := m.ListOutlets(ctx, OutletFilter{City: "上海"})
	oid := outs[0].ID
	// 6 个格，8 个并发抢：分配成功数必须 ≤6 且无重复
	const workers = 8
	var mu sync.Mutex
	success := 0
	seen := map[int64]bool{}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sizes := []string{"S", "M", "L"}
			l, err := m.AllocateLocker(ctx, oid, sizes[n%3])
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				if seen[l.ID] {
					t.Errorf("柜格被重复分配: %d", l.ID)
				}
				seen[l.ID] = true
			} else if !errors.Is(err, ErrNoFreeLocker) {
				t.Errorf("非预期错误: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if success != 6 {
		t.Fatalf("应恰好分配6格，实际%d", success)
	}
}

func TestRulesAndOrders(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	outs, _ := m.ListOutlets(ctx, OutletFilter{})
	oid := outs[0].ID
	rule, err := m.GetRule(ctx, oid)
	if err != nil || rule.PriceS != 500 {
		t.Fatalf("规则查询失败: %v", err)
	}
	if _, err := m.GetRule(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("无规则应NotFound")
	}
	// 覆盖更新
	if err := m.UpsertRule(ctx, &Rule{OutletID: oid, Mode: "daily", PriceS: 600, PriceM: 800, PriceL: 1100, FreeMinutes: 5}); err != nil {
		t.Fatal(err)
	}
	rule2, _ := m.GetRule(ctx, oid)
	if rule2.Mode != "daily" || rule2.PriceS != 600 {
		t.Fatal("规则覆盖失败")
	}
	// 订单
	order := &Order{OrderNo: "SL1", UserID: 1, OutletID: oid, LockerID: 3, Size: "S", PayMode: "prepaid", Status: "active", StartedAt: time.Now(), Code: "123456"}
	if err := m.CreateOrder(ctx, order); err != nil {
		t.Fatal(err)
	}
	if err := m.CreateOrder(ctx, &Order{OrderNo: "SL1", UserID: 1, OutletID: oid, Size: "S", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("订单号冲突应报错")
	}
	got, err := m.GetOrder(ctx, order.ID)
	if err != nil || got.OrderNo != "SL1" {
		t.Fatalf("订单查询失败: %v", err)
	}
	if _, err := m.GetOrder(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在订单应NotFound")
	}
	byCode, err := m.GetOrderByCode(ctx, "123456", true)
	if err != nil || byCode.ID != order.ID {
		t.Fatalf("取件码查询失败: %v", err)
	}
	// activeOnly=false 也应能查到
	if _, err := m.GetOrderByCode(ctx, "123456", false); err != nil {
		t.Fatal("结束订单取件码查询失败")
	}
	if _, err := m.GetOrderByCode(ctx, "000000", true); !errors.Is(err, ErrNotFound) {
		t.Fatal("错误取件码应NotFound")
	}
	// 过滤列表
	orders, err := m.ListOrders(ctx, OrderFilter{UserID: 1, Status: "active"})
	if err != nil || len(orders) != 1 {
		t.Fatalf("订单列表失败: %v", err)
	}
	orders2, _ := m.ListOrders(ctx, OrderFilter{OutletID: oid, Status: "finished"})
	if len(orders2) != 0 {
		t.Fatal("finished过滤应空")
	}
	// 结算更新
	order.FeeFen = 500
	order.RefundFen = 1000
	order.Status = "finished"
	order.EndedAt = time.Now()
	if err := m.UpdateOrderSettlement(ctx, order); err != nil {
		t.Fatal(err)
	}
	got2, _ := m.GetOrder(ctx, order.ID)
	if got2.FeeFen != 500 || got2.Status != "finished" || got2.EndedAt.IsZero() {
		t.Fatalf("结算未生效: %+v", got2)
	}
	if err := m.UpdateOrderSettlement(ctx, &Order{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在订单应NotFound")
	}
	// 支付与营收
	if err := m.AddPayment(ctx, &Payment{OrderNo: "SL1", Kind: "prepaid", AmountFen: 1500, Status: "paid"}); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRevenue(ctx, &RevenueRecord{OutletID: oid, OrderID: order.ID, Kind: "income", AmountFen: 500}); err != nil {
		t.Fatal(err)
	}
	inc, ref, ext, err := m.SumRevenue(ctx, 0, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil || inc != 500 || ref != 0 || ext != 0 {
		t.Fatalf("营收汇总失败: %d/%d/%d %v", inc, ref, ext, err)
	}
	recs, err := m.ListRevenue(ctx, oid, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil || len(recs) != 1 {
		t.Fatalf("营收明细失败: %v", err)
	}
}

func TestCoupons(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	def := &CouponDef{Name: "8折券", Ctype: "discount", Value: 80, Scope: "all",
		ValidFrom: time.Now().AddDate(0, 0, -1), ValidTo: time.Now().AddDate(0, 0, 1), Enabled: true, TotalCount: 10}
	if err := m.CreateCouponDef(ctx, def); err != nil {
		t.Fatal(err)
	}
	defs, _ := m.ListCouponDefs(ctx)
	if len(defs) != 1 || defs[0].Name != "8折券" {
		t.Fatalf("券定义列表失败: %v", defs)
	}
	ci := &CouponInstance{DefID: def.ID, UserID: 1, Status: "usable"}
	if err := m.CreateCouponInstance(ctx, ci); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetCouponInstance(ctx, ci.ID)
	if err != nil || got.Status != "usable" || got.Def.Name != "8折券" {
		t.Fatalf("券实例查询失败: %+v %v", got, err)
	}
	if _, err := m.GetCouponInstance(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在券应NotFound")
	}
	// 用户券列表
	uc, err := m.ListUserCoupons(ctx, 1)
	if err != nil || len(uc) != 1 {
		t.Fatalf("用户券列表失败: %v", err)
	}
	if uc2, _ := m.ListUserCoupons(ctx, 2); len(uc2) != 0 {
		t.Fatal("他人券不应出现")
	}
	// 核销（一次性）
	if err := m.UseCouponInstance(ctx, ci.ID, 77); err != nil {
		t.Fatal(err)
	}
	got2, _ := m.GetCouponInstance(ctx, ci.ID)
	if got2.Status != "used" || got2.UsedOrderID != 77 {
		t.Fatalf("核销未生效: %+v", got2)
	}
	// 重复核销应失败
	if err := m.UseCouponInstance(ctx, ci.ID, 78); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复核销应冲突: %v", err)
	}
	// 停用
	if err := m.SetCouponEnabled(ctx, def.ID, false); err != nil {
		t.Fatal(err)
	}
	defs2, _ := m.ListCouponDefs(ctx)
	if defs2[0].Enabled {
		t.Fatal("停用未生效")
	}
	if err := m.SetCouponEnabled(ctx, 999, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在券停用应NotFound")
	}
}

func TestAlertsTicketsKBChat(t *testing.T) {
	m := newSeeded(t)
	ctx := context.Background()
	// 告警
	_ = m.CreateAlert(ctx, &Alert{Title: "A1", Level: "high", OutletID: 1, Status: "open"})
	_ = m.CreateAlert(ctx, &Alert{Title: "A2", Level: "mid", OutletID: 2, Status: "open"})
	_ = m.CreateAlert(ctx, &Alert{Title: "A3", Level: "mid", OutletID: 1, Status: "handled"})
	open, _ := m.ListAlerts(ctx, 0, "open")
	if len(open) != 2 {
		t.Fatalf("open告警应2: %d", len(open))
	}
	byOutlet, _ := m.ListAlerts(ctx, 1, "")
	if len(byOutlet) != 2 {
		t.Fatalf("网点告警应2: %d", len(byOutlet))
	}
	a, err := m.GetAlert(ctx, open[0].ID)
	if err != nil || a.Title != "A2" {
		t.Fatalf("告警查询失败: %v", err)
	}
	if _, err := m.GetAlert(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在告警应NotFound")
	}
	if err := m.HandleAlert(ctx, open[0].ID, "张三"); err != nil {
		t.Fatal(err)
	}
	if err := m.HandleAlert(ctx, open[0].ID, "李四"); !errors.Is(err, ErrConflict) {
		t.Fatal("重复处理应冲突")
	}
	a2, _ := m.GetAlert(ctx, open[0].ID)
	if a2.Status != "handled" || a2.Handler != "张三" || a2.HandledAt.IsZero() {
		t.Fatalf("处理未生效: %+v", a2)
	}
	// 工单
	_ = m.CreateTicket(ctx, &Ticket{TicketNo: "TK1", UserID: 1, Question: "Q1", Status: "pending"})
	_ = m.CreateTicket(ctx, &Ticket{TicketNo: "TK2", UserID: 2, Question: "Q2", Status: "resolved"})
	pending, _ := m.ListTickets(ctx, "pending")
	if len(pending) != 1 {
		t.Fatalf("pending工单应1: %d", len(pending))
	}
	n, _ := m.CountTodayTickets(ctx, time.Now())
	if n != 2 {
		t.Fatalf("今日工单应2: %d", n)
	}
	tk := pending[0]
	tk.Status = "resolved"
	tk.Reply = "done"
	tk.ResolvedAt = time.Now()
	if err := m.UpdateTicket(ctx, tk); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateTicket(ctx, &Ticket{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在工单应NotFound")
	}
	// 知识库
	m.SeedKB("收费", "收费,价格", "5元/小时")
	kb, _ := m.ListKB(ctx)
	if len(kb) != 1 {
		t.Fatalf("知识库应1条: %d", len(kb))
	}
	// 客服消息
	_ = m.AppendMessage(ctx, &ChatMessage{UserID: 1, Sender: "user", Content: "hi"})
	_ = m.AppendMessage(ctx, &ChatMessage{UserID: 1, Sender: "bot", Content: "hello"})
	msgs, _ := m.ListMessages(ctx, 1, 0, 10)
	if len(msgs) != 2 {
		t.Fatalf("消息数应2: %d", len(msgs))
	}
	msgs2, _ := m.ListMessages(ctx, 1, msgs[0].ID, 10)
	if len(msgs2) != 1 {
		t.Fatalf("afterId过滤失败: %d", len(msgs2))
	}
	if msgs3, _ := m.ListMessages(ctx, 2, 0, 10); len(msgs3) != 0 {
		t.Fatal("他人消息不应出现")
	}
	// 会话状态
	_ = m.SetChatState(ctx, 1, "waiting")
	cs, _ := m.GetChatSession(ctx, 1)
	if cs.State != "waiting" {
		t.Fatalf("状态未生效: %s", cs.State)
	}
	_ = m.SetChatState(ctx, 1, "agent")
	cs2, _ := m.GetChatSession(ctx, 1)
	if cs2.State != "agent" {
		t.Fatal("状态未生效2")
	}
	agents, _ := m.ListAgentSessions(ctx)
	if len(agents) != 1 {
		t.Fatalf("人工会话应1: %d", len(agents))
	}
}

func TestMemStoreImplementsStore(t *testing.T) {
	var _ Store = NewMem()
}
