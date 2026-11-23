// Package httpapi 提供 Smart-Locker 三端 REST API 与静态资源。
package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"smart-locker/internal/biz"
	"smart-locker/internal/store"
)

// Server API 服务器
type Server struct {
	st store.Store

	mu           sync.Mutex
	dutyOverride *bool // 客服工作台"模拟下班"开关；nil 表示按工作时间自动判定
}

// New 创建服务器
func New(st store.Store) *Server { return &Server{st: st} }

// StaffClaims 员工会话信息
type StaffClaims struct {
	Staff *store.Staff
	Token string
}

// UserClaims 用户会话信息
type UserClaims struct {
	Token string
	refID int64
	phone string
}

// DutyOn 判断人工客服是否在岗：工作时间 09:00–21:00，工作台开关可覆盖。
func (s *Server) DutyOn(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dutyOverride != nil {
		return *s.dutyOverride
	}
	h, m := now.Hour(), now.Minute()
	if (h > 9 || (h == 9 && m >= 0)) && h < 21 {
		return true
	}
	return false
}

// SetDutyOverride 设置模拟下班开关
func (s *Server) SetDutyOverride(on *bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dutyOverride = on
}

func (s *Server) hashPwd(pwd string) string {
	h := sha256.Sum256([]byte(pwd))
	return hex.EncodeToString(h[:])
}

// ---------- 基础工具 ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodePercentText 解码形如 %E5%AF%84... 的历史脏数据；无 %XX 则原样返回。
func decodePercentText(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func queryInt64(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return v
}

func tokenFrom(r *http.Request) string {
	tk := r.Header.Get("X-Token")
	if tk == "" {
		tk = r.URL.Query().Get("token")
	}
	return tk
}

func newToken() string {
	b := make([]byte, 24)
	_, _ = cryptoRand.Read(b)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:32])
}

var errUnauthorized = errors.New("未登录或登录已失效")

// cryptoRand 随机源
var cryptoRand = rand.Reader

// authUser 用户端鉴权（JWT）
func (s *Server) authUser(r *http.Request) (*UserClaims, error) {
	tk := tokenFrom(r)
	claims, err := parseJWT(tk)
	if err != nil || claims.Kind != "user" {
		return nil, errUnauthorized
	}
	u, err := s.st.GetUserByID(r.Context(), refIDOf(claims))
	if err != nil || u.TokenVer != claims.Ver {
		return nil, errUnauthorized
	}
	return &UserClaims{Token: tk, refID: u.ID, phone: u.Phone}, nil
}

// authStaff 员工端鉴权（JWT）：校验签名/版本、员工启用状态
func (s *Server) authStaff(r *http.Request) (*StaffClaims, error) {
	tk := tokenFrom(r)
	claims, err := parseJWT(tk)
	if err != nil || claims.Kind != "staff" {
		return nil, errUnauthorized
	}
	st, err := s.st.GetStaff(r.Context(), refIDOf(claims))
	if err != nil || st.TokenVer != claims.Ver {
		return nil, errUnauthorized
	}
	if st.Status != "active" {
		return nil, errors.New("账号已被禁用")
	}
	return &StaffClaims{Staff: st, Token: tk}, nil
}

func refIDOf(c *lockerClaims) int64 {
	var id int64
	for _, ch := range c.Subject {
		if ch < '0' || ch > '9' {
			return 0
		}
		id = id*10 + int64(ch-'0')
	}
	return id
}

// requireRole 页面级权限拦截：role 在 allowed 中才放行
func (s *Server) requireRole(claims *StaffClaims, allowed ...string) error {
	for _, a := range allowed {
		if claims.Staff.Role == a {
			return nil
		}
	}
	return errors.New("无权限访问该模块")
}

// checkOutletScope 网点管理员数据范围校验：site_admin 只能访问本网点
func (s *Server) checkOutletScope(claims *StaffClaims, outletID int64) error {
	if claims.Staff.Role == "site_admin" && claims.Staff.ScopeOutletID != outletID {
		return errors.New("无权限访问其他网点数据")
	}
	return nil
}

// userAuthWrap 统一用户鉴权包装
func (s *Server) userAuthWrap(w http.ResponseWriter, r *http.Request) (*UserClaims, bool) {
	claims, err := s.authUser(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return nil, false
	}
	return claims, true
}

