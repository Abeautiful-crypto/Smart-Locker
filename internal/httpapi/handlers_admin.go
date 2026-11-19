package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"smart-locker/internal/biz"
	"smart-locker/internal/store"
)

// ---------- 登录 ----------

type adminLoginReq struct {
	Identity string `json:"identity"` // 账号或手机号
	Password string `json:"password"`
}

// handleAdminLogin 后台登录：按账号自动判定角色
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginReq
	if err := readJSON(r, &req); err != nil || req.Identity == "" || req.Password == "" {
		writeErr(w, 400, "请输入账号与密码")
		return
	}
	st, err := s.st.GetStaffByIdentity(r.Context(), req.Identity)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 400, "账号或密码错误")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	if st.Status != "active" {
		writeErr(w, 403, "账号已被禁用，请联系管理员")
		return
	}
	if st.PasswordHash != s.hashPwd(req.Password) {
		writeErr(w, 400, "账号或密码错误")
		return
	}
	tk, err := s.grantStaff(r.Context(), st.ID)
	if err != nil {
		writeErr(w, 500, "会话创建失败")
		return
	}
	writeJSON(w, 200, map[string]any{
		"token": tk, "name": st.Name, "role": st.Role, "menus": menusOf(st.Role),
		"scopeOutletId": st.ScopeOutletID, "account": st.Account,
	})
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	_ = s.st.DeleteSession(r.Context(), claims.Token)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// staffJSON 员工脱敏输出（密码不明文展示）
func staffJSON(st *store.Staff) map[string]any {
	return map[string]any{
		"id": st.ID, "empNo": st.EmpNo, "account": st.Account, "name": st.Name, "phone": st.Phone,
		"role": st.Role, "status": st.Status, "scopeOutletId": st.ScopeOutletID, "createdAt": st.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// ---------- 数据看板 ----------

// handleDashboard 四核心指标 + 近7日趋势
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "today"
	}
	now := time.Now()
	var from, to time.Time
	switch rng {
	case "yesterday":
		y := now.AddDate(0, 0, -1)
		from = time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, now.Location())
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = now
	default:
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	}
	outletID := claims.Staff.ScopeOutletID

	income, refund, extra, err := s.st.SumRevenue(r.Context(), outletID, from, to)
	if err != nil {
		writeErr(w, 500, "统计失败")
		return
	}
	revenue := income - refund + extra
	orderCount, err := s.st.CountOrdersBetween(r.Context(), outletID, from, to)
	if err != nil {
		writeErr(w, 500, "统计失败")
		return
	}
	// 柜格使用率
	lockers, _ := s.st.ListLockers(r.Context(), outletID, 0)
	used, total := 0, len(lockers)
	for _, l := range lockers {
		if l.Status == "occupied" {
			used++
		}
	}
	usage := 0.0
	if total > 0 {
		usage = float64(used) / float64(total) * 100
	}
	openAlerts, _ := s.st.ListAlerts(r.Context(), outletID, "open")

	// 近 7 日订单趋势
	trend := []map[string]any{}
	for i := 6; i >= 0; i-- {
		d := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		next := d.AddDate(0, 0, 1)
		c, _ := s.st.CountOrdersBetween(r.Context(), outletID, d, next)
		trend = append(trend, map[string]any{"date": d.Format("01-02"), "orders": c})
	}
	writeJSON(w, 200, map[string]any{
		"range": rng, "revenue": revenue, "income": income, "refund": refund, "extra": extra,
		"orders": orderCount, "usageRate": usage, "usedLockers": used, "totalLockers": total,
		"openAlerts": len(openAlerts), "trend": trend,
	})
}

// ---------- 网点管理 ----------

// outletOut 网点输出

func outletOut(o *store.Outlet, idle int, managerName string) map[string]any {
	return map[string]any{
		"id": o.ID, "name": o.Name, "address": o.Address, "city": o.City, "status": o.Status,
		"managerStaffId": o.ManagerStaffID, "manager": managerName, "idle": idle,
		"lat": o.Lat, "lng": o.Lng, "createdAt": o.CreatedAt.Format("2006-01-02"),
	}
}

