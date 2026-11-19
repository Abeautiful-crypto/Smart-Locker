package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const testDBName = "smart_locker_test"

// newTestMySQL 建独立测试库并执行迁移；MySQL 不可用时跳过
func newTestMySQL(t *testing.T) *MySQLStore {
	t.Helper()
	dsn := os.Getenv("LOCKER_TEST_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true"
	}
	base, err := openDB(dsn)
	if err != nil {
		t.Skipf("本地 MySQL 不可用，跳过集成测试: %v", err)
	}
	_, _ = base.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	_, err = base.Exec(fmt.Sprintf("CREATE DATABASE %s DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", testDBName))
	if err != nil {
		t.Fatalf("建测试库失败: %v", err)
	}
	_ = base.Close()

	st, err := NewMySQL(fmt.Sprintf("%s/%s?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true", serverOf(dsn), testDBName))
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	if err := st.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// serverOf 提取 DSN 的服务器部分（/dbname 之前）
func serverOf(dsn string) string {
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			return dsn[:i]
		}
	}
	return dsn
}

func seedMySQLOutlet(t *testing.T, st *MySQLStore) int64 {
	t.Helper()
	ctx := context.Background()
	o := &Outlet{Name: fmt.Sprintf("测试站%d", time.Now().UnixNano()%100000), Address: "上海市黄浦区", City: "上海", Status: "normal", Lat: 31.23, Lng: 121.47}
	if err := st.CreateOutlet(ctx, o); err != nil {
		t.Fatal(err)
	}
	g := &Group{OutletID: o.ID, Name: "A组"}
	if err := st.CreateGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	for i, size := range []string{"S", "S", "S", "M", "M", "L"} {
		l := &Locker{OutletID: o.ID, GroupID: g.ID, Code: fmt.Sprintf("A-%03d", i+1), Size: size, Status: "idle"}
		if err := st.CreateLocker(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpsertRule(ctx, &Rule{OutletID: o.ID, Mode: "hourly", PriceS: 500, PriceM: 700, PriceL: 1000,
		DepositS: 1000, DepositM: 2000, DepositL: 3000, CapS: 5000, CapM: 7000, CapL: 10000, FreeMinutes: 10}); err != nil {
		t.Fatal(err)
	}
	return o.ID
}

func TestMySQLUsersSessionsStaff(t *testing.T) {
	st := newTestMySQL(t)
	ctx := context.Background()
	u, err := st.CreateUser(ctx, "13900000001")
	if err != nil || u.ID == 0 {
		t.Fatalf("注册失败: %v", err)
	}
	if _, err := st.CreateUser(ctx, "13900000001"); !errors.Is(err, ErrConflict) {
		t.Fatal("重复注册应冲突")
	}
	u2, _ := st.GetUserByPhone(ctx, "13900000001")
	if u2.ID != u.ID {
		t.Fatal("手机号查询不符")
	}
	u3, _ := st.GetUserByID(ctx, u.ID)
	if u3.ID != u.ID {
		t.Fatal("ID查询不符")
	}
	if err := st.SetPickupPwd(ctx, u.ID, "hash1"); err != nil {
		t.Fatal(err)
	}
	u4, _ := st.GetUserByID(ctx, u.ID)
	if u4.PickupPwd != "hash1" {
		t.Fatal("密码未保存")
	}
	// 会话
	if err := st.PutSession(ctx, &SessionRow{Token: "t1", Kind: "user", RefID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	sess, err := st.GetSession(ctx, "t1")
	if err != nil || sess.Kind != "user" {
		t.Fatalf("会话查询失败: %v", err)
	}
	if err := st.PutSession(ctx, &SessionRow{Token: "t2", Kind: "staff", RefID: 1, ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatalf("插入过期会话失败: %v", err)
	}
	if _, err := st.GetSession(ctx, "t2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("过期会话应NotFound: %v", err)
	}
	_ = st.PutSession(ctx, &SessionRow{Token: "t3", Kind: "staff", RefID: 5, ExpiresAt: time.Now().Add(time.Hour)})
	_ = st.DeleteSessionsFor(ctx, "staff", 5)
	if _, err := st.GetSession(ctx, "t3"); !errors.Is(err, ErrNotFound) {
		t.Fatal("批量删除失败")
	}
	_ = st.DeleteSession(ctx, "t1")
	if _, err := st.GetSession(ctx, "t1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除会话失败")
	}
	// 员工（种子数据已有 super 等账号，测试用唯一前缀）
	uniq := time.Now().UnixNano() % 1000000
	stf := &Staff{EmpNo: fmt.Sprintf("T%d", uniq), Account: fmt.Sprintf("tsuper%d", uniq), Name: "测试超管", Phone: fmt.Sprintf("138%07d", uniq), Role: "super", PasswordHash: "h", Status: "active", ScopeOutletID: 0}
	if err := st.CreateStaff(ctx, stf); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateStaff(ctx, &Staff{EmpNo: stf.EmpNo, Account: "ta2" + fmt.Sprint(uniq), Name: "x", Phone: "139" + fmt.Sprint(uniq), Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("工号冲突应报错")
	}
	if err := st.CreateStaff(ctx, &Staff{EmpNo: "T2" + fmt.Sprint(uniq), Account: stf.Account, Name: "x", Phone: "137" + fmt.Sprint(uniq), Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("账号冲突应报错")
	}
	if err := st.CreateStaff(ctx, &Staff{EmpNo: "T3" + fmt.Sprint(uniq), Account: "ta3" + fmt.Sprint(uniq), Name: "x", Phone: stf.Phone, Role: "service", PasswordHash: "h", Status: "active"}); !errors.Is(err, ErrConflict) {
		t.Fatal("手机号冲突应报错")
	}
	got, err := st.GetStaffByIdentity(ctx, stf.Account)
	if err != nil || got.Role != "super" {
		t.Fatalf("员工查询失败: %v", err)
	}
	got2, err := st.GetStaffByIdentity(ctx, stf.Phone)
	if err != nil || got2.ID != got.ID {
		t.Fatal("手机号查询员工失败")
	}
	if _, err := st.GetStaffByIdentity(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在员工应NotFound")
	}
	if err := st.UpdateStaffRole(ctx, got.ID, "service"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateStaffPassword(ctx, got.ID, "h2"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateStaffStatus(ctx, got.ID, "disabled"); err != nil {
		t.Fatal(err)
	}
	got3, _ := st.GetStaffByIdentity(ctx, stf.Account)
	if got3.Role != "service" || got3.PasswordHash != "h2" || got3.Status != "disabled" {
		t.Fatalf("员工更新未生效: %+v", got3)
	}
	if err := st.UpdateStaffStatus(ctx, 999, "active"); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在员工应NotFound")
	}
	list, _ := st.ListStaff(ctx)
	if len(list) < 1 {
		t.Fatalf("员工列表至少1: %d", len(list))
	}
}

func TestMySQLOutletsLockersAllocate(t *testing.T) {
	st := newTestMySQL(t)
	ctx := context.Background()
	oid := seedMySQLOutlet(t, st)
	// 列表过滤（种子含8个演示网点，断言相对数量）
	outs, err := st.ListOutlets(ctx, OutletFilter{City: "上海", Status: "normal"})
	if err != nil || len(outs) < 1 {
		t.Fatalf("网点列表失败: %v", err)
	}
	found := false
	for _, o := range outs {
		if o.ID == oid {
			found = true
		}
	}
	if !found {
		t.Fatal("新建网点应出现在上海列表")
	}
	if outs2, _ := st.ListOutlets(ctx, OutletFilter{City: "不存在的城市"}); len(outs2) != 0 {
		t.Fatal("未知城市过滤应空")
	}
	o, err := st.GetOutlet(ctx, oid)
	if err != nil || o.ID != oid || o.City != "上海" {
		t.Fatalf("网点查询失败: %v", err)
	}
	if _, err := st.GetOutlet(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在网点应NotFound")
	}
	o.Status = "paused"
	if err := st.UpdateOutlet(ctx, o); err != nil {
		t.Fatal(err)
	}
	o2, _ := st.GetOutlet(ctx, oid)
	if o2.Status != "paused" {
		t.Fatal("网点更新未生效")
	}
	if err := st.UpdateOutlet(ctx, &Outlet{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在网点应NotFound")
	}
	// 柜格
	lockers, err := st.ListLockers(ctx, oid, 0)
	if err != nil || len(lockers) != 6 {
		t.Fatalf("柜格列表应6: %d %v", len(lockers), err)
	}
	lk := lockers[0]
	got, err := st.GetLocker(ctx, lk.ID)
	if err != nil || got.Code != lk.Code {
		t.Fatalf("柜格查询失败: %v", err)
	}
	if _, err := st.GetLocker(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在柜格应NotFound")
	}
	lk.Status = "occupied"
	lk.CurrentOrderID = 42
	if err := st.UpdateLocker(ctx, lk); err != nil {
		t.Fatal(err)
	}
	got2, _ := st.GetLocker(ctx, lk.ID)
	if got2.Status != "occupied" || got2.CurrentOrderID != 42 {
		t.Fatal("柜格更新未生效")
	}
	// 空闲统计
	idle, _ := st.CountIdleByOutlet(ctx, oid)
	if idle != 5 {
		t.Fatalf("空闲应5: %d", idle)
	}
	cityIdle, _ := st.IdleCountByCity(ctx, "上海")
	if cityIdle < 5 {
		t.Fatalf("城市空闲至少5: %d", cityIdle)
	}
	// 分配
	l, err := st.AllocateLocker(ctx, oid, "S")
	if err != nil || l.Status != "occupied" {
		t.Fatalf("分配失败: %v", err)
	}
	// L格仅1个，分配后即耗尽
	lL, err := st.AllocateLocker(ctx, oid, "L")
	if err != nil || lL.Status != "occupied" {
		t.Fatalf("分配L失败: %v", err)
	}
	if _, err := st.AllocateLocker(ctx, oid, "L"); !errors.Is(err, ErrNoFreeLocker) {
		t.Fatal("L耗尽应报错")
	}
	// 恢复
	l.Status = "idle"
	l.CurrentOrderID = 0
	_ = st.UpdateLocker(ctx, l)
	lL.Status = "idle"
	_ = st.UpdateLocker(ctx, lL)
	if _, err := st.AllocateLocker(ctx, oid, "L"); err != nil {
		t.Fatalf("恢复后再分配失败: %v", err)
	}
	// 并发分配不超卖
	st2 := newTestMySQL(t)
	oid2 := seedMySQLOutlet(t, st2)
	var mu sync.Mutex
	success := 0
	seen := map[int64]bool{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sizes := []string{"S", "M", "L"}
			lk2, err := st2.AllocateLocker(ctx, oid2, sizes[n%3])
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				if seen[lk2.ID] {
					t.Errorf("柜格重复分配: %d", lk2.ID)
				}
				seen[lk2.ID] = true
			} else if !errors.Is(err, ErrNoFreeLocker) {
				t.Errorf("非预期错误: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if success != 6 {
		t.Fatalf("并发应恰好分配6: %d", success)
	}
}

func TestMySQLOrdersRevenue(t *testing.T) {
	st := newTestMySQL(t)
	ctx := context.Background()
	oid := seedMySQLOutlet(t, st)
	orderNo := fmt.Sprintf("SLT%d", time.Now().UnixNano()%1000000)
	order := &Order{OrderNo: orderNo, UserID: 1, OutletID: oid, Size: "S", PayMode: "prepaid",
		PrepaidFen: 1500, DepositFen: 1000, Code: fmt.Sprintf("%06d", time.Now().UnixNano()%1000000), Status: "active", StartedAt: time.Now()}
	if err := st.CreateOrder(ctx, order); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateOrder(ctx, &Order{OrderNo: orderNo, UserID: 1, OutletID: oid, Size: "S", PayMode: "prepaid", Status: "active", StartedAt: time.Now()}); !errors.Is(err, ErrConflict) {
		t.Fatalf("订单号冲突应报错: %v", err)
	}
	got, err := st.GetOrder(ctx, order.ID)
	if err != nil || got.OrderNo != orderNo || got.PrepaidFen != 1500 {
		t.Fatalf("订单查询失败: %+v %v", got, err)
	}
	if got.EndedAt.Year() >= 2000 {
		t.Fatal("进行中订单 EndedAt 应为零值")
	}
	if _, err := st.GetOrder(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在订单应NotFound")
	}
	byCode, err := st.GetOrderByCode(ctx, order.Code, true)
	if err != nil || byCode.ID != order.ID {
		t.Fatalf("取件码查询失败: %v", err)
	}
	// 结束后 activeOnly 查不到
	order.Status = "finished"
	order.EndedAt = time.Now()
	order.FeeFen = 500
	order.RefundFen = 1000
	order.SettleNote = "退款1000"
	if err := st.UpdateOrderSettlement(ctx, order); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOrderByCode(ctx, order.Code, true); !errors.Is(err, ErrNotFound) {
		t.Fatal("结束后 activeOnly 应NotFound")
	}
	if _, err := st.GetOrderByCode(ctx, order.Code, false); err != nil {
		t.Fatal("结束后仍应可查")
	}
	got2, _ := st.GetOrder(ctx, order.ID)
	if got2.FeeFen != 500 || got2.RefundFen != 1000 || got2.EndedAt.IsZero() || got2.SettleNote != "退款1000" {
		t.Fatalf("结算未生效: %+v", got2)
	}
	if err := st.UpdateOrderSettlement(ctx, &Order{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在订单应NotFound")
	}
	// 过滤
	if ls, _ := st.ListOrders(ctx, OrderFilter{UserID: 1, Status: "finished"}); len(ls) != 1 {
		t.Fatal("finished列表应1")
	}
	if ls, _ := st.ListOrders(ctx, OrderFilter{OutletID: oid, Status: "active"}); len(ls) != 0 {
		t.Fatal("active列表应0")
	}
	// 支付流水
	if err := st.AddPayment(ctx, &Payment{OrderNo: got2.OrderNo, Kind: "prepaid", AmountFen: 1500, Channel: "wechat_mock", Status: "paid"}); err != nil {
		t.Fatal(err)
	}
	// 营收
	now := time.Now()
	from := now.Add(-time.Hour)
	_ = st.AddRevenue(ctx, &RevenueRecord{OutletID: oid, OrderID: order.ID, Kind: "income", AmountFen: 500})
	_ = st.AddRevenue(ctx, &RevenueRecord{OutletID: oid, OrderID: order.ID, Kind: "refund", AmountFen: 1000})
	inc, ref, ext, err := st.SumRevenue(ctx, 0, from, now.Add(time.Hour))
	if err != nil || inc != 500 || ref != 1000 {
		t.Fatalf("营收汇总失败: %d/%d/%d %v", inc, ref, ext, err)
	}
	recs, err := st.ListRevenue(ctx, oid, from, now.Add(time.Hour))
	if err != nil || len(recs) != 2 {
		t.Fatalf("营收明细应2: %d %v", len(recs), err)
	}
	// 今日订单数
	n, err := st.CountOrdersBetween(ctx, 0, from, now.Add(time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("订单统计应1: %d %v", n, err)
	}
}

func TestMySQLRulesCoupons(t *testing.T) {
	st := newTestMySQL(t)
	ctx := context.Background()
	oid := seedMySQLOutlet(t, st)
	// 规则覆盖
	if err := st.UpsertRule(ctx, &Rule{OutletID: oid, Mode: "daily", PriceS: 600, PriceM: 800, PriceL: 1100, FreeMinutes: 5}); err != nil {
		t.Fatal(err)
	}
	rule, err := st.GetRule(ctx, oid)
	if err != nil || rule.Mode != "daily" || rule.PriceS != 600 {
		t.Fatalf("规则覆盖失败: %+v %v", rule, err)
	}
	if _, err := st.GetRule(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("无规则应NotFound")
	}
	// 优惠券
	def := &CouponDef{Name: fmt.Sprintf("测试8折券%d", time.Now().UnixNano()%100000), Ctype: "discount", Value: 80, Scope: "outlet", OutletID: oid,
		ValidFrom: time.Now().AddDate(0, 0, -1), ValidTo: time.Now().AddDate(0, 0, 1), Enabled: true, TotalCount: 10, Note: "n"}
	if err := st.CreateCouponDef(ctx, def); err != nil {
		t.Fatal(err)
	}
	defs, _ := st.ListCouponDefs(ctx)
	if len(defs) < 1 {
		t.Fatalf("券定义列表失败: %d", len(defs))
	}
	ci := &CouponInstance{DefID: def.ID, UserID: 1, Status: "usable"}
	if err := st.CreateCouponInstance(ctx, ci); err != nil {
		t.Fatal(err)
	}
	t.Logf("DEBUG def.ID=%d ci.ID=%d", def.ID, ci.ID)
	got, err := st.GetCouponInstance(ctx, ci.ID)
	if err != nil || got.Status != "usable" || got.DefID != def.ID || got.Def.OutletID != oid || got.Def.Value != 80 {
		t.Fatalf("券实例查询失败: %+v %v", got, err)
	}
	uc, _ := st.ListUserCoupons(ctx, 1)
	if len(uc) < 1 {
		t.Fatalf("用户券列表至少1: %d", len(uc))
	}
	if err := st.UseCouponInstance(ctx, ci.ID, 77); err != nil {
		t.Fatal(err)
	}
	if err := st.UseCouponInstance(ctx, ci.ID, 78); !errors.Is(err, ErrConflict) {
		t.Fatal("重复核销应冲突")
	}
	got2, _ := st.GetCouponInstance(ctx, ci.ID)
	if got2.Status != "used" || got2.UsedOrderID != 77 {
		t.Fatalf("核销未生效: %+v", got2)
	}
	if err := st.SetCouponEnabled(ctx, def.ID, false); err != nil {
		t.Fatal(err)
	}
	defs2, _ := st.ListCouponDefs(ctx)
	if defs2[0].Enabled {
		t.Fatal("停用未生效")
	}
	if err := st.SetCouponEnabled(ctx, 999, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在券应NotFound")
	}
}

func TestMySQLAlertsTicketsKBChat(t *testing.T) {
	st := newTestMySQL(t)
	ctx := context.Background()
	// 告警
	_ = st.CreateAlert(ctx, &Alert{Title: "柜门超时", Level: "high", Source: "system_check", OutletID: 1, LockerID: 3, Description: "d", Status: "open"})
	_ = st.CreateAlert(ctx, &Alert{Title: "网络异常", Level: "mid", Source: "system_check", OutletID: 2, Status: "open"})
	open, _ := st.ListAlerts(ctx, 0, "open")
	if len(open) != 2 {
		t.Fatalf("open告警应2: %d", len(open))
	}
	a, err := st.GetAlert(ctx, open[0].ID)
	if err != nil || a.Title == "" {
		t.Fatalf("告警查询失败: %v", err)
	}
	if _, err := st.GetAlert(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatal("不存在告警应NotFound")
	}
	if err := st.HandleAlert(ctx, open[0].ID, "张三"); err != nil {
		t.Fatal(err)
	}
	if err := st.HandleAlert(ctx, open[0].ID, "李四"); !errors.Is(err, ErrConflict) {
		t.Fatal("重复处理应冲突")
	}
	a2, _ := st.GetAlert(ctx, open[0].ID)
	if a2.Status != "handled" || a2.Handler != "张三" || a2.HandledAt.IsZero() {
		t.Fatalf("处理未生效: %+v", a2)
	}
	// 工单
	_ = st.CreateTicket(ctx, &Ticket{TicketNo: "TK20261119-001", UserID: 1, Question: "柜门打不开", Category: "", Status: "pending"})
	_ = st.CreateTicket(ctx, &Ticket{TicketNo: "TK20261119-002", UserID: 2, Question: "Q2", Status: "resolved"})
	pending, _ := st.ListTickets(ctx, "pending")
	if len(pending) != 1 {
		t.Fatalf("pending工单应1: %d", len(pending))
	}
	all, _ := st.ListTickets(ctx, "")
	if len(all) != 2 {
		t.Fatalf("全部工单应2: %d", len(all))
	}
	n, _ := st.CountTodayTickets(ctx, time.Now())
	if n != 2 {
		t.Fatalf("今日工单应2: %d", n)
	}
	tk := pending[0]
	tk.Status = "resolved"
	tk.Reply = "已处理"
	tk.ResolvedAt = time.Now()
	if err := st.UpdateTicket(ctx, tk); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateTicket(ctx, &Ticket{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Fatal("更新不存在工单应NotFound")
	}
	// 知识库（种子已有若干条，断言包含新条目）
	st.SeedKB(fmt.Sprintf("测试分类%d", time.Now().UnixNano()%100000), "收费,价格", "5元/小时")
	kb, err := st.ListKB(ctx)
	if err != nil || len(kb) < 1 {
		t.Fatalf("知识库应非空: %d %v", len(kb), err)
	}
	// 客服消息与会话
	_ = st.AppendMessage(ctx, &ChatMessage{UserID: 1, Sender: "user", Content: "你好"})
	_ = st.AppendMessage(ctx, &ChatMessage{UserID: 1, Sender: "bot", Content: "您好"})
	msgs, _ := st.ListMessages(ctx, 1, 0, 10)
	if len(msgs) != 2 {
		t.Fatalf("消息应2: %d", len(msgs))
	}
	if msgs2, _ := st.ListMessages(ctx, 1, msgs[0].ID, 10); len(msgs2) != 1 {
		t.Fatal("afterId过滤失败")
	}
	_ = st.SetChatState(ctx, 1, "waiting")
	cs, err := st.GetChatSession(ctx, 1)
	if err != nil || cs.State != "waiting" {
		t.Fatalf("会话状态失败: %+v %v", cs, err)
	}
	_ = st.SetChatState(ctx, 1, "agent")
	agents, _ := st.ListAgentSessions(ctx)
	if len(agents) != 1 {
		t.Fatalf("人工会话应1: %d", len(agents))
	}
}

func TestMySQLMigrateIdempotent(t *testing.T) {
	st := newTestMySQL(t)
	// 重复执行迁移应幂等
	if err := st.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatalf("重复迁移应幂等: %v", err)
	}
}
