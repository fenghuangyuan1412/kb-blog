// Package config 集中读取运行配置，全部来自 .env 或环境变量。
// 目标：让"缺配置"在启动时就炸掉并说清缺哪个，而不是运行到一半 panic。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config 是服务运行所需的全部配置。
type Config struct {
	Addr           string   // 监听地址，如 :8080
	BaseURL        string   // 对外可见地址，用于 Cookie 与 SEO
	DatabaseURL    string   // postgres://...
	JWTSecret      string   // HS256 签名密钥
	AdminEmail     string   // 唯一管理员邮箱
	AdminPassHash  string   // 管理员密码的 bcrypt 哈希
	CookieName     string   // JWT Cookie 名
	CookieSecure   bool     // 生产必须 true
	AllowedOrigins []string // CORS 白名单
}

// Load 从当前目录往上找 .env 并载入（已存在的真实环境变量优先），再校验必填项。
// 这样脚本无论从仓库根还是 backend/ 下调用都能读到配置。
func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}

	cfg := &Config{
		Addr:           getenv("ADDR", ":8080"),
		BaseURL:        getenv("BASE_URL", "http://localhost:8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AdminEmail:     os.Getenv("ADMIN_EMAIL"),
		AdminPassHash:  os.Getenv("ADMIN_PASSWORD_HASH"),
		CookieName:     getenv("COOKIE_NAME", "kb_token"),
		CookieSecure:   getenv("COOKIE_SECURE", "false") == "true",
		AllowedOrigins: splitList(os.Getenv("ALLOWED_ORIGINS")),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.AdminEmail == "" {
		missing = append(missing, "ADMIN_EMAIL")
	}
	if cfg.AdminPassHash == "" {
		missing = append(missing, "ADMIN_PASSWORD_HASH")
	}
	if len(cfg.JWTSecret) > 0 && len(cfg.JWTSecret) < 32 {
		missing = append(missing, "JWT_SECRET(长度不足32)")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(".env 缺少必需配置: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// DatabaseURL 只取连接串，供迁移和 seed 这类不需要认证配置的工具命令使用。
func DatabaseURL() (string, error) {
	if err := loadDotEnv(); err != nil {
		return "", err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return "", fmt.Errorf("缺少 DATABASE_URL，请检查 .env")
	}
	return url, nil
}

// loadDotEnv 载入仓库根的 .env。找不到 .env 不算错（CI / Docker 直接注入环境变量）。
// godotenv.Load 不覆盖已存在的环境变量。
func loadDotEnv() error {
	envFile, err := findEnvFile()
	if err != nil {
		return nil
	}
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("读取 %s 失败: %w", envFile, err)
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// findEnvFile 从当前工作目录逐级往上找 .env，最多上溯 5 层。
func findEnvFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录失败: %w", err)
	}
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("没找到 .env，请先在仓库根目录 copy .env.example .env")
}
