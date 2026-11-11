// Package openapi 实现「OpenAPI 调用方管理」的服务层（需求 11169）。
//
// 与 internal/api/openapi（鉴权中间件）共用同一个 secret 包做哈希与加解密，
// 保证「签发密钥」与「校验密钥」两边的算法/格式完全一致。
//
// 数据表：open.open_api_client / open.open_api_credential / open.open_api_invoke_log（V300183）。
// 口径要点：
//   - 密钥**只在签发/轮换的那一刻返回一次明文**，之后任何接口都不回显（库里只有哈希或密文）；
//   - 停用可逆（flag_enable），吊销不可逆（flag_revoke + dt_revoke + reason_revoke，原因必填）；
//   - 授权范围 scope_str 用 jsonb 单列，保存时校验元素形状。
package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// 认证模式。
const (
	AuthModeAPIKey = "API_KEY"
	AuthModeAKSK   = "AK_SK"
)

// 可签发的认证模式（BEARER / MTLS / BASIC 预留，不允许在管理接口里签发）。
var issuableAuthModes = []string{AuthModeAPIKey, AuthModeAKSK}

// 配置号（sys.sys_config 19xxx 号段）。
const (
	ConfigAuthMode  = 19001
	ConfigTimeWin   = 19002
	ConfigLogRetain = 19003
	ConfigSecretKID = 19004
	ConfigSecretKey = 19005
	ConfigFallback  = 19006
)

// 业务错误（handler 按类型映射 HTTP 状态码）。
var (
	// ErrInvalidRequest 入参非法（400）。
	ErrInvalidRequest = errors.New("入参非法")
	// ErrNotFound 目标记录不存在（404）。
	ErrNotFound = errors.New("记录不存在")
	// ErrConflict 状态冲突（409），如重复吊销。
	ErrConflict = errors.New("状态冲突")
	// ErrNotReady 依赖未就绪（503），如 19005 加密密钥未配置。
	ErrNotReady = errors.New("依赖未配置")
)

// invalid 构造带上下文的入参错误。
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequest, fmt.Sprintf(format, args...))
}

// SettingsView 鉴权配置视图（不含任何密钥明文）。
type SettingsView struct {
	Mode             string `json:"mode"`               // 19001：0 关闭 / 1 启用 / 2 宽松
	SignTimeWindow   int    `json:"sign_time_window"`   // 19002：秒
	LogRetainDays    int    `json:"log_retain_days"`    // 19003：天
	SecretKID        string `json:"secret_kid"`         // 19004
	SecretKeyReady   bool   `json:"secret_key_ready"`   // 19005 是否已配置（只回布尔）
	FallbackKeyReady bool   `json:"fallback_key_ready"` // 19006 是否已配置
}

// SettingsRequest 保存鉴权配置（只允许改 19001～19003，密钥不在此处改）。
type SettingsRequest struct {
	Mode           string `json:"mode"`
	SignTimeWindow int    `json:"sign_time_window"`
	LogRetainDays  int    `json:"log_retain_days"`
}

// Client 调用方（列表/详情共用）。
type Client struct {
	IDN          int64    `json:"idn"`
	ClientCode   string   `json:"client_code"`
	ClientName   string   `json:"client_name"`
	OwnerUID     string   `json:"owner_uid"`
	ContactEmail string   `json:"contact_email"`
	ContractNo   string   `json:"contract_no"`
	AllowCIDR    []string `json:"allow_cidr"`
	RateLimitRPM int      `json:"rate_limit_rpm"`
	QuotaDay     int      `json:"quota_day"`
	DTExpire     string   `json:"dt_expire"` // RFC3339 或空串
	FlagEnable   string   `json:"flag_enable"`
	Remark       string   `json:"remark"`
	GMTUpdate    string   `json:"gmt_update"`

	CountCredential int `json:"count_credential"`
	CountCredActive int `json:"count_credential_active"`
	CountInvoke     int `json:"count_invoke"`
}

// ClientRequest 新建/更新调用方（idn=0 表示新建）。
type ClientRequest struct {
	IDN          int64    `json:"idn"`
	ClientCode   string   `json:"client_code"`
	ClientName   string   `json:"client_name"`
	OwnerUID     string   `json:"owner_uid"`
	ContactEmail string   `json:"contact_email"`
	ContractNo   string   `json:"contract_no"`
	AllowCIDR    []string `json:"allow_cidr"`
	RateLimitRPM int      `json:"rate_limit_rpm"`
	QuotaDay     int      `json:"quota_day"`
	DTExpire     string   `json:"dt_expire"`
	Remark       string   `json:"remark"`
}

