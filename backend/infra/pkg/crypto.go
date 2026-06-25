/*
 * crypto.go
 * 功能：密码加密与校验工具，基于 bcrypt
 * 时间戳：2026-04-20
 */

package pkg

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword 使用 bcrypt 对明文密码进行加密
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 比较明文密码与 bcrypt 哈希是否匹配
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
