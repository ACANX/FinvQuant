package openapi

// STS 临时凭据（需求 11191）：换证接口签发 2 小时有效的 AK/SK + SecurityToken。
//
// 三要素：`TA_LIVE_`(AK) + `TS_LIVE_`(SK) + `ST_LIVE_`(SecurityToken)，请求都要带
// `X-META-Token`，且 SecurityToken 作为待签串第 7 段参与签名。
//
// 与长期凭据的差别：
//   - 固定 2 小时有效期，不可续期，也不能用 STS 再换 STS（防凭据链无限延长）；
//   - 不落库：Redis `finv:openapi:sts:<AK>`（TTL 2h），SK 加密存放，过期即失效；
//   - 权限继承父凭证（scope / IP 白名单 / 调用方限流），审计记 `auth_mode='STS'` + 父凭证 idn。

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
	"github.com/redis/go-redis/v9"
)

const stsKeyPrefix = "finv:openapi:sts:"

// StsTTL 临时凭据有效期（固定 2 小时，调用方不能指定）。
const StsTTL = 2 * time.Hour

// AuthModeSTS 审计里的认证模式取值。
const AuthModeSTS = "STS"

// StsIssued 换证接口返回的三要素（明文只出现这一次）。
type StsIssued struct {
	AccessKeyID   string    `json:"access_key_id"`
	SecretKey     string    `json:"secret_key"`
	SecurityToken string    `json:"security_token"`
	ExpireAt      time.Time `json:"expire_at"`
}

// stsState Redis 里保存的临时凭据（SK 与库内同口径加密）。
type stsState struct {
	ParentIDN     int64    `json:"parent_idn"`
	ClientIDN     int64    `json:"client_idn"`
	AccessKeyID   string   `json:"access_key_id"`
	SecretCipher  string   `json:"secret_cipher"`
	TokenFP       string   `json:"token_fp"`
	SignAlgorithm string   `json:"sign_algorithm"`
	Scope         []string `json:"scope"`
	AllowCIDR     []string `json:"allow_cidr"`
	RateLimitRPM  int      `json:"rate_limit_rpm"`
	QuotaDay      int      `json:"quota_day"`
	ExpireAt      int64    `json:"expire_at"` // Unix 秒
}

// StsStore STS 临时凭据的签发与查询（Redis）。
type StsStore struct {
	rdb *redis.Client
	now func() time.Time
}

// NewStsStore 创建 STS 存储；rdb 为 nil 时换证接口报「未就绪」。
func NewStsStore(rdb *redis.Client) *StsStore {
	return &StsStore{rdb: rdb, now: time.Now}
}

// TokenFingerprint SecurityToken 指纹（sha256 前 16 位大写十六进制），用于校验与审计。
func TokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return strings.ToUpper(hex.EncodeToString(sum[:]))[:16]
}

// MatchToken 恒定时间比较 token 指纹。
func MatchToken(expectedFP, provided string) bool {
	if expectedFP == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedFP), []byte(TokenFingerprint(provided))) == 1
}

