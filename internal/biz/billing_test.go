package biz

import (
	"testing"
	"time"
)

func stdRule() *Rule {
	return &Rule{Mode: ModeHourly, PriceS: 500, PriceM: 700, PriceL: 1000,
		DepositS: 1000, DepositM: 2000, DepositL: 3000,
		CapS: 5000, CapM: 7000, CapL: 10000, FreeMinutes: 10}
}

func TestSizePriceDepositCap(t *testing.T) {
	r := stdRule()
	cases := map[string][3]int{
		SizeS: {500, 1000, 5000},
		SizeM: {700, 2000, 7000},
		SizeL: {1000, 3000, 10000},
	}
	for size, want := range cases {
		p, err := r.SizePrice(size)
		if err != nil || p != want[0] {
			t.Fatalf("%s price got %d,%v want %d", size, p, err, want[0])
		}
		d, _ := r.SizeDeposit(size)
		c, _ := r.SizeCap(size)
		if d != want[1] || c != want[2] {
			t.Fatalf("%s deposit/cap got %d/%d want %d/%d", size, d, c, want[1], want[2])
		}
	}
	if _, err := r.SizePrice("X"); err == nil {
		t.Fatal("未知规格应报错")
	}
	if _, err := r.SizeDeposit("X"); err == nil {
		t.Fatal("未知规格押金应报错")
	}
	if _, err := r.SizeCap("X"); err == nil {
		t.Fatal("未知规格封顶应报错")
	}
}

func TestCalcFeeFree(t *testing.T) {
	r := stdRule()
	start := time.Date(2026, 11, 19, 10, 0, 0, 0, time.Local)
	for _, m := range []int{0, 5, 10} {
		fee, minutes := CalcFee(r, SizeS, start, start.Add(time.Duration(m)*time.Minute))
		if fee != 0 {
			t.Fatalf("免费时长内(%d分)费用应为0，得 %d", m, fee)
		}
		if minutes != m {
			t.Fatalf("分钟数 got %d want %d", minutes, m)
		}
	}
	// 10分01秒 → 11分钟收费1小时
	fee, _ := CalcFee(r, SizeS, start, start.Add(10*time.Minute+time.Second))
	if fee != 500 {
		t.Fatalf("超免费时长应按1小时计 500，得 %d", fee)
	}
}

func TestCalcFeeHourly(t *testing.T) {
	r := stdRule()
	start := time.Date(2026, 11, 19, 10, 0, 0, 0, time.Local)
	// 61分钟 → 51分钟计费 → 1小时
	fee, _ := CalcFee(r, SizeM, start, start.Add(61*time.Minute))
	if fee != 700 {
		t.Fatalf("中格1小时应700，得 %d", fee)
	}
	// 2小时10分 → 2小时
	fee, _ = CalcFee(r, SizeM, start, start.Add(130*time.Minute))
	if fee != 1400 {
		t.Fatalf("中格2小时应1400，得 %d", fee)
	}
	// 25小时 → 25*500=12500 vs 封顶50*2=10000
	fee, _ = CalcFee(r, SizeS, start, start.Add(25*time.Hour))
	if fee != 10000 {
		t.Fatalf("小格25小时封顶应10000，得 %d", fee)
	}
	// 49小时 → 49*500=24500 vs 封顶50*3=15000
	fee, _ = CalcFee(r, SizeS, start, start.Add(49*time.Hour))
	if fee != 15000 {
		t.Fatalf("小格49小时封顶应15000，得 %d", fee)
	}
}

func TestCalcFeeDaily(t *testing.T) {
	r := stdRule()
	r.Mode = ModeDaily
	start := time.Date(2026, 11, 19, 10, 0, 0, 0, time.Local)
	fee, _ := CalcFee(r, SizeL, start, start.Add(10*time.Hour+time.Minute))
	if fee != 1000 {
		t.Fatalf("按日1天应1000，得 %d", fee)
	}
	fee, _ = CalcFee(r, SizeL, start, start.Add(25*time.Hour))
	if fee != 2000 {
		t.Fatalf("按日2天应2000，得 %d", fee)
	}
}

func TestCalcFeeEndBeforeStart(t *testing.T) {
	r := stdRule()
	start := time.Date(2026, 11, 19, 10, 0, 0, 0, time.Local)
	fee, minutes := CalcFee(r, SizeS, start, start.Add(-time.Hour))
	if fee != 0 || minutes != 0 {
		t.Fatalf("结束早于开始应0，得 %d/%d", fee, minutes)
	}
}

func TestPrepaidAmount(t *testing.T) {
	r := stdRule()
	for size, want := range map[string]int{SizeS: 1500, SizeM: 2700, SizeL: 4000} {
		got, err := PrepaidAmount(r, size)
		if err != nil || got != want {
			t.Fatalf("%s 预付 got %d,%v want %d", size, got, err, want)
		}
	}
	if _, err := PrepaidAmount(r, "X"); err == nil {
		t.Fatal("未知规格应报错")
	}
}

