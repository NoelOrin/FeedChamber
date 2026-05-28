package pkg

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷
type Claims struct {
	Username string `json:"username"`
	UserUUID string `json:"user_uuid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// signingKey 获取 JWT 签名密钥，优先使用 JWT_SECRET 环境变量。
func signingKey() []byte {
	key := os.Getenv("JWT_SECRET")
	if key == "" {
		key = "orinforum-default-secret"
	}
	return []byte(key)
}

// GenerateToken 生成 access_token（24h 过期）
func GenerateToken(username, userUUID, role string) (string, error) {
	claims := Claims{
		Username: username,
		UserUUID: userUUID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey())
}

// GenerateRefreshToken 生成 refresh_token（7d 过期）
func GenerateRefreshToken(username, userUUID, role string) (string, error) {
	claims := Claims{
		Username: username,
		UserUUID: userUUID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        newJTI(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey())
}

// ParseToken 解析并验证 JWT token
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return signingKey(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func IsTokenExpired(claims *Claims) bool {
	return time.Now().Unix() > claims.ExpiresAt.Unix()
}

// newJTI 生成随机 JWT ID
func newJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}