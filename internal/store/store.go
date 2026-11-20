// Package store 定义数据模型与存储接口，提供 MySQL 实现与内存实现（测试/演示）。
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 表示目标记录不存在
var ErrNotFound = errors.New("not found")

// ErrConflict 表示唯一约束/状态冲突
var ErrConflict = errors.New("conflict")

// ErrNoFreeLocker 表示无可用柜格
var ErrNoFreeLocker = errors.New("no free locker")

// DefaultLoginPwdHash 演示环境统一初始登录密码 "123456" 的 SHA-256 值。
// 新注册用户默认写入；历史用户由迁移 007 统一刷入。
const DefaultLoginPwdHash = "8d969eef6ecad3c29a3a629280e686cf0c3f5d5a86aff3ca12020c923adc6c92"

type User struct {
	ID        int64
	Phone     string
	PickupPwd string
	LoginPwd  string // 登录密码哈希（空表示仅注册未设置）
	Avatar    string // 头像（emoji 或短标识）
	CreatedAt time.Time
}

type Staff struct {
	ID            int64
	EmpNo         string
	Account       string
	Name          string
	Phone         string
	Role          string // super / site_admin / service
	PasswordHash  string
	Status        string // active / disabled
	ScopeOutletID int64  // 0 表示不限
	CreatedAt     time.Time
}

type Outlet struct {
	ID             int64
	Name           string
	Address        string
	City           string
	Status         string // normal / paused
	ManagerStaffID int64
	Lat            float64
	Lng            float64
	CreatedAt      time.Time
}

type Group struct {
	ID       int64
	OutletID int64
	Name     string
}

type Locker struct {
	ID             int64
	OutletID       int64
	GroupID        int64
	Code           string
	Size           string // S / M / L
	Status         string // idle / occupied / fault
	FaultSource    string // user_report / system_check / ""
	FaultNote      string
	CurrentOrderID int64
}

type Rule struct {
	OutletID    int64
	Mode        string // hourly / daily
	PriceS      int
	PriceM      int
	PriceL      int
	DepositS    int
	DepositM    int
	DepositL    int
	CapS        int
	CapM        int
	CapL        int
	FreeMinutes int
}

type Order struct {
	ID                int64
	OrderNo           string
	UserID            int64
	OutletID          int64
	LockerID          int64
	Size              string
	PayMode           string // prepaid / deposit
	PrepaidFen        int
	DepositFen        int
	CouponInstanceID  int64
	CouponDiscountFen int
	Code              string
	Status            string // active / finished
	StartedAt         time.Time
	EndedAt           time.Time
	FeeFen            int
	RefundFen         int
	ExtraFen          int
	ExtraPaid         bool
	SettleNote        string
	PickupMethod      string
	CreatedAt         time.Time
}

type CouponDef struct {
	ID         int64
	Name       string
	Ctype      string // discount / hour / amount
	Value      int
	Scope      string // all / outlet
	OutletID   int64
	ValidFrom  time.Time
	ValidTo    time.Time
	Enabled    bool
	TotalCount int
	UsedCount  int
	Note       string
	CreatedAt  time.Time
}

type CouponInstance struct {
	ID          int64
	DefID       int64
	UserID      int64
	Status      string // usable / used / expired
	UsedOrderID int64
	CreatedAt   time.Time
	Def         *CouponDef // 联查填充，可能为 nil
}

type Alert struct {
	ID          int64
	Title       string
	Level       string // high / mid
	Source      string
	OutletID    int64
	LockerID    int64
	Description string
	Status      string // open / handled
	Handler     string
	CreatedAt   time.Time
	HandledAt   time.Time
}

type Ticket struct {
	ID         int64
	TicketNo   string
	UserID     int64
	Question   string
	Category   string
	Status     string // pending / resolved
	Reply      string
	CreatedAt  time.Time
	ResolvedAt time.Time
}

type ChatMessage struct {
	ID        int64
	UserID    int64
	Sender    string // user / bot / agent / system
	Content   string
	Channel   string
	CreatedAt time.Time
}

type ChatSession struct {
	UserID    int64
	State     string // bot / waiting / agent
	UpdatedAt time.Time
}

type SessionRow struct {
	Token     string
	Kind      string // user / staff
	RefID     int64
	ExpiresAt time.Time
}

type Payment struct {
	ID        int64
	OrderNo   string
	Kind      string // prepaid / deposit / extra
	AmountFen int
	Channel   string
	Status    string // paid / refunded
	CreatedAt time.Time
}

type RevenueRecord struct {
	ID        int64
	OutletID  int64
	GroupID   int64
	OrderID   int64
	Kind      string // income / refund / extra
	AmountFen int
	CreatedAt time.Time
}

// OrderFilter 订单列表过滤条件
type OrderFilter struct {
	UserID   int64
	OutletID int64
	Status   string
	Limit    int
}

// OutletFilter 网点过滤
type OutletFilter struct {
	City   string
	Status string
}

