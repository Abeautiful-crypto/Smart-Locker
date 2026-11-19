package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"smart-locker/internal/store"
)

// newTestServer 构造带演示数据的测试服务器
func newTestServer(t *testing.T) (*Server, *httptest.Server, *store.MemStore) {
	t.Helper()
	ms := store.NewMem()
	seedDemo(t, ms)
	srv := New(ms)
	mux := http.NewServeMux()
	srv.Routes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return srv, ts, ms
}

func seedDemo(t *testing.T, ms *store.MemStore) {
	t.Helper()
	ctx := context.Background()
	o, err := ms.SeedOutlet("人民广场站", "上海", map[string]int{"S": 2, "M": 2, "L": 2})
	if err != nil {
		t.Fatal(err)
	}
	_ = ms.UpsertRule(ctx, &store.Rule{OutletID: o.ID, Mode: "hourly", PriceS: 500, PriceM: 700, PriceL: 1000,
		DepositS: 1000, DepositM: 2000, DepositL: 3000, CapS: 5000, CapM: 7000, CapL: 10000, FreeMinutes: 10})
	ms.SeedKB("收费计费", "收费,计费,价格", "小格5元/小时")
	ms.SeedKB("退款", "退款,退钱", "原路退回")
	_ = ms.CreateStaff(ctx, &store.Staff{EmpNo: "E1", Account: "super", Name: "超管", Phone: "13800000001", Role: "super", PasswordHash: hashOf("000000"), Status: "active"})
	_ = ms.CreateStaff(ctx, &store.Staff{EmpNo: "E2", Account: "site_rm", Name: "网点管", Phone: "13800000002", Role: "site_admin", PasswordHash: hashOf("000000"), Status: "active", ScopeOutletID: o.ID})
	_ = ms.CreateStaff(ctx, &store.Staff{EmpNo: "E3", Account: "svc", Name: "客服", Phone: "13800000003", Role: "service", PasswordHash: hashOf("000000"), Status: "active"})
	_ = ms.CreateCouponDef(ctx, &store.CouponDef{Name: "8折券", Ctype: "discount", Value: 80, Scope: "all",
		ValidFrom: time.Now().AddDate(0, 0, -1), ValidTo: time.Now().AddDate(0, 0, 30), Enabled: true, TotalCount: 100})
	_ = ms.CreateCouponDef(ctx, &store.CouponDef{Name: "停用券", Ctype: "amount", Value: 100, Scope: "all",
		ValidFrom: time.Now().AddDate(0, 0, -1), ValidTo: time.Now().AddDate(0, 0, 30), Enabled: false, TotalCount: 100})
}

func hashOf(pwd string) string {
	h := sha256.Sum256([]byte(pwd))
	return hex.EncodeToString(h[:])
}

// call 通用请求辅助
func call(t *testing.T, ts *httptest.Server, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, ts.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败 %s: %v", path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func login(t *testing.T, ts *httptest.Server, phone string) string {
	t.Helper()
	_, out := call(t, ts, "POST", "/api/user/login", "", map[string]any{"phone": phone, "code": "1234", "agree": true})
	tk, _ := out["token"].(string)
	if tk == "" {
		t.Fatalf("登录失败: %v", out)
	}
	return tk
}

func adminLogin(t *testing.T, ts *httptest.Server, ident string) string {
	t.Helper()
	_, out := call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": ident, "password": "000000"})
	tk, _ := out["token"].(string)
	if tk == "" {
		t.Fatalf("后台登录失败: %v", out)
	}
	return tk
}

// createOrder 创建一笔进行中订单
func createOrder(t *testing.T, ts *httptest.Server, token string, outletID int64, size, mode string, couponID int64) map[string]any {
	t.Helper()
	code, out := call(t, ts, "POST", "/api/orders", token, map[string]any{
		"outletId": outletID, "size": size, "payMode": mode, "couponInstanceId": couponID})
	if code != 200 {
		t.Fatalf("下单失败(%d): %v", code, out)
	}
	return out
}

func outletIDOf(t *testing.T, ts *httptest.Server) int64 {
	t.Helper()
	_, out := call(t, ts, "GET", "/api/outlets?city=%E4%B8%8A%E6%B5%B7", "", nil)
	arr, _ := out["0"].(map[string]any)
	if arr == nil {
		// map 结构时按 JSON 数组解析会失败，直接用原始请求
		req, _ := http.NewRequest("GET", ts.URL+"/api/outlets?city=%E4%B8%8A%E6%B5%B7", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var list []map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&list)
		if len(list) == 0 {
			t.Fatal("无网点")
		}
		v, _ := list[0]["id"].(float64)
		return int64(v)
	}
	v, _ := arr["id"].(float64)
	return int64(v)
}

