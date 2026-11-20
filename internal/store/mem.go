package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemStore 内存实现：单元测试与无 MySQL 环境演示（DSN=mem）。
type MemStore struct {
	mu sync.Mutex

	autoID     int64
	users      map[int64]*User
	staff      map[int64]*Staff
	sessions   map[string]*SessionRow
	outlets    map[int64]*Outlet
	groups     map[int64]*Group
	lockers    map[int64]*Locker
	rules      map[int64]*Rule
	orders     map[int64]*Order
	couponDefs map[int64]*CouponDef
	couponIns  map[int64]*CouponInstance
	alerts     map[int64]*Alert
	tickets    map[int64]*Ticket
	kb         []map[string]string
	messages   []*ChatMessage
	chatSess   map[int64]*ChatSession
	payments   []*Payment
	revenue    []*RevenueRecord
}

// NewMem 创建空内存存储
func NewMem() *MemStore {
	return &MemStore{
		users: map[int64]*User{}, staff: map[int64]*Staff{}, sessions: map[string]*SessionRow{},
		outlets: map[int64]*Outlet{}, groups: map[int64]*Group{}, lockers: map[int64]*Locker{},
		rules: map[int64]*Rule{}, orders: map[int64]*Order{}, couponDefs: map[int64]*CouponDef{},
		couponIns: map[int64]*CouponInstance{}, alerts: map[int64]*Alert{}, tickets: map[int64]*Ticket{},
		chatSess: map[int64]*ChatSession{}, kb: []map[string]string{},
	}
}

func (m *MemStore) nextID() int64 {
	m.autoID++
	return m.autoID
}

func (m *MemStore) Close() error { return nil }

// ---------- 用户 ----------

func (m *MemStore) CreateUser(_ context.Context, phone string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Phone == phone {
			return nil, ErrConflict
		}
	}
	u := &User{ID: m.nextID(), Phone: phone, CreatedAt: time.Now()}
	m.users[u.ID] = u
	return cloneUser(u), nil
}

func (m *MemStore) GetUserByPhone(_ context.Context, phone string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Phone == phone {
			return cloneUser(u), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemStore) GetUserByID(_ context.Context, id int64) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneUser(u), nil
}

func (m *MemStore) SetPickupPwd(_ context.Context, userID int64, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.PickupPwd = hash
	return nil
}
func (m *MemStore) SetAvatar(_ context.Context, userID int64, avatar string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.Avatar = avatar
	return nil
}

// ---------- 员工与会话 ----------

func (m *MemStore) CreateStaff(_ context.Context, st *Staff) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.staff {
		if s.EmpNo == st.EmpNo || s.Account == st.Account || s.Phone == st.Phone {
			return ErrConflict
		}
	}
	st.ID = m.nextID()
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	m.staff[st.ID] = st
	return nil
}

