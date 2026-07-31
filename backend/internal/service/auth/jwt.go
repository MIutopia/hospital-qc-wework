package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 自定义声明
type Claims struct {
	DoctorID int64  `json:"doctor_id"`
	CaseID   int64  `json:"case_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTService JWT 鉴权服务
type JWTService struct {
	secret      string
	expireHours int
}

// NewJWTService 创建 JWT 服务
func NewJWTService(secret string, expireHours int) *JWTService {
	if secret == "" {
		secret = "default-dev-secret-change-in-production"
	}
	if expireHours <= 0 {
		expireHours = 24
	}
	return &JWTService{
		secret:      secret,
		expireHours: expireHours,
	}
}

// GenerateToken 生成 JWT Token
// doctorID: 医生 ID
// caseID: 病例 ID（可以为 0，用于查看所有任务的 token）
func (s *JWTService) GenerateToken(doctorID, caseID int64) (string, error) {
	now := time.Now()
	claims := &Claims{
		DoctorID: doctorID,
		CaseID:   caseID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(s.expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:       fmt.Sprintf("tok_%d_%d", now.UnixNano(), doctorID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// ValidateToken 验证 JWT Token，返回 Claims
func (s *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名方法: %v", token.Header["alg"])
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("Token 无效")
	}

	return claims, nil
}