func TestUserLoginAndRegister(t *testing.T) {
	_, ts, _ := newTestServer(t)
	// 未勾选协议
	code, out := call(t, ts, "POST", "/api/user/login", "", map[string]any{"phone": "13900000001", "code": "1234", "agree": false})
	if code != 400 {
		t.Fatalf("未勾选协议应400: %d %v", code, out)
	}
	// 验证码错误
	code, _ = call(t, ts, "POST", "/api/user/login", "", map[string]any{"phone": "13900000001", "code": "9999", "agree": true})
	if code != 400 {
		t.Fatalf("验证码错误应400: %d", code)
	}
	// 手机号非法
	code, _ = call(t, ts, "POST", "/api/user/login", "", map[string]any{"phone": "12345", "code": "1234", "agree": true})
	if code != 400 {
		t.Fatalf("非法手机号应400: %d", code)
	}
	// 正常登录（自动注册）
	tk := login(t, ts, "13900000001")
	if tk == "" {
		t.Fatal("token为空")
	}
	// 重复登录同一手机号（不重复注册）
	tk2 := login(t, ts, "13900000001")
	if tk2 == "" {
		t.Fatal("重复登录失败")
	}
	// 发送验证码
	code, out = call(t, ts, "POST", "/api/user/send-code", "", map[string]any{"phone": "13900000002"})
	if code != 200 {
		t.Fatalf("验证码发送应200: %v", out)
	}
	if c, _ := out["code"].(string); c != "1234" {
		t.Fatalf("演示验证码应1234: %v", out)
	}
	// 未登录访问
	code, _ = call(t, ts, "GET", "/api/user/coupons", "", nil)
	if code != 401 {
		t.Fatalf("未登录应401: %d", code)
	}
}

func TestUserLoginValidation(t *testing.T) {
	_, ts, _ := newTestServer(t)
	// 空请求体
	req, _ := http.NewRequest("POST", ts.URL+"/api/user/login", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("坏JSON应400: %d", resp.StatusCode)
	}
}

func TestQuoteAndCities(t *testing.T) {
	_, ts, _ := newTestServer(t)
	oid := outletIDOf(t, ts)
	code, out := call(t, ts, "POST", "/api/quote", "", map[string]any{"outletId": oid, "size": "S", "payMode": "prepaid"})
	if code != 200 {
		t.Fatalf("试算失败: %v", out)
	}
	if v, _ := out["prepaid"].(float64); v != 1500 {
		t.Fatalf("小格预付应1500: %v", out)
	}
	if v, _ := out["deposit"].(float64); v != 1000 {
		t.Fatalf("押金应1000: %v", out)
	}
	// 押金模式
	code, out = call(t, ts, "POST", "/api/quote", "", map[string]any{"outletId": oid, "size": "M", "payMode": "deposit"})
	if code != 200 {
		t.Fatalf("押金试算失败: %v", out)
	}
	if v, _ := out["payNow"].(float64); v != 2000 {
		t.Fatalf("押金模式应付2000: %v", out)
	}
	// 非法规格
	code, _ = call(t, ts, "POST", "/api/quote", "", map[string]any{"outletId": oid, "size": "X"})
	if code != 400 {
		t.Fatalf("非法规格应400: %d", code)
	}
	// 非法模式
	code, _ = call(t, ts, "POST", "/api/quote", "", map[string]any{"outletId": oid, "size": "S", "payMode": "weird"})
	if code != 400 {
		t.Fatalf("非法模式应400: %d", code)
	}
	// 城市列表
	code, out = call(t, ts, "GET", "/api/cities", "", nil)
	if code != 200 {
		t.Fatalf("城市应200: %v", out)
	}
}

func TestStoreAndPickupFlowFree(t *testing.T) {
	_, ts, _ := newTestServer(t)
	tk := login(t, ts, "13900000003")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	if v, _ := order["paid"].(float64); v != 1500 {
		t.Fatalf("预付应1500: %v", order)
	}
	orderID := int64(order["orderId"].(float64))
	pickCode, _ := order["code"].(string)
	lockerCode, _ := order["lockerCode"].(string)
	if lockerCode == "" || len(pickCode) != 6 {
		t.Fatalf("开柜信息缺失: %v", order)
	}
	// 取件码错误
	code, _ := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "code", "code": "000000"})
	if code != 400 {
		t.Fatalf("错误取件码应400: %d", code)
	}
	// 正确取件码
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "code", "code": pickCode})
	if code != 200 {
		t.Fatalf("取件验证失败: %v", out)
	}
	// 关门结算（免费时长内）
	code, out = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算失败: %v", out)
	}
	if k, _ := out["kind"].(string); k != "free" {
		t.Fatalf("免费时长内应free: %v", out)
	}
	if r, _ := out["refund"].(float64); r != 1500 {
		t.Fatalf("应全额退款1500: %v", out)
	}
	// 订单已结束，重复关门应409
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 409 {
		t.Fatalf("重复结算应409: %d", code)
	}
	// 柜格恢复空闲
	code, out = call(t, ts, "GET", "/api/outlets/"+fmt.Sprint(oid), "", nil)
	if code != 200 {
		t.Fatalf("网点详情失败: %v", out)
	}
	byStatus, _ := out["byStatus"].(map[string]any)
	if v, _ := byStatus["idle"].(float64); v != 6 {
		t.Fatalf("应恢复6格空闲: %v", byStatus)
	}
	// 历史订单
	code, out = call(t, ts, "GET", "/api/orders/history", tk, nil)
	if code != 200 {
		t.Fatalf("历史订单失败: %v", out)
	}
}