// Credential 凭证（不含任何哈希/密文）。
type Credential struct {
	IDN           int64    `json:"idn"`
	ClientIDN     int64    `json:"client_idn"`
	ClientCode    string   `json:"client_code"`
	CredentialNo  string   `json:"credential_no"`
	AuthMode      string   `json:"auth_mode"`
	AccessKeyID   string   `json:"access_key_id"`
	KeyPrefix     string   `json:"key_prefix"`
	HasSecret     bool     `json:"has_secret"`
	SignAlgorithm string   `json:"sign_algorithm"`
	Scope         []string `json:"scope"` // 形如 "POST /OpenAPI/V1/..." 的可读展示
	AllowCIDR     []string `json:"allow_cidr"`
	DTExpire      string   `json:"dt_expire"`
	DTLastUse     string   `json:"dt_last_use"`
	CountUse      int64    `json:"count_use"`
	FlagEnable    string   `json:"flag_enable"`
	FlagRevoke    string   `json:"flag_revoke"`
	DTRevoke      string   `json:"dt_revoke"`
	ReasonRevoke  string   `json:"reason_revoke"`
	Remark        string   `json:"remark"`
	GMTUpdate     string   `json:"gmt_update"`
}

// CredentialCreateRequest 签发凭证。
type CredentialCreateRequest struct {
	ClientIDN     int64    `json:"client_idn"`
	CredentialNo  string   `json:"credential_no"`
	AuthMode      string   `json:"auth_mode"`
	SignAlgorithm string   `json:"sign_algorithm"`
	Scope         []string `json:"scope"` // 形如 "POST /OpenAPI/V1/..."
	AllowCIDR     []string `json:"allow_cidr"`
	DTExpire      string   `json:"dt_expire"`
	Remark        string   `json:"remark"`

	// AccessKey 由服务层生成（服务端签发，不接受前端指定），不参与 JSON 编解码。
	AccessKey string `json:"-"`
}

// IssuedSecret 签发/轮换结果：**明文只在这里出现一次**。
type IssuedSecret struct {
	Credential Credential `json:"credential"`
	AccessKey  string     `json:"access_key"`
	SecretKey  string     `json:"secret_key"` // API_KEY 模式为静态 Key 本身；AK_SK 模式为 SK
	Note       string     `json:"note"`
}

// InvokeLog 调用审计行。
type InvokeLog struct {
	IDN           int64  `json:"idn"`
	ClientIDN     int64  `json:"client_idn"`
	CredentialIDN int64  `json:"credential_idn"`
	AccessKeyID   string `json:"access_key_id"`
	KFP           string `json:"kfp"`
	AuthMode      string `json:"auth_mode"`
	RequestID     string `json:"request_id"`
	DTRequest     string `json:"dt_request"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Query         string `json:"query_str"`
	StatusCode    int    `json:"status_code"`
	BizCode       int    `json:"biz_code"`
	FlagPass      string `json:"flag_pass"`
	ReasonCode    string `json:"reason_code"`
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`
	BodySHA256    string `json:"body_sha256"`
	CountBody     int    `json:"count_body"`
	LatencyMs     int    `json:"latency_ms"`
}

// InvokeLogFilter 调用审计查询条件。
type InvokeLogFilter struct {
	ClientIDN     int64
	CredentialIDN int64
	AccessKeyID   string
	KFP           string
	FlagPass      string
	ReasonCode    string
	DTFrom        time.Time
	DTTo          time.Time
	Page          int
	PageSize      int
}

// ---------------- 校验 ----------------

var (
	clientCodePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,31}$`)
	credentialNoPatten = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{1,47}$`)
	kidPattern         = regexp.MustCompile(`^[A-Za-z0-9._-]{1,16}$`)
)

// normalizeMode 归一鉴权模式：只接受 0/1/2。
func normalizeMode(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "0":
		return "0", nil
	case "1":
		return "1", nil
	case "2":
		return "2", nil
	default:
		return "", invalid("mode 只能是 0（关闭）/ 1（启用）/ 2（宽松），实际 %q", raw)
	}
}

// validateClient 校验并归一调用方入参。
func validateClient(req ClientRequest) (ClientRequest, error) {
	req.ClientCode = strings.TrimSpace(req.ClientCode)
	req.ClientName = strings.TrimSpace(req.ClientName)
	req.OwnerUID = strings.TrimSpace(req.OwnerUID)
	req.ContactEmail = strings.TrimSpace(req.ContactEmail)
	req.ContractNo = strings.TrimSpace(req.ContractNo)
	req.Remark = strings.TrimSpace(req.Remark)

	if !clientCodePattern.MatchString(req.ClientCode) {
		return req, invalid("client_code 只允许小写字母/数字/._-，2~32 位（如 cdeu-hook）")
	}
	if req.ClientName == "" {
		return req, invalid("client_name 不能为空")
	}
	if len([]rune(req.ClientName)) > 128 {
		return req, invalid("client_name 超过 128 字符")
	}
	if req.RateLimitRPM < 0 {
		return req, invalid("rate_limit_rpm 不能为负数（0 表示不限）")
	}
	if req.QuotaDay < 0 {
		return req, invalid("quota_day 不能为负数（0 表示不限）")
	}
	cidrs, err := normalizeCIDRs(req.AllowCIDR)
	if err != nil {
		return req, err
	}
	req.AllowCIDR = cidrs
	if _, err := parseExpire(req.DTExpire); err != nil {
		return req, err
	}
	return req, nil
}

