package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"smart-locker/internal/biz"
	"smart-locker/internal/store"
)

// ---------- 登录注册 ----------

type sendCodeReq struct {
	Phone string `json:"phone"`
}

func validPhone(p string) bool {
	if len(p) != 11 {
		return false
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return false
		}
	}
	return strings.HasPrefix(p, "1")
}

// handleSendCode 发送验证码（演示环境固定 1234）
func (s *Server) handleSendCode(w http.ResponseWriter, r *http.Request) {
	var req sendCodeReq
	if err := readJSON(r, &req); err != nil || !validPhone(req.Phone) {
		writeErr(w, 400, "手机号格式不正确")
		return
	}
	writeJSON(w, 200, map[string]string{"code": "1234", "note": "演示环境验证码固定为1234"})
}

type userLoginReq struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
	Agree bool   `json:"agree"`
}

// handleUserLogin 手机号+验证码登录，未注册自动注册；必须勾选协议
func (s *Server) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	var req userLoginReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if !validPhone(req.Phone) {
		writeErr(w, 400, "手机号格式不正确")
		return
	}
	if req.Code != "1234" {
		writeErr(w, 400, "验证码错误")
		return
	}
	if !req.Agree {
		writeErr(w, 400, "请先阅读并同意用户协议与隐私政策")
		return
	}
	u, err := s.st.GetUserByPhone(r.Context(), req.Phone)
	if errors.Is(err, store.ErrNotFound) {
		u, err = s.st.CreateUser(r.Context(), req.Phone)
		if err != nil {
			writeErr(w, 500, "注册失败")
			return
		}
	} else if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	tk, err := s.grantUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "会话创建失败")
		return
	}
	writeJSON(w, 200, map[string]any{"token": tk, "phone": u.Phone, "userId": u.ID, "avatar": u.Avatar, "hasPickupPwd": u.PickupPwd != ""})
}

func (s *Server) handleUserLogout(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authUser(r)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}
	_ = s.st.DeleteSession(r.Context(), claims.Token)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// userAuthWrap 由 server.go 统一提供

func (s *Server) handleUserMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"userId": claims.refID})
}

type pickupPwdReq struct {
	Password string `json:"password"`
}

