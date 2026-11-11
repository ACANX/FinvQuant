package openapi

// 随机数与请求 ID（鉴权链路上的小块工具）。

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// requestIDPattern 客户端可透传的追踪 ID 形态：8~64 位十六进制。
var requestIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{8,64}$`)

// fillRandom 用 crypto/rand 填满缓冲区。
func fillRandom(buf []byte) error {
	_, err := rand.Read(buf)
	return err
}

// resolveRequestID 决定本次请求的追踪 ID。
//
// 允许客户端透传（便于跨系统串联链路），但**只接受 8~64 位十六进制并统一转大写**；
// 其它任何形态（含小写会被归一、中文/超长/带分隔符直接忽略）一律由服务端重新生成。
// 这样响应头 X-Request-Id 与审计表 request_id 的值恒为「大写十六进制」，也不会被
// 调用方用来污染审计或把多个请求伪造成同一个 ID（需求 11182）。
func resolveRequestID(incoming string) string {
	if value := strings.TrimSpace(incoming); requestIDPattern.MatchString(value) {
		return strings.ToUpper(value)
	}
	return newRequestID()
}

// newRequestID 生成 32 位十六进制请求 ID（与响应头 X-Request-Id 一致，便于对日志）。
//
// 需求 11177：纯大写，与审计表里的其它十六进制字段（body_sha256）口径一致。
func newRequestID() string {
	buf := make([]byte, 16)
	if err := fillRandom(buf); err != nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(buf))
}