// validateCredential 校验签发入参。
func validateCredential(req CredentialCreateRequest) (CredentialCreateRequest, error) {
	// 凭证号统一大写入库（需求 11176），避免与人工抄写的形式不一致
	req.CredentialNo = strings.ToUpper(strings.TrimSpace(req.CredentialNo))
	req.AuthMode = strings.ToUpper(strings.TrimSpace(req.AuthMode))
	req.SignAlgorithm = strings.ToUpper(strings.TrimSpace(req.SignAlgorithm))
	req.Remark = strings.TrimSpace(req.Remark)

	if req.ClientIDN <= 0 {
		return req, invalid("client_idn 必须为正整数")
	}
	if req.CredentialNo == "" {
		return req, invalid("credential_no 不能为空")
	}
	if !credentialNoPatten.MatchString(req.CredentialNo) {
		return req, invalid("credential_no 只允许字母/数字/._-，2~48 位（字母统一大写）")
	}
	if !contains(issuableAuthModes, req.AuthMode) {
		return req, invalid("auth_mode 只能是 %s（BEARER / MTLS / BASIC 为预留，暂不允许签发）",
			strings.Join(issuableAuthModes, " / "))
	}
	if req.AuthMode == AuthModeAPIKey {
		// 静态 Key 不走签名，但列上有 NOT NULL + CHECK，统一落 HMAC-SHA256（与表默认值一致）
		req.SignAlgorithm = "HMAC-SHA256"
	} else {
		if req.SignAlgorithm == "" {
			req.SignAlgorithm = "HMAC-SHA256"
		}
		if req.SignAlgorithm != "HMAC-SHA256" {
			return req, invalid("sign_algorithm 目前只支持 HMAC-SHA256（HMAC-SHA512 / ED25519 为预留）")
		}
	}
	scope, err := normalizeScope(req.Scope)
	if err != nil {
		return req, err
	}
	req.Scope = scope
	cidrs, err := normalizeCIDRs(req.AllowCIDR)
	if err != nil {
		return req, err
	}
	req.AllowCIDR = cidrs
	if _, err := parseExpire(req.DTExpire); err != nil {
		return req, err
	}
	return req, nil
}

// normalizeScope 校验授权范围："METHOD PATH" 形式；空数组表示全部接口。
func normalizeScope(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.Fields(item)
		if len(parts) != 2 {
			return nil, invalid("scope 元素必须是「方法 路径」两段，实际 %q", item)
		}
		method := strings.ToUpper(parts[0])
		if method != "*" && method != "GET" && method != "POST" && method != "PUT" &&
			method != "PATCH" && method != "DELETE" {
			return nil, invalid("scope 方法非法: %q", parts[0])
		}
		if !strings.HasPrefix(parts[1], "/") {
			return nil, invalid("scope 路径必须以 / 开头: %q", parts[1])
		}
		out = append(out, method+" "+parts[1])
	}
	return out, nil
}

// normalizeCIDRs 校验白名单（支持裸 IP 与 CIDR，统一成 CIDR 文本）。
func normalizeCIDRs(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			item = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		if _, _, err := net.ParseCIDR(item); err != nil {
			return nil, invalid("allow_cidr 含非法网段: %q", item)
		}
		out = append(out, item)
	}
	return out, nil
}

// parseExpire 解析到期时间：空串表示长期；接受 RFC3339 与 "2006-01-02"。
func parseExpire(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}
	return nil, invalid("dt_expire 需为 RFC3339 或 2006-01-02 格式，实际 %q", raw)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// maskKey 脱敏（审计与列表展示用）。
func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "***" + key[len(key)-4:]
}

// ---------------- scope 序列化 ----------------

// scopeToJSON 把 "METHOD PATH" 列表转成 scope_str 的 jsonb 文本。
func scopeToJSON(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	type rule struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	rules := make([]rule, 0, len(items))
	for _, item := range items {
		parts := strings.Fields(item)
		if len(parts) != 2 {
			continue
		}
		rules = append(rules, rule{Method: parts[0], Path: parts[1]})
	}
	encoded, err := json.Marshal(rules)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// scopeToText 把 scope_str 的 jsonb 文本还原成 "METHOD PATH" 列表（展示用）。
func scopeToText(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}
	var rules []struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		method := rule.Method
		if method == "" {
			method = "*"
		}
		out = append(out, method+" "+rule.Path)
	}
	return out
}