// Store 业务存储接口。
// 实现须保证：AllocateLocker 原子分配（同一柜格不被并发双占）。
type Store interface {
	// 用户
	CreateUser(ctx context.Context, phone string) (*User, error)
	GetUserByPhone(ctx context.Context, phone string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	SetPickupPwd(ctx context.Context, userID int64, hash string) error
	SetAvatar(ctx context.Context, userID int64, avatar string) error
	SetLoginPwd(ctx context.Context, userID int64, hash string) error

	// 员工与会话
	CreateStaff(ctx context.Context, s *Staff) error
	GetStaff(ctx context.Context, id int64) (*Staff, error)
	GetStaffByIdentity(ctx context.Context, ident string) (*Staff, error)
	ListStaff(ctx context.Context) ([]*Staff, error)
	UpdateStaffRole(ctx context.Context, id int64, role string) error
	UpdateStaffStatus(ctx context.Context, id int64, status string) error
	UpdateStaffPassword(ctx context.Context, id int64, hash string) error

	PutSession(ctx context.Context, s *SessionRow) error
	GetSession(ctx context.Context, token string) (*SessionRow, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionsFor(ctx context.Context, kind string, refID int64) error

	// 网点/柜组/柜格
	CreateOutlet(ctx context.Context, o *Outlet) error
	UpdateOutlet(ctx context.Context, o *Outlet) error
	GetOutlet(ctx context.Context, id int64) (*Outlet, error)
	ListOutlets(ctx context.Context, f OutletFilter) ([]*Outlet, error)
	CountIdleByOutlet(ctx context.Context, outletID int64) (int, error)
	IdleCountByCity(ctx context.Context, city string) (int, error)

	CreateGroup(ctx context.Context, g *Group) error
	ListGroups(ctx context.Context, outletID int64) ([]*Group, error)

	ListLockers(ctx context.Context, outletID, groupID int64) ([]*Locker, error)
	GetLocker(ctx context.Context, id int64) (*Locker, error)
	AllocateLocker(ctx context.Context, outletID int64, size string) (*Locker, error)
	UpdateLocker(ctx context.Context, l *Locker) error

	// 规则
	GetRule(ctx context.Context, outletID int64) (*Rule, error)
	UpsertRule(ctx context.Context, r *Rule) error

	// 订单
	CreateOrder(ctx context.Context, o *Order) error
	GetOrder(ctx context.Context, id int64) (*Order, error)
	GetOrderByNo(ctx context.Context, no string) (*Order, error)
	GetOrderByCode(ctx context.Context, code string, activeOnly bool) (*Order, error)
	ListOrders(ctx context.Context, f OrderFilter) ([]*Order, error)
	UpdateOrderSettlement(ctx context.Context, o *Order) error
	CountOrdersBetween(ctx context.Context, outletID int64, from, to time.Time) (int, error)

	// 优惠券
	CreateCouponDef(ctx context.Context, d *CouponDef) error
	GetCouponDef(ctx context.Context, id int64) (*CouponDef, error)
	ListCouponDefs(ctx context.Context) ([]*CouponDef, error)
	SetCouponEnabled(ctx context.Context, id int64, enabled bool) error
	CreateCouponInstance(ctx context.Context, ci *CouponInstance) error
	GetCouponInstance(ctx context.Context, id int64) (*CouponInstance, error)
	ListUserCoupons(ctx context.Context, userID int64) ([]*CouponInstance, error)
	UseCouponInstance(ctx context.Context, id, orderID int64) error

	// 告警
	CreateAlert(ctx context.Context, a *Alert) error
	GetAlert(ctx context.Context, id int64) (*Alert, error)
	ListAlerts(ctx context.Context, outletID int64, status string) ([]*Alert, error)
	HandleAlert(ctx context.Context, id int64, handler string) error

	// 工单
	CreateTicket(ctx context.Context, t *Ticket) error
	ListTickets(ctx context.Context, status string) ([]*Ticket, error)
	CountTodayTickets(ctx context.Context, day time.Time) (int, error)
	UpdateTicket(ctx context.Context, t *Ticket) error

	// 知识库
	ListKB(ctx context.Context) ([]map[string]string, error)

	// 客服会话
	AppendMessage(ctx context.Context, m *ChatMessage) error
	ListMessages(ctx context.Context, userID int64, afterID int64, limit int) ([]*ChatMessage, error)
	GetChatSession(ctx context.Context, userID int64) (*ChatSession, error)
	SetChatState(ctx context.Context, userID int64, state string) error
	ListAgentSessions(ctx context.Context) ([]*ChatSession, error)

	// 支付与营收
	AddPayment(ctx context.Context, p *Payment) error
	AddRevenue(ctx context.Context, r *RevenueRecord) error
	ListRevenue(ctx context.Context, outletID int64, from, to time.Time) ([]*RevenueRecord, error)
	SumRevenue(ctx context.Context, outletID int64, from, to time.Time) (income, refund, extra int, err error)

	// 员工侧统计
	Close() error
}
