package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mysqllib "github.com/go-sql-driver/mysql"
)

// MySQLStore 基于 MySQL 的存储实现
type MySQLStore struct {
	db *sql.DB
}

// NewMySQL 打开 MySQL 连接并执行迁移与种子数据
// openDB 打开数据库连接并探活（驱动 loc 与会话时区统一固定 +08:00，
// 避免服务器/运行机器为 UTC 时 NOW() 与驱动写入的墙钟错位 8 小时）
func openDB(dsn string) (*sql.DB, error) {
	dsn = strings.Replace(dsn, "loc=Local", "loc=Asia%2FShanghai", 1)
	if !strings.Contains(dsn, "time_zone=") {
		if strings.Contains(dsn, "?") {
			dsn += "&time_zone=%27%2B08%3A00%27"
		} else {
			dsn += "?time_zone=%27%2B08%3A00%27"
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func NewMySQL(dsn string) (*MySQLStore, error) {
	db, err := openDB(dsn)
	if err != nil {
		return nil, err
	}
	return &MySQLStore{db: db}, nil
}

// Migrate 执行 migrations 目录下按文件名排序的 SQL（按语句拆分，幂等）。
func (s *MySQLStore) Migrate(ctx context.Context, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(content)) {
			if _, err := s.db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("执行 %s: %w", f, err)
			}
		}
	}
	return nil
}

// splitSQL 按分号拆分语句，忽略以 -- 开头的注释行
func splitSQL(content string) []string {
	out := []string{}
	var cur strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(cur.String())
			stmt = strings.TrimSuffix(stmt, ";")
			if strings.TrimSpace(stmt) != "" {
				out = append(out, stmt)
			}
			cur.Reset()
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

func (s *MySQLStore) Close() error { return s.db.Close() }

// ---------- 用户 ----------

func (s *MySQLStore) CreateUser(ctx context.Context, phone string) (*User, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO users(phone) VALUES(?)", phone)
	if err != nil {
		if isDup(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Phone: phone, CreatedAt: time.Now()}, nil
}

func scanUser(row *sql.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Phone, &u.PickupPwd, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *MySQLStore) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT id,phone,pickup_pwd,created_at FROM users WHERE phone=?", phone))
}

func (s *MySQLStore) SetPickupPwd(ctx context.Context, userID int64, hash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET pickup_pwd=? WHERE id=?", hash, userID)
	return err
}

func (s *MySQLStore) GetUserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, "SELECT id,phone,pickup_pwd,created_at FROM users WHERE id=?", id))
}

// CreateLocker 新建柜格
func (s *MySQLStore) CreateLocker(ctx context.Context, l *Locker) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO lockers(outlet_id,group_id,code,size,status) VALUES(?,?,?,?,?)",
		l.OutletID, l.GroupID, l.Code, l.Size, l.Status)
	if err != nil {
		if isDup(err) {
			return ErrConflict
		}
		return err
	}
	l.ID, _ = res.LastInsertId()
	return nil
}

// isDup 判断是否唯一键冲突
func isDup(err error) bool {
	var me *mysqllib.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return false
}

// SeedKB 知识库种子（幂等）
func (s *MySQLStore) SeedKB(category, keywords, answer string) {
	_, _ = s.db.ExecContext(context.Background(),
		"INSERT INTO kb_entries(category,keywords,answer) SELECT ?,?,? FROM DUAL WHERE NOT EXISTS(SELECT 1 FROM kb_entries WHERE category=?)",
		category, keywords, answer, category)
}

// ---------- 员工 ----------

const staffCols = "id,emp_no,account,name,phone,role,password_hash,status,IFNULL(scope_outlet_id,0),created_at"