func (m *MemStore) GetStaff(_ context.Context, id int64) (*Staff, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.staff[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneStaff(s), nil
}

func (m *MemStore) GetStaffByIdentity(_ context.Context, ident string) (*Staff, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.staff {
		if s.Account == ident || s.Phone == ident {
			return cloneStaff(s), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemStore) ListStaff(_ context.Context) ([]*Staff, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Staff{}
	for _, s := range m.staff {
		out = append(out, cloneStaff(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) UpdateStaffRole(_ context.Context, id int64, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.staff[id]
	if !ok {
		return ErrNotFound
	}
	s.Role = role
	return nil
}

func (m *MemStore) UpdateStaffStatus(_ context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.staff[id]
	if !ok {
		return ErrNotFound
	}
	s.Status = status
	return nil
}

func (m *MemStore) UpdateStaffPassword(_ context.Context, id int64, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.staff[id]
	if !ok {
		return ErrNotFound
	}
	s.PasswordHash = hash
	return nil
}

func (m *MemStore) PutSession(_ context.Context, sess *SessionRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sess.Token] = sess
	return nil
}

func (m *MemStore) GetSession(_ context.Context, token string) (*SessionRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[token]
	if !ok || sess.ExpiresAt.Before(time.Now()) {
		return nil, ErrNotFound
	}
	return sess, nil
}

func (m *MemStore) DeleteSession(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

func (m *MemStore) DeleteSessionsFor(_ context.Context, kind string, refID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tk, sess := range m.sessions {
		if sess.Kind == kind && sess.RefID == refID {
			delete(m.sessions, tk)
		}
	}
	return nil
}

// ---------- 网点 ----------

func (m *MemStore) CreateOutlet(_ context.Context, o *Outlet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o.ID = m.nextID()
	m.outlets[o.ID] = o
	return nil
}

func (m *MemStore) UpdateOutlet(_ context.Context, o *Outlet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.outlets[o.ID]; !ok {
		return ErrNotFound
	}
	m.outlets[o.ID] = o
	return nil
}

func (m *MemStore) GetOutlet(_ context.Context, id int64) (*Outlet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.outlets[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneOutlet(o), nil
}

func (m *MemStore) ListOutlets(_ context.Context, f OutletFilter) ([]*Outlet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Outlet{}
	for _, o := range m.outlets {
		if f.City != "" && o.City != f.City {
			continue
		}
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		out = append(out, cloneOutlet(o))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) CountIdleByOutlet(_ context.Context, outletID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, l := range m.lockers {
		if l.OutletID == outletID && l.Status == "idle" {
			n++
		}
	}
	return n, nil
}

func (m *MemStore) IdleCountByCity(_ context.Context, city string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, l := range m.lockers {
		o, ok := m.outlets[l.OutletID]
		if ok && o.City == city && o.Status == "normal" && l.Status == "idle" {
			n++
		}
	}
	return n, nil
}

// ---------- 柜组/柜格 ----------

func (m *MemStore) CreateGroup(_ context.Context, g *Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g.ID = m.nextID()
	m.groups[g.ID] = g
	return nil
}

func (m *MemStore) ListGroups(_ context.Context, outletID int64) ([]*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Group{}
	for _, g := range m.groups {
		if g.OutletID == outletID {
			out = append(out, &Group{ID: g.ID, OutletID: g.OutletID, Name: g.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) ListLockers(_ context.Context, outletID, groupID int64) ([]*Locker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Locker{}
	for _, l := range m.lockers {
		if outletID > 0 && l.OutletID != outletID {
			continue
		}
		if groupID > 0 && l.GroupID != groupID {
			continue
		}
		out = append(out, cloneLocker(l))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) GetLocker(_ context.Context, id int64) (*Locker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.lockers[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneLocker(l), nil
}

// AllocateLocker 原子分配：互斥锁保护下筛选并占用
func (m *MemStore) AllocateLocker(_ context.Context, outletID int64, size string) (*Locker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := []int64{}
	for _, l := range m.lockers {
		if l.OutletID == outletID && l.Size == size && l.Status == "idle" {
			ids = append(ids, l.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) == 0 {
		return nil, ErrNoFreeLocker
	}
	l := m.lockers[ids[0]]
	l.Status = "occupied"
	return cloneLocker(l), nil
}

func (m *MemStore) UpdateLocker(_ context.Context, l *Locker) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.lockers[l.ID]; !ok {
		return ErrNotFound
	}
	m.lockers[l.ID] = cloneLocker(l)
	return nil
}

// ---------- 规则 ----------

func (m *MemStore) GetRule(_ context.Context, outletID int64) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rules[outletID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneRule(r), nil
}

func (m *MemStore) UpsertRule(_ context.Context, r *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[r.OutletID] = cloneRule(r)
	return nil
}

// ---------- 订单 ----------

func (m *MemStore) CreateOrder(_ context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.orders {
		if e.OrderNo == o.OrderNo {
			return ErrConflict
		}
	}
	o.ID = m.nextID()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	m.orders[o.ID] = o
	return nil
}

func (m *MemStore) GetOrder(_ context.Context, id int64) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneOrder(o), nil
}

func (m *MemStore) GetOrderByNo(_ context.Context, no string) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.orders {
		if o.OrderNo == no {
			return cloneOrder(o), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemStore) GetOrderByCode(_ context.Context, code string, activeOnly bool) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var found *Order
	for _, o := range m.orders {
		if o.Code == code && (!activeOnly || o.Status == "active") {
			if found == nil || o.ID > found.ID {
				found = o
			}
		}
	}
	if found == nil {
		return nil, ErrNotFound
	}
	return cloneOrder(found), nil
}

func (m *MemStore) ListOrders(_ context.Context, f OrderFilter) ([]*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Order{}
	for _, o := range m.orders {
		if f.UserID > 0 && o.UserID != f.UserID {
			continue
		}
		if f.OutletID > 0 && o.OutletID != f.OutletID {
			continue
		}
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		out = append(out, cloneOrder(o))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *MemStore) UpdateOrderSettlement(_ context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.orders[o.ID]; !ok {
		return ErrNotFound
	}
	m.orders[o.ID] = cloneOrder(o)
	return nil
}

func (m *MemStore) CountOrdersBetween(_ context.Context, outletID int64, from, to time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, o := range m.orders {
		if !o.CreatedAt.Before(from) && !o.CreatedAt.After(to) {
			if outletID > 0 && o.OutletID != outletID {
				continue
			}
			n++
		}
	}
	return n, nil
}

// ---------- 优惠券 ----------

func (m *MemStore) CreateCouponDef(_ context.Context, d *CouponDef) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d.ID = m.nextID()
	m.couponDefs[d.ID] = d
	return nil
}

func (m *MemStore) GetCouponDef(_ context.Context, id int64) (*CouponDef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.couponDefs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneCouponDef(d), nil
}

func (m *MemStore) ListCouponDefs(_ context.Context) ([]*CouponDef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*CouponDef{}
	for _, d := range m.couponDefs {
		out = append(out, cloneCouponDef(d))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *MemStore) SetCouponEnabled(_ context.Context, id int64, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.couponDefs[id]
	if !ok {
		return ErrNotFound
	}
	d.Enabled = enabled
	return nil
}

func (m *MemStore) CreateCouponInstance(_ context.Context, ci *CouponInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ci.ID = m.nextID()
	m.couponIns[ci.ID] = ci
	return nil
}

func (m *MemStore) GetCouponInstance(_ context.Context, id int64) (*CouponInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ci, ok := m.couponIns[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneCouponInstance(ci, m.couponDefs), nil
}

func (m *MemStore) ListUserCoupons(_ context.Context, userID int64) ([]*CouponInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*CouponInstance{}
	for _, ci := range m.couponIns {
		if ci.UserID == userID {
			out = append(out, cloneCouponInstance(ci, m.couponDefs))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *MemStore) UseCouponInstance(_ context.Context, id, orderID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ci, ok := m.couponIns[id]
	if !ok || ci.Status != "usable" {
		return ErrConflict
	}
	d, ok := m.couponDefs[ci.DefID]
	if !ok || !d.Enabled {
		return ErrConflict
	}
	now := time.Now()
	if now.Before(d.ValidFrom) || now.After(d.ValidTo) {
		return ErrConflict
	}
	ci.Status = "used"
	ci.UsedOrderID = orderID
	d.UsedCount++
	return nil
}

// ---------- 告警 ----------

func (m *MemStore) CreateAlert(_ context.Context, a *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.ID = m.nextID()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	m.alerts[a.ID] = a
	return nil
}

func (m *MemStore) GetAlert(_ context.Context, id int64) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.alerts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneAlert(a), nil
}

func (m *MemStore) ListAlerts(_ context.Context, outletID int64, status string) ([]*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Alert{}
	for _, a := range m.alerts {
		if outletID > 0 && a.OutletID != outletID {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		out = append(out, cloneAlert(a))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *MemStore) HandleAlert(_ context.Context, id int64, handler string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.alerts[id]
	if !ok || a.Status != "open" {
		return ErrConflict
	}
	a.Status = "handled"
	a.Handler = handler
	a.HandledAt = time.Now()
	return nil
}

// ---------- 工单 ----------

func (m *MemStore) CreateTicket(_ context.Context, t *Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.ID = m.nextID()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	m.tickets[t.ID] = t
	return nil
}

func (m *MemStore) ListTickets(_ context.Context, status string) ([]*Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Ticket{}
	for _, t := range m.tickets {
		if status != "" && t.Status != status {
			continue
		}
		out = append(out, cloneTicket(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *MemStore) CountTodayTickets(_ context.Context, day time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, t := range m.tickets {
		if sameDay(t.CreatedAt, day) {
			n++
		}
	}
	return n, nil
}

func (m *MemStore) UpdateTicket(_ context.Context, t *Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tickets[t.ID]; !ok {
		return ErrNotFound
	}
	m.tickets[t.ID] = cloneTicket(t)
	return nil
}

// ---------- 知识库 ----------

func (m *MemStore) ListKB(_ context.Context) ([]map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]string, 0, len(m.kb))
	for _, e := range m.kb {
		out = append(out, map[string]string{"category": e["category"], "keywords": e["keywords"], "answer": e["answer"]})
	}
	return out, nil
}

// SeedKB 测试辅助：写入知识库条目
func (m *MemStore) SeedKB(category, keywords, answer string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kb = append(m.kb, map[string]string{"category": category, "keywords": keywords, "answer": answer})
}

// ---------- 客服会话 ----------

func (m *MemStore) AppendMessage(_ context.Context, msg *ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.ID = m.nextID()
	msg.CreatedAt = time.Now()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MemStore) ListMessages(_ context.Context, userID int64, afterID int64, limit int) ([]*ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	out := []*ChatMessage{}
	for _, msg := range m.messages {
		if msg.UserID == userID && msg.ID > afterID {
			out = append(out, &ChatMessage{ID: msg.ID, UserID: msg.UserID, Sender: msg.Sender, Content: msg.Content, Channel: msg.Channel, CreatedAt: msg.CreatedAt})
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemStore) GetChatSession(_ context.Context, userID int64) (*ChatSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cs, ok := m.chatSess[userID]; ok {
		return &ChatSession{UserID: cs.UserID, State: cs.State, UpdatedAt: cs.UpdatedAt}, nil
	}
	return &ChatSession{UserID: userID, State: "bot", UpdatedAt: time.Now()}, nil
}

func (m *MemStore) SetChatState(_ context.Context, userID int64, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatSess[userID] = &ChatSession{UserID: userID, State: state, UpdatedAt: time.Now()}
	return nil
}

func (m *MemStore) ListAgentSessions(_ context.Context) ([]*ChatSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*ChatSession{}
	for _, cs := range m.chatSess {
		out = append(out, &ChatSession{UserID: cs.UserID, State: cs.State, UpdatedAt: cs.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// ---------- 支付与营收 ----------

func (m *MemStore) AddPayment(_ context.Context, p *Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments = append(m.payments, p)
	if p.ID == 0 {
		p.ID = m.nextID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	return nil
}

func (m *MemStore) AddRevenue(_ context.Context, r *RevenueRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID = m.nextID()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	m.revenue = append(m.revenue, r)
	return nil
}

func (m *MemStore) ListRevenue(_ context.Context, outletID int64, from, to time.Time) ([]*RevenueRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*RevenueRecord{}
	for _, r := range m.revenue {
		if !r.CreatedAt.Before(from) && !r.CreatedAt.After(to) {
			if outletID > 0 && r.OutletID != outletID {
				continue
			}
			out = append(out, &RevenueRecord{ID: r.ID, OutletID: r.OutletID, OrderID: r.OrderID, GroupID: r.GroupID, Kind: r.Kind, AmountFen: r.AmountFen, CreatedAt: r.CreatedAt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) SumRevenue(_ context.Context, outletID int64, from, to time.Time) (income, refund, extra int, err error) {
	list, err := m.ListRevenue(context.Background(), outletID, from, to)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, r := range list {
		switch r.Kind {
		case "income":
			income += r.AmountFen
		case "refund":
			refund += r.AmountFen
		case "extra":
			extra += r.AmountFen
		}
	}
	return income, refund, extra, nil
}

// ---------- clones ----------

func cloneUser(u *User) *User                { c := *u; return &c }
func cloneStaff(s *Staff) *Staff             { c := *s; return &c }
func cloneOutlet(o *Outlet) *Outlet          { c := *o; return &c }
func cloneLocker(l *Locker) *Locker          { c := *l; return &c }
func cloneRule(r *Rule) *Rule                { c := *r; return &c }
func cloneOrder(o *Order) *Order             { c := *o; return &c }
func cloneCouponDef(d *CouponDef) *CouponDef { c := *d; return &c }
func cloneAlert(a *Alert) *Alert             { c := *a; return &c }
func cloneTicket(t *Ticket) *Ticket          { c := *t; return &c }

func cloneCouponInstance(ci *CouponInstance, defs map[int64]*CouponDef) *CouponInstance {
	c := *ci
	if d, ok := defs[ci.DefID]; ok {
		dd := *d
		c.Def = &dd
	}
	return &c
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// SeedOutlet 测试辅助：创建网点+柜组+柜格
func (m *MemStore) SeedOutlet(name, city string, counts map[string]int) (*Outlet, error) {
	o := &Outlet{Name: name, Address: city, City: city, Status: "normal", Lat: 31.23, Lng: 121.47}
	ctx := context.Background()
	if err := m.CreateOutlet(ctx, o); err != nil {
		return nil, err
	}
	g := &Group{OutletID: o.ID, Name: "A组"}
	if err := m.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seq := 0
	for _, size := range []string{"S", "M", "L"} {
		for i := 0; i < counts[size]; i++ {
			seq++
			l := &Locker{OutletID: o.ID, GroupID: g.ID, Code: fmt.Sprintf("A-%03d", seq), Size: size, Status: "idle"}
			l.ID = m.nextID()
			m.lockers[l.ID] = l
		}
	}
	return o, nil
}