// handleSetPickupPwd 设置取件密码
func (s *Server) handleSetPickupPwd(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	var req pickupPwdReq
	if err := readJSON(r, &req); err != nil || len(req.Password) < 6 {
		writeErr(w, 400, "取件密码至少6位")
		return
	}
	if err := s.st.SetPickupPwd(r.Context(), claims.refID, s.hashPwd(req.Password)); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handleSetAvatar 设置用户头像（emoji 或短标识）
func (s *Server) handleSetAvatar(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	var req struct {
		Avatar string `json:"avatar"`
	}
	if err := readJSON(r, &req); err != nil || req.Avatar == "" {
		writeErr(w, 400, "头像不能为空")
		return
	}
	// 支持 emoji（≤4 字符）或压缩后的图片 data URL（VARCHAR(16000) 约 12KB）
	if len(req.Avatar) > 12*1024 || (len([]rune(req.Avatar)) > 4 && !strings.HasPrefix(req.Avatar, "data:image/")) {
		writeErr(w, 400, "头像格式不正确或过大")
		return
	}
	if err := s.st.SetAvatar(r.Context(), claims.refID, req.Avatar); err != nil {
		writeErr(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- 网点 ----------

// handleCities 已开通服务的城市与空闲柜格数
func (s *Server) handleCities(w http.ResponseWriter, r *http.Request) {
	cities := []string{"上海", "郑州", "深圳", "杭州"}
	out := []map[string]any{}
	for _, c := range cities {
		n, err := s.st.IdleCountByCity(r.Context(), c)
		if err != nil {
			writeErr(w, 500, "查询失败")
			return
		}
		out = append(out, map[string]any{"city": c, "idle": n})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleListOutlets(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	outs, err := s.st.ListOutlets(r.Context(), store.OutletFilter{City: city, Status: "normal"})
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	res := []map[string]any{}
	for _, o := range outs {
		idle, _ := s.st.CountIdleByOutlet(r.Context(), o.ID)
		rule, _ := s.st.GetRule(r.Context(), o.ID)
		item := map[string]any{"id": o.ID, "name": o.Name, "address": o.Address, "city": o.City, "idle": idle, "lat": o.Lat, "lng": o.Lng}
		if rule != nil {
			item["priceS"] = rule.PriceS
			item["priceM"] = rule.PriceM
			item["priceL"] = rule.PriceL
			item["mode"] = rule.Mode
		}
		res = append(res, item)
	}
	writeJSON(w, 200, res)
}

func (s *Server) handleOutletDetail(w http.ResponseWriter, r *http.Request) {
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
	groups, _ := s.st.ListGroups(r.Context(), o.ID)
	lockers, _ := s.st.ListLockers(r.Context(), o.ID, 0)
	bySize := map[string]int{"S": 0, "M": 0, "L": 0}
	byStatus := map[string]int{"idle": 0, "occupied": 0, "fault": 0}
	for _, l := range lockers {
		byStatus[l.Status]++
		if l.Status == "idle" {
			bySize[l.Size]++
		}
	}
	writeJSON(w, 200, map[string]any{
		"id": o.ID, "name": o.Name, "address": o.Address, "city": o.City, "status": o.Status,
		"lat": o.Lat, "lng": o.Lng,
		"groups": groups, "idleBySize": bySize, "byStatus": byStatus,
	})
}

// handleOutletRule 网点收费规则（公开，支付页展示）
func (s *Server) handleOutletRule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	rule, err := s.st.GetRule(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "网点未配置规则")
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

// ---------- 计费试算与下单 ----------

type quoteReq struct {
	OutletID int64  `json:"outletId"`
	Size     string `json:"size"`
	PayMode  string `json:"payMode"`
}

func (s *Server) ruleOf(r *http.Request, outletID int64) (*biz.Rule, error) {
	rule, err := s.st.GetRule(r.Context(), outletID)
	if err != nil {
		return nil, err
	}
	b := biz.Rule{Mode: rule.Mode, PriceS: rule.PriceS, PriceM: rule.PriceM, PriceL: rule.PriceL,
		DepositS: rule.DepositS, DepositM: rule.DepositM, DepositL: rule.DepositL,
		CapS: rule.CapS, CapM: rule.CapM, CapL: rule.CapL, FreeMinutes: rule.FreeMinutes}
	b.Normalize()
	return &b, nil
}

// handleQuote 支付页计费展示
func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	var req quoteReq
	if err := readJSON(r, &req); err != nil || req.OutletID == 0 {
		writeErr(w, 400, "参数错误")
		return
	}
	if req.PayMode == "" {
		req.PayMode = biz.PayPrepaid
	}
	if req.PayMode != biz.PayPrepaid && req.PayMode != biz.PayDeposit {
		writeErr(w, 400, "计费模式不支持")
		return
	}
	br, err := s.ruleOf(r, req.OutletID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "网点未配置收费规则")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	price, err := br.SizePrice(req.Size)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	deposit, _ := br.SizeDeposit(req.Size)
	cap, _ := br.SizeCap(req.Size)
	prepaid, _ := biz.PrepaidAmount(br, req.Size)
	modeName := "按时计费"
	if br.Mode == biz.ModeDaily {
		modeName = "按日计费"
	}
	resp := map[string]any{
		"mode": br.Mode, "modeName": modeName, "price": price, "deposit": deposit,
		"cap": cap, "freeMinutes": br.FreeMinutes, "prepaid": prepaid,
	}
	if req.PayMode == biz.PayDeposit {
		resp["payNow"] = deposit
		resp["payName"] = "押金模式（先押金，按实际时长扣费）"
	} else {
		resp["payNow"] = prepaid
		resp["payName"] = "预付模式（1小时费用+押金）"
	}
	writeJSON(w, 200, resp)
}

type createOrderReq struct {
	OutletID         int64  `json:"outletId"`
	Size             string `json:"size"`
	PayMode          string `json:"payMode"`
	CouponInstanceID int64  `json:"couponInstanceId"`
}

// handleCreateOrder 支付成功→分配柜格→生成订单与取件码（模拟柜门弹开）
func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	var req createOrderReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	if req.PayMode == "" {
		req.PayMode = biz.PayPrepaid
	}
	if req.PayMode != biz.PayPrepaid && req.PayMode != biz.PayDeposit {
		writeErr(w, 400, "计费模式不支持")
		return
	}
	o, err := s.st.GetOutlet(r.Context(), req.OutletID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, 404, "网点不存在")
		return
	}
	if err != nil || o.Status != "normal" {
		writeErr(w, 400, "网点已暂停服务")
		return
	}
	br, err := s.ruleOf(r, req.OutletID)
	if err != nil {
		writeErr(w, 500, "网点未配置收费规则")
		return
	}
	if _, err := br.SizePrice(req.Size); err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	// 优惠券校验（下单选择；可用状态由启用开关与有效期控制）
	if req.CouponInstanceID > 0 {
		ci, err := s.st.GetCouponInstance(r.Context(), req.CouponInstanceID)
		if err != nil || ci.UserID != claims.refID {
			writeErr(w, 400, "优惠券不可用")
			return
		}
		now := time.Now()
		if ci.Status != "usable" || !ci.Def.Enabled || now.Before(ci.Def.ValidFrom) || now.After(ci.Def.ValidTo) {
			writeErr(w, 400, "优惠券已停用或不在有效期")
			return
		}
		if ci.Def.Scope == "outlet" && ci.Def.OutletID != req.OutletID {
			writeErr(w, 400, "优惠券不适用于该网点")
			return
		}
		// 下单时仅登记券选择，实际抵扣在结束订单时结算
	}

	// 分配空闲柜格（异常/占用柜格不参与分配）
	locker, err := s.st.AllocateLocker(r.Context(), req.OutletID, req.Size)
	if errors.Is(err, store.ErrNoFreeLocker) {
		writeErr(w, 409, "该规格暂无空闲柜格，请更换规格或网点")
		return
	}
	if err != nil {
		writeErr(w, 500, "分配柜格失败")
		return
	}

	prepaid, _ := biz.PrepaidAmount(br, req.Size)
	deposit, _ := br.SizeDeposit(req.Size)
	now := time.Now()
	order := &store.Order{
		OrderNo: biz.GenOrderNo(now), UserID: claims.refID, OutletID: req.OutletID, LockerID: locker.ID,
		Size: req.Size, PayMode: req.PayMode, PrepaidFen: prepaid, DepositFen: deposit,
		CouponInstanceID: req.CouponInstanceID, Code: biz.GenCode(), Status: "active", StartedAt: now,
	}
	if err := s.st.CreateOrder(r.Context(), order); err != nil {
		writeErr(w, 500, "创建订单失败")
		return
	}
	// 支付流水（模拟微信支付）
	paidFen := prepaid
	if req.PayMode == biz.PayDeposit {
		paidFen = deposit
	}
	_ = s.st.AddPayment(r.Context(), &store.Payment{OrderNo: order.OrderNo, Kind: req.PayMode, AmountFen: paidFen, Channel: "wechat_mock", Status: "paid"})
	// 柜格关联订单
	locker.CurrentOrderID = order.ID
	_ = s.st.UpdateLocker(r.Context(), locker)

	writeJSON(w, 200, map[string]any{
		"orderId": order.ID, "orderNo": order.OrderNo, "lockerId": locker.ID, "lockerCode": locker.Code,
		"code": order.Code, "paid": paidFen, "startedAt": order.StartedAt.Format(time.RFC3339),
		"note": "柜门已弹开，请放入物品并关门",
	})
}

// ---------- 订单查询 ----------

func orderJSON(o *store.Order) map[string]any {
	m := map[string]any{
		"id": o.ID, "orderNo": o.OrderNo, "outletId": o.OutletID, "lockerId": o.LockerID, "size": o.Size,
		"payMode": o.PayMode, "prepaid": o.PrepaidFen, "deposit": o.DepositFen,
		"couponInstanceId": o.CouponInstanceID, "couponDiscount": o.CouponDiscountFen,
		"code": o.Code, "status": o.Status,
		"startedAt": o.StartedAt.Format(time.RFC3339),
		"fee":       o.FeeFen, "refund": o.RefundFen, "extra": o.ExtraFen, "extraPaid": o.ExtraPaid,
		"settleNote": o.SettleNote, "pickupMethod": o.PickupMethod,
	}
	if !o.EndedAt.IsZero() {
		m["endedAt"] = o.EndedAt.Format(time.RFC3339)
	}
	return m
}

func (s *Server) handleCurrentOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	orders, err := s.st.ListOrders(r.Context(), store.OrderFilter{UserID: claims.refID, Status: "active"})
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, o := range orders {
		m := orderJSON(o)
		lk, _ := s.st.GetLocker(r.Context(), o.LockerID)
		if lk != nil {
			m["lockerCode"] = lk.Code
		}
		// 剩余时长提示（按 24h 封顶展示已用时长）
		minutes := int(time.Since(o.StartedAt).Minutes())
		m["usedMinutes"] = minutes
		out = append(out, m)
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleHistoryOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	orders, err := s.st.ListOrders(r.Context(), store.OrderFilter{UserID: claims.refID, Status: "finished"})
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

func (s *Server) handleOrderDetail(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	o, err := s.st.GetOrder(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && o.UserID != claims.refID) {
		writeErr(w, 404, "订单不存在")
		return
	}
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	m := orderJSON(o)
	// 结算展示状态：no_due/due/refund/free（due 需补交时订单仍为进行中）
	switch {
	case o.ExtraFen > 0 && !o.ExtraPaid:
		m["settleStatus"] = "due"
	case o.Status == "finished":
		switch {
		case o.FeeFen == 0 && o.RefundFen > 0:
			m["settleStatus"] = "free"
		case o.RefundFen > 0:
			m["settleStatus"] = "refund"
		default:
			m["settleStatus"] = "no_due"
		}
	}
	writeJSON(w, 200, m)
}

type reportReq struct {
	Problem string `json:"problem"`
}

// handleReportProblem 上报柜格异常：柜格标记异常并生成告警，订单取消并全额退款
func (s *Server) handleReportProblem(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	var req reportReq
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Problem) == "" {
		writeErr(w, 400, "请选择问题类型")
		return
	}
	o, err := s.st.GetOrder(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && o.UserID != claims.refID) {
		writeErr(w, 404, "订单不存在")
		return
	}
	if err != nil || o.Status != "active" {
		writeErr(w, 409, "订单状态不允许上报")
		return
	}
	lk, _ := s.st.GetLocker(r.Context(), o.LockerID)
	if lk != nil {
		lk.Status = "fault"
		lk.FaultSource = "user_report"
		lk.FaultNote = req.Problem
		lk.CurrentOrderID = 0
		_ = s.st.UpdateLocker(r.Context(), lk)
		_ = s.st.CreateAlert(r.Context(), &store.Alert{
			Title: "用户上报柜格异常", Level: "mid", Source: "user_report",
			OutletID: o.OutletID, LockerID: lk.ID,
			Description: "柜格" + lk.Code + "：" + req.Problem, Status: "open",
		})
	}
	// 全额退款（未放入物品）
	o.Status = "finished"
	o.EndedAt = time.Now()
	o.FeeFen = 0
	o.RefundFen = o.PrepaidFen
	if o.PayMode == biz.PayDeposit {
		o.RefundFen = o.DepositFen
	}
	o.SettleNote = "柜门异常上报，全额退款"
	_ = s.st.UpdateOrderSettlement(r.Context(), o)
	_ = s.st.AddPayment(r.Context(), &store.Payment{OrderNo: o.OrderNo, Kind: "refund", AmountFen: o.RefundFen, Channel: "wechat_mock", Status: "refunded"})
	writeJSON(w, 200, map[string]any{"ok": true, "refund": o.RefundFen, "note": "异常已上报，订单关闭并全额退款"})
}

// ---------- 取件与结算 ----------

type pickupReq struct {
	Method   string `json:"method"` // code / scan / face / password
	Code     string `json:"code"`
	Password string `json:"password"`
}

// handlePickup 取件验证：取件码/扫码/人脸/密码任一通过即开柜
func (s *Server) handlePickup(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	var req pickupReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, "请求格式错误")
		return
	}
	var order *store.Order
	var err error
	switch req.Method {
	case "code":
		if len(req.Code) != 6 {
			writeErr(w, 400, "请输入6位取件码")
			return
		}
		order, err = s.st.GetOrderByCode(r.Context(), req.Code, true)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 400, "取件码错误或订单已结束")
			return
		}
	case "scan":
		// 手机扫码取件：取本人当前进行中订单（默认方式）
		orders, _ := s.st.ListOrders(r.Context(), store.OrderFilter{UserID: claims.refID, Status: "active", Limit: 1})
		if len(orders) == 0 {
			writeErr(w, 400, "当前没有进行中的订单")
			return
		}
		order = orders[0]
	case "face":
		// 人脸识别（演示）：直接命中本人当前订单
		orders, _ := s.st.ListOrders(r.Context(), store.OrderFilter{UserID: claims.refID, Status: "active", Limit: 1})
		if len(orders) == 0 {
			writeErr(w, 400, "当前没有进行中的订单")
			return
		}
		order = orders[0]
	case "password":
		if req.Password == "" {
			writeErr(w, 400, "请输入取件密码")
			return
		}
		orders, _ := s.st.ListOrders(r.Context(), store.OrderFilter{UserID: claims.refID, Status: "active", Limit: 1})
		if len(orders) == 0 {
			writeErr(w, 400, "当前没有进行中的订单")
			return
		}
		u, _ := s.st.GetUserByID(r.Context(), claims.refID)
		if u == nil || u.PickupPwd == "" || u.PickupPwd != s.hashPwd(req.Password) {
			writeErr(w, 400, "取件密码错误")
			return
		}
		order = orders[0]
	default:
		writeErr(w, 400, "取件方式不支持")
		return
	}
	if err != nil || order == nil || order.UserID != claims.refID {
		writeErr(w, 403, "订单不存在或无权操作")
		return
	}
	lk, _ := s.st.GetLocker(r.Context(), order.LockerID)
	lockerCode := ""
	if lk != nil {
		lockerCode = lk.Code
	}
	writeJSON(w, 200, map[string]any{
		"orderId": order.ID, "orderNo": order.OrderNo, "lockerCode": lockerCode,
		"note": "校验通过，柜门已弹开，请取出物品并关门",
	})
}

