package biz

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"
)

// GenOrderNo 生成订单号：SL + 时间戳 + 随机 4 位
func GenOrderNo(t time.Time) string {
	r := randSuffix(4)
	return fmt.Sprintf("SL%s%s", t.Format("20060102150405"), r)
}

// GenCode 生成 6 位取件码
func GenCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// GenTicketNo 生成工单号：日期 + 当日序号，如 TK20261119-001
func GenTicketNo(t time.Time, seq int) string {
	return fmt.Sprintf("TK%s-%03d", t.Format("20060102"), seq)
}

func randSuffix(n int) string {
	const digits = "0123456789"
	out := make([]byte, n)
	for i := range out {
		v, _ := rand.Int(rand.Reader, big.NewInt(10))
		out[i] = digits[v.Int64()]
	}
	return string(out)
}

// MatchKB 知识库关键词打分匹配：命中关键词越多得分越高，得分相同取关键词更长（更精确）的条目。
// 未命中返回 nil。不暴露答案来源，调用方只取 Answer。
func MatchKB(entries []KBItem, question string) *KBItem {
	q := normalize(question)
	if q == "" {
		return nil
	}
	best := -1
	bestScore := 0
	bestLen := 0
	for i, e := range entries {
		score := 0
		length := 0
		for _, kw := range strings.Split(e.Keywords, ",") {
			kw = normalize(kw)
			if kw == "" {
				continue
			}
			if strings.Contains(q, kw) {
				score++
				length += len([]rune(kw))
			}
		}
		if score > bestScore || (score == bestScore && score > 0 && length > bestLen) {
			best = i
			bestScore = score
			bestLen = length
		}
	}
	if best < 0 || bestScore == 0 {
		return nil
	}
	return &entries[best]
}

// KBItem 知识库条目
type KBItem struct {
	ID       int64
	Category string
	Keywords string
	Answer   string
}

// normalize 小写化并去空白，中文关键字匹配不受影响
func normalize(s string) string {
	b := strings.Builder{}
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
