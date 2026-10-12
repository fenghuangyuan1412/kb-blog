package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/fenghuangyuan1412/kb-blog/backend/internal/config"
	"github.com/fenghuangyuan1412/kb-blog/backend/internal/model"
)

// 认证测试不碰数据库，凭据直接在测试里造，确保 CI 上也能跑。
const testSecret = "0123456789012345678901234567890123456789"

func newAuthSvc(t *testing.T, password string) *AuthService {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("生成测试哈希失败: %v", err)
	}
	return NewAuthService(&config.Config{
		AdminEmail:    "admin@kb.local",
		AdminPassHash: string(hash),
		JWTSecret:     testSecret,
	})
}

func TestLogin_SuccessAndRoundTrip(t *testing.T) {
	svc := newAuthSvc(t, "correct horse")

	token, err := svc.Login("ADMIN@kb.local", "correct horse")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("不像 JWT: %q", token)
	}
	email, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("Verify 失败: %v", err)
	}
	if email != "ADMIN@kb.local" {
		t.Fatalf("subject 回传不对: %q", email)
	}
	if svc.AdminEmail() != "admin@kb.local" {
		t.Fatalf("AdminEmail 应返回配置值: %q", svc.AdminEmail())
	}
}

func TestLogin_Rejects(t *testing.T) {
	svc := newAuthSvc(t, "correct horse")

	cases := []struct {
		name        string
		email, pass string
		want        error
	}{
		{"密码错", "admin@kb.local", "wrong", model.ErrWrongCredential},
		{"邮箱不存在", "hacker@kb.local", "correct horse", model.ErrWrongCredential},
		{"邮箱为空", "  ", "correct horse", model.ErrInvalidInput},
		{"密码为空", "admin@kb.local", "", model.ErrInvalidInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Login(c.email, c.pass)
			if !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

// 两种失败原因必须一致，否则可以用响应区分出管理员邮箱。
func TestLogin_ErrorsAreIndistinguishable(t *testing.T) {
	svc := newAuthSvc(t, "correct horse")

	_, e1 := svc.Login("admin@kb.local", "bad-password")
	_, e2 := svc.Login("nobody@kb.local", "bad-password")
	if e1 != e2 {
		t.Fatalf("错误信息可被用来枚举邮箱: %q vs %q", e1, e2)
	}
}

func TestVerify_Rejects(t *testing.T) {
	svc := newAuthSvc(t, "correct horse")

	valid, err := svc.Issue("admin@kb.local")
	if err != nil {
		t.Fatalf("Issue 失败: %v", err)
	}

	tampered := valid[:len(valid)-3] + "abc"

	cases := []struct {
		name  string
		token string
	}{
		{"空令牌", ""},
		{"乱码", "not.a.token"},
		{"签名被改动", tampered},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Verify(c.token); !errors.Is(err, model.ErrUnauthorized) {
				t.Fatalf("want ErrUnauthorized, got %v", err)
			}
		})
	}

	t.Run("别的密钥签的令牌", func(t *testing.T) {
		other := NewAuthService(&config.Config{AdminEmail: "a@b.c", JWTSecret: "9999999999999999999999999999999999999999"})
		forged, err := other.Issue("admin@kb.local")
		if err != nil {
			t.Fatalf("伪造令牌失败: %v", err)
		}
		if _, err := svc.Verify(forged); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatalf("换密钥签的令牌不该通过: %v", err)
		}
	})

	t.Run("过期令牌", func(t *testing.T) {
		expired := signWith(t, testSecret, jwt.RegisteredClaims{
			Subject:   "admin@kb.local",
			Issuer:    "kb-blog",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		})
		if _, err := svc.Verify(expired); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatalf("过期令牌不该通过: %v", err)
		}
	})

	t.Run("alg=none 令牌", func(t *testing.T) {
		none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
			Subject:   "admin@kb.local",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL)),
		})
		s, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("构造 none 令牌失败: %v", err)
		}
		if _, err := svc.Verify(s); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatalf("none 算法不该通过: %v", err)
		}
	})
}

func signWith(t *testing.T, secret string, claims jwt.RegisteredClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return token
}

// 令牌有效期是 7 天，Issue 必须带上这个过期时间。
func TestIssue_TTL(t *testing.T) {
	svc := newAuthSvc(t, "x")
	token, err := svc.Issue("admin@kb.local")
	if err != nil {
		t.Fatalf("Issue 失败: %v", err)
	}
	claims := &jwt.RegisteredClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(testSecret), nil
	}); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("没有过期时间: %v", err)
	}
	issued, err := claims.GetIssuedAt()
	if err != nil {
		t.Fatalf("没有签发时间: %v", err)
	}
	if got := exp.Time.Sub(issued.Time); got != TokenTTL {
		t.Fatalf("TTL 应为 %v, 实际 %v", TokenTTL, got)
	}
	if claims.Issuer != "kb-blog" {
		t.Fatalf("issuer 不对: %q", claims.Issuer)
	}
}
