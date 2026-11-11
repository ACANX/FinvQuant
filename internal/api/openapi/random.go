package openapi

// 随机数与请求 ID（鉴权链路上的小块工具）。

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// fillRandom 用 crypto/rand 填满缓冲区。
func fillRandom(buf []byte) error {
	_, err := rand.Read(buf)
	return err
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