func scanStaff(rows *sql.Rows) ([]*Staff, error) {
	out := []*Staff{}
	for rows.Next() {
		st := &Staff{}
		if err := rows.Scan(&st.ID, &st.EmpNo, &st.Account, &st.Name, &st.Phone, &st.Role, &st.PasswordHash, &st.Status, &st.ScopeOutletID, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateStaff(ctx context.Context, st *Staff) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO staff(emp_no,account,name,phone,role,password_hash,status,scope_outlet_id) VALUES(?,?,?,?,?,?,?,?)",
		st.EmpNo, st.Account, st.Name, st.Phone, st.Role, st.PasswordHash, st.Status, nullableInt(st.ScopeOutletID))
	if err != nil {
		if isDup(err) {
			return ErrConflict
		}
		return err
	}
	st.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) GetStaff(ctx context.Context, id int64) (*Staff, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+staffCols+" FROM staff WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanStaff(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) GetStaffByIdentity(ctx context.Context, ident string) (*Staff, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+staffCols+" FROM staff WHERE account=? OR phone=? LIMIT 1", ident, ident)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanStaff(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListStaff(ctx context.Context) ([]*Staff, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+staffCols+" FROM staff ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStaff(rows)
}

func (s *MySQLStore) UpdateStaffRole(ctx context.Context, id int64, role string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE staff SET role=? WHERE id=?", role, id)
	return err
}

func (s *MySQLStore) UpdateStaffStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE staff SET status=? WHERE id=?", status, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) UpdateStaffPassword(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE staff SET password_hash=? WHERE id=?", hash, id)
	return err
}

// ---------- 会话 ----------

func (s *MySQLStore) PutSession(ctx context.Context, sess *SessionRow) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions(token,kind,ref_id,expires_at) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE ref_id=VALUES(ref_id), expires_at=VALUES(expires_at), kind=VALUES(kind)",
		sess.Token, sess.Kind, sess.RefID, sess.ExpiresAt)
	return err
}

func (s *MySQLStore) GetSession(ctx context.Context, token string) (*SessionRow, error) {
	sess := &SessionRow{}
	err := s.db.QueryRowContext(ctx, "SELECT token,kind,ref_id,expires_at FROM sessions WHERE token=? AND expires_at>NOW()", token).
		Scan(&sess.Token, &sess.Kind, &sess.RefID, &sess.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return sess, err
}

func (s *MySQLStore) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token=?", token)
	return err
}

func (s *MySQLStore) DeleteSessionsFor(ctx context.Context, kind string, refID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE kind=? AND ref_id=?", kind, refID)
	return err
}

// ---------- 网点 ----------

const outletCols = "id,name,address,city,status,IFNULL(manager_staff_id,0),lat,lng,created_at"