func TestPickupMethods(t *testing.T) {
	_, ts, _ := newTestServer(t)
	tk := login(t, ts, "13900000004")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "M", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	// 扫码取件
	code, _ := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	if code != 200 {
		t.Fatal("扫码取件应通过")
	}
	// 人脸
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "face"})
	if code != 200 {
		t.Fatal("人脸取件应通过")
	}
	// 密码未设置 → 拒绝
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "password", "password": "123456"})
	if code != 400 {
		t.Fatal("未设置密码应拒绝")
	}
	// 设置取件密码
	code, _ = call(t, ts, "POST", "/api/user/pickup-pwd", tk, map[string]any{"password": "123456"})
	if code != 200 {
		t.Fatal("设置密码应成功")
	}
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "password", "password": "123456"})
	if code != 200 {
		t.Fatal("密码取件应通过")
	}
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "password", "password": "wrong!"})
	if code != 400 {
		t.Fatal("错误密码应拒绝")
	}
	// 不支持方式
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "voice"})
	if code != 400 {
		t.Fatal("不支持方式应400")
	}
}

func TestSettleDueAndPayExtra(t *testing.T) {
	srv, ts, ms := newTestServer(t)
	tk := login(t, ts, "13900000005")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	// 将开始时间拨回2小时前 → 费用 2*500=1000，预付1500 → 退500（不足补交）
	ctx := context.Background()
	ord, _ := ms.GetOrder(ctx, orderID)
	ord.StartedAt = time.Now().Add(-2*time.Hour - 15*time.Minute)
	_ = ms.UpdateOrderSettlement(ctx, ord)

	_, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算失败: %v", out)
	}
	// 2h15m → 免10分 → 2h5m → 3小时? ceil(125/60)=3h → 1500 vs 预付1500 → even
	if k, _ := out["kind"].(string); k != "even" {
		t.Fatalf("2小时15分应恰好even: %v", out)
	}
	// 拨到10小时前 → 费用10*500=5000 → 补3500
	order2 := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID2 := int64(order2["orderId"].(float64))
	ord2, _ := ms.GetOrder(ctx, orderID2)
	ord2.StartedAt = time.Now().Add(-10 * time.Hour)
	_ = ms.UpdateOrderSettlement(ctx, ord2)
	_, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID2), tk, map[string]any{"method": "scan"})
	code, out = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID2), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算失败: %v", out)
	}
	if e, _ := out["extra"].(float64); e != 3500 {
		t.Fatalf("应补交3500: %v", out)
	}
	// 订单仍是 active，柜格仍占用
	ord2, _ = ms.GetOrder(ctx, orderID2)
	if ord2.Status != "active" {
		t.Fatalf("需补交订单应保持active: %s", ord2.Status)
	}
	// 补交
	code, out = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pay-extra", orderID2), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("补交失败: %v", out)
	}
	if s, _ := out["status"].(string); s != "finished" {
		t.Fatalf("补交后应结束: %v", out)
	}
	// 重复补交应409
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pay-extra", orderID2), tk, map[string]any{})
	if code != 409 {
		t.Fatalf("重复补交应409: %d", code)
	}
	_ = srv
}

func TestSettleWithCoupon(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	tk := login(t, ts, "13900000006")
	oid := outletIDOf(t, ts)
	// 领取8折券
	defs, _ := ms.ListCouponDefs(ctx)
	var usableDef *store.CouponDef
	for _, d := range defs {
		if d.Enabled && d.Ctype == "discount" {
			usableDef = d
			break
		}
	}
	ci := &store.CouponInstance{DefID: usableDef.ID, UserID: uidOf(t, ts, tk), Status: "usable"}
	_ = ms.CreateCouponInstance(ctx, ci)
	// 以登录用户身份下单
	order := createOrder(t, ts, tk, oid, "S", "prepaid", ci.ID)
	orderID := int64(order["orderId"].(float64))
	// 65分钟 → 免10分 → 55分钟计费1小时=500，8折抵100 → 400 → 退1100
	ord, _ := ms.GetOrder(ctx, orderID)
	ord.StartedAt = time.Now().Add(-65 * time.Minute)
	_ = ms.UpdateOrderSettlement(ctx, ord)
	_, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算失败: %v", out)
	}
	if c, _ := out["coupon"].(float64); c != 100 {
		t.Fatalf("8折券应抵100: %v", out)
	}
	if r, _ := out["refund"].(float64); r != 1100 {
		t.Fatalf("应退1100: %v", out)
	}
	// 券已核销
	ci2, _ := ms.GetCouponInstance(ctx, ci.ID)
	if ci2.Status != "used" || ci2.UsedOrderID != orderID {
		t.Fatalf("券应核销: %+v", ci2)
	}
}

