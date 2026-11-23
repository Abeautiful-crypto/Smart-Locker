package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"smart-locker/internal/store"
)

var errInjected = errors.New("injected store error")

// failStore 可注入错误的存储包装（继承 MemStore 全部行为）
type failStore struct {
	store.MemStore
	fail atomic.Value // string
}

func (f *failStore) shouldFail(name string) bool {
	v, _ := f.fail.Load().(string)
	return v == name
}

func (f *failStore) GetUserByPhone(c context.Context, p string) (*store.User, error) {
	if f.shouldFail("GetUserByPhone") {
		return nil, errInjected
	}
	return f.MemStore.GetUserByPhone(c, p)
}
func (f *failStore) CreateUser(c context.Context, p string) (*store.User, error) {
	if f.shouldFail("CreateUser") {
		return nil, errInjected
	}
	return f.MemStore.CreateUser(c, p)
}
func (f *failStore) SetPickupPwd(c context.Context, id int64, h string) error {
	if f.shouldFail("SetPickupPwd") {
		return errInjected
	}
	return f.MemStore.SetPickupPwd(c, id, h)
}
func (f *failStore) IdleCountByCity(c context.Context, city string) (int, error) {
	if f.shouldFail("IdleCountByCity") {
		return 0, errInjected
	}
	return f.MemStore.IdleCountByCity(c, city)
}
func (f *failStore) ListOutlets(c context.Context, fl store.OutletFilter) ([]*store.Outlet, error) {
	if f.shouldFail("ListOutlets") {
		return nil, errInjected
	}
	return f.MemStore.ListOutlets(c, fl)
}
func (f *failStore) GetOutlet(c context.Context, id int64) (*store.Outlet, error) {
	if f.shouldFail("GetOutlet") {
		return nil, errInjected
	}
	return f.MemStore.GetOutlet(c, id)
}
func (f *failStore) UpdateOutlet(c context.Context, o *store.Outlet) error {
	if f.shouldFail("UpdateOutlet") {
		return errInjected
	}
	return f.MemStore.UpdateOutlet(c, o)
}
func (f *failStore) CreateOutlet(c context.Context, o *store.Outlet) error {
	if f.shouldFail("CreateOutlet") {
		return errInjected
	}
	return f.MemStore.CreateOutlet(c, o)
}
func (f *failStore) GetRule(c context.Context, id int64) (*store.Rule, error) {
	if f.shouldFail("GetRule") {
		return nil, errInjected
	}
	return f.MemStore.GetRule(c, id)
}
func (f *failStore) UpsertRule(c context.Context, r *store.Rule) error {
	if f.shouldFail("UpsertRule") {
		return errInjected
	}
	return f.MemStore.UpsertRule(c, r)
}
func (f *failStore) ListLockers(c context.Context, oid, gid int64) ([]*store.Locker, error) {
	if f.shouldFail("ListLockers") {
		return nil, errInjected
	}
	return f.MemStore.ListLockers(c, oid, gid)
}
func (f *failStore) GetLocker(c context.Context, id int64) (*store.Locker, error) {
	if f.shouldFail("GetLocker") {
		return nil, errInjected
	}
	return f.MemStore.GetLocker(c, id)
}
func (f *failStore) UpdateLocker(c context.Context, l *store.Locker) error {
	if f.shouldFail("UpdateLocker") {
		return errInjected
	}
	return f.MemStore.UpdateLocker(c, l)
}
func (f *failStore) AllocateLocker(c context.Context, oid int64, size string) (*store.Locker, error) {
	if f.shouldFail("AllocateLocker") {
		return nil, errInjected
	}
	return f.MemStore.AllocateLocker(c, oid, size)
}
func (f *failStore) CreateOrder(c context.Context, o *store.Order) error {
	if f.shouldFail("CreateOrder") {
		return errInjected
	}
	return f.MemStore.CreateOrder(c, o)
}
func (f *failStore) GetOrder(c context.Context, id int64) (*store.Order, error) {
	if f.shouldFail("GetOrder") {
		return nil, errInjected
	}
	return f.MemStore.GetOrder(c, id)
}
func (f *failStore) GetOrderByCode(c context.Context, code string, active bool) (*store.Order, error) {
	if f.shouldFail("GetOrderByCode") {
		return nil, errInjected
	}
	return f.MemStore.GetOrderByCode(c, code, active)
}
func (f *failStore) ListOrders(c context.Context, fl store.OrderFilter) ([]*store.Order, error) {
	if f.shouldFail("ListOrders") {
		return nil, errInjected
	}
	return f.MemStore.ListOrders(c, fl)
}
func (f *failStore) UpdateOrderSettlement(c context.Context, o *store.Order) error {
	if f.shouldFail("UpdateOrderSettlement") {
		return errInjected
	}
	return f.MemStore.UpdateOrderSettlement(c, o)
}
func (f *failStore) AddPayment(c context.Context, p *store.Payment) error {
	if f.shouldFail("AddPayment") {
		return errInjected
	}
	return f.MemStore.AddPayment(c, p)
}
func (f *failStore) AddRevenue(c context.Context, r *store.RevenueRecord) error {
	if f.shouldFail("AddRevenue") {
		return errInjected
	}
	return f.MemStore.AddRevenue(c, r)
}
func (f *failStore) SumRevenue(c context.Context, oid int64, from, to time.Time) (int, int, int, error) {
	if f.shouldFail("SumRevenue") {
		return 0, 0, 0, errInjected
	}
	return f.MemStore.SumRevenue(c, oid, from, to)
}
func (f *failStore) CountOrdersBetween(c context.Context, oid int64, from, to time.Time) (int, error) {
	if f.shouldFail("CountOrdersBetween") {
		return 0, errInjected
	}
	return f.MemStore.CountOrdersBetween(c, oid, from, to)
}
func (f *failStore) ListRevenue(c context.Context, oid int64, from, to time.Time) ([]*store.RevenueRecord, error) {
	if f.shouldFail("ListRevenue") {
		return nil, errInjected
	}
	return f.MemStore.ListRevenue(c, oid, from, to)
}
func (f *failStore) ListAlerts(c context.Context, oid int64, status string) ([]*store.Alert, error) {
	if f.shouldFail("ListAlerts") {
		return nil, errInjected
	}
	return f.MemStore.ListAlerts(c, oid, status)
}
func (f *failStore) GetAlert(c context.Context, id int64) (*store.Alert, error) {
	if f.shouldFail("GetAlert") {
		return nil, errInjected
	}
	return f.MemStore.GetAlert(c, id)
}
func (f *failStore) HandleAlert(c context.Context, id int64, h string) error {
	if f.shouldFail("HandleAlert") {
		return errInjected
	}
	return f.MemStore.HandleAlert(c, id, h)
}
func (f *failStore) ListStaff(c context.Context) ([]*store.Staff, error) {
	if f.shouldFail("ListStaff") {
		return nil, errInjected
	}
	return f.MemStore.ListStaff(c)
}
func (f *failStore) GetStaff(c context.Context, id int64) (*store.Staff, error) {
	if f.shouldFail("GetStaff") {
		return nil, errInjected
	}
	return f.MemStore.GetStaff(c, id)
}
func (f *failStore) IncrTokenVer(c context.Context, kind string, refID int64) error {
	if f.shouldFail("IncrTokenVer") {
		return errInjected
	}
	return f.MemStore.IncrTokenVer(c, kind, refID)
}
func (f *failStore) GetStaffByIdentity(c context.Context, id string) (*store.Staff, error) {
	if f.shouldFail("GetStaffByIdentity") {
		return nil, errInjected
	}
	return f.MemStore.GetStaffByIdentity(c, id)
}
func (f *failStore) CreateStaff(c context.Context, s *store.Staff) error {
	if f.shouldFail("CreateStaff") {
		return errInjected
	}
	return f.MemStore.CreateStaff(c, s)
}
func (f *failStore) UpdateStaffRole(c context.Context, id int64, r string) error {
	if f.shouldFail("UpdateStaffRole") {
		return errInjected
	}
	return f.MemStore.UpdateStaffRole(c, id, r)
}
func (f *failStore) UpdateStaffPassword(c context.Context, id int64, h string) error {
	if f.shouldFail("UpdateStaffPassword") {
		return errInjected
	}
	return f.MemStore.UpdateStaffPassword(c, id, h)
}
func (f *failStore) UpdateStaffStatus(c context.Context, id int64, s string) error {
	if f.shouldFail("UpdateStaffStatus") {
		return errInjected
	}
	return f.MemStore.UpdateStaffStatus(c, id, s)
}
func (f *failStore) ListCouponDefs(c context.Context) ([]*store.CouponDef, error) {
	if f.shouldFail("ListCouponDefs") {
		return nil, errInjected
	}
	return f.MemStore.ListCouponDefs(c)
}
func (f *failStore) CreateCouponDef(c context.Context, d *store.CouponDef) error {
	if f.shouldFail("CreateCouponDef") {
		return errInjected
	}
	return f.MemStore.CreateCouponDef(c, d)
}
func (f *failStore) SetCouponEnabled(c context.Context, id int64, e bool) error {
	if f.shouldFail("SetCouponEnabled") {
		return errInjected
	}
	return f.MemStore.SetCouponEnabled(c, id, e)
}
func (f *failStore) GetCouponInstance(c context.Context, id int64) (*store.CouponInstance, error) {
	if f.shouldFail("GetCouponInstance") {
		return nil, errInjected
	}
	return f.MemStore.GetCouponInstance(c, id)
}
func (f *failStore) ListUserCoupons(c context.Context, uid int64) ([]*store.CouponInstance, error) {
	if f.shouldFail("ListUserCoupons") {
		return nil, errInjected
	}
	return f.MemStore.ListUserCoupons(c, uid)
}
func (f *failStore) UseCouponInstance(c context.Context, id, oid int64) error {
	if f.shouldFail("UseCouponInstance") {
		return errInjected
	}
	return f.MemStore.UseCouponInstance(c, id, oid)
}
func (f *failStore) GetUserByID(c context.Context, id int64) (*store.User, error) {
	if f.shouldFail("GetUserByID") {
		return nil, errInjected
	}
	return f.MemStore.GetUserByID(c, id)
}
func (f *failStore) GetSession(c context.Context, tk string) (*store.SessionRow, error) {
	if f.shouldFail("GetSession") {
		return nil, errInjected
	}
	return f.MemStore.GetSession(c, tk)
}
func (f *failStore) ListKB(c context.Context) ([]map[string]string, error) {
	if f.shouldFail("ListKB") {
		return nil, errInjected
	}
	return f.MemStore.ListKB(c)
}
func (f *failStore) ListMessages(c context.Context, uid, after int64, limit int) ([]*store.ChatMessage, error) {
	if f.shouldFail("ListMessages") {
		return nil, errInjected
	}
	return f.MemStore.ListMessages(c, uid, after, limit)
}
func (f *failStore) ListAgentSessions(c context.Context) ([]*store.ChatSession, error) {
	if f.shouldFail("ListAgentSessions") {
		return nil, errInjected
	}
	return f.MemStore.ListAgentSessions(c)
}
func (f *failStore) ListTickets(c context.Context, status string) ([]*store.Ticket, error) {
	if f.shouldFail("ListTickets") {
		return nil, errInjected
	}
	return f.MemStore.ListTickets(c, status)
}
func (f *failStore) UpdateTicket(c context.Context, t *store.Ticket) error {
	if f.shouldFail("UpdateTicket") {
		return errInjected
	}
	return f.MemStore.UpdateTicket(c, t)
}

