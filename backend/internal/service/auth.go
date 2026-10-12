// Package service 放业务规则和事务边界，不碰 http，也不写 SQL。
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

// 单管理员，凭据来自 .env（见 DECISIONS D8）。
type AuthService struct {
	cfg *config.Config
}

func NewAuthService(cfg *config.Config) *AuthService {
	return &AuthService{cfg: cfg}
}

// TokenTTL 决定令牌和 Cookie 的有效期，两处必须一致。
const TokenTTL = 7 * 24 * time.Hour

// Login 校验邮箱和密码，通过则签发 JWT。
// 邮箱不存在和密码错给同一个错误，避免被枚举。
// AdminEmail 返回配置里的管理员邮箱，供登录成功后回显。
func (s *AuthService) AdminEmail() string { return s.cfg.AdminEmail }

func (s *AuthService) Login(email, password string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return "", fmt.Errorf("%w: 邮箱和密码都不能为空", model.ErrInvalidInput)
	}
	if !strings.EqualFold(email, s.cfg.AdminEmail) {
		return "", model.ErrWrongCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.AdminPassHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return "", model.ErrWrongCredential
		}
		return "", fmt.Errorf("校验密码哈希失败: %w", err)
	}
	return s.Issue(email)
}

// Issue 签发 HS256 JWT，subject 是管理员邮箱。
func (s *AuthService) Issue(email string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   email,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		Issuer:    "kb-blog",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("签发令牌失败: %w", err)
	}
	return token, nil
}

// Verify 校验令牌并返回管理员邮箱。
func (s *AuthService) Verify(token string) (string, error) {
	if token == "" {
		return "", model.ErrUnauthorized
	}
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		return "", model.ErrUnauthorized
	}
	email, err := claims.GetSubject()
	if err != nil || email == "" {
		return "", model.ErrUnauthorized
	}
	return email, nil
}
