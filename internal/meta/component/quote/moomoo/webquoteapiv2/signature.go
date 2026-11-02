// Package webquoteapiv2 封装 moomoo（富途）网页行情接口 /quote-api/quote-v2/*。
//
// 协议口径见 Docs（厂商文档）：1108-Kline.md 与 MoomooQuoteV2WebSignature.md。
// 本包只做「协议 → 统一结构」的转换，不碰数据库；落库由调用方（cdeu 侧）负责。
package webquoteapiv2

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"net/url"
	"strings"
)

// 签名常量（见 MoomooQuoteV2WebSignature.md §2）。
const (
	// signKey HMAC 密钥，来自网页前端产物，非用户凭据。
	signKey = "quote_web"
	// hmacTruncLen HMAC-SHA512 十六进制结果截取长度。
	hmacTruncLen = 10
	// tokenLen 最终 quote-token 长度。
	tokenLen = 10
	// emptyInputFallback 空输入降级字面量（对应前端 if (input.length <= 0) input = "quote"）。
	emptyInputFallback = "quote"
)

// Param 有序查询参数。
//
// 签名对「键序 + 值类型」逐字节敏感（规则 R2/R7/R8），因此这里用切片而不是
// map，避免 Go 的 map 遍历顺序破坏键序。
type Param struct {
	Name  string
	Value string // 必须已字符串化（规则 R8）
}

// Params 便于按调用顺序构造参数列表。
func Params(pairs ...string) []Param {
	params := make([]Param, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		params = append(params, Param{Name: pairs[i], Value: pairs[i+1]})
	}
	return params
}

// SignInput 把有序参数拼成签名输入的紧凑 JSON。
//
// 实现不经过 encoding/json：JSON 对象键序与紧凑格式是签名契约的一部分，
// 用标准库排序/缩进会直接产出错误的 token。
func SignInput(params []Param) string {
	if len(params) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(params) * 24)
	b.WriteByte('{')
	for i, p := range params {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(escapeJSONString(p.Name))
		b.WriteString(`":"`)
		b.WriteString(escapeJSONString(p.Value))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// Sign 生成 quote-token：HMAC-SHA512 → 取前 10 → SHA-256 → 取前 10。
//
// 空输入按协议降级为字面量 "quote"（§2 Step 1）。
func Sign(input string) string {
	if len(input) == 0 {
		input = emptyInputFallback
	}
	mac := hmac.New(sha512.New, []byte(signKey))
	mac.Write([]byte(input))
	first := hex.EncodeToString(mac.Sum(nil))[:hmacTruncLen]

	sum := sha256.Sum256([]byte(first))
	return hex.EncodeToString(sum[:])[:tokenLen]
}

// SignParams 生成有序参数对应的 quote-token。
func SignParams(params []Param) string { return Sign(SignInput(params)) }

// SignedQuery 按参数顺序拼查询串（键序即签名键序，不能用 url.Values.Encode 排序）。
func SignedQuery(params []Param) string {
	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(p.Name))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(p.Value))
	}
	return b.String()
}

// escapeJSONString 按 JSON 最小转义处理字符串（规则 R4：非 ASCII 保留原始 UTF-8）。
func escapeJSONString(s string) string {
	if !strings.ContainsAny(s, `"\\`+"\n\r\t") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
