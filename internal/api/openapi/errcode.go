// Package openapi 实现外部 OpenAPI（/OpenAPI/V1）的鉴权与审计（需求 11168）。
//
// 与内部接口的登录会话体系（/API/V1 的 Bearer 令牌）完全独立：
//
//   - 模式一：静态 API Key —— `X-API-Key: fk_live_xxx` 或 `Authorization: Bearer fk_live_xxx`；
//   - 模式二：AK/SK 签名 —— `Authorization: FinvAK <ak>:<sig>` + `X-Finv-Timestamp` + `X-Finv-Nonce`；
//   - 预留：BEARER（JWT）/ MTLS（证书指纹）/ BASIC（用户名口令），当前一律按「尚未启用」拒绝。
//
// 表结构见 Deploy/Migrations/V300183（open.open_api_client / open_api_credential / open_api_invoke_log），
// 逐列字典见 Docs/DataDictMapping/OpenApiAuth.md，错误码目录见 Docs/DevSpec/ErrorCodeSpec.md §1.2。
package openapi

import "net/http"

// 外部 OpenAPI 鉴权错误码（190xx 子段，见 ErrorCodeSpec §1.2）。
//
// 号段归属与 HTTP 映射是**契约**：改动必须同步 ErrorCodeSpec 与本包的 errcode_test.go。
const (
	CodeKeyMissing     = 19001 // 未携带任何凭证
	CodeKeyInvalid     = 19002 // 凭证未知或密钥不匹配（刻意合并 AK 不存在与 SK 不符，防枚举）
	CodeCredentialStop = 19003 // 凭证停用 / 已吊销 / 调用方或凭证过期
	CodeCIDRDeny       = 19004 // 来源 IP 不在白名单
	CodeSignMismatch   = 19005 // 签名不匹配
	CodeTimestampSkew  = 19006 // 时间戳超出允许窗口
	CodeNonceReplay    = 19007 // nonce 在窗口内重复（重放嫌疑）
	CodeRateLimit      = 19008 // 超出速率或每日配额
	CodeScopeDeny      = 19009 // 授权范围不含该接口
	CodeAuthNotReady   = 19010 // 平台侧鉴权未就绪（密钥未配置 / 认证模式尚未启用）
)

// CodeRange 是 190xx 子段的闭区间，供契约测试断言号段归属。
var CodeRange = [2]int{19001, 19099}

// AllCodes 返回全部已定义错误码（契约测试遍历用）。
func AllCodes() []int {
	return []int{
		CodeKeyMissing, CodeKeyInvalid, CodeCredentialStop, CodeCIDRDeny, CodeSignMismatch,
		CodeTimestampSkew, CodeNonceReplay, CodeRateLimit, CodeScopeDeny, CodeAuthNotReady,
	}
}

// 审计日志 reason_code 取值（open.open_api_invoke_log.reason_code，varchar(32)）。
const (
	ReasonNoKey        = "NO_KEY"
	ReasonBadKey       = "BAD_KEY"
	ReasonRevoke       = "REVOKE"
	ReasonExpire       = "EXPIRE"
	ReasonCIDRDeny     = "CIDR_DENY"
	ReasonSignMismatch = "SIGN_MISMATCH"
	ReasonTimestamp    = "TS_SKEW"
	ReasonNonceReplay  = "NONCE_REPLAY"
	ReasonScopeDeny    = "SCOPE_DENY"
	ReasonRateLimit    = "RATE_LIMIT"
	ReasonNotReady     = "NOT_READY"
)

// statusOf 错误码到 HTTP 状态码的映射（契约的一部分）。
func statusOf(code int) int {
	switch code {
	case CodeKeyMissing, CodeKeyInvalid, CodeCIDRDeny, CodeSignMismatch,
		CodeTimestampSkew, CodeNonceReplay:
		return http.StatusUnauthorized
	case CodeCredentialStop, CodeScopeDeny:
		return http.StatusForbidden
	case CodeRateLimit:
		return http.StatusTooManyRequests
	case CodeAuthNotReady:
		return http.StatusServiceUnavailable
	default:
		return http.StatusUnauthorized
	}
}

// messageOf 错误码对应的安全文案（不泄露内部细节，如具体是哪一列不匹配）。
func messageOf(code int) string {
	switch code {
	case CodeKeyMissing:
		return "未携带 API Key 或签名信息"
	case CodeKeyInvalid:
		return "凭证无效或密钥不匹配"
	case CodeCredentialStop:
		return "凭证已停用、已吊销或已过期"
	case CodeCIDRDeny:
		return "来源地址不在允许的白名单内"
	case CodeSignMismatch:
		return "签名校验失败"
	case CodeTimestampSkew:
		return "请求时间戳超出允许范围，请校准系统时间"
	case CodeNonceReplay:
		return "请求 nonce 重复（疑似重放）"
	case CodeRateLimit:
		return "超出调用频率或配额限制，请稍后重试"
	case CodeScopeDeny:
		return "该凭证无权访问此接口"
	case CodeAuthNotReady:
		return "平台鉴权尚未就绪，请联系管理员"
	default:
		return "鉴权失败"
	}
}

// reasonOf 错误码到审计 reason_code 的映射。
func reasonOf(code int) string {
	switch code {
	case CodeKeyMissing:
		return ReasonNoKey
	case CodeKeyInvalid:
		return ReasonBadKey
	case CodeCredentialStop:
		return ReasonRevoke
	case CodeCIDRDeny:
		return ReasonCIDRDeny
	case CodeSignMismatch:
		return ReasonSignMismatch
	case CodeTimestampSkew:
		return ReasonTimestamp
	case CodeNonceReplay:
		return ReasonNonceReplay
	case CodeRateLimit:
		return ReasonRateLimit
	case CodeScopeDeny:
		return ReasonScopeDeny
	case CodeAuthNotReady:
		return ReasonNotReady
	default:
		return ReasonBadKey
	}
}