// Issue 用父凭证签发一套临时凭据；只允许长期 AK/SK 凭证换证。
func (s *StsStore) Issue(ctx context.Context, parent *Credential, key []byte) (StsIssued, error) {
	if s.rdb == nil {
		return StsIssued{}, errors.New("STS 未就绪（Redis 未配置）")
	}
	if parent == nil || parent.AuthMode != AuthModeAKSK {
		return StsIssued{}, errors.New("只有 AK/SK 长期凭证可以换取 STS 临时凭据")
	}

	tokenAK, err := secret.RandomToken(16)
	if err != nil {
		return StsIssued{}, err
	}
	tokenSK, err := secret.RandomToken(24)
	if err != nil {
		return StsIssued{}, err
	}
	tokenST, err := secret.RandomToken(24)
	if err != nil {
		return StsIssued{}, err
	}

	plainSK := secret.NormalizeKey(secret.PrefixTempSK + tokenSK)
	plainToken := secret.NormalizeKey(secret.PrefixToken + tokenST)
	cipherText, err := secret.Encrypt(plainSK, key)
	if err != nil {
		return StsIssued{}, err
	}
	expireAt := s.now().Add(StsTTL)
	state := stsState{
		ParentIDN:     parent.IDN,
		ClientIDN:     parent.ClientIDN,
		AccessKeyID:   secret.NormalizeKey(secret.PrefixTempAK + tokenAK),
		SecretCipher:  cipherText,
		TokenFP:       TokenFingerprint(plainToken),
		SignAlgorithm: parent.SignAlgorithm,
		Scope:         scopeToText(parent.Scope),
		AllowCIDR:     ipNetsToText(parent.CredentialIPs),
		RateLimitRPM:  parent.RateLimitRPM,
		QuotaDay:      parent.QuotaDay,
		ExpireAt:      expireAt.Unix(),
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return StsIssued{}, err
	}
	if err := s.rdb.Set(ctx, stsKeyPrefix+state.AccessKeyID, payload, StsTTL).Err(); err != nil {
		return StsIssued{}, fmt.Errorf("写入临时凭据失败: %w", err)
	}
	return StsIssued{
		AccessKeyID:   state.AccessKeyID,
		SecretKey:     plainSK,
		SecurityToken: plainToken,
		ExpireAt:      expireAt,
	}, nil
}

// Lookup 按临时 AK 取回凭据与 token 指纹；不存在/已过期返回 ErrCredentialNotFound。
func (s *StsStore) Lookup(ctx context.Context, accessKeyID string) (*Credential, string, string, error) {
	if s.rdb == nil {
		return nil, "", "", ErrCredentialNotFound
	}
	raw, err := s.rdb.Get(ctx, stsKeyPrefix+strings.TrimSpace(accessKeyID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, "", "", ErrCredentialNotFound
	}
	if err != nil {
		return nil, "", "", err
	}
	var state stsState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, "", "", fmt.Errorf("临时凭据数据损坏: %w", err)
	}
	if s.now().Unix() >= state.ExpireAt {
		return nil, "", "", ErrCredentialNotFound
	}
	expireAt := time.Unix(state.ExpireAt, 0)
	credential := &Credential{
		IDN:           state.ParentIDN, // 审计记父凭证，便于按调用方聚合
		ClientIDN:     state.ClientIDN,
		AuthMode:      AuthModeSTS,
		AccessKeyID:   state.AccessKeyID,
		SecretCipher:  state.SecretCipher,
		SignAlgorithm: state.SignAlgorithm,
		CredentialIPs: parseCIDRs(state.AllowCIDR),
		RateLimitRPM:  state.RateLimitRPM,
		QuotaDay:      state.QuotaDay,
		CredentialExp: &expireAt,
		FlagEnable:    true,
		clientEnable:  true,
		Scope:         parseScopeText(state.Scope),
	}
	return credential, state.TokenFP, state.SecretCipher, nil
}

// ipNetsToText 把 *net.IPNet 列表转回 CIDR 文本（存 Redis 用）。
func ipNetsToText(nets []*net.IPNet) []string {
	out := make([]string, 0, len(nets))
	for _, item := range nets {
		if item != nil {
			out = append(out, item.String())
		}
	}
	return out
}

// scopeToText 把授权范围转成「方法 路径」文本数组。
func scopeToText(rules []ScopeRule) []string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		out = append(out, strings.TrimSpace(rule.Method+" "+rule.Path))
	}
	return out
}

// parseScopeText 把「方法 路径」文本数组还原成 ScopeRule。
func parseScopeText(items []string) []ScopeRule {
	out := make([]ScopeRule, 0, len(items))
	for _, item := range items {
		parts := strings.Fields(item)
		if len(parts) != 2 {
			continue
		}
		out = append(out, ScopeRule{Method: parts[0], Path: parts[1]})
	}
	return out
}

