package openapi

// AK/SK 签名协议（需求 11168，文档 Docs/API/OpenAPI/Auth.md）。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// AuthSchemePrefix 签名认证方案前缀（需求 11179）：
//
//	Authorization: META1-HMAC-SHA256 Credential=AK_LIVE_…, Signature=<base64 签名>
//
// 取「META1」+ 算法名，与 AWS 的 `AWS4-HMAC-SHA256 ...` 同构：
// 一眼能看出用的是哪种签名算法，也便于网关/审计区分「静态 Key」与「签名」两类认证。
const AuthSchemePrefix = "META1-"

// HeaderTimestamp / HeaderNonce 签名请求头（需求 11179 由 X-Finv-* 改为 X-META-*）。
const (
	HeaderTimestamp = "X-META-TS"
	HeaderNonce     = "X-META-Nonce"
)

// ErrBadAuthHeader Authorization 头格式非法。
var ErrBadAuthHeader = errors.New("Authorization 头格式非法")

// SignRequest 待签请求的要素（由中间件从 *http.Request 提取）。
type SignRequest struct {
	Method    string
	Path      string
	RawQuery  string
	BodyHash  string // sha256(body) 十六进制；GET/空体为 sha256("") 的十六进制
	Timestamp string
	Nonce     string
}

// CanonicalString 生成待签串：
//
//	METHOD \n PATH \n SORTED_QUERY \n BODY_SHA256 \n TIMESTAMP \n NONCE
//
// SORTED_QUERY：参数名升序、同名参数的值也升序，键值均做 RFC 3986 百分号编码（空格 → %20）。
func CanonicalString(req SignRequest) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(req.Method)),
		req.Path,
		canonicalQuery(req.RawQuery),
		req.BodyHash,
		req.Timestamp,
		req.Nonce,
	}, "\n")
}

// canonicalQuery 规范化查询串；解析失败时退回原始串（宁可让签名失败，也不静默改变语义）。
func canonicalQuery(rawQuery string) string {
	rawQuery = strings.TrimPrefix(rawQuery, "?")
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(values))
	for _, key := range keys {
		// 同名参数也排序：调用方实现（不同语言库）对重复参数的顺序不保证一致，
		// 固定成「键升序、同名值也升序」才能稳定复算签名。
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, value := range items {
			parts = append(parts, escapeRFC3986(key)+"="+escapeRFC3986(value))
		}
	}
	return strings.Join(parts, "&")
}

// escapeRFC3986 在 url.QueryEscape 基础上把 `+` 还原为 `%20`（RFC 3986 口径）。
func escapeRFC3986(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

// signWith 用 SK 计算签名（Base64）。
func signWith(algorithm, secret, canonical string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(algorithm)) {
	case "", "HMAC-SHA256":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(canonical))
		return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
	case "HMAC-SHA512", "ED25519":
		// 预留算法：签名串格式与服务端实现都未定稿，先明确拒绝，避免「看起来支持其实没验」。
		return "", errors.New("签名算法 " + algorithm + " 尚未启用")
	default:
		return "", errors.New("不支持的签名算法 " + algorithm)
	}
}

// Sign 计算签名（导出供调用方样例与测试使用）。
func Sign(algorithm, secret, canonical string) (string, error) {
	return signWith(algorithm, secret, canonical)
}

// VerifySignature 恒定时间比较签名。
func VerifySignature(algorithm, secret, canonical, provided string) bool {
	expected, err := signWith(algorithm, secret, canonical)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(provided)))
}

// ParseSignHeader 解析 `Authorization: META1-<算法> Credential=<AK>, Signature=<签名>`。
//
// 解析口径：
//   - 方案前缀 `META1-` 大小写不敏感，其后紧跟算法名（到第一个空白为止，统一转大写）；
//   - 参数按 `,` 分隔、每段 `k=v`，参数名大小写不敏感，值两端空白与可选双引号会被去掉；
//   - `Credential` 与 `Signature` 必须同时存在。
func ParseSignHeader(header string) (algorithm, accessKeyID, signature string, ok bool) {
	header = strings.TrimSpace(header)
	if len(header) <= len(AuthSchemePrefix) || !strings.EqualFold(header[:len(AuthSchemePrefix)], AuthSchemePrefix) {
		return "", "", "", false
	}
	rest := strings.TrimSpace(header[len(AuthSchemePrefix):])

	space := strings.IndexAny(rest, " \t")
	if space <= 0 {
		return "", "", "", false
	}
	algorithm = strings.ToUpper(strings.TrimSpace(rest[:space]))

	for _, part := range strings.Split(rest[space+1:], ",") {
		name, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "credential":
			accessKeyID = strings.Trim(strings.TrimSpace(value), `"`)
		case "signature":
			signature = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	if algorithm == "" || accessKeyID == "" || signature == "" {
		return "", "", "", false
	}
	return algorithm, accessKeyID, signature, true
}

// ParseTimestamp 解析 Unix 秒时间戳。
func ParseTimestamp(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("缺少时间戳")
	}
	return strconv.ParseInt(raw, 10, 64)
}

// BodyHash 请求体摘要（空体也要参与签名，统一用 sha256("")）。
//
// 需求 11177：输出**纯大写**十六进制，审计表 body_sha256 与待签串共用同一个值，
// 调用方算签名时也必须用大写形式。
func BodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
