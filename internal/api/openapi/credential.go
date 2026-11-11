package openapi

// 凭证读取与解密（open.open_api_client / open.open_api_credential，需求 11167/11168）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

// 认证模式（open.open_api_credential.auth_mode）。
const (
	AuthModeAPIKey = "APIKEY"
	AuthModeAKSK   = "AKSK"
	AuthModeBearer = "BEARER"
	AuthModeMTLS   = "MTLS"
	AuthModeBasic  = "BASIC"
)

// Credential 一次鉴权所需的凭证 + 调用方信息。
type Credential struct {
	IDN           int64
	CredentialNo  string
	AuthMode      string
	AccessKeyID   string
	SecretHash    string // 静态 Key：sha256 十六进制（或 BASIC 的 argon2id PHC）
	SecretCipher  string // AK/SK：AES-256-GCM 密文（Base64）
	SecretCipherP string // 轮换期旧 SK 密文
	SecretKID     string
	SignAlgorithm string
	// TokenFP STS 的 SecurityToken 指纹（仅 auth_mode=STS）；空表示非 STS。
	TokenFP string
	// ParentIDN STS 的派生来源凭证 idn（0 = 页面直发，无父凭证）。
	ParentIDN int64
	// 父凭证状态（与主查询一起 LEFT JOIN 出来，避免 STS 校验时再查一次表）：
	// ParentFound=false 表示 parent_idn 指向的行已不存在（被物理删除/清理）。
	ParentFound      bool
	ParentFlagEnable bool
	ParentFlagRevoke bool
	ParentExp        *time.Time
	Scope            []ScopeRule
	// ScopeBroken 授权范围数据损坏（scope_str 不是合法的对象数组）。
	// 解析失败时**不放行**：按「无权限」拒绝，避免坏数据把凭证悄悄放大成「全部接口」（需求 11190）。
	ScopeBroken   bool
	CredentialIPs []*net.IPNet

	ClientIDN    int64
	ClientCode   string
	ClientIPs    []*net.IPNet
	RateLimitRPM int
	QuotaDay     int

	FlagEnable    bool
	FlagRevoke    bool
	CredentialExp *time.Time
	ClientExp     *time.Time

	clientEnable bool // 调用方 flag_enable（未导出，避免构造假凭证时漏填）
}

// ScopeRule 授权范围条目（scope_str 数组元素）。
type ScopeRule struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// Active 凭证与调用方当前是否可用（停用/吊销/过期）。
//
// 返回 false 时同时给出审计原因码：吊销 → REVOKE，其余（停用/过期/调用方停用）→ EXPIRE。
func (c *Credential) Active(now time.Time) (bool, string) {
	if c.FlagRevoke {
		return false, ReasonRevoke
	}
	if !c.FlagEnable || !c.ClientEnable() {
		return false, ReasonExpire
	}
	if c.CredentialExp != nil && !now.Before(*c.CredentialExp) {
		return false, ReasonExpire
	}
	if c.ClientExp != nil && !now.Before(*c.ClientExp) {
		return false, ReasonExpire
	}
	return true, ""
}

// ClientEnable 调用方是否启用（读取时由 SQL 的 flag_enable 置位）。
func (c *Credential) ClientEnable() bool { return c.clientEnable }

// AllowIP 判定来源 IP 是否通过白名单（调用方级 ∩ 凭证级；两边都为空表示不限）。
func (c *Credential) AllowIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if len(c.ClientIPs) > 0 && !matchAny(c.ClientIPs, ip) {
		return false
	}
	if len(c.CredentialIPs) > 0 && !matchAny(c.CredentialIPs, ip) {
		return false
	}
	return true
}

