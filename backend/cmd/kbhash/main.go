// kbhash 生成管理员密码的 bcrypt 哈希，输出到 .env 的 ADMIN_PASSWORD_HASH。
// 用法：go run ./cmd/kbhash '你的密码'
// 密码本身不入库、不进日志，只打印哈希。
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "" {
		fmt.Fprintln(os.Stderr, "用法: go run ./cmd/kbhash <密码>")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "生成哈希失败:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