// phoneOf 保留备用
func phoneOf(c *UserClaims) string { return c.phone }

var _ = phoneOf

// settleOrder 执行订单结算并写营收流水（内部共用）
func (s *Server) settleOrder(r *http.Request, o *store.Order, method string) (*biz.Settlement, error) {
	br, err := s.ruleOf(r, o.OutletID)
	if err != nil {
		return nil, err
	}
	// 优惠券：结束订单时结算
	var def *biz.CouponDef
	if o.CouponInstanceID > 0 {
		ci, err := s.st.GetCouponInstance(r.Context(), o.CouponInstanceID)
		if err == nil && ci.Status == "usable" && ci.Def.Enabled {
			now := time.Now()
			if !now.Before(ci.Def.ValidFrom) && !now.After(ci.Def.ValidTo) {
				def = &biz.CouponDef{Name: ci.Def.Name, Type: ci.Def.Ctype, Value: ci.Def.Value}
			}
		}
	}
	paid := o.PrepaidFen
	if o.PayMode == biz.PayDeposit {
		paid = o.DepositFen
	}
	settle := biz.Settle(br, o.Size, o.PayMode, paid, def, o.StartedAt, time.Now())

	if o.CouponInstanceID > 0 && settle.CouponFen > 0 {
		if err := s.st.UseCouponInstance(r.Context(), o.CouponInstanceID, o.ID); err == nil {
			// 核销成功
		} else {
			// 核销失败（停用/过期）则取消抵扣，重算
			settle2 := biz.Settle(br, o.Size, o.PayMode, paid, nil, o.StartedAt, time.Now())
			settle = settle2
		}
	}
	o.FeeFen = settle.FeeFen
	o.RefundFen = settle.RefundFen
	o.ExtraFen = settle.ExtraFen
	o.SettleNote = settle.Note
	o.PickupMethod = method
	if settle.ExtraFen == 0 {
		o.Status = "finished"
		o.EndedAt = time.Now()
	} else {
		// 需补交：保持 active，补交完成后再结束
		o.Status = "active"
	}
	if err := s.st.UpdateOrderSettlement(r.Context(), o); err != nil {
		return nil, err
	}
	// 退款流水
	if settle.RefundFen > 0 {
		_ = s.st.AddPayment(r.Context(), &store.Payment{OrderNo: o.OrderNo, Kind: "refund", AmountFen: settle.RefundFen, Channel: "wechat_mock", Status: "refunded"})
	}
	// 营收流水：预付在收款时未记，这里按净收入记账（income=实收费用，refund=退款）
	lk, _ := s.st.GetLocker(r.Context(), o.LockerID)
	groupID := int64(0)
	if lk != nil {
		groupID = lk.GroupID
	}
	_ = s.st.AddRevenue(r.Context(), &store.RevenueRecord{OutletID: o.OutletID, GroupID: groupID, OrderID: o.ID, Kind: "income", AmountFen: settle.FeeFen})
	if settle.RefundFen > 0 {
		_ = s.st.AddRevenue(r.Context(), &store.RevenueRecord{OutletID: o.OutletID, GroupID: groupID, OrderID: o.ID, Kind: "refund", AmountFen: settle.RefundFen})
	}
	if settle.ExtraFen > 0 {
		_ = s.st.AddRevenue(r.Context(), &store.RevenueRecord{OutletID: o.OutletID, GroupID: groupID, OrderID: o.ID, Kind: "extra", AmountFen: settle.ExtraFen})
	}
	return &settle, nil
}

