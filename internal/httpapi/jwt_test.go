package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"smart-locker/internal/store"
)

func TestJWTAuthAndInvalidation(t *testing.T) {
	st := store.NewMem()
	s := New(st)
	ctx := context.Background()

	u, err := st.CreateUser(ctx, "13900007777")
	if err != nil {
		t.Fatal(err)
	}
	stf := &store.Staff{EmpNo: "T1", Account: "jwt_admin", Name: "测试", Phone: "13900007778", Role: "super",
		PasswordHash: s.hashPwd("000000"), Status: "active"}
	if err := st.CreateStaff(ctx, stf); err != nil {
		t.Fatal(err)
	}

	// 1) 签发 JWT（格式应为三段式 header.payload.signature）
	utk, err := s.grantUser(u)
	if err != nil || len(strings.Split(utk, ".")) != 3 {
		t.Fatalf("用户 JWT 签发失败: %v %s", err, utk)
	}
	stk, err := s.grantStaff(stf)
	if err != nil || len(strings.Split(stk, ".")) != 3 {
		t.Fatalf("员工 JWT 签发失败: %v %s", err, stk)
	}

	// 2) 解析与 claims 校验
	uc, err := parseJWT(utk)
	if err != nil || uc.Kind != "user" || refIDOf(uc) != u.ID || uc.Ver != 0 {
		t.Fatalf("用户 JWT claims 错误: %+v %v", uc, err)
	}
	sc, err := parseJWT(stk)
	if err != nil || sc.Kind != "staff" || refIDOf(sc) != stf.ID {
		t.Fatalf("员工 JWT claims 错误: %+v %v", sc, err)
	}

	// 3) 篡改/伪造 token 应拒绝
	if _, err := parseJWT(utk + "tampered"); err == nil {
		t.Fatal("篡改 token 应拒绝")
	}
	if _, err := parseJWT("not.a.jwt"); err == nil {
		t.Fatal("非法 token 应拒绝")
	}

	// 4) 过期 token 应拒绝
	expired, _ := signJWT("user", u.ID, 0, -time.Hour)
	if _, err := parseJWT(expired); err == nil {
		t.Fatal("过期 token 应拒绝")
	}

	// 5) 用户退出：token_ver+1 → 旧 JWT 鉴权失败
	if err := st.IncrTokenVer(ctx, "user", u.ID); err != nil {
		t.Fatal(err)
	}
	r := jwtReq("GET", "/api/user/me", utk)
	if _, err := s.authUser(r); err == nil {
		t.Fatal("退出后旧 JWT 应失效")
	}

	// 6) 员工改角色：token_ver+1 → 旧 JWT 鉴权失败
	if err := st.IncrTokenVer(ctx, "staff", stf.ID); err != nil {
		t.Fatal(err)
	}
	r = jwtReq("GET", "/api/admin/dashboard", stk)
	if _, err := s.authStaff(r); err == nil {
		t.Fatal("改角色后旧 JWT 应失效")
	}

	// 7) 新签发的 JWT 可正常鉴权
	u2, _ := st.GetUserByID(ctx, u.ID)
	utk2, _ := s.grantUser(u2)
	if _, err := s.authUser(jwtReq("GET", "/api/user/me", utk2)); err != nil {
		t.Fatalf("新 JWT 应可用: %v", err)
	}
}

func jwtReq(method, path, token string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("X-Token", token)
	return r
}
