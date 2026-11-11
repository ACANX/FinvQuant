// Package secret 提供外部 OpenAPI 凭证的密钥学工具（静态 Key 哈希 / SK 加解密）。
//
// 单独成包是为了让「鉴权中间件」（internal/api/openapi）与「管理接口」
// （internal/meta/finv/openapi）共用同一套实现，避免两处各写一遍 AES-GCM 与哈希口径。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// 静态 Key 前缀（便于人工识别与日志检索）。
const (
	PrefixAPIKey = "fk_live_"
	PrefixAK     = "ak_live_"
)

// HashKey 计算静态 Key 的存储哈希（sha256 十六进制）。
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// MatchKey 恒定时间比较静态 Key 与库内哈希（哈希大小写不敏感，防人工改乱）。
func MatchKey(storedHash, provided string) bool {
	if storedHash == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(storedHash)), []byte(HashKey(provided))) == 1
}

// Encrypt 用 AES-256-GCM 加密明文（SK），输出 Base64(nonce||ciphertext)。
//
// 格式约定：12 字节随机 nonce + 密文（含 16 字节 tag），Base64 标准编码。
func Encrypt(plain string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("密钥必须是 32 字节（AES-256）")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt 解密 SK 密文；空密文返回空串与 nil（由调用方按「验签必然失败」处理）。
func Decrypt(cipherText string, key []byte) (string, error) {
	cipherText = strings.TrimSpace(cipherText)
	if cipherText == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", errors.New("密钥必须是 32 字节（AES-256）")
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("密文不是合法 Base64: %w", err)
	}
	if len(raw) < 12+16 {
		return "", errors.New("密文长度不足")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, raw[:12], raw[12:], nil)
	if err != nil {
		return "", errors.New("密文解密失败（密钥或 KID 不匹配）")
	}
	return string(plain), nil
}

// DecodeKey 解码 sys_config 19005 里的 Base64 密钥。
func DecodeKey(base64Key string) ([]byte, error) {
	raw := strings.TrimSpace(base64Key)
	if raw == "" {
		return nil, errors.New("sys_config 19005（OpenAPI 凭证加密密钥）未配置")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("sys_config 19005 不是合法的 Base64")
	}
	if len(key) != 32 {
		return nil, errors.New("sys_config 19005 必须是 32 字节（AES-256）密钥")
	}
	return key, nil
}

// RandomToken 生成 n 字节随机数的十六进制串（用于密钥/乱数）。
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// EncodeKeyForTest 把密钥字节编码为 sys_config 19005 期望的 Base64 文本
// （仅供联调测试与运维脚本生成配置值，生产路径不会用它写库）。
func EncodeKeyForTest(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}