func TestCouponDisabledRejected(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	tk := login(t, ts, "13900000007")
	oid := outletIDOf(t, ts)
	// 停用券实例
	defs, _ := ms.ListCouponDefs(ctx)
	var disDef *store.CouponDef
	for _, d := range defs {
		if !d.Enabled {
			disDef = d
			break
		}
	}
	ci := &store.CouponInstance{DefID: disDef.ID, UserID: uidOf(t, ts, tk), Status: "usable"}
	_ = ms.CreateCouponInstance(ctx, ci)
	code, _ := call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": oid, "size": "S", "payMode": "prepaid", "couponInstanceId": ci.ID})
	if code != 400 {
		t.Fatalf("停用券下单应400: %d", code)
	}
	// C端券列表：usable 不含停用券
	code, out := call(t, ts, "GET", "/api/user/coupons", tk, nil)
	if code != 200 {
		t.Fatalf("券列表失败: %v", out)
	}
	usableArr, _ := out["usable"].([]any)
	expiredArr, _ := out["expired"].([]any)
	if len(usableArr) != 0 {
		t.Fatalf("停用券不应出现在可用列表: %v", usableArr)
	}
	if len(expiredArr) != 1 {
		t.Fatalf("停用券应置灰展示: %v", expiredArr)
	}
}

func TestReportProblemCreatesFaultAndAlert(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	tk := login(t, ts, "13900000008")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	lockerID := int64(order["lockerId"].(float64))
	// 上报
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/report", orderID), tk, map[string]any{"problem": "柜门未弹开"})
	if code != 200 {
		t.Fatalf("上报失败: %v", out)
	}
	if r, _ := out["refund"].(float64); r != 1500 {
		t.Fatalf("上报应全额退款: %v", out)
	}
	lk, _ := ms.GetLocker(ctx, lockerID)
	if lk.Status != "fault" || lk.FaultSource != "user_report" {
		t.Fatalf("柜格应标记异常: %+v", lk)
	}
	alerts, _ := ms.ListAlerts(ctx, oid, "open")
	if len(alerts) != 1 {
		t.Fatalf("应生成1条告警: %d", len(alerts))
	}
	// 重复上报应409
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/report", orderID), tk, map[string]any{"problem": "again"})
	if code != 409 {
		t.Fatalf("重复上报应409: %d", code)
	}
}