func TestCalcCouponDiscount(t *testing.T) {
	fee := 2000
	// 8折：减免20%
	d := CalcCouponDiscount(&CouponDef{Type: CouponDiscount, Value: 80}, fee, 500)
	if d != 400 {
		t.Fatalf("8折抵扣应400，得 %d", d)
	}
	// 1小时券：抵1小时费用
	d = CalcCouponDiscount(&CouponDef{Type: CouponHour, Value: 60}, fee, 500)
	if d != 500 {
		t.Fatalf("1小时券应抵500，得 %d", d)
	}
	// 1元券
	d = CalcCouponDiscount(&CouponDef{Type: CouponAmount, Value: 100}, fee, 500)
	if d != 100 {
		t.Fatalf("1元券应抵100，得 %d", d)
	}
	// 不超过订单金额
	d = CalcCouponDiscount(&CouponDef{Type: CouponAmount, Value: 9900}, 500, 500)
	if d != 500 {
		t.Fatalf("抵扣应封顶订单金额500，得 %d", d)
	}
	// 无券/零费/非法值
	if CalcCouponDiscount(nil, fee, 500) != 0 {
		t.Fatal("无券应为0")
	}
	if CalcCouponDiscount(&CouponDef{Type: CouponAmount, Value: 100}, 0, 500) != 0 {
		t.Fatal("零费用应为0")
	}
	if CalcCouponDiscount(&CouponDef{Type: CouponDiscount, Value: 100}, fee, 500) != 0 {
		t.Fatal("非法折扣应为0")
	}
	if CalcCouponDiscount(&CouponDef{Type: "unknown", Value: 10}, fee, 500) != 0 {
		t.Fatal("未知类型应为0")
	}
	if CalcCouponDiscount(&CouponDef{Type: CouponHour, Value: 90}, fee, 500) != 500 {
		t.Fatal("非整小时时长券按1小时计")
	}
}

func TestSettleFreeFullRefund(t *testing.T) {
	r := stdRule()
	start := time.Now()
	s := Settle(r, SizeS, PayPrepaid, 1500, nil, start, start.Add(5*time.Minute))
	if s.Kind != "free" || s.FeeFen != 0 || s.RefundFen != 1500 || s.ExtraFen != 0 {
		t.Fatalf("免费时长内结算不符: %+v", s)
	}
}

func TestSettleRefundBalance(t *testing.T) {
	r := stdRule()
	start := time.Now()
	// 1小时10分 → 1小时费用500，预付1500 → 退1000
	s := Settle(r, SizeS, PayPrepaid, 1500, nil, start, start.Add(70*time.Minute))
	if s.Kind != "refund" || s.FeeFen != 500 || s.RefundFen != 1000 {
		t.Fatalf("提前取件退款不符: %+v", s)
	}
}

func TestSettleDue(t *testing.T) {
	r := stdRule()
	start := time.Now()
	// 10小时 → 10*500=5000，预付1500 → 补3500
	s := Settle(r, SizeS, PayPrepaid, 1500, nil, start, start.Add(10*time.Hour+5*time.Minute))
	if s.Kind != "due" || s.ExtraFen != 3500 {
		t.Fatalf("超时应补交3500: %+v", s)
	}
}

func TestSettleEven(t *testing.T) {
	r := stdRule()
	start := time.Now()
	s := Settle(r, SizeS, PayPrepaid, 500, nil, start, start.Add(70*time.Minute))
	if s.Kind != "even" || s.RefundFen != 0 || s.ExtraFen != 0 {
		t.Fatalf("恰好相等应even: %+v", s)
	}
}

func TestSettleWithCoupon(t *testing.T) {
	r := stdRule()
	start := time.Now()
	// 2小时费用1000，8折券抵200 → 800，预付1500 → 退700
	def := &CouponDef{Type: CouponDiscount, Value: 80}
	s := Settle(r, SizeS, PayPrepaid, 1500, def, start, start.Add(130*time.Minute))
	if s.CouponFen != 200 || s.FeeFen != 800 || s.RefundFen != 700 {
		t.Fatalf("8折券结算不符: %+v", s)
	}
	// 押金模式：押金1000，费用超押金 → 补交
	s = Settle(r, SizeS, PayDeposit, 1000, nil, start, start.Add(3*time.Hour))
	if s.Kind != "due" || s.ExtraFen != 500 {
		t.Fatalf("押金模式超时应补500: %+v", s)
	}
	// 押金模式：费用不足押金 → 退余额
	s = Settle(r, SizeS, PayDeposit, 1000, nil, start, start.Add(70*time.Minute))
	if s.Kind != "refund" || s.RefundFen != 500 {
		t.Fatalf("押金模式应退500: %+v", s)
	}
}

func TestNormalize(t *testing.T) {
	r := &Rule{Mode: "bad", FreeMinutes: -5}
	r.Normalize()
	if r.Mode != ModeHourly || r.FreeMinutes != 0 {
		t.Fatalf("Normalize不符: %+v", r)
	}
	r2 := &Rule{Mode: ModeDaily, FreeMinutes: 10}
	r2.Normalize()
	if r2.Mode != ModeDaily || r2.FreeMinutes != 10 {
		t.Fatalf("Normalize不应改动合法值: %+v", r2)
	}
}
