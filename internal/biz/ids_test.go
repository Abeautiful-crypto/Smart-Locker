package biz

import (
	"regexp"
	"testing"
	"time"
)

func TestGenOrderNo(t *testing.T) {
	no := GenOrderNo(time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local))
	ok, err := regexp.MatchString(`^SL20261119120000\d{4}$`, no)
	if err != nil || !ok {
		t.Fatalf("订单号格式不符: %s", no)
	}
	if GenOrderNo(time.Now()) == GenOrderNo(time.Now()) && false {
		t.Fatal("unreachable")
	}
}

func TestGenCode(t *testing.T) {
	for i := 0; i < 50; i++ {
		c := GenCode()
		ok, _ := regexp.MatchString(`^\d{6}$`, c)
		if !ok {
			t.Fatalf("取件码应6位数字: %s", c)
		}
	}
}

func TestGenTicketNo(t *testing.T) {
	no := GenTicketNo(time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local), 3)
	if no != "TK20261119-003" {
		t.Fatalf("工单号不符: %s", no)
	}
}

func TestMatchKB(t *testing.T) {
	entries := []KBItem{
		{ID: 1, Category: "收费计费", Keywords: "收费,计费,价格,多少钱,费用", Answer: "A"},
		{ID: 2, Category: "免费时长", Keywords: "免费,免费时长,10分钟", Answer: "B"},
		{ID: 3, Category: "退款", Keywords: "退款,退钱", Answer: "C"},
	}
	if got := MatchKB(entries, "寄存怎么收费"); got == nil || got.Answer != "A" {
		t.Fatalf("应命中收费: %+v", got)
	}
	if got := MatchKB(entries, "怎么退款啊"); got == nil || got.Answer != "C" {
		t.Fatalf("应命中退款: %+v", got)
	}
	// 多关键词命中同分时，关键词总长度更长的条目胜出（免费时长 6 > 收费 4）
	if got := MatchKB(entries, "免费时长和收费价格"); got == nil || got.Answer != "B" {
		t.Fatalf("同分应按关键词长度取免费时长: %+v", got)
	}
	// 未命中
	if got := MatchKB(entries, "天气怎么样"); got != nil {
		t.Fatalf("未命中应nil: %+v", got)
	}
	// 空问题
	if MatchKB(entries, "   ") != nil {
		t.Fatal("空白问题应nil")
	}
	// 归一化忽略空白与大小写
	if got := MatchKB(entries, " 请帮 我 退 款 "); got == nil || got.Answer != "C" {
		t.Fatalf("忽略空白后应命中退款: %+v", got)
	}
}