// handleAdminOutlets 网点列表（超管全部；其他角色 403）
func (s *Server) handleAdminOutlets(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outs, err := s.st.ListOutlets(r.Context(), store.OutletFilter{})
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	staffs, _ := s.st.ListStaff(r.Context())
	nameOf := map[int64]string{}
	for _, st := range staffs {
		nameOf[st.ID] = st.Name
	}
	out := []map[string]any{}
	for _, o := range outs {
		idle, _ := s.st.CountIdleByOutlet(r.Context(), o.ID)
		out = append(out, outletOut(o, idle, nameOf[o.ManagerStaffID]))
	}
	writeJSON(w, 200, out)
}

type outletReq struct {
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	Status    string  `json:"status"`
	ManagerID int64   `json:"managerStaffId"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
}

func (s *Server) handleAdminCreateOutlet(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req outletReq
	if err := readJSON(r, &req); err != nil || req.Name == "" || req.City == "" {
		writeErr(w, 400, "网点名称与城市必填")
		return
	}
	if req.Status == "" {
		req.Status = "normal"
	}
	o := &store.Outlet{Name: req.Name, Address: req.Address, City: req.City, Status: req.Status, ManagerStaffID: req.ManagerID, Lat: req.Lat, Lng: req.Lng}
	if err := s.st.CreateOutlet(r.Context(), o); err != nil {
		writeErr(w, 500, "创建失败")
		return
	}
	// 默认柜组与规则
	g := &store.Group{OutletID: o.ID, Name: "A组"}
	_ = s.st.CreateGroup(r.Context(), g)
	_ = s.st.UpsertRule(r.Context(), &store.Rule{OutletID: o.ID, Mode: "hourly", PriceS: 500, PriceM: 700, PriceL: 1000,
		DepositS: 1000, DepositM: 2000, DepositL: 3000, CapS: 5000, CapM: 7000, CapL: 10000, FreeMinutes: 10})
	writeJSON(w, 200, map[string]any{"id": o.ID})
}

func (s *Server) handleAdminUpdateOutlet(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	o, err := s.st.GetOutlet(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "网点不存在")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	var req outletReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.Name != "" {
		o.Name = req.Name
	}
	if req.Address != "" {
		o.Address = req.Address
	}
	if req.City != "" {
		o.City = req.City
	}
	if req.Status == "normal" || req.Status == "paused" {
		o.Status = req.Status
	}
	o.ManagerStaffID = req.ManagerID
	if req.Lat != 0 {
		o.Lat = req.Lat
	}
	if req.Lng != 0 {
		o.Lng = req.Lng
	}
	if err := s.st.UpdateOutlet(r.Context(), o); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- 柜格监控 ----------

func lockerJSON(l *store.Locker) map[string]any {
	return map[string]any{
		"id": l.ID, "outletId": l.OutletID, "groupId": l.GroupID, "code": l.Code, "size": l.Size,
		"status": l.Status, "faultSource": l.FaultSource, "faultNote": l.FaultNote, "orderId": l.CurrentOrderID,
	}
}

// handleAdminLockers 按柜组展示柜格实时状态
func (s *Server) handleAdminLockers(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outletID := queryInt64(r, "outletId")
	if outletID == 0 && claims.Staff.Role == "site_admin" {
		outletID = claims.Staff.ScopeOutletID
	}
	if err := s.checkOutletScope(claims, outletID); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	groupID := queryInt64(r, "groupId")
	lockers, err := s.st.ListLockers(r.Context(), outletID, groupID)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, l := range lockers {
		out = append(out, lockerJSON(l))
	}
	writeJSON(w, 200, out)
}

// handleAdminLockerDetail 柜格明细：状态、关联订单、异常来源与问题
func (s *Server) handleAdminLockerDetail(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	l, err := s.st.GetLocker(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "柜格不存在")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	if err := s.checkOutletScope(claims, l.OutletID); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	m := lockerJSON(l)
	if l.CurrentOrderID > 0 {
		if o, err := s.st.GetOrder(r.Context(), l.CurrentOrderID); err == nil {
			m["order"] = orderJSON(o)
		}
	}
	writeJSON(w, 200, m)
}

type lockerActionReq struct {
	Action string `json:"action"` // remote_open / repair / switch
	Note   string `json:"note"`
	ToSize string `json:"toSize"`
}

// handleAdminLockerAction 异常柜格操作：远程开柜/报修/切换柜格
func (s *Server) handleAdminLockerAction(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	l, err := s.st.GetLocker(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "柜格不存在")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	if err := s.checkOutletScope(claims, l.OutletID); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req lockerActionReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	switch req.Action {
	case "remote_open":
		if l.Status != "fault" && l.Status != "occupied" {
			writeErr(w, 409, "空闲柜格无需远程开柜")
			return
		}
		_ = s.st.CreateAlert(r.Context(), &store.Alert{Title: "远程开柜", Level: "mid", Source: "admin",
			OutletID: l.OutletID, LockerID: l.ID, Description: claims.Staff.Name + " 远程开柜 " + l.Code, Status: "handled", Handler: claims.Staff.Name})
		writeJSON(w, 200, map[string]any{"ok": true, "note": "柜门已远程弹开"})
	case "repair":
		if l.Status != "fault" {
			writeErr(w, 409, "柜格未处于异常状态")
			return
		}
		if l.CurrentOrderID > 0 {
			writeErr(w, 409, "柜格仍有关联订单，不能报修恢复")
			return
		}
		l.Status = "idle"
		l.FaultSource = ""
		l.FaultNote = ""
		if err := s.st.UpdateLocker(r.Context(), l); err != nil {
			writeErr(w, 500, "保存失败")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "note": "报修完成，柜格恢复空闲"})
	case "switch":
		// 切换柜格：将关联订单转移到同规格空闲柜格
		if l.CurrentOrderID == 0 {
			writeErr(w, 409, "柜格无关联订单")
			return
		}
		newLocker, err := s.st.AllocateLocker(r.Context(), l.OutletID, l.Size)
		if errors.Is(err, store.ErrNoFreeLocker) {
			writeErr(w, 409, "同规格暂无空闲柜格")
			return
		}
		if err != nil {
			writeErr(w, 500, "切换失败")
			return
		}
		o, err := s.st.GetOrder(r.Context(), l.CurrentOrderID)
		if err != nil {
			// 回滚：释放已占用的新柜格
			newLocker.Status = "idle"
			newLocker.CurrentOrderID = 0
			_ = s.st.UpdateLocker(r.Context(), newLocker)
			writeErr(w, 500, "订单查询失败")
			return
		}
		o.LockerID = newLocker.ID
		_ = s.st.UpdateOrderSettlement(r.Context(), o)
		newLocker.CurrentOrderID = o.ID
		_ = s.st.UpdateLocker(r.Context(), newLocker)
		l.Status = "fault"
		l.FaultSource = "system_check"
		l.FaultNote = "订单切换至 " + newLocker.Code
		l.CurrentOrderID = 0
		_ = s.st.UpdateLocker(r.Context(), l)
		writeJSON(w, 200, map[string]any{"ok": true, "newLocker": newLocker.Code})
	default:
		writeErr(w, 400, "操作类型不支持")
	}
}

// ---------- 收费规则 ----------

func (s *Server) handleAdminGetRule(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathInt(r, "outletId")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	rule, err := s.st.GetRule(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "未配置")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	writeJSON(w, 200, map[string]any{
		"outletId": rule.OutletID, "mode": rule.Mode,
		"priceS": rule.PriceS, "priceM": rule.PriceM, "priceL": rule.PriceL,
		"depositS": rule.DepositS, "depositM": rule.DepositM, "depositL": rule.DepositL,
		"capS": rule.CapS, "capM": rule.CapM, "capL": rule.CapL, "freeMinutes": rule.FreeMinutes,
	})
}

type ruleReq struct {
	Mode        string `json:"mode"`
	PriceS      int    `json:"priceS"`
	PriceM      int    `json:"priceM"`
	PriceL      int    `json:"priceL"`
	DepositS    int    `json:"depositS"`
	DepositM    int    `json:"depositM"`
	DepositL    int    `json:"depositL"`
	CapS        int    `json:"capS"`
	CapM        int    `json:"capM"`
	CapL        int    `json:"capL"`
	FreeMinutes int    `json:"freeMinutes"`
}

func (s *Server) handleAdminSetRule(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathInt(r, "outletId")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req ruleReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.Mode != biz.ModeHourly && req.Mode != biz.ModeDaily {
		writeErr(w, 400, "收费模式仅支持按时/按日")
		return
	}
	if req.PriceS <= 0 || req.PriceM <= 0 || req.PriceL <= 0 || req.FreeMinutes < 0 {
		writeErr(w, 400, "单价与免费时长参数不合法")
		return
	}
	if req.Mode == biz.ModeHourly && (req.CapS <= 0 || req.CapM <= 0 || req.CapL <= 0) {
		writeErr(w, 400, "按时计费必须配置24小时封顶价")
		return
	}
	rule := &store.Rule{OutletID: id, Mode: req.Mode, PriceS: req.PriceS, PriceM: req.PriceM, PriceL: req.PriceL,
		DepositS: req.DepositS, DepositM: req.DepositM, DepositL: req.DepositL,
		CapS: req.CapS, CapM: req.CapM, CapL: req.CapL, FreeMinutes: req.FreeMinutes}
	if err := s.st.UpsertRule(r.Context(), rule); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func pathInt(r *http.Request, name string) (int64, error) {
	return pathID(r, name)
}

// ---------- 优惠券管理 ----------

func couponDefJSON(d *store.CouponDef) map[string]any {
	return map[string]any{
		"id": d.ID, "name": d.Name, "type": d.Ctype, "value": d.Value, "scope": d.Scope, "outletId": d.OutletID,
		"validFrom": d.ValidFrom.Format("2006-01-02"), "validTo": d.ValidTo.Format("2006-01-02"),
		"enabled": d.Enabled, "total": d.TotalCount, "used": d.UsedCount, "note": d.Note,
	}
}

func (s *Server) handleAdminCoupons(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	defs, err := s.st.ListCouponDefs(r.Context())
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, d := range defs {
		out = append(out, couponDefJSON(d))
	}
	writeJSON(w, 200, out)
}

type couponReq struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     int    `json:"value"`
	Scope     string `json:"scope"`
	OutletID  int64  `json:"outletId"`
	ValidFrom string `json:"validFrom"`
	ValidTo   string `json:"validTo"`
	Total     int    `json:"totalCount"`
	Note      string `json:"note"`
}

// handleAdminCreateCoupon 新建优惠券（信息完整才生成）
func (s *Server) handleAdminCreateCoupon(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req couponReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if strings.TrimSpace(req.Name) == "" || req.Type == "" || req.ValidFrom == "" || req.ValidTo == "" || req.Total <= 0 {
		writeErr(w, 400, "券名称、类型、有效期、发放数量均为必填")
		return
	}
	if req.Type != biz.CouponDiscount && req.Type != biz.CouponHour && req.Type != biz.CouponAmount {
		writeErr(w, 400, "优惠类型仅支持折扣/时长/金额")
		return
	}
	if req.Type == biz.CouponDiscount && (req.Value <= 0 || req.Value >= 100) {
		writeErr(w, 400, "折扣券取值应为1-99（如80表示8折）")
		return
	}
	if req.Type == biz.CouponAmount && req.Value <= 0 {
		writeErr(w, 400, "金额券面额必须大于0")
		return
	}
	from, e1 := time.ParseInLocation("2006-01-02", req.ValidFrom, time.Local)
	to, e2 := time.ParseInLocation("2006-01-02", req.ValidTo, time.Local)
	if e1 != nil || e2 != nil || to.Before(from) {
		writeErr(w, 400, "有效期格式错误或结束早于开始")
		return
	}
	if req.Scope == "" {
		req.Scope = "all"
	}
	if req.Scope == "outlet" && req.OutletID == 0 {
		writeErr(w, 400, "适用网点必填")
		return
	}
	d := &store.CouponDef{Name: req.Name, Ctype: req.Type, Value: req.Value, Scope: req.Scope, OutletID: req.OutletID,
		ValidFrom: from, ValidTo: to, Enabled: true, TotalCount: req.Total, Note: req.Note}
	if err := s.st.CreateCouponDef(r.Context(), d); err != nil {
		writeErr(w, 500, "创建失败")
		return
	}
	writeJSON(w, 200, map[string]any{"id": d.ID})
}

func (s *Server) handleAdminToggleCoupon(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil || req.Enabled == nil {
		writeErr(w, 400, "请提供 enabled 开关")
		return
	}
	if err := s.st.SetCouponEnabled(r.Context(), id, *req.Enabled); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- 订单管理 ----------

func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outletID := queryInt64(r, "outletId")
	if claims.Staff.Role == "site_admin" {
		outletID = claims.Staff.ScopeOutletID
	}
	orders, err := s.st.ListOrders(r.Context(), store.OrderFilter{OutletID: outletID, Status: r.URL.Query().Get("status")})
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, o := range orders {
		out = append(out, orderJSON(o))
	}
	writeJSON(w, 200, out)
}

// ---------- 营收明细 ----------

// handleAdminRevenue 按网点/柜组/订单三个维度
func (s *Server) handleAdminRevenue(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outletID := queryInt64(r, "outletId")
	if claims.Staff.Role == "site_admin" {
		outletID = claims.Staff.ScopeOutletID
	}
	dim := r.URL.Query().Get("dim")
	if dim == "" {
		dim = "outlet"
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	records, err := s.st.ListRevenue(r.Context(), outletID, from, now)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	income, refund, extra, _ := s.st.SumRevenue(r.Context(), outletID, from, now)
	rows := []map[string]any{}
	if dim == "outlet" {
		agg := map[int64]*[3]int{}
		for _, rec := range records {
			a, ok := agg[rec.OutletID]
			if !ok {
				a = &[3]int{}
				agg[rec.OutletID] = a
			}
			switch rec.Kind {
			case "income":
				a[0] += rec.AmountFen
			case "refund":
				a[1] += rec.AmountFen
			case "extra":
				a[2] += rec.AmountFen
			}
		}
		for oid, a := range agg {
			name := ""
			if o, err := s.st.GetOutlet(r.Context(), oid); err == nil {
				name = o.Name
			}
			rows = append(rows, map[string]any{"outletId": oid, "name": name, "income": a[0], "refund": a[1], "extra": a[2], "net": a[0] - a[1] + a[2]})
		}
	} else if dim == "group" {
		agg := map[int64]*[3]int{}
		for _, rec := range records {
			a, ok := agg[rec.GroupID]
			if !ok {
				a = &[3]int{}
				agg[rec.GroupID] = a
			}
			switch rec.Kind {
			case "income":
				a[0] += rec.AmountFen
			case "refund":
				a[1] += rec.AmountFen
			case "extra":
				a[2] += rec.AmountFen
			}
		}
		for gid, a := range agg {
			rows = append(rows, map[string]any{"groupId": gid, "name": groupName(r, gid), "income": a[0], "refund": a[1], "extra": a[2], "net": a[0] - a[1] + a[2]})
		}
	} else {
		for _, rec := range records {
			rows = append(rows, map[string]any{
				"orderId": rec.OrderID, "kind": rec.Kind, "amount": rec.AmountFen,
				"time": rec.CreatedAt.Format("2006-01-02 15:04:05"), "outletId": rec.OutletID, "groupId": rec.GroupID,
			})
		}
	}
	writeJSON(w, 200, map[string]any{"dim": dim, "income": income, "refund": refund, "extra": extra,
		"total": income - refund + extra, "rows": rows})
}

func groupName(r *http.Request, gid int64) string {
	return fmt.Sprintf("柜组#%d", gid)
}

// handleAdminRevenueExport 导出对账单 CSV
func (s *Server) handleAdminRevenueExport(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outletID := queryInt64(r, "outletId")
	if claims.Staff.Role == "site_admin" {
		outletID = claims.Staff.ScopeOutletID
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	records, err := s.st.ListRevenue(r.Context(), outletID, from, now)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=revenue.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"流水ID", "网点ID", "柜组ID", "订单ID", "类型", "金额(分)", "时间"})
	for _, rec := range records {
		_ = cw.Write([]string{fmt.Sprintf("%d", rec.ID), fmt.Sprintf("%d", rec.OutletID), fmt.Sprintf("%d", rec.GroupID),
			fmt.Sprintf("%d", rec.OrderID), rec.Kind, fmt.Sprintf("%d", rec.AmountFen), rec.CreatedAt.Format("2006-01-02 15:04:05")})
	}
	cw.Flush()
}

// ---------- 异常告警 ----------

func alertJSON(a *store.Alert) map[string]any {
	m := map[string]any{
		"id": a.ID, "title": a.Title, "level": a.Level, "source": a.Source, "outletId": a.OutletID,
		"lockerId": a.LockerID, "description": a.Description, "status": a.Status, "handler": a.Handler,
		"time": a.CreatedAt.Format("2006-01-02 15:04"),
	}
	if !a.HandledAt.IsZero() {
		m["handledAt"] = a.HandledAt.Format("2006-01-02 15:04")
	}
	return m
}

func (s *Server) handleAdminAlerts(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	outletID := queryInt64(r, "outletId")
	if claims.Staff.Role == "site_admin" {
		outletID = claims.Staff.ScopeOutletID
	}
	alerts, err := s.st.ListAlerts(r.Context(), outletID, r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, a := range alerts {
		out = append(out, alertJSON(a))
	}
	writeJSON(w, 200, map[string]any{"alerts": out})
}

func (s *Server) handleAdminHandleAlert(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super", "site_admin"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	a, err := s.st.GetAlert(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "告警不存在")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	if err := s.checkOutletScope(claims, a.OutletID); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	if err := s.st.HandleAlert(r.Context(), id, claims.Staff.Name); err != nil {
		writeErr(w, 409, "告警已被处理")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- 员工管理 ----------

func (s *Server) handleAdminStaffList(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	staffs, err := s.st.ListStaff(r.Context())
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, st := range staffs {
		out = append(out, staffJSON(st))
	}
	writeJSON(w, 200, out)
}

type staffReq struct {
	EmpNo    string `json:"empNo"`
	Account  string `json:"account"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Password string `json:"password"`
	OutletID int64  `json:"scopeOutletId"`
}