func TestNoFreeLocker(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	tk := login(t, ts, "13900000009")
	oid := outletIDOf(t, ts)
	// 把 S 格全部占用
	lockers, _ := ms.ListLockers(ctx, oid, 0)
	count := 0
	var lastOrder map[string]any
	for _, l := range lockers {
		if l.Size == "S" {
			count++
			if count <= 2 {
				lastOrder = createOrder(t, ts, tk, oid, "S", "prepaid", 0)
			}
		}
	}
	_ = lastOrder
	code, out := call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": oid, "size": "S", "payMode": "prepaid"})
	if code != 409 {
		t.Fatalf("无空闲应409: %d %v", code, out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "暂无空闲") {
		t.Fatalf("错误提示不符: %v", out)
	}
}

func TestAdminRolesAndScope(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	siteTk := adminLogin(t, ts, "site_rm")
	svcTk := adminLogin(t, ts, "svc")
	oid := outletIDOf(t, ts)

	// 角色菜单
	_, out := call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": "svc", "password": "000000"})
	menus, _ := out["menus"].([]any)
	if len(menus) != 1 {
		t.Fatalf("客服应仅1个菜单: %v", menus)
	}
	// 客服访问员工管理 403
	if code, _ := call(t, ts, "GET", "/api/admin/staff", svcTk, nil); code != 403 {
		t.Fatal("客服访问员工管理应403")
	}
	// 网点管理员访问网点管理 403
	if code, _ := call(t, ts, "GET", "/api/admin/outlets", siteTk, nil); code != 403 {
		t.Fatal("网点管理员访问网点管理应403")
	}
	// 网点管理员访问其他网点柜格 403
	if code, _ := call(t, ts, "GET", "/api/admin/lockers?outletId=999", siteTk, nil); code != 403 {
		t.Fatal("跨网点柜格应403")
	}
	// 本网点 OK
	if code, _ := call(t, ts, "GET", fmt.Sprintf("/api/admin/lockers?outletId=%d", oid), siteTk, nil); code != 200 {
		t.Fatal("本网点柜格应200")
	}
	// 未登录
	if code, _ := call(t, ts, "GET", "/api/admin/staff", "", nil); code != 401 {
		t.Fatal("未登录应401")
	}
	// 密码错误
	code, _ := call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": "super", "password": "111111"})
	if code != 400 {
		t.Fatal("密码错误应400")
	}
	_ = ms
	_ = ctx
	_ = superTk
}

func TestStaffLifecycle(t *testing.T) {
	_, ts, _ := newTestServer(t)
	superTk := adminLogin(t, ts, "super")
	// 新建员工
	code, out := call(t, ts, "POST", "/api/admin/staff", superTk, map[string]any{
		"empNo": "E9", "account": "new_svc", "name": "新客服", "phone": "13899990000", "role": "service", "password": "000000", "scopeOutletId": 1})
	if code != 200 {
		t.Fatalf("新建员工失败: %v", out)
	}
	staffID := int64(out["id"].(float64))
	// 新员工可登录
	newTk := adminLogin(t, ts, "new_svc")
	if newTk == "" {
		t.Fatal("新员工应可登录")
	}
	// 修改角色 → 登录态失效
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/role", staffID), superTk, map[string]any{"role": "site_admin"})
	if code != 200 {
		t.Fatal("改角色应成功")
	}
	if code, _ := call(t, ts, "GET", "/api/admin/cs/sessions", newTk, nil); code != 401 {
		t.Fatalf("改角色后旧登录态应401: %d", code)
	}
	// 重登后以新角色工作
	tk2 := adminLogin(t, ts, "new_svc")
	if code, _ := call(t, ts, "GET", fmt.Sprintf("/api/admin/lockers?outletId=1"), tk2, nil); code != 200 {
		t.Fatal("改角色后应以site_admin访问")
	}
	// 重置密码
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/reset-pwd", staffID), superTk, map[string]any{"password": "654321"})
	if code != 200 {
		t.Fatal("重置密码应成功")
	}
	// 禁用
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/toggle", staffID), superTk, map[string]any{"status": "disabled"})
	if code != 200 {
		t.Fatal("禁用应成功")
	}
	if code, _ := call(t, ts, "POST", "/api/admin/login", "", map[string]any{"identity": "new_svc", "password": "654321"}); code != 403 {
		t.Fatal("禁用后登录应403")
	}
	// 非法角色
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/staff/%d/role", staffID), superTk, map[string]any{"role": "hacker"})
	if code != 400 {
		t.Fatal("非法角色应400")
	}
	// 缺字段新建
	code, _ = call(t, ts, "POST", "/api/admin/staff", superTk, map[string]any{"account": "x"})
	if code != 400 {
		t.Fatal("缺字段应400")
	}
}

func TestCouponAdminFlow(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	// 新建券
	code, out := call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "1元券", "type": "amount", "value": 100, "scope": "all",
		"validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 50, "note": "立减1元"})
	if code != 200 {
		t.Fatalf("新建券失败: %v", out)
	}
	defID := int64(out["id"].(float64))
	// C端可见
	tk := login(t, ts, "13900000010")
	ci := &store.CouponInstance{DefID: defID, UserID: uidOf(t, ts, tk), Status: "usable"}
	_ = ms.CreateCouponInstance(ctx, ci)
	_, list := call(t, ts, "GET", "/api/user/coupons", tk, nil)
	if len(list["usable"].([]any)) != 1 {
		t.Fatalf("新券应同步C端: %v", list)
	}
	// 停用 → C端立即不可用
	off := false
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/coupons/%d/toggle", defID), superTk, map[string]any{"enabled": off})
	if code != 200 {
		t.Fatal("停用应成功")
	}
	_, list = call(t, ts, "GET", "/api/user/coupons", tk, nil)
	if len(list["usable"].([]any)) != 0 {
		t.Fatalf("停用后C端应立即可见变化: %v", list)
	}
	// 信息不全
	code, _ = call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{"name": "bad"})
	if code != 400 {
		t.Fatal("信息不全应400")
	}
	// 非法折扣
	code, _ = call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "bad", "type": "discount", "value": 150, "validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 1})
	if code != 400 {
		t.Fatal("非法折扣应400")
	}
	// 网点券适用范围校验
	code, out = call(t, ts, "POST", "/api/admin/coupons", superTk, map[string]any{
		"name": "网点券", "type": "amount", "value": 100, "scope": "outlet", "outletId": 1,
		"validFrom": "2026-01-01", "validTo": "2026-12-31", "totalCount": 10})
	if code != 200 {
		t.Fatalf("网点券创建失败: %v", out)
	}
	defID2 := int64(out["id"].(float64))
	ci2 := &store.CouponInstance{DefID: defID2, UserID: uidOf(t, ts, tk), Status: "usable"}
	_ = ms.CreateCouponInstance(ctx, ci2)
	code, _ = call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": 999999, "size": "S", "payMode": "prepaid", "couponInstanceId": ci2.ID})
	if code != 404 {
		t.Fatalf("不存在网点应404: %d", code)
	}
}

