/*
 * beta_token.go
 * 功能：Beta Token 的签发与解析；有效期计算以当天凌晨 04:00 为界
 * 时间戳：2026-05-29
 */

package pkg

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var betaSecret = []byte("sesame-sauce-beta-secret-key-2026") // 与登录 JWT 使用不同 secret

// BetaClaims 定义 Beta Token 的负载结构
 type BetaClaims struct {
	Type string `json:"type"`
	jwt.RegisteredClaims
}

// nextFourAM 计算给定时间的下一个凌晨 04:00
// 若当前时间已过 04:00，则返回次日 04:00；否则返回当天 04:00
func nextFourAM(t time.Time) time.Time {
	y, m, d := t.Date()
	fourAM := time.Date(y, m, d, 4, 0, 0, 0, t.Location())
	if t.After(fourAM) || t.Equal(fourAM) {
		fourAM = fourAM.Add(24 * time.Hour)
	}
	return fourAM
}

// GenerateBetaToken 签发一个 Beta Token，有效期至当天（以 04:00 为界）的截止时间点
func GenerateBetaToken() (string, time.Time, error) {
	now := time.Now()
	exp := nextFourAM(now)

	claims := BetaClaims{
		Type: "beta",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   "beta_access",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(betaSecret)
	return tokenString, exp, err
}

// ParseBetaToken 解析 Beta Token 并返回 Claims；校验签名、exp、type
func ParseBetaToken(tokenString string) (*BetaClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &BetaClaims{}, func(token *jwt.Token) (interface{}, error) {
		return betaSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*BetaClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid beta token")
	}

	if claims.Type != "beta" {
		return nil, fmt.Errorf("token type mismatch")
	}

	return claims, nil
}