// Routes 注册路由
func (s *Server) Routes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})

	// 用户端
	mux.HandleFunc("POST /api/user/send-code", s.handleSendCode)
	mux.HandleFunc("POST /api/user/login", s.handleUserLogin)
	mux.HandleFunc("GET /api/user/me", s.handleUserMe)
	mux.HandleFunc("POST /api/user/pickup-pwd", s.handleSetPickupPwd)
	mux.HandleFunc("POST /api/user/avatar", s.handleSetAvatar)
	mux.HandleFunc("POST /api/user/login-pwd", s.handleSetLoginPwd)
	mux.HandleFunc("GET /api/user/coupons", s.handleUserCoupons)
	mux.HandleFunc("GET /api/user/coupons/available", s.handleAvailableCoupons)
	mux.HandleFunc("POST /api/user/coupons/{id}/claim", s.handleClaimCoupon)
	mux.HandleFunc("POST /api/user/logout", s.handleUserLogout)

	// 网点（公开）
	mux.HandleFunc("GET /api/cities", s.handleCities)
	mux.HandleFunc("GET /api/outlets", s.handleListOutlets)
	mux.HandleFunc("GET /api/outlets/{id}", s.handleOutletDetail)
	mux.HandleFunc("GET /api/outlets/{id}/rule", s.handleOutletRule)

	// 订单与存取件
	mux.HandleFunc("POST /api/quote", s.handleQuote)
	mux.HandleFunc("POST /api/orders", s.handleCreateOrder)
	mux.HandleFunc("GET /api/orders/current", s.handleCurrentOrders)
	mux.HandleFunc("GET /api/orders/history", s.handleHistoryOrders)
	mux.HandleFunc("GET /api/orders/{id}", s.handleOrderDetail)
	mux.HandleFunc("POST /api/orders/{id}/report", s.handleReportProblem)
	mux.HandleFunc("POST /api/orders/{id}/pickup", s.handlePickup)
	// 柜机触屏端：公共设备免登录取件/结算（仅凭取件码）
	mux.HandleFunc("POST /api/kiosk/pickup", s.handleKioskPickup)
	mux.HandleFunc("POST /api/kiosk/orders/{id}/pickup-done", s.handleKioskPickupDone)
	mux.HandleFunc("POST /api/orders/{id}/pickup-done", s.handlePickupDone)
	mux.HandleFunc("POST /api/orders/{id}/pay-extra", s.handlePayExtra)

	// 在线客服
	mux.HandleFunc("GET /api/ws/chat", s.handleUserWS)
	mux.HandleFunc("GET /api/admin/ws/chat", s.handleAdminWS)
	mux.HandleFunc("GET /api/chat/state", s.handleChatState)
	mux.HandleFunc("POST /api/chat/send", s.handleChatSend)
	mux.HandleFunc("POST /api/chat/transfer", s.handleChatTransfer)
	mux.HandleFunc("GET /api/chat/messages", s.handleChatMessages)

	// 管理后台
	mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("GET /api/admin/dashboard", s.handleDashboard)
	mux.HandleFunc("GET /api/admin/outlets", s.handleAdminOutlets)
	mux.HandleFunc("POST /api/admin/outlets", s.handleAdminCreateOutlet)
	mux.HandleFunc("PUT /api/admin/outlets/{id}", s.handleAdminUpdateOutlet)
	mux.HandleFunc("GET /api/admin/lockers", s.handleAdminLockers)
	mux.HandleFunc("GET /api/admin/lockers/{id}", s.handleAdminLockerDetail)
	mux.HandleFunc("POST /api/admin/lockers/{id}/action", s.handleAdminLockerAction)
	mux.HandleFunc("GET /api/admin/rules/{outletId}", s.handleAdminGetRule)
	mux.HandleFunc("PUT /api/admin/rules/{outletId}", s.handleAdminSetRule)
	mux.HandleFunc("GET /api/admin/coupons", s.handleAdminCoupons)
	mux.HandleFunc("POST /api/admin/coupons", s.handleAdminCreateCoupon)
	mux.HandleFunc("POST /api/admin/coupons/{id}/toggle", s.handleAdminToggleCoupon)
	mux.HandleFunc("GET /api/admin/orders", s.handleAdminOrders)
	mux.HandleFunc("GET /api/admin/revenue", s.handleAdminRevenue)
	mux.HandleFunc("GET /api/admin/revenue/export", s.handleAdminRevenueExport)
	mux.HandleFunc("GET /api/admin/alerts", s.handleAdminAlerts)
	mux.HandleFunc("POST /api/admin/alerts/{id}/handle", s.handleAdminHandleAlert)
	mux.HandleFunc("GET /api/admin/staff", s.handleAdminStaffList)
	mux.HandleFunc("POST /api/admin/staff", s.handleAdminCreateStaff)
	mux.HandleFunc("POST /api/admin/staff/{id}/role", s.handleAdminStaffRole)
	mux.HandleFunc("POST /api/admin/staff/{id}/reset-pwd", s.handleAdminStaffResetPwd)
	mux.HandleFunc("POST /api/admin/staff/{id}/toggle", s.handleAdminStaffToggle)
	mux.HandleFunc("GET /api/admin/cs/duty", s.handleAdminGetDuty)
	mux.HandleFunc("POST /api/admin/cs/duty", s.handleAdminSetDuty)
	mux.HandleFunc("GET /api/admin/cs/sessions", s.handleAdminCSSessions)
	mux.HandleFunc("GET /api/admin/cs/messages", s.handleAdminCSMessages)
	mux.HandleFunc("POST /api/admin/cs/reply", s.handleAdminCSReply)
	mux.HandleFunc("GET /api/admin/tickets", s.handleAdminTickets)
	mux.HandleFunc("POST /api/admin/tickets/{id}/resolve", s.handleAdminResolveTicket)
}

// grantUser 签发用户 JWT（24h）
func (s *Server) grantUser(u *store.User) (string, error) {
	return signJWT("user", u.ID, u.TokenVer, 24*time.Hour)
}

// grantStaff 签发员工 JWT（8h）
func (s *Server) grantStaff(st *store.Staff) (string, error) {
	return signJWT("staff", st.ID, st.TokenVer, 8*time.Hour)
}

// 角色可见菜单（权限矩阵）
var roleMenus = map[string][]string{
	"super":      {"dashboard", "outlets", "monitor", "rules", "coupons", "orders", "revenue", "alerts", "staff", "cs"},
	"site_admin": {"dashboard", "monitor", "orders", "revenue", "alerts"},
	"service":    {"cs"},
}

func menusOf(role string) []string {
	return roleMenus[role]
}

var _ = biz.GenCode // 保持引用