func TestChatKBAndTicket(t *testing.T) {
	_, ts, _ := newTestServer(t)
	tk := login(t, ts, "13900000011")
	// 知识库命中
	code, out := call(t, ts, "POST", "/api/chat/send", tk, map[string]any{"content": "寄存怎么收费"})
	if code != 200 {
		t.Fatalf("发送失败: %v", out)
	}
	if a, _ := out["answered"].(bool); !a {
		t.Fatalf("应命中知识库: %v", out)
	}
	// 转人工：先通过客服开关模拟下班 → 工单
	svcTk := adminLogin(t, ts, "svc")
	code, _ = call(t, ts, "POST", "/api/admin/cs/duty", svcTk, map[string]any{"on": false})
	if code != 200 {
		t.Fatal("设置下班应成功")
	}
	code, out = call(t, ts, "POST", "/api/chat/send", tk, map[string]any{"content": "我的包裹丢了，非常着急"})
	if code != 200 {
		t.Fatalf("发送失败: %v", out)
	}
	ticketNo, _ := out["ticket"].(string)
	if ticketNo == "" {
		t.Fatalf("下班应记工单: %v", out)
	}
	// 消息历史
	_, hist := call(t, ts, "GET", "/api/chat/messages", tk, nil)
	msgs, _ := hist["messages"].([]any)
	if len(msgs) < 4 {
		t.Fatalf("消息历史应完整: %v", hist)
	}
	// 主动转人工（下班）
	code, out = call(t, ts, "POST", "/api/chat/transfer", tk, map[string]any{})
	if code != 200 {
		t.Fatalf("转人工失败: %v", out)
	}
	// 客服开班后转人工 → waiting
	code, _ = call(t, ts, "POST", "/api/admin/cs/duty", svcTk, map[string]any{"on": true})
	if code != 200 {
		t.Fatal("设置在岗应成功")
	}
	code, out = call(t, ts, "POST", "/api/chat/transfer", tk, map[string]any{})
	if code != 200 {
		t.Fatalf("转人工失败: %v", out)
	}
	if st, _ := out["state"].(string); st != "waiting" {
		t.Fatalf("在岗时应进入排队: %v", out)
	}
	// 客服回复（认领）
	code, _ = call(t, ts, "POST", "/api/admin/cs/reply", svcTk, map[string]any{"userId": 0, "content": "您好，请问订单号多少？"})
	if code != 400 {
		t.Fatalf("缺少userId应400: %d", code)
	}
	// 找到用户ID
	_, me := call(t, ts, "GET", "/api/chat/state", tk, nil)
	_ = me
	scode, sess := call(t, ts, "GET", "/api/admin/cs/sessions", svcTk, nil)
	if scode != 200 {
		t.Fatal("会话列表应200")
	}
	sessArr, _ := sess["sessions"].([]any)
	if len(sessArr) == 0 {
		t.Fatal("工作台应有会话")
	}
	uid := int64(sessArr[0].(map[string]any)["userId"].(float64))
	code, _ = call(t, ts, "POST", "/api/admin/cs/reply", svcTk, map[string]any{"userId": uid, "content": "客服为您服务"})
	if code != 200 {
		t.Fatal("客服回复应成功")
	}
	// 用户收到 agent 消息
	_, hist = call(t, ts, "GET", "/api/chat/messages", tk, nil)
	found := false
	for _, m := range hist["messages"].([]any) {
		if m.(map[string]any)["sender"] == "agent" {
			found = true
		}
	}
	if !found {
		t.Fatal("用户应收到人工消息")
	}
	// bot 阶段用户不能被直接回复
	tk2 := login(t, ts, "13900000012")
	call(t, ts, "POST", "/api/chat/send", tk2, map[string]any{"content": "收费多少"})
	code, _ = call(t, ts, "POST", "/api/admin/cs/reply", svcTk, map[string]any{"userId": uidOf(t, ts, tk2), "content": "hi"})
	if code != 409 {
		t.Fatalf("bot阶段直接回复应409: %d", code)
	}
	// 工单列表与结案
	code, tickets := call(t, ts, "GET", "/api/admin/tickets?status=pending", svcTk, nil)
	if code != 200 {
		t.Fatalf("工单列表失败: %v", tickets)
	}
	tArr, _ := tickets["tickets"].([]any)
	if len(tArr) == 0 {
		t.Fatal("应有待处理工单")
	}
	tid := int64(tArr[0].(map[string]any)["id"].(float64))
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/tickets/%d/resolve", tid), svcTk, map[string]any{"reply": "已处理完毕"})
	if code != 200 {
		t.Fatal("结案应成功")
	}
	// 空回复
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/tickets/%d/resolve", tid), svcTk, map[string]any{"reply": ""})
	if code != 400 {
		t.Fatal("空回复应400")
	}
	// 客服下班开关还原
	code, _ = call(t, ts, "POST", "/api/admin/cs/duty", svcTk, map[string]any{"on": false})
	if code != 200 {
		t.Fatal("下班开关应成功")
	}
	_, duty := call(t, ts, "GET", "/api/admin/cs/duty", svcTk, nil)
	if d, _ := duty["duty"].(bool); d {
		t.Fatalf("下班状态应为false: %v", duty)
	}
}

func uidOf(t *testing.T, ts *httptest.Server, token string) int64 {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+"/api/user/me", nil)
	req.Header.Set("X-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	v, _ := out["userId"].(float64)
	return int64(v)
}

