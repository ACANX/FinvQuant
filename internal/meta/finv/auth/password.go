// Package auth 提供系统用户认证：密码哈希、会话管理与登录编排。
//
// 密码存储策略（需求 10761）：
//   - 算法 argon2id（OWASP 推荐），参数 m=64MB、t=3、p=2、keyLen=32
//   - 盐独立存放于 sys.sys_user.password_salt（hex，32 字符）
//   - 哈希以 PHC 字符串存放于 sys.sys_user.password，绝不落明文
//   - 校验使用 crypto/subtle 常量时间比较，避免时序侧信道
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// 哈希参数与长度约束。
const (
	// SaltLength 盐字节数；hex 编码后 32 字符，适配 password_salt varchar(64)。
	SaltLength = 16
	// KeyLength 派生密钥字节数。
	KeyLength = 32

	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MiB
	argonThreads uint8  = 2
)

// ErrInvalidStoredHash 表示库中哈希串格式非法。
var ErrInvalidStoredHash = errors.New("密码哈希格式非法")

// GenerateSalt 生成随机盐并以 hex 返回。
func GenerateSalt() (string, error) {
	buf := make([]byte, SaltLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机盐失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Hash 用给定 hex 盐计算 argon2id 哈希，返回 PHC 编码串。
//
// 输出形如 $argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>，
// 约 97 字符，适配 password varchar(128)。
func Hash(plain, saltHex string) (string, error) {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return "", fmt.Errorf("盐不是合法 hex 串: %w", err)
	}
	if len(salt) == 0 {
		return "", errors.New("盐不能为空")
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify 校验明文密码与 PHC 编码哈希是否匹配（常量时间比较）。
func Verify(plain, encoded string) bool {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(plain), salt, params.time, params.memory, params.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// hashParams 从 PHC 串解析出的参数。
type hashParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// decodeHash 解析 PHC 编码串，返回参数、盐与期望密钥。
func decodeHash(encoded string) (hashParams, []byte, []byte, error) {
	var p hashParams
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidStoredHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrInvalidStoredHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, nil, nil, ErrInvalidStoredHash
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return p, nil, nil, ErrInvalidStoredHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return p, nil, nil, ErrInvalidStoredHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return p, nil, nil, ErrInvalidStoredHash
	}
	return p, salt, key, nil
}