// handlePickupDone 取件关门→结算（多退少补）→柜格释放
func (s *Server) handlePickupDone(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	o, err := s.st.GetOrder(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && o.UserID != claims.refID) {
		writeErr(w, 404, "订单不存在")
		return
	}
	if err != nil || o.Status != "active" {
		writeErr(w, 409, "订单已结束或状态不允许")
		return
	}
	settle, err := s.settleOrder(r, o, "code")
	if err != nil {
		writeErr(w, 500, "结算失败")
		return
	}
	// 释放柜格
	if lk, err := s.st.GetLocker(r.Context(), o.LockerID); err == nil {
		if settle.ExtraFen == 0 {
			lk.Status = "idle"
		}
		lk.CurrentOrderID = 0
		_ = s.st.UpdateLocker(r.Context(), lk)
	}
	resp := map[string]any{
		"fee": settle.FeeFen, "coupon": settle.CouponFen, "refund": settle.RefundFen,
		"extra": settle.ExtraFen, "kind": settle.Kind, "note": settle.Note,
		"status": o.Status,
	}
	writeJSON(w, 200, resp)
}

// handlePayExtra 补交差额后结束订单
func (s *Server) handlePayExtra(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, 400, "参数错误")
		return
	}
	o, err := s.st.GetOrder(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && o.UserID != claims.refID) {
		writeErr(w, 404, "订单不存在")
		return
	}
	if err != nil || o.Status != "active" || o.ExtraFen <= 0 || o.ExtraPaid {
		writeErr(w, 409, "订单无需补交或状态不允许")
		return
	}
	_ = s.st.AddPayment(r.Context(), &store.Payment{OrderNo: o.OrderNo, Kind: "extra", AmountFen: o.ExtraFen, Channel: "wechat_mock", Status: "paid"})
	o.ExtraPaid = true
	o.Status = "finished"
	o.EndedAt = time.Now()
	o.SettleNote += "；已补交差额"
	if err := s.st.UpdateOrderSettlement(r.Context(), o); err != nil {
		writeErr(w, 500, "更新失败")
		return
	}
	if lk, err := s.st.GetLocker(r.Context(), o.LockerID); err == nil {
		lk.Status = "idle"
		lk.CurrentOrderID = 0
		_ = s.st.UpdateLocker(r.Context(), lk)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "paid": o.ExtraFen, "status": "finished"})
}