func matchAny(nets []*net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// InScope 判定接口是否在授权范围内：空数组 = 全部接口；
// 路径支持精确匹配、尾部 `/*` 前缀匹配、以及不带通配时的「前缀 + /」匹配。
func (c *Credential) InScope(method, path string) bool {
	if len(c.Scope) == 0 {
		return true
	}
	for _, rule := range c.Scope {
		if rule.Method != "" && !strings.EqualFold(rule.Method, method) && rule.Method != "*" {
			continue
		}
		switch {
		case rule.Path == "" || rule.Path == "*" || rule.Path == path:
			return true
		case strings.HasSuffix(rule.Path, "/*"):
			if strings.HasPrefix(path, strings.TrimSuffix(rule.Path, "*")) {
				return true
			}
		case strings.HasSuffix(rule.Path, "/"):
			if strings.HasPrefix(path, rule.Path) {
				return true
			}
		}
	}
	return false
}

// HashStaticKey 计算静态 Key 的存储哈希（委托给共享的 secret 包）。
func HashStaticKey(key string) string { return secret.HashKey(key) }

// MatchStaticKey 恒定时间比较静态 Key 与库内哈希。
func MatchStaticKey(storedHash, provided string) bool { return secret.MatchKey(storedHash, provided) }

// FingerprintStaticKey 计算静态 Key 指纹（sha256 前 16 位大写），供审计严格区分密钥（需求 11186）。
func FingerprintStaticKey(key string) string { return secret.FingerprintKey(key) }

// DecryptSecret 解密 AK/SK 的 SK 密文（委托给共享的 secret 包）。
func DecryptSecret(cipherText string, key []byte) (string, error) {
	return secret.Decrypt(cipherText, key)
}

// EncryptSecret 加密 SK（服务端签发/轮换与测试使用）。
func EncryptSecret(plain string, key []byte) (string, error) { return secret.Encrypt(plain, key) }

// CredentialSource 凭证来源抽象（便于单测注入假数据）。
type CredentialSource interface {
	Lookup(ctx context.Context, accessKeyID string) (*Credential, error)
}

// ErrCredentialNotFound 凭证不存在。
var ErrCredentialNotFound = errors.New("凭证不存在")

const selectCredentialSQL = `
SELECT c.idn, c.credential_no, c.auth_mode, c.access_key_id,
       COALESCE(c.secret_hash, ''), COALESCE(c.secret_cipher, ''), COALESCE(c.secret_cipher_prev, ''),
       COALESCE(c.secret_kid, ''), c.sign_algorithm,
       COALESCE(c.token_fp, ''), c.parent_idn,
       COALESCE(c.scope_str::text, '[]'), COALESCE(c.allow_cidr::text[], '{}'),
       c.dt_expire, c.flag_enable, c.flag_revoke,
       cl.idn, cl.client_code, COALESCE(cl.allow_cidr::text[], '{}'),
       cl.rate_limit_rpm, cl.quota_day, cl.dt_expire, cl.flag_enable,
       (p.idn IS NOT NULL), COALESCE(p.flag_enable, '0'), COALESCE(p.flag_revoke, '0'), p.dt_expire
  FROM open.open_api_credential c
  JOIN open.open_api_client cl ON cl.idn = c.client_idn
  LEFT JOIN open.open_api_credential p ON p.idn = c.parent_idn
 WHERE c.access_key_id = $1`

// PGSource 从 open schema 读取凭证。
type PGSource struct {
	pool *pgxpool.Pool
}

// NewPGSource 创建基于 pgx 的凭证来源。
func NewPGSource(pool *pgxpool.Pool) *PGSource { return &PGSource{pool: pool} }

// Lookup 按公开标识读取凭证；不存在返回 ErrCredentialNotFound。
//
// 需求 11176：公开标识（静态 Key / AK）统一大写存储，因此先按原样查，
// 查不到再按大写查一次——这样调用方手写小写也能命中。
func (s *PGSource) Lookup(ctx context.Context, accessKeyID string) (*Credential, error) {
	if s.pool == nil {
		return nil, errors.New("数据库连接池未初始化")
	}
	accessKeyID = strings.TrimSpace(accessKeyID)
	if accessKeyID == "" {
		return nil, ErrCredentialNotFound
	}
	credential, err := s.lookupExact(ctx, accessKeyID)
	if err == nil {
		return credential, nil
	}
	if errors.Is(err, ErrCredentialNotFound) {
		if upper := strings.ToUpper(accessKeyID); upper != accessKeyID {
			return s.lookupExact(ctx, upper)
		}
	}
	return nil, err
}

// lookupExact 按给定标识精确查询。
func (s *PGSource) lookupExact(ctx context.Context, accessKeyID string) (*Credential, error) {
	return s.queryOne(ctx, selectCredentialSQL, accessKeyID)
}

// queryOne 执行凭证查询并映射成 Credential。
func (s *PGSource) queryOne(ctx context.Context, query string, arg any) (*Credential, error) {
	credential := &Credential{}
	var (
		scopeJSON       string
		credentialCIDRs []string
		clientCIDRs     []string
		credFlagEnable  string
		credFlagRevoke  string
		clientFlagEnab  string
		parentFlagEnab  string
		parentFlagRevok string
	)
	err := s.pool.QueryRow(ctx, query, arg).Scan(
		&credential.IDN, &credential.CredentialNo, &credential.AuthMode, &credential.AccessKeyID,
		&credential.SecretHash, &credential.SecretCipher, &credential.SecretCipherP,
		&credential.SecretKID, &credential.SignAlgorithm,
		&credential.TokenFP, &credential.ParentIDN,
		&scopeJSON, &credentialCIDRs,
		&credential.CredentialExp, &credFlagEnable, &credFlagRevoke,
		&credential.ClientIDN, &credential.ClientCode, &clientCIDRs,
		&credential.RateLimitRPM, &credential.QuotaDay, &credential.ClientExp, &clientFlagEnab,
		&credential.ParentFound, &parentFlagEnab, &parentFlagRevok, &credential.ParentExp,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCredentialNotFound
	}
	if err != nil {
		return nil, err
	}

	credential.FlagEnable = credFlagEnable == "1"
	credential.FlagRevoke = credFlagRevoke == "1"
	credential.clientEnable = clientFlagEnab == "1"
	credential.ParentFlagEnable = parentFlagEnab == "1"
	credential.ParentFlagRevoke = parentFlagRevok == "1"
	credential.Scope, credential.ScopeBroken = parseScope(scopeJSON)
	credential.CredentialIPs = parseCIDRs(credentialCIDRs)
	credential.ClientIPs = parseCIDRs(clientCIDRs)
	return credential, nil
}

func parseCIDRs(values []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, netValue, err := net.ParseCIDR(value); err == nil {
			nets = append(nets, netValue)
			continue
		}
		// 允许单 IP（PG 的 cidr 文本一般为 a.b.c.d/32，这里兜底手填的裸 IP）
		if ip := net.ParseIP(value); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			if _, netValue, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.String(), bits)); err == nil {
				nets = append(nets, netValue)
			}
		}
	}
	return nets
}

// parseScope 解析 scope_str。
//
// 只有「空串 / null / 空数组」表示**无授权范围限制（全部接口）**，这是约定语义；
// 其它任何解析失败（非法 JSON、元素是字符串而非 {"method","path"} 对象）都视为**数据损坏**，
// 返回 broken=true 让调用方拒绝请求——绝不能把坏数据当成「全部接口」放行（需求 11190）。
func parseScope(raw string) ([]ScopeRule, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return nil, false
	}
	var rules []ScopeRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, true
	}
	// 每条规则必须真的带 path（method 可空 = 任意方法）；否则视为损坏
	for _, rule := range rules {
		if strings.TrimSpace(rule.Path) == "" {
			return nil, true
		}
	}
	return rules, false
}