func (s *Server) handleAdminCreateStaff(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req staffReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.EmpNo == "" || req.Account == "" || req.Name == "" || !validPhone(req.Phone) || req.Password == "" {
		writeErr(w, 400, "工号、账号、姓名、手机号、密码均为必填（手机号11位）")
		return
	}
	if req.Role != "super" && req.Role != "site_admin" && req.Role != "service" {
		writeErr(w, 400, "角色仅支持 super/site_admin/service")
		return
	}
	st := &store.Staff{EmpNo: req.EmpNo, Account: req.Account, Name: req.Name, Phone: req.Phone, Role: req.Role,
		PasswordHash: s.hashPwd(req.Password), Status: "active", ScopeOutletID: req.OutletID}
	if err := s.st.CreateStaff(r.Context(), st); err != nil {
		writeErr(w, 409, "工号、账号或手机号已存在")
		return
	}
	writeJSON(w, 200, staffJSON(st))
}

// handleAdminStaffRole 修改角色：登录态立即失效
func (s *Server) handleAdminStaffRole(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req struct {
		Role    string `json:"role"`
		OutletID int64 `json:"scopeOutletId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.Role != "super" && req.Role != "site_admin" && req.Role != "service" {
		writeErr(w, 400, "角色不支持")
		return
	}
	if err := s.st.UpdateStaffRole(r.Context(), id, req.Role); err != nil {
		writeErr(w, 500, "修改失败")
		return
	}
	// 角色变更 → 登录态立即失效
	_ = s.st.DeleteSessionsFor(r.Context(), "staff", id)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleAdminStaffResetPwd(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Password) < 6 {
		writeErr(w, 400, "新密码至少6位")
		return
	}
	if err := s.st.UpdateStaffPassword(r.Context(), id, s.hashPwd(req.Password)); err != nil {
		writeErr(w, 500, "重置失败")
		return
	}
	_ = s.st.DeleteSessionsFor(r.Context(), "staff", id)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleAdminStaffToggle(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil || (req.Status != "active" && req.Status != "disabled") {
		writeErr(w, 400, "状态仅支持 active/disabled")
		return
	}
	if err := s.st.UpdateStaffStatus(r.Context(), id, req.Status); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	if req.Status == "disabled" {
		_ = s.st.DeleteSessionsFor(r.Context(), "staff", id)
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- 客服工作台 ----------

func (s *Server) handleAdminGetDuty(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service", "super"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"duty": s.DutyOn(time.Now()), "workHours": "09:00-21:00"})
}

func (s *Server) handleAdminSetDuty(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req struct {
		On *bool `json:"on"`
	}
	if err := readJSON(r, &req); err != nil || req.On == nil {
		writeErr(w, 400, "请提供 on 开关")
		return
	}
	s.SetDutyOverride(req.On)
	writeJSON(w, 200, map[string]bool{"ok": true, "duty": *req.On})
}

func (s *Server) handleAdminCSSessions(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	sessions, err := s.st.ListAgentSessions(r.Context())
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	pending, _ := s.st.ListTickets(r.Context(), "pending")
	out := []map[string]any{}
	for _, cs := range sessions {
		name := ""
		if u, err := s.st.GetUserByID(r.Context(), cs.UserID); err == nil {
			name = maskPhone(u.Phone)
		}
		out = append(out, map[string]any{"userId": cs.UserID, "userName": name, "state": cs.State,
			"updatedAt": cs.UpdatedAt.Format("2006-01-02 15:04:05"), "online": cs.State != "bot"})
	}
	writeJSON(w, 200, map[string]any{"sessions": out, "pendingTickets": len(pending)})
}

func maskPhone(p string) string {
	if len(p) == 11 {
		return p[:3] + "****" + p[7:]
	}
	return p
}

func (s *Server) handleAdminCSMessages(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	userID := queryInt64(r, "userId")
	after := queryInt64(r, "afterId")
	msgs, err := s.st.ListMessages(r.Context(), userID, after, 200)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, m := range msgs {
		out = append(out, map[string]any{"id": m.ID, "sender": m.Sender, "content": m.Content,
			"channel": m.Channel, "time": m.CreatedAt.Format("15:04:05")})
	}
	writeJSON(w, 200, map[string]any{"messages": out})
}

type csReplyReq struct {
	UserID  int64  `json:"userId"`
	Content string `json:"content"`
}

// handleAdminCSReply 人工客服回复：认领排队会话
func (s *Server) handleAdminCSReply(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	var req csReplyReq
	if err := readJSON(r, &req); err != nil || req.UserID == 0 || strings.TrimSpace(req.Content) == "" {
		writeErr(w, 400, "参数错误")
		return
	}
	cs, _ := s.st.GetChatSession(r.Context(), req.UserID)
	if cs.State == "bot" {
		writeErr(w, 409, "该用户仍在智能客服阶段，需用户转人工后才能回复")
		return
	}
	_ = s.st.SetChatState(r.Context(), req.UserID, "agent")
	_ = s.st.AppendMessage(r.Context(), &store.ChatMessage{UserID: req.UserID, Sender: "agent", Content: req.Content, Channel: "websocket"})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleAdminTickets(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	tickets, err := s.st.ListTickets(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, t := range tickets {
		user := ""
		if u, err := s.st.GetUserByID(r.Context(), t.UserID); err == nil {
			user = maskPhone(u.Phone)
		}
		m := map[string]any{
			"id": t.ID, "ticketNo": t.TicketNo, "user": user, "question": t.Question,
			"category": t.Category, "status": t.Status, "reply": t.Reply,
			"time": t.CreatedAt.Format("2006-01-02 15:04"),
		}
		out = append(out, m)
	}
	writeJSON(w, 200, map[string]any{"tickets": out})
}

type resolveReq struct {
	Reply string `json:"reply"`
}

func (s *Server) handleAdminResolveTicket(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authStaff(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	if err := s.requireRole(claims, "service"); err != nil {
		writeErr(w, 403, err.Error())
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req resolveReq
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Reply) == "" {
		writeErr(w, 400, "请填写处理回复")
		return
	}
	tickets, _ := s.st.ListTickets(r.Context(), "")
	var target *store.Ticket
	for _, t := range tickets {
		if t.ID == id {
			target = t
			break
		}
	}
	if target == nil {
		writeErr(w, 404, "工单不存在")
		return
	}
	target.Status = "resolved"
	target.Reply = req.Reply
	target.ResolvedAt = time.Now()
	if err := s.st.UpdateTicket(r.Context(), target); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
