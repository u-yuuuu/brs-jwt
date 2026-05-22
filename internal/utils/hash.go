package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashPasswordSHA256 создает SHA-256 хеш пароля
func HashPasswordSHA256(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPasswordSHA256 проверяет пароль против SHA-256 хеша
func VerifyPasswordSHA256(password, hash string) bool {
	return HashPasswordSHA256(password) == hash
}

// SimpleHash создает SHA-256 хеш (для session token)
func SimpleHash(input string) string {
	return HashPasswordSHA256(input)
}

func VerifyPassword(password, hash string) bool {
	return VerifyPasswordSHA256(password, hash)
}
