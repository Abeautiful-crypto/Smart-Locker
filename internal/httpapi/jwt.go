package httpapi

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT 认证：替代原 sessions 表令牌。claims 携带用户/员工 id、类型与令牌版本(ver)。
// 服务端在退出/改角色/禁用时递增 token_ver，使旧 JWT 立即失效（满足"登录态立即失效"约束）。
var jwtSecret = func() []byte {
	if s := os.Getenv("LOCKER_JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("smart-locker-demo-jwt-secret-2026") // 演示环境默认密钥
}()

type lockerClaims struct {
	Kind string `json:"kind"` // user / staff
	Ver  int    `json:"ver"`  // 令牌版本（与 users/staff.token_ver 比对）
	jwt.RegisteredClaims
}

// signJWT 签发令牌
func signJWT(kind string, refID int64, ver int, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := lockerClaims{
		Kind: kind,
		Ver:  ver,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   itoa(refID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "smart-locker",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

// parseJWT 验证签名/过期并返回 claims
func parseJWT(token string) (*lockerClaims, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	claims := &lockerClaims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
