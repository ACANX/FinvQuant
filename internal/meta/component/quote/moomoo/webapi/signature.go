package webapi

// 请求签名（传统 API Key 认证）。
//
// 签名原文由 5 个字段组成，字段之间用 \n 连接（空字段也保留位置）：
//
//	{timestamp_ms}\n{HTTP_METHOD}\n{request_path}\n{query_string}\n{sha256(body)_hex}
//
// query_string 是最终请求里的原始查询串（不含开头的 ?）；body_part 是原始请求体字节的
// SHA256 小写十六进制摘要，没有请求体时为空字符串。
//
// 该函数导出便于单测与排障（打印签名原文时不会泄露私钥）。

import "crypto/sha256"

// SigningText 构造签名原文。
//
// timestampMs 为毫秒时间戳字符串（与请求头 X-Timestamp 一致）；method 用大写；
// path 不含域名与查询参数；query 不含开头的 ?；body 为原始请求体（无 body 传 nil）。
func SigningText(timestampMs, method, path, query string, body []byte) string {
	bodyHash := ""
	if len(body) > 0 {
		sum := sha256.Sum256(body)
		bodyHash = hexLower(sum[:])
	}
	return timestampMs + "\n" + method + "\n" + path + "\n" + query + "\n" + bodyHash
}

// hexLower 字节 → 小写十六进制。
func hexLower(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0f]
	}
	return string(out)
}