func newFailServer(t *testing.T) (*failStore, *httptest.Server) {
	t.Helper()
	fs := &failStore{MemStore: *store.NewMem()}
	seedDemo(t, &fs.MemStore)
	srv := New(fs)
	mux := http.NewServeMux()
	srv.Routes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return fs, ts
}

type injCase struct {
	fail   string
	method string
	path   string
	body   map[string]any
	token  string
	want   int
}

func TestInjectedErrors(t *testing.T) {
	fs, ts := newFailServer(t)
	tk := login(t, ts, "13900009200")
	superTk := adminLogin(t, ts, "super")
	svcTk := adminLogin(t, ts, "svc")
	oid := outletIDOf(t, ts)

	// 预置资源
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	occLockerPath := fmtSprintf("/api/admin/lockers/%d/action", int64(order["lockerId"].(float64)))
	lockers, _ := fs.ListLockers(context.Background(), oid, 0)
	var idleLocker *store.Locker
	for _, l := range lockers {
		if l.Status == "idle" && l.Size != "S" {
			idleLocker = l
			break
		}
	}
	idleLocker.Status = "fault"
	_ = fs.UpdateLocker(context.Background(), idleLocker)
	faultLockerPath := fmtSprintf("/api/admin/lockers/%d/action", idleLocker.ID)
	_ = fs.CreateAlert(context.Background(), &store.Alert{Title: "注入告警", Level: "mid", OutletID: oid, Status: "open"})
	alerts, _ := fs.ListAlerts(context.Background(), oid, "open")
	alertPath := fmtSprintf("/api/admin/alerts/%d/handle", alerts[0].ID)
	uid := uidOf(t, ts, tk)
	defs, _ := fs.ListCouponDefs(context.Background())
	_ = fs.CreateCouponInstance(context.Background(), &store.CouponInstance{DefID: defs[0].ID, UserID: uid, Status: "usable"})
	_ = fs.CreateTicket(context.Background(), &store.Ticket{TicketNo: "TKT1", UserID: uid, Question: "q", Status: "pending"})
	tickets, _ := fs.ListTickets(context.Background(), "pending")
	ticketPath := fmtSprintf("/api/admin/tickets/%d/resolve", tickets[0].ID)

	cases := []injCase{
		// 管理端（super）
		{"GetStaffByIdentity", "POST", "/api/admin/login", map[string]any{"identity": "super", "password": "000000"}, "", 500},
		{"SumRevenue", "GET", "/api/admin/dashboard", nil, superTk, 500},
		{"CountOrdersBetween", "GET", "/api/admin/dashboard", nil, superTk, 500},
		{"ListOutlets", "GET", "/api/admin/outlets", nil, superTk, 500},
		{"GetOutlet", "PUT", "/api/admin/outlets/1", map[string]any{"name": "x"}, superTk, 500},
		{"UpdateOutlet", "PUT", "/api/admin/outlets/1", map[string]any{"name": "x"}, superTk, 500},
		{"CreateOutlet", "POST", "/api/admin/outlets", map[string]any{"name": "x", "city": "上海"}, superTk, 500},
		{"ListLockers", "GET", "/api/admin/lockers?outletId=1", nil, superTk, 500},
		{"GetLocker", "GET", "/api/admin/lockers/1", nil, superTk, 500},
		{"GetLocker", "GET", fmtSprintf("/api/admin/lockers/%d", idleLocker.ID), nil, superTk, 500},
		{"GetRule", "GET", "/api/admin/rules/1", nil, superTk, 500},
		{"UpsertRule", "PUT", "/api/admin/rules/1", map[string]any{"mode": "daily", "priceS": 1, "priceM": 1, "priceL": 1}, superTk, 500},
		{"ListCouponDefs", "GET", "/api/admin/coupons", nil, superTk, 500},
		{"CreateCouponDef", "POST", "/api/admin/coupons", map[string]any{"name": "x", "type": "hour", "value": 60, "validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 1}, superTk, 500},
		{"SetCouponEnabled", "POST", "/api/admin/coupons/1/toggle", map[string]any{"enabled": false}, superTk, 500},
		{"ListOrders", "GET", "/api/admin/orders", nil, superTk, 500},
		{"ListRevenue", "GET", "/api/admin/revenue?dim=outlet", nil, superTk, 500},
		{"ListRevenue", "GET", "/api/admin/revenue?dim=group", nil, superTk, 500},
		{"ListRevenue", "GET", "/api/admin/revenue?dim=order", nil, superTk, 500},
		{"ListRevenue", "GET", "/api/admin/revenue/export", nil, superTk, 500},
		{"ListAlerts", "GET", "/api/admin/alerts", nil, superTk, 500},
		{"GetAlert", "POST", alertPath, nil, superTk, 500},
		{"HandleAlert", "POST", alertPath, nil, superTk, 409},
		{"ListStaff", "GET", "/api/admin/staff", nil, superTk, 500},
		{"CreateStaff", "POST", "/api/admin/staff", map[string]any{"empNo": "Z9", "account": "z9", "name": "z", "phone": "13999990009", "role": "service", "password": "000000"}, superTk, 409},
		{"UpdateStaffRole", "POST", "/api/admin/staff/1/role", map[string]any{"role": "service"}, superTk, 500},
		{"UpdateStaffPassword", "POST", "/api/admin/staff/1/reset-pwd", map[string]any{"password": "123456"}, superTk, 500},
		{"UpdateStaffStatus", "POST", "/api/admin/staff/1/toggle", map[string]any{"status": "disabled"}, superTk, 500},
		{"GetLocker", "POST", occLockerPath, map[string]any{"action": "switch"}, superTk, 500},
		{"AllocateLocker", "POST", occLockerPath, map[string]any{"action": "switch"}, superTk, 500},
		{"GetOrder", "POST", occLockerPath, map[string]any{"action": "switch"}, superTk, 500},
		{"UpdateLocker", "POST", faultLockerPath, map[string]any{"action": "repair"}, superTk, 500},
		{"UpdateOrderSettlement", "POST", fmtSprintf("/api/orders/%d/pickup-done", orderID), nil, tk, 500},
		// 客服端（svc）
		{"ListAgentSessions", "GET", "/api/admin/cs/sessions", nil, svcTk, 500},
		{"ListMessages", "GET", "/api/admin/cs/messages?userId=1", nil, svcTk, 500},
		{"ListTickets", "GET", "/api/admin/tickets", nil, svcTk, 500},
		{"ListTickets", "POST", ticketPath, map[string]any{"reply": "x"}, svcTk, 404},
		{"UpdateTicket", "POST", ticketPath, map[string]any{"reply": "x"}, svcTk, 500},
		// 公开接口
		{"IdleCountByCity", "GET", "/api/cities", nil, "", 500},
		{"ListOutlets", "GET", "/api/outlets", nil, "", 500},
		{"GetOutlet", "GET", "/api/outlets/1", nil, "", 500},
		{"GetRule", "GET", "/api/outlets/1/rule", nil, "", 500},
		{"GetRule", "POST", "/api/quote", map[string]any{"outletId": 1, "size": "S"}, "", 500},
		// 用户端（tk）
		{"GetUserByID", "GET", "/api/user/me", nil, tk, 401},
		{"SetPickupPwd", "POST", "/api/user/pickup-pwd", map[string]any{"password": "123456"}, tk, 500},
		{"GetRule", "POST", "/api/orders", map[string]any{"outletId": 1, "size": "S"}, tk, 500},
		{"GetCouponInstance", "POST", "/api/orders", map[string]any{"outletId": 1, "size": "S", "couponInstanceId": 1}, tk, 400},
		{"AllocateLocker", "POST", "/api/orders", map[string]any{"outletId": 1, "size": "S"}, tk, 500},
		{"CreateOrder", "POST", "/api/orders", map[string]any{"outletId": 1, "size": "S"}, tk, 500},
		{"ListOrders", "GET", "/api/orders/current", nil, tk, 500},
		{"ListOrders", "GET", "/api/orders/history", nil, tk, 500},
		{"GetOrder", "GET", fmtSprintf("/api/orders/%d", orderID), nil, tk, 500},
		{"GetOrder", "POST", fmtSprintf("/api/orders/%d/report", orderID), map[string]any{"problem": "x"}, tk, 409},
		{"GetOrder", "POST", fmtSprintf("/api/orders/%d/pickup-done", orderID), nil, tk, 409},
		{"GetOrder", "POST", fmtSprintf("/api/orders/%d/pay-extra", orderID), nil, tk, 409},
		{"GetOrderByCode", "POST", "/api/orders/999/pickup", map[string]any{"method": "code", "code": "123456"}, tk, 403},
		{"GetUserByID", "POST", "/api/orders/999/pickup", map[string]any{"method": "password", "password": "123456"}, tk, 401},
		{"ListUserCoupons", "GET", "/api/user/coupons", nil, tk, 500},
		{"ListKB", "POST", "/api/chat/send", map[string]any{"content": "hello"}, tk, 500},
		{"ListMessages", "GET", "/api/chat/messages", nil, tk, 500},
		{"GetUserByID", "GET", "/api/chat/state", nil, tk, 401},
	}
	for _, c := range cases {
		fs.fail.Store(c.fail)
		code, out := call(t, ts, c.method, c.path, c.token, c.body)
		if code != c.want {
			t.Errorf("%s %s fail=%s: 期望%d 得 %d %v", c.method, c.path, c.fail, c.want, code, out)
		}
	}
	fs.fail.Store("")

	// JWT 鉴权失效路径：GetStaff 失败 → 401
	fs.fail.Store("GetStaff")
	code, _ := call(t, ts, "GET", "/api/admin/dashboard", superTk, nil)
	if code != 401 {
		t.Fatalf("注入鉴权错误应401: %d", code)
	}
	fs.fail.Store("")
}

func fmtSprintf(f string, a ...any) string { return fmt.Sprintf(f, a...) }