// ---------- 优惠券（C端） ----------

func couponJSON(ci *store.CouponInstance, now time.Time) map[string]any {
	d := ci.Def
	usable := ci.Status == "usable" && d.Enabled && !now.Before(d.ValidFrom) && !now.After(d.ValidTo)
	typeName := ""
	valDesc := ""
	switch d.Ctype {
	case biz.CouponDiscount:
		typeName = "折扣券"
		valDesc = fmtFrac(d.Value)
	case biz.CouponHour:
		typeName = "时长券"
		valDesc = fmtMinutes(d.Value)
	case biz.CouponAmount:
		typeName = "金额券"
		valDesc = fmtFen(d.Value)
	}
	return map[string]any{
		"id": ci.ID, "name": d.Name, "type": typeName, "valueDesc": valDesc,
		"scope": d.Scope, "outletId": d.OutletID,
		"validFrom": d.ValidFrom.Format("2006-01-02"), "validTo": d.ValidTo.Format("2006-01-02"),
		"status": ci.Status, "usable": usable, "note": d.Note,
	}
}

func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}

func fmtFrac(v int) string {
	return intToString(v/10) + "." + intToString(v%10) + "折"
}

func fmtMinutes(m int) string { return intToString(m) + "分钟免费" }

func fmtFen(fen int) string { return intToString(fen/100) + "元" }

