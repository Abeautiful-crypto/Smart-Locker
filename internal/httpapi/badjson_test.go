package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// TestBadJSONBodies 向所有写接口发送坏JSON，覆盖解析失败分支
func TestBadJSONBodies(t *testing.T) {
	_, ts, _ := newTestServer(t)
	superTk := adminLogin(t, ts, "super")
	svcTk := adminLogin(t, ts, "svc")
	tk := login(t, ts, "13900009300")

	endpoints := []struct {
		method, path, token string
	}{
		{"POST", "/api/user/login", ""},
		{"POST", "/api/user/send-code", ""},
		{"POST", "/api/user/pickup-pwd", tk},
		{"POST", "/api/quote", ""},
		{"POST", "/api/orders", tk},
		{"POST", "/api/chat/send", tk},
		{"POST", "/api/admin/login", ""},
		{"POST", "/api/admin/outlets", superTk},
		{"PUT", "/api/admin/outlets/1", superTk},
		{"POST", "/api/admin/lockers/1/action", superTk},
		{"PUT", "/api/admin/rules/1", superTk},
		{"POST", "/api/admin/coupons", superTk},
		{"POST", "/api/admin/coupons/1/toggle", superTk},
		{"POST", "/api/admin/staff", superTk},
		{"POST", "/api/admin/staff/1/role", superTk},
		{"POST", "/api/admin/staff/1/reset-pwd", superTk},
		{"POST", "/api/admin/staff/1/toggle", superTk},
		{"POST", "/api/admin/cs/duty", svcTk},
		{"POST", "/api/admin/cs/reply", svcTk},
		{"POST", "/api/admin/tickets/1/resolve", svcTk},
	}
	for _, e := range endpoints {
		req, _ := http.NewRequest(e.method, ts.URL+e.path, strings.NewReader("{bad json"))
		req.Header.Set("Content-Type", "application/json")
		if e.token != "" {
			req.Header.Set("X-Token", e.token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Errorf("%s %s 坏JSON应4xx: %d", e.method, e.path, resp.StatusCode)
		}
	}

	// 看板时间范围
	code, _ := call(t, ts, "GET", "/api/admin/dashboard?range=yesterday", superTk, nil)
	if code != 200 {
		t.Fatal("昨日看板应200")
	}
	code, _ = call(t, ts, "GET", "/api/admin/dashboard?range=month", superTk, nil)
	if code != 200 {
		t.Fatal("本月看板应200")
	}
	code, _ = call(t, ts, "GET", "/api/admin/dashboard?range=weird", superTk, nil)
	if code != 200 {
		t.Fatal("未知范围默认今日应200")
	}
	// 网点管理员看板（本网点数据）
	siteTk := adminLogin(t, ts, "site_rm")
	code, _ = call(t, ts, "GET", "/api/admin/dashboard", siteTk, nil)
	if code != 200 {
		t.Fatal("网点管理员看板应200")
	}
	// 不存在的柜格明细 404
	code, _ = call(t, ts, "GET", "/api/admin/lockers/999999", superTk, nil)
	if code != 404 {
		t.Fatal("不存在柜格应404")
	}
	// 员工创建重复 → 409
	call(t, ts, "POST", "/api/admin/staff", superTk, map[string]any{
		"empNo": "Q1", "account": "q1", "name": "q", "phone": "13999880001", "role": "service", "password": "000000"})
	code, _ = call(t, ts, "POST", "/api/admin/staff", superTk, map[string]any{
		"empNo": "Q1", "account": "q2", "name": "q", "phone": "13999880002", "role": "service", "password": "000000"})
	if code != 409 {
		t.Fatalf("工号重复应409: %d", code)
	}
	// 客服在岗状态读取（svc）
	code, _ = call(t, ts, "GET", "/api/admin/cs/duty", svcTk, nil)
	if code != 200 {
		t.Fatal("客服读在岗应200")
	}
	// 超管不能改下班开关
	code, _ = call(t, ts, "POST", "/api/admin/cs/duty", superTk, map[string]any{"on": true})
	if code != 403 {
		t.Fatalf("超管改在岗应403: %d", code)
	}
	// 手机号边界
	if validPhone("1380000000") || validPhone("a38000000001") || validPhone("23800000001") {
		t.Fatal("手机号校验不符")
	}
	if !validPhone("13800000001") {
		t.Fatal("合法手机号应通过")
	}
}