func TestDashboardAndRevenue(t *testing.T) {
	_, ts, ms := newTestServer(t)
	superTk := adminLogin(t, ts, "super")
	tk := login(t, ts, "13900000013")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	_, _ = call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk, map[string]any{"method": "scan"})
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup-done", orderID), tk, map[string]any{})
	if code != 200 {
		t.Fatalf("结算失败: %v", out)
	}
	// 看板
	ctx0 := context.Background()
	{
		f0 := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
		inc, ref, ext, _ := ms.SumRevenue(ctx0, 0, f0, time.Now())
		t.Logf("DEBUG sum inc=%d ref=%d ext=%d", inc, ref, ext)
	}
	code, dash := call(t, ts, "GET", "/api/admin/dashboard?range=today", superTk, nil)
	if code != 200 {
		t.Fatalf("看板失败: %v", dash)
	}
	orders, _ := dash["orders"].(float64)
	if orders != 1 {
		t.Fatalf("看板订单数应为1: %v", dash)
	}
	income, _ := dash["income"].(float64)
	refund, _ := dash["refund"].(float64)
	if income != 0 || refund != 1500 {
		t.Fatalf("免费订单收款0退款1500: %v", dash)
	}
	trend, _ := dash["trend"].([]any)
	if len(trend) != 7 {
		t.Fatalf("趋势应7天: %v", trend)
	}
	// 营收明细（订单维度）
	code, rev := call(t, ts, "GET", fmt.Sprintf("/api/admin/revenue?dim=order&outletId=%d", oid), superTk, nil)
	if code != 200 {
		t.Fatalf("营收失败: %v", rev)
	}
	if total, _ := rev["total"].(float64); total != -1500 {
		t.Fatalf("净营收应为-1500(退款): %v", rev)
	}
	// 网点维度
	code, _ = call(t, ts, "GET", "/api/admin/revenue?dim=outlet", superTk, nil)
	if code != 200 {
		t.Fatal("网点维度应200")
	}
	// 柜组维度
	code, _ = call(t, ts, "GET", "/api/admin/revenue?dim=group", superTk, nil)
	if code != 200 {
		t.Fatal("柜组维度应200")
	}
	// CSV 导出
	req, _ := http.NewRequest("GET", ts.URL+"/api/admin/revenue/export", nil)
	req.Header.Set("X-Token", superTk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if !strings.Contains(buf.String(), "流水ID") {
		t.Fatalf("CSV应含表头: %s", buf.String())
	}
}

func TestOutletAndRuleAdmin(t *testing.T) {
	_, ts, _ := newTestServer(t)
	superTk := adminLogin(t, ts, "super")
	// 创建网点
	code, out := call(t, ts, "POST", "/api/admin/outlets", superTk, map[string]any{
		"name": "测试站", "address": "测试地址", "city": "上海", "status": "normal", "lat": 31.2, "lng": 121.4})
	if code != 200 {
		t.Fatalf("创建网点失败: %v", out)
	}
	newID := int64(out["id"].(float64))
	// 更新
	code, _ = call(t, ts, "PUT", fmt.Sprintf("/api/admin/outlets/%d", newID), superTk, map[string]any{"status": "paused"})
	if code != 200 {
		t.Fatal("更新网点应成功")
	}
	// 暂停网点下单应被拒
	tk := login(t, ts, "13900000014")
	code, _ = call(t, ts, "POST", "/api/orders", tk, map[string]any{"outletId": newID, "size": "S", "payMode": "prepaid"})
	if code != 400 {
		t.Fatalf("暂停网点下单应400: %d", code)
	}
	// 规则配置
	code, _ = call(t, ts, "PUT", fmt.Sprintf("/api/admin/rules/%d", newID), superTk, map[string]any{
		"mode": "hourly", "priceS": 600, "priceM": 800, "priceL": 1100,
		"depositS": 1200, "depositM": 2200, "depositL": 3200, "capS": 6000, "capM": 8000, "capL": 11000, "freeMinutes": 15})
	if code != 200 {
		t.Fatal("保存规则应成功")
	}
	code, rule := call(t, ts, "GET", fmt.Sprintf("/api/admin/rules/%d", newID), superTk, nil)
	if code != 200 {
		t.Fatalf("读取规则失败: %v", rule)
	}
	if v, _ := rule["priceS"].(float64); v != 600 {
		t.Fatalf("单价应已生效: %v", rule)
	}
	// 非法模式
	code, _ = call(t, ts, "PUT", fmt.Sprintf("/api/admin/rules/%d", newID), superTk, map[string]any{"mode": "bad", "priceS": 1})
	if code != 400 {
		t.Fatal("非法模式应400")
	}
	// 非法单价
	code, _ = call(t, ts, "PUT", fmt.Sprintf("/api/admin/rules/%d", newID), superTk, map[string]any{
		"mode": "hourly", "priceS": 0, "priceM": 800, "priceL": 1100})
	if code != 400 {
		t.Fatal("零单价应400")
	}
	// 按时模式缺封顶
	code, _ = call(t, ts, "PUT", fmt.Sprintf("/api/admin/rules/%d", newID), superTk, map[string]any{
		"mode": "hourly", "priceS": 600, "priceM": 800, "priceL": 1100})
	if code != 400 {
		t.Fatal("缺封顶应400")
	}
}