// handleUserCoupons 可使用/已过期两个标签页数据
func (s *Server) handleUserCoupons(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	list, err := s.st.ListUserCoupons(r.Context(), claims.refID)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	now := time.Now()
	usable := []map[string]any{}
	expired := []map[string]any{}
	for _, ci := range list {
		m := couponJSON(ci, now)
		if m["usable"].(bool) {
			usable = append(usable, m)
		} else {
			expired = append(expired, m)
		}
	}
	writeJSON(w, 200, map[string]any{"usable": usable, "expired": expired})
}

// ---------- 在线客服 ----------

func (s *Server) handleChatState(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	cs, _ := s.st.GetChatSession(r.Context(), claims.refID)
	writeJSON(w, 200, map[string]any{
		"state": cs.State, "duty": s.DutyOn(time.Now()),
		"channel": "websocket", "fallback": "http-polling",
	})
}

type chatSendReq struct {
	Content string `json:"content"`
}

// handleChatSend 智能客服：知识库打分命中直接回答；未命中按在岗状态转人工或记工单
func (s *Server) handleChatSend(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	var req chatSendReq
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeErr(w, 400, "消息不能为空")
		return
	}
	ctx := r.Context()
	_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "user", Content: req.Content, Channel: "http"})
	notifyUserMessage(claims.refID, "user", req.Content)

	cs, _ := s.st.GetChatSession(ctx, claims.refID)
	if cs.State == "agent" {
		// 人工接待中，等待客服回复
		writeJSON(w, 200, map[string]any{"ok": true, "state": "agent"})
		return
	}
	if cs.State == "waiting" {
		writeJSON(w, 200, map[string]any{"ok": true, "state": "waiting", "note": "排队中，人工客服接入后回复"})
		return
	}
	// 智能客服应答：知识库匹配（不展示来源）
	kbList, err := s.st.ListKB(ctx)
	if err != nil {
		writeErr(w, 500, "知识库不可用")
		return
	}
	entries := make([]biz.KBItem, 0, len(kbList))
	for _, e := range kbList {
		entries = append(entries, biz.KBItem{Category: e["category"], Keywords: e["keywords"], Answer: e["answer"]})
	}
	if hit := biz.MatchKB(entries, req.Content); hit != nil {
		_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "bot", Content: hit.Answer, Channel: "websocket"})
		writeJSON(w, 200, map[string]any{"ok": true, "answered": true, "state": "bot"})
		return
	}
	// 未命中
	if s.DutyOn(time.Now()) {
		_ = s.st.SetChatState(ctx, claims.refID, "waiting")
		_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "system", Content: "正在为您转接人工客服，请稍候…", Channel: "http"})
		writeJSON(w, 200, map[string]any{"ok": true, "answered": false, "state": "waiting", "note": "人工在岗，已转接排队"})
		return
	}
	// 人工下班：记录工单
	seq, _ := s.st.CountTodayTickets(ctx, time.Now())
	seq++
	ticketNo := biz.GenTicketNo(time.Now(), seq)
	_ = s.st.CreateTicket(ctx, &store.Ticket{TicketNo: ticketNo, UserID: claims.refID, Question: req.Content, Status: "pending"})
	_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "system",
		Content: "人工客服已下班，您的问题已记录为工单 " + ticketNo + "，我们将在次日优先处理并主动联系您。紧急问题请拨打客服热线 400-800-1234。",
		Channel: "local"})
	writeJSON(w, 200, map[string]any{"ok": true, "answered": false, "state": "bot", "ticket": ticketNo})
}