func scanOutlets(rows *sql.Rows) ([]*Outlet, error) {
	out := []*Outlet{}
	for rows.Next() {
		o := &Outlet{}
		if err := rows.Scan(&o.ID, &o.Name, &o.Address, &o.City, &o.Status, &o.ManagerStaffID, &o.Lat, &o.Lng, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateOutlet(ctx context.Context, o *Outlet) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO outlets(name,address,city,status,manager_staff_id,lat,lng) VALUES(?,?,?,?,?,?,?)",
		o.Name, o.Address, o.City, o.Status, nullableInt(o.ManagerStaffID), o.Lat, o.Lng)
	if err != nil {
		return err
	}
	o.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) UpdateOutlet(ctx context.Context, o *Outlet) error {
	res, err := s.db.ExecContext(ctx, "UPDATE outlets SET name=?,address=?,city=?,status=?,manager_staff_id=?,lat=?,lng=? WHERE id=?",
		o.Name, o.Address, o.City, o.Status, nullableInt(o.ManagerStaffID), o.Lat, o.Lng, o.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) GetOutlet(ctx context.Context, id int64) (*Outlet, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+outletCols+" FROM outlets WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanOutlets(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListOutlets(ctx context.Context, f OutletFilter) ([]*Outlet, error) {
	q := "SELECT " + outletCols + " FROM outlets WHERE 1=1"
	var args []any
	if f.City != "" {
		q += " AND city=?"
		args = append(args, f.City)
	}
	if f.Status != "" {
		q += " AND status=?"
		args = append(args, f.Status)
	}
	q += " ORDER BY id"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOutlets(rows)
}

func (s *MySQLStore) CountIdleByOutlet(ctx context.Context, outletID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM lockers WHERE outlet_id=? AND status='idle'", outletID).Scan(&n)
	return n, err
}

func (s *MySQLStore) IdleCountByCity(ctx context.Context, city string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM lockers l JOIN outlets o ON o.id=l.outlet_id WHERE o.city=? AND o.status='normal' AND l.status='idle'", city).Scan(&n)
	return n, err
}

// ---------- 柜组/柜格 ----------

func (s *MySQLStore) CreateGroup(ctx context.Context, g *Group) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO cabinet_groups(outlet_id,name) VALUES(?,?)", g.OutletID, g.Name)
	if err != nil {
		return err
	}
	g.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) ListGroups(ctx context.Context, outletID int64) ([]*Group, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,outlet_id,name FROM cabinet_groups WHERE outlet_id=? ORDER BY id", outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Group{}
	for rows.Next() {
		g := &Group{}
		if err := rows.Scan(&g.ID, &g.OutletID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

const lockerCols = "id,outlet_id,group_id,code,size,status,IFNULL(fault_source,''),fault_note,IFNULL(current_order_id,0)"

func scanLockers(rows *sql.Rows) ([]*Locker, error) {
	out := []*Locker{}
	for rows.Next() {
		l := &Locker{}
		if err := rows.Scan(&l.ID, &l.OutletID, &l.GroupID, &l.Code, &l.Size, &l.Status, &l.FaultSource, &l.FaultNote, &l.CurrentOrderID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *MySQLStore) ListLockers(ctx context.Context, outletID, groupID int64) ([]*Locker, error) {
	q := "SELECT " + lockerCols + " FROM lockers WHERE 1=1"
	var args []any
	if outletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, outletID)
	}
	if groupID > 0 {
		q += " AND group_id=?"
		args = append(args, groupID)
	}
	q += " ORDER BY group_id,code"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLockers(rows)
}

func (s *MySQLStore) GetLocker(ctx context.Context, id int64) (*Locker, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+lockerCols+" FROM lockers WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanLockers(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

// AllocateLocker 原子分配一个空闲柜格：行锁+状态条件更新，防并发双占。
func (s *MySQLStore) AllocateLocker(ctx context.Context, outletID int64, size string) (*Locker, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM lockers WHERE outlet_id=? AND size=? AND status='idle' ORDER BY id LIMIT 1 FOR UPDATE",
		outletID, size).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, ErrNoFreeLocker
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE lockers SET status='occupied' WHERE id=?", id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetLocker(ctx, id)
}

func (s *MySQLStore) UpdateLocker(ctx context.Context, l *Locker) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE lockers SET status=?,fault_source=?,fault_note=?,current_order_id=? WHERE id=?",
		l.Status, nullableStr(l.FaultSource), l.FaultNote, nullableInt0(l.CurrentOrderID), l.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- 规则 ----------

const ruleCols = "outlet_id,mode,price_s,price_m,price_l,deposit_s,deposit_m,deposit_l,cap_s,cap_m,cap_l,free_minutes"

func scanRule(row *sql.Row) (*Rule, error) {
	r := &Rule{}
	err := row.Scan(&r.OutletID, &r.Mode, &r.PriceS, &r.PriceM, &r.PriceL, &r.DepositS, &r.DepositM, &r.DepositL, &r.CapS, &r.CapM, &r.CapL, &r.FreeMinutes)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return r, err
}

func (s *MySQLStore) GetRule(ctx context.Context, outletID int64) (*Rule, error) {
	return scanRule(s.db.QueryRowContext(ctx, "SELECT "+ruleCols+" FROM outlet_rules WHERE outlet_id=?", outletID))
}

func (s *MySQLStore) UpsertRule(ctx context.Context, r *Rule) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO outlet_rules("+ruleCols+") VALUES(?,?,?,?,?,?,?,?,?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE mode=VALUES(mode),price_s=VALUES(price_s),price_m=VALUES(price_m),price_l=VALUES(price_l),"+
			"deposit_s=VALUES(deposit_s),deposit_m=VALUES(deposit_m),deposit_l=VALUES(deposit_l),"+
			"cap_s=VALUES(cap_s),cap_m=VALUES(cap_m),cap_l=VALUES(cap_l),free_minutes=VALUES(free_minutes)",
		r.OutletID, r.Mode, r.PriceS, r.PriceM, r.PriceL, r.DepositS, r.DepositM, r.DepositL, r.CapS, r.CapM, r.CapL, r.FreeMinutes)
	return err
}

// ---------- 订单 ----------

const orderCols = "id,order_no,user_id,outlet_id,locker_id,size,pay_mode,prepaid_fen,deposit_fen,IFNULL(coupon_instance_id,0),coupon_discount_fen,code,status,started_at,COALESCE(ended_at,CAST('1000-01-01' AS DATETIME)),fee_fen,refund_fen,extra_fen,extra_paid,settle_note,pickup_method,created_at"

func scanOrders(rows *sql.Rows) ([]*Order, error) {
	out := []*Order{}
	for rows.Next() {
		o := &Order{}
		if err := rows.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.OutletID, &o.LockerID, &o.Size, &o.PayMode, &o.PrepaidFen, &o.DepositFen,
			&o.CouponInstanceID, &o.CouponDiscountFen, &o.Code, &o.Status, &o.StartedAt, &o.EndedAt, &o.FeeFen, &o.RefundFen, &o.ExtraFen,
			&o.ExtraPaid, &o.SettleNote, &o.PickupMethod, &o.CreatedAt); err != nil {
			return nil, err
		}
		if o.EndedAt.Year() < 2000 {
			o.EndedAt = time.Time{}
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateOrder(ctx context.Context, o *Order) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO orders(order_no,user_id,outlet_id,locker_id,size,pay_mode,prepaid_fen,deposit_fen,coupon_instance_id,coupon_discount_fen,code,status,started_at) "+
			"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)",
		o.OrderNo, o.UserID, o.OutletID, o.LockerID, o.Size, o.PayMode, o.PrepaidFen, o.DepositFen, nullableInt0(o.CouponInstanceID),
		o.CouponDiscountFen, o.Code, o.Status, o.StartedAt)
	if err != nil {
		if isDup(err) {
			return ErrConflict
		}
		return err
	}
	o.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) GetOrder(ctx context.Context, id int64) (*Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+orderCols+" FROM orders WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanOrders(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) GetOrderByNo(ctx context.Context, no string) (*Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+orderCols+" FROM orders WHERE order_no=?", no)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanOrders(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) GetOrderByCode(ctx context.Context, code string, activeOnly bool) (*Order, error) {
	q := "SELECT " + orderCols + " FROM orders WHERE code=?"
	if activeOnly {
		q += " AND status='active'"
	}
	q += " ORDER BY id DESC LIMIT 1"
	rows, err := s.db.QueryContext(ctx, q, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanOrders(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListOrders(ctx context.Context, f OrderFilter) ([]*Order, error) {
	q := "SELECT " + orderCols + " FROM orders WHERE 1=1"
	var args []any
	if f.UserID > 0 {
		q += " AND user_id=?"
		args = append(args, f.UserID)
	}
	if f.OutletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, f.OutletID)
	}
	if f.Status != "" {
		q += " AND status=?"
		args = append(args, f.Status)
	}
	q += " ORDER BY id DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (s *MySQLStore) UpdateOrderSettlement(ctx context.Context, o *Order) error {
	var ended any
	if !o.EndedAt.IsZero() {
		ended = o.EndedAt
	}
	res, err := s.db.ExecContext(ctx,
		"UPDATE orders SET status=?,ended_at=?,fee_fen=?,refund_fen=?,extra_fen=?,extra_paid=?,settle_note=?,coupon_discount_fen=?,pickup_method=? WHERE id=?",
		o.Status, ended, o.FeeFen, o.RefundFen, o.ExtraFen, o.ExtraPaid, o.SettleNote, o.CouponDiscountFen, o.PickupMethod, o.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) CountOrdersBetween(ctx context.Context, outletID int64, from, to time.Time) (int, error) {
	q := "SELECT COUNT(*) FROM orders WHERE created_at>=? AND created_at<=?"
	var args []any = []any{from, to}
	if outletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, outletID)
	}
	var n int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// ---------- 优惠券 ----------

const couponDefCols = "id,name,ctype,value,scope,IFNULL(outlet_id,0),valid_from,valid_to,enabled,total_count,used_count,note,created_at"

func scanCouponDefs(rows *sql.Rows) ([]*CouponDef, error) {
	out := []*CouponDef{}
	for rows.Next() {
		d := &CouponDef{}
		if err := rows.Scan(&d.ID, &d.Name, &d.Ctype, &d.Value, &d.Scope, &d.OutletID, &d.ValidFrom, &d.ValidTo, &d.Enabled, &d.TotalCount, &d.UsedCount, &d.Note, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateCouponDef(ctx context.Context, d *CouponDef) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO coupon_defs(name,ctype,value,scope,outlet_id,valid_from,valid_to,enabled,total_count,note) VALUES(?,?,?,?,?,?,?,?,?,?)",
		d.Name, d.Ctype, d.Value, d.Scope, nullableInt0(d.OutletID), d.ValidFrom.Format("2006-01-02"), d.ValidTo.Format("2006-01-02"), d.Enabled, d.TotalCount, d.Note)
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) GetCouponDef(ctx context.Context, id int64) (*CouponDef, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+couponDefCols+" FROM coupon_defs WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanCouponDefs(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListCouponDefs(ctx context.Context) ([]*CouponDef, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+couponDefCols+" FROM coupon_defs ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouponDefs(rows)
}

func (s *MySQLStore) SetCouponEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := s.db.ExecContext(ctx, "UPDATE coupon_defs SET enabled=? WHERE id=?", enabled, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) CreateCouponInstance(ctx context.Context, ci *CouponInstance) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO coupon_instances(def_id,user_id,status) VALUES(?,?,?)", ci.DefID, ci.UserID, ci.Status)
	if err != nil {
		return err
	}
	ci.ID, _ = res.LastInsertId()
	return nil
}

const couponInstCols = "ci.id,ci.def_id,ci.user_id,ci.status,IFNULL(ci.used_order_id,0),ci.created_at,d.id,d.name,d.ctype,d.value,d.scope,IFNULL(d.outlet_id,0),d.valid_from,d.valid_to,d.enabled,d.total_count,d.used_count,d.note,d.created_at"

func scanCouponInstances(rows *sql.Rows) ([]*CouponInstance, error) {
	out := []*CouponInstance{}
	for rows.Next() {
		ci := &CouponInstance{Def: &CouponDef{}}
		if err := rows.Scan(&ci.ID, &ci.DefID, &ci.UserID, &ci.Status, &ci.UsedOrderID, &ci.CreatedAt,
			&ci.Def.ID, &ci.Def.Name, &ci.Def.Ctype, &ci.Def.Value, &ci.Def.Scope, &ci.Def.OutletID, &ci.Def.ValidFrom, &ci.Def.ValidTo,
			&ci.Def.Enabled, &ci.Def.TotalCount, &ci.Def.UsedCount, &ci.Def.Note, &ci.Def.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ci)
	}
	return out, rows.Err()
}

func (s *MySQLStore) GetCouponInstance(ctx context.Context, id int64) (*CouponInstance, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+couponInstCols+" FROM coupon_instances ci JOIN coupon_defs d ON d.id=ci.def_id WHERE ci.id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanCouponInstances(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListUserCoupons(ctx context.Context, userID int64) ([]*CouponInstance, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+couponInstCols+" FROM coupon_instances ci JOIN coupon_defs d ON d.id=ci.def_id WHERE ci.user_id=? ORDER BY ci.id DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouponInstances(rows)
}

func (s *MySQLStore) UseCouponInstance(ctx context.Context, id, orderID int64) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE coupon_instances ci JOIN coupon_defs d ON d.id=ci.def_id SET ci.status='used', ci.used_order_id=?, d.used_count=d.used_count+1 "+
			"WHERE ci.id=? AND ci.status='usable' AND d.enabled=1 AND CURDATE() BETWEEN d.valid_from AND d.valid_to", orderID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// ---------- 告警 ----------

const alertCols = "id,title,level,source,outlet_id,IFNULL(locker_id,0),description,status,handler,created_at,COALESCE(handled_at,CAST('1000-01-01' AS DATETIME))"

func scanAlerts(rows *sql.Rows) ([]*Alert, error) {
	out := []*Alert{}
	for rows.Next() {
		a := &Alert{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Level, &a.Source, &a.OutletID, &a.LockerID, &a.Description, &a.Status, &a.Handler, &a.CreatedAt, &a.HandledAt); err != nil {
			return nil, err
		}
		if a.HandledAt.Year() < 2000 {
			a.HandledAt = time.Time{}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateAlert(ctx context.Context, a *Alert) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO alerts(title,level,source,outlet_id,locker_id,description,status) VALUES(?,?,?,?,?,?,?)",
		a.Title, a.Level, a.Source, a.OutletID, nullableInt0(a.LockerID), a.Description, a.Status)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) GetAlert(ctx context.Context, id int64) (*Alert, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+alertCols+" FROM alerts WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanAlerts(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *MySQLStore) ListAlerts(ctx context.Context, outletID int64, status string) ([]*Alert, error) {
	q := "SELECT " + alertCols + " FROM alerts WHERE 1=1"
	var args []any
	if outletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, outletID)
	}
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	q += " ORDER BY id DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows)
}

func (s *MySQLStore) HandleAlert(ctx context.Context, id int64, handler string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE alerts SET status='handled',handler=?,handled_at=NOW() WHERE id=? AND status='open'", handler, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// ---------- 工单 ----------

const ticketCols = "id,ticket_no,user_id,question,category,status,reply,created_at,COALESCE(resolved_at,CAST('1000-01-01' AS DATETIME))"

func scanTickets(rows *sql.Rows) ([]*Ticket, error) {
	out := []*Ticket{}
	for rows.Next() {
		t := &Ticket{}
		if err := rows.Scan(&t.ID, &t.TicketNo, &t.UserID, &t.Question, &t.Category, &t.Status, &t.Reply, &t.CreatedAt, &t.ResolvedAt); err != nil {
			return nil, err
		}
		if t.ResolvedAt.Year() < 2000 {
			t.ResolvedAt = time.Time{}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateTicket(ctx context.Context, t *Ticket) error {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tickets(ticket_no,user_id,question,category,status) VALUES(?,?,?,?,?)",
		t.TicketNo, t.UserID, t.Question, t.Category, t.Status)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) ListTickets(ctx context.Context, status string) ([]*Ticket, error) {
	q := "SELECT " + ticketCols + " FROM tickets WHERE 1=1"
	var args []any
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	q += " ORDER BY id DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTickets(rows)
}

func (s *MySQLStore) CountTodayTickets(ctx context.Context, day time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tickets WHERE DATE(created_at)=DATE(?)", day).Scan(&n)
	return n, err
}

func (s *MySQLStore) UpdateTicket(ctx context.Context, t *Ticket) error {
	var resolved any
	if !t.ResolvedAt.IsZero() {
		resolved = t.ResolvedAt
	}
	res, err := s.db.ExecContext(ctx, "UPDATE tickets SET status=?,reply=?,resolved_at=? WHERE id=?", t.Status, t.Reply, resolved, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- 知识库 ----------

func (s *MySQLStore) ListKB(ctx context.Context) ([]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT category,keywords,answer FROM kb_entries ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var cat, kw, ans string
		if err := rows.Scan(&cat, &kw, &ans); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"category": cat, "keywords": kw, "answer": ans})
	}
	return out, rows.Err()
}

// ---------- 客服会话 ----------

func (s *MySQLStore) AppendMessage(ctx context.Context, m *ChatMessage) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO chat_messages(user_id,sender,content,channel) VALUES(?,?,?,?)",
		m.UserID, m.Sender, m.Content, m.Channel)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	m.CreatedAt = time.Now()
	return nil
}

const msgCols = "id,user_id,sender,content,channel,created_at"

func (s *MySQLStore) ListMessages(ctx context.Context, userID int64, afterID int64, limit int) ([]*ChatMessage, error) {
	q := "SELECT " + msgCols + " FROM chat_messages WHERE user_id=? AND id>?"
	if limit <= 0 {
		limit = 100
	}
	q += fmt.Sprintf(" ORDER BY id ASC LIMIT %d", limit)
	rows, err := s.db.QueryContext(ctx, q, userID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ChatMessage{}
	for rows.Next() {
		m := &ChatMessage{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.Sender, &m.Content, &m.Channel, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MySQLStore) GetChatSession(ctx context.Context, userID int64) (*ChatSession, error) {
	cs := &ChatSession{}
	err := s.db.QueryRowContext(ctx, "SELECT user_id,state,updated_at FROM chat_sessions WHERE user_id=?", userID).
		Scan(&cs.UserID, &cs.State, &cs.UpdatedAt)
	if err == sql.ErrNoRows {
		return &ChatSession{UserID: userID, State: "bot", UpdatedAt: time.Now()}, nil
	}
	return cs, err
}

func (s *MySQLStore) SetChatState(ctx context.Context, userID int64, state string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO chat_sessions(user_id,state) VALUES(?,?) ON DUPLICATE KEY UPDATE state=VALUES(state)", userID, state)
	return err
}

func (s *MySQLStore) ListAgentSessions(ctx context.Context) ([]*ChatSession, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT user_id,state,updated_at FROM chat_sessions ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ChatSession{}
	for rows.Next() {
		cs := &ChatSession{}
		if err := rows.Scan(&cs.UserID, &cs.State, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// ---------- 支付与营收 ----------

func (s *MySQLStore) AddPayment(ctx context.Context, p *Payment) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO payments(order_no,kind,amount_fen,channel,status) VALUES(?,?,?,?,?)",
		p.OrderNo, p.Kind, p.AmountFen, p.Channel, p.Status)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

func (s *MySQLStore) AddRevenue(ctx context.Context, r *RevenueRecord) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO revenue_records(outlet_id,order_id,group_id,kind,amount_fen) VALUES(?,?,?,?,?)",
		r.OutletID, r.OrderID, r.GroupID, r.Kind, r.AmountFen)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

const revenueCols = "id,outlet_id,order_id,group_id,kind,amount_fen,created_at"

func scanRevenue(rows *sql.Rows) ([]*RevenueRecord, error) {
	out := []*RevenueRecord{}
	for rows.Next() {
		r := &RevenueRecord{}
		if err := rows.Scan(&r.ID, &r.OutletID, &r.OrderID, &r.GroupID, &r.Kind, &r.AmountFen, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *MySQLStore) ListRevenue(ctx context.Context, outletID int64, from, to time.Time) ([]*RevenueRecord, error) {
	q := "SELECT " + revenueCols + " FROM revenue_records WHERE created_at>=? AND created_at<=?"
	var args []any = []any{from, to}
	if outletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, outletID)
	}
	q += " ORDER BY id"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRevenue(rows)
}

func (s *MySQLStore) SumRevenue(ctx context.Context, outletID int64, from, to time.Time) (income, refund, extra int, err error) {
	q := "SELECT kind, IFNULL(SUM(amount_fen),0) FROM revenue_records WHERE created_at>=? AND created_at<=?"
	var args []any = []any{from, to}
	if outletID > 0 {
		q += " AND outlet_id=?"
		args = append(args, outletID)
	}
	q += " GROUP BY kind"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var amt int
		if err := rows.Scan(&kind, &amt); err != nil {
			return 0, 0, 0, err
		}
		switch kind {
		case "income":
			income = amt
		case "refund":
			refund = amt
		case "extra":
			extra = amt
		}
	}
	return income, refund, extra, rows.Err()
}

// ---------- helpers ----------

func nullableInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullableInt0(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullableStr(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