func TestLockerMonitorAndActions(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	tk := login(t, ts, "13900000015")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk, oid, "S", "prepaid", 0)
	lockerID := int64(order["lockerId"].(float64))
	// 柜格列表
	code, list := call(t, ts, "GET", fmt.Sprintf("/api/admin/lockers?outletId=%d", oid), superTk, nil)
	if code != 200 {
		t.Fatalf("柜格列表失败: %v", list)
	}
	// 明细含订单
	code, detail := call(t, ts, "GET", fmt.Sprintf("/api/admin/lockers/%d", lockerID), superTk, nil)
	if code != 200 {
		t.Fatalf("明细失败: %v", detail)
	}
	if _, ok := detail["order"]; !ok {
		t.Fatalf("明细应含关联订单: %v", detail)
	}
	// 远程开柜
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", lockerID), superTk, map[string]any{"action": "remote_open"})
	if code != 200 {
		t.Fatal("远程开柜应成功")
	}
	// 切换柜格
	code, out := call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", lockerID), superTk, map[string]any{"action": "switch"})
	if code != 200 {
		t.Fatalf("切换柜格失败: %v", out)
	}
	if _, ok := out["newLocker"]; !ok {
		t.Fatalf("切换应返回新柜格: %v", out)
	}
	// 原柜格已 fault，可报修恢复
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", lockerID), superTk, map[string]any{"action": "repair"})
	if code != 200 {
		t.Fatal("报修恢复应成功")
	}
	lk, _ := ms.GetLocker(ctx, lockerID)
	if lk.Status != "idle" {
		t.Fatalf("报修后应空闲: %s", lk.Status)
	}
	// 非法操作
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/lockers/%d/action", lockerID), superTk, map[string]any{"action": "boom"})
	if code != 400 {
		t.Fatal("非法操作应400")
	}
	// 告警列表
	code, alerts := call(t, ts, "GET", "/api/admin/alerts?status=open", superTk, nil)
	if code != 200 {
		t.Fatalf("告警列表失败: %v", alerts)
	}
	arr, _ := alerts["alerts"].([]any)
	// remote_open 生成 handled 告警，switch 生成 fault 但无告警（直接标记）
	if len(arr) != 0 {
		t.Fatalf("当前不应有open告警: %d", len(arr))
	}
}

func TestAlertHandle(t *testing.T) {
	_, ts, ms := newTestServer(t)
	ctx := context.Background()
	superTk := adminLogin(t, ts, "super")
	oid := outletIDOf(t, ts)
	_ = ms.CreateAlert(ctx, &store.Alert{Title: "柜门超时未关", Level: "high", Source: "system_check", OutletID: oid, Status: "open"})
	_ = ms.CreateAlert(ctx, &store.Alert{Title: "网络异常", Level: "mid", Source: "system_check", OutletID: oid, Status: "open"})
	code, list := call(t, ts, "GET", "/api/admin/alerts?status=open", superTk, nil)
	arr, _ := list["alerts"].([]any)
	if len(arr) != 2 {
		t.Fatalf("应2条open告警: %d", len(arr))
	}
	aid := int64(arr[0].(map[string]any)["id"].(float64))
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/alerts/%d/handle", aid), superTk, map[string]any{})
	if code != 200 {
		t.Fatal("处理告警应成功")
	}
	// 重复处理
	code, _ = call(t, ts, "POST", fmt.Sprintf("/api/admin/alerts/%d/handle", aid), superTk, map[string]any{})
	if code != 409 {
		t.Fatal("重复处理应409")
	}
	code, list = call(t, ts, "GET", "/api/admin/alerts?status=open", superTk, nil)
	arr, _ = list["alerts"].([]any)
	if len(arr) != 1 {
		t.Fatalf("处理后应剩1条: %d", len(arr))
	}
}

func TestOrderDetailPermission(t *testing.T) {
	_, ts, _ := newTestServer(t)
	tk1 := login(t, ts, "13900000016")
	tk2 := login(t, ts, "13900000017")
	oid := outletIDOf(t, ts)
	order := createOrder(t, ts, tk1, oid, "S", "prepaid", 0)
	orderID := int64(order["orderId"].(float64))
	// 他人访问他人订单 404
	if code, _ := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID), tk2, nil); code != 404 {
		t.Fatal("越权访问他人订单应404")
	}
	// 本人可访问
	if code, _ := call(t, ts, "GET", fmt.Sprintf("/api/orders/%d", orderID), tk1, nil); code != 200 {
		t.Fatal("本人访问应200")
	}
	// 他人扫码取件：无本人订单应400（不泄露他人订单信息）
	if code, _ := call(t, ts, "POST", fmt.Sprintf("/api/orders/%d/pickup", orderID), tk2, map[string]any{"method": "scan"}); code != 400 {
		t.Fatal("他人取件应400")
	}
}

func TestDutyAutoByTime(t *testing.T) {
	srv, _, _ := newTestServer(t)
	if srv.DutyOn(time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)) != true {
		t.Fatal("工作时间应在岗")
	}
	if srv.DutyOn(time.Date(2026, 11, 19, 22, 0, 0, 0, time.Local)) != false {
		t.Fatal("22点应下班")
	}
	if srv.DutyOn(time.Date(2026, 11, 19, 8, 59, 0, 0, time.Local)) != false {
		t.Fatal("9点前应下班")
	}
	// 覆盖开关
	on := true
	srv.SetDutyOverride(&on)
	if !srv.DutyOn(time.Date(2026, 11, 19, 3, 0, 0, 0, time.Local)) {
		t.Fatal("开关覆盖后应在岗")
	}
	off := false
	srv.SetDutyOverride(&off)
	if srv.DutyOn(time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)) {
		t.Fatal("开关覆盖下班后应不在岗")
	}
}

func TestHealthAndBadPaths(t *testing.T) {
	_, ts, _ := newTestServer(t)
	code, out := call(t, ts, "GET", "/api/health", "", nil)
	if code != 200 {
		t.Fatalf("健康检查应200: %v", out)
	}
	// 不存在的订单
	if code, _ := call(t, ts, "GET", "/api/orders/99999", login(t, ts, "13900000018"), nil); code != 404 {
		t.Fatal("不存在订单应404")
	}
}