// handleChatTransfer 用户主动转人工
func (s *Server) handleChatTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if s.DutyOn(time.Now()) {
		_ = s.st.SetChatState(ctx, claims.refID, "waiting")
		_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "system", Content: "正在为您转接人工客服，请稍候…", Channel: "http"})
		writeJSON(w, 200, map[string]any{"state": "waiting"})
		return
	}
	_ = s.st.AppendMessage(ctx, &store.ChatMessage{UserID: claims.refID, Sender: "system",
		Content: "当前为非工作时间（09:00-21:00），人工客服不在线。您可直接输入问题，未命中知识库将自动记录为工单，次日优先处理。",
		Channel: "local"})
	writeJSON(w, 200, map[string]any{"state": "bot", "duty": false})
}

func (s *Server) handleChatMessages(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.userAuthWrap(w, r)
	if !ok {
		return
	}
	after := queryInt64(r, "afterId")
	msgs, err := s.st.ListMessages(r.Context(), claims.refID, after, 100)
	if err != nil {
		writeErr(w, 500, "查询失败")
		return
	}
	out := []map[string]any{}
	for _, m := range msgs {
		out = append(out, map[string]any{
			"id": m.ID, "sender": m.Sender, "content": m.Content, "channel": m.Channel,
			"time": m.CreatedAt.Format("15:04:05"),
		})
	}
	cs, _ := s.st.GetChatSession(r.Context(), claims.refID)
	writeJSON(w, 200, map[string]any{"messages": out, "state": cs.State, "duty": s.DutyOn(time.Now())})
}