// authenticateSTS 校验 STS 临时凭据（三要素）：凭据存在/未过期 → token → 状态/IP →
// 时间窗 → nonce → 签名（待签串含 token）。
func (m *Middleware) authenticateSTS(settings Settings, req *http.Request, body []byte,
	invocation *Invocation, algorithm, accessKeyID, signature string) (*Credential, int, string) {

	invocation.AuthMode = AuthModeSTS
	token := strings.TrimSpace(req.Header.Get(HeaderToken))
	if token == "" {
		return nil, CodeKeyInvalid, "STS 临时凭据必须携带 " + HeaderToken
	}
	invocation.TokenFP = TokenFingerprint(token)

	credential, tokenFP, cipherText, err := m.sts.Lookup(req.Context(), accessKeyID)
	if errors.Is(err, ErrCredentialNotFound) {
		return nil, CodeKeyInvalid, "STS 临时凭据不存在或已过期"
	}
	if err != nil {
		return nil, CodeAuthNotReady, "查询 STS 临时凭据失败: " + err.Error()
	}
	if !MatchToken(tokenFP, token) {
		return nil, CodeKeyInvalid, "SecurityToken 不匹配"
	}
	if !strings.EqualFold(strings.TrimSpace(credential.SignAlgorithm), algorithm) {
		return credential, CodeSignMismatch,
			"签名算法不一致（请求 " + algorithm + "，凭据 " + credential.SignAlgorithm + "）"
	}
	if ok, reason := credential.Active(m.now()); !ok {
		return credential, CodeCredentialStop, "临时凭据不可用（" + reason + "）"
	}
	if !credential.AllowIP(invocation.IP) {
		return credential, CodeCIDRDeny, "来源 IP 不在白名单"
	}

	timestamp := strings.TrimSpace(req.Header.Get(HeaderTimestamp))
	nonce := strings.TrimSpace(req.Header.Get(HeaderNonce))
	ts, err := ParseTimestamp(timestamp)
	if err != nil || nonce == "" {
		return credential, CodeKeyInvalid, "缺少 " + HeaderTimestamp + " 或 " + HeaderNonce
	}
	if !noncePattern.MatchString(nonce) {
		return credential, CodeKeyInvalid, "nonce 必须为 8~64 位字母/数字/_/-"
	}
	skew := m.now().Sub(time.Unix(ts, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > settings.SignTimeWindow {
		return credential, CodeTimestampSkew, "时间戳偏差 " + skew.String()
	}

	// nonce 按临时 AK 隔离，避免与长期凭证互相占坑
	fresh, err := m.redis.AcquireNonce(req.Context(), "sts:"+TokenFingerprint(accessKeyID),
		nonce, settings.NonceKeepWindow)
	if err != nil {
		m.logf("[openapi] nonce 检查失败（fail-close=%t）: %v", m.redisFailClose, err)
		if m.redisFailClose {
			return credential, CodeAuthNotReady, "防重放状态不可用（Redis 故障），请稍后重试"
		}
	} else if !fresh {
		return credential, CodeNonceReplay, "nonce 重复"
	}

	key, err := settings.SecretKeyBytes()
	if err != nil {
		return credential, CodeAuthNotReady, err.Error()
	}
	plainSK, err := secret.Decrypt(cipherText, key)
	if err != nil {
		return credential, CodeAuthNotReady, "解密临时 SK 失败: " + err.Error()
	}

	canonical := CanonicalString(SignRequest{
		Method:    req.Method,
		Path:      req.URL.Path,
		RawQuery:  req.URL.RawQuery,
		BodyHash:  BodyHash(body),
		Timestamp: timestamp,
		Nonce:     nonce,
		Token:     token,
	})
	if !VerifySignature(credential.SignAlgorithm, plainSK, canonical, signature) {
		return credential, CodeSignMismatch, "签名不匹配"
	}
	return credential, 0, ""
}
