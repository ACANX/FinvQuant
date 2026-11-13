package openapi

// STS 临时凭据（需求 11191 / 11202）。
//
// 设计（需求 11210 定稿）：`auth_mode='STS'` 下面有**两类行**，靠 `token_fp` 区分——
//   - 兑换凭证（`token_fp IS NULL`）：管理员签发的长期 AK/SK（`AK_LIVE_`/`SK_LIVE_`），
//     **只能**调用 `POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue`，不能直接调业务接口；
//   - 临时凭据（`token_fp IS NOT NULL`）：用兑换凭证换来的三要素（SAK/SSK/SST），有效期 2 小时，
//     `parent_idn` 指向兑换凭证、`dt_expire`=签发时刻+2h，**只能**调业务接口、不能再换证。
// 两类凭证与 AKSK 模式三者互相隔离：AKSK 不能派生临时凭据，兑换凭证不能调业务接口。

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

// StsTTL 临时凭据有效期（固定 2 小时，调用方不能指定）。
const StsTTL = 2 * time.Hour

// AuthModeSTS 认证模式取值（与迁移 V300190 的 CHECK 一致）。
const AuthModeSTS = "STS"

// stsPurgeKeep 过期 STS 行的保留期（超过即物理删除，便于事后追溯一段时间）。
const stsPurgeKeep = 7 * 24 * time.Hour

// StsIssued 换证结果（三要素明文只出现这一次）。
type StsIssued struct {
	CredentialIDN int64     `json:"credential_idn"`
	CredentialNo  string    `json:"credential_no"`
	AccessKeyID   string    `json:"access_key_id"`
	SecretKey     string    `json:"secret_key"`
	SecurityToken string    `json:"security_token"`
	ExpireAt      time.Time `json:"expire_at"`
}

// StsStore STS 临时凭据的签发（落库）+ 过期清理。
type StsStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewStsStore 创建 STS 存储。
func NewStsStore(pool *pgxpool.Pool) *StsStore {
	return &StsStore{pool: pool, now: time.Now}
}

// TokenFingerprint SecurityToken 指纹（sha256 前 16 位大写十六进制）。
func TokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return strings.ToUpper(fmt.Sprintf("%x", sum[:]))[:16]
}

// MatchToken 恒定时间比较 token 指纹。
func MatchToken(expectedFP, provided string) bool {
	if expectedFP == "" || provided == "" {
		return false
	}
	expected := strings.ToUpper(strings.TrimSpace(expectedFP))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(TokenFingerprint(provided))) == 1
}

// Issue 用兑换凭证换一套 STS 三要素并落库。
//
// parent 必须是 `auth_mode='STS'` 的兑换凭证（`token_fp` 为空）；三要素的授权范围继承兑换凭证的
// `scope_str`（留空 = 全部接口），有效期固定 StsTTL。
// 三要素前缀（需求 11194）：SAK_LIVE_ / SSK_LIVE_ / SST_LIVE_。
func (s *StsStore) Issue(ctx context.Context, parent *Credential, key []byte, operator string) (StsIssued, error) {
	if s.pool == nil {
		return StsIssued{}, errors.New("STS 未就绪（数据库连接池未初始化）")
	}
	if parent == nil || parent.ClientIDN <= 0 {
		return StsIssued{}, errors.New("签发 STS 临时凭据需要指定调用方")
	}

	randomAK, err := secret.RandomToken(16)
	if err != nil {
		return StsIssued{}, err
	}
	randomSK, err := secret.RandomToken(24)
	if err != nil {
		return StsIssued{}, err
	}
	randomToken, err := secret.RandomToken(24)
	if err != nil {
		return StsIssued{}, err
	}

	sak := secret.NormalizeKey(secret.PrefixTempAK + randomAK)
	ssk := secret.NormalizeKey(secret.PrefixTempSK + randomSK)
	token := secret.NormalizeKey(secret.PrefixToken + randomToken)
	cipherText, err := secret.Encrypt(ssk, key)
	if err != nil {
		return StsIssued{}, err
	}

	expireAt := s.now().Add(StsTTL)
	credentialNo := fmt.Sprintf("STS-%d-%s", parent.ClientIDN, expireAt.Format("20060102T150405"))
	scopeJSON := scopeToJSONText(parent.Scope)
	cidrText := ipNetsToText(parent.CredentialIPs)

	// 需求 11228：凭证行与授权范围明细必须原子写入（鉴权读明细表，缺一行就是全部放行）
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return StsIssued{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var idn int64
	err = tx.QueryRow(ctx, `
INSERT INTO open.open_api_credential
    (client_idn, credential_no, auth_mode, access_key_id, key_prefix,
     secret_cipher, secret_kid, sign_algorithm, scope_str, allow_cidr,
     dt_expire, token_fp, parent_idn, flag_enable, flag_revoke, remark, create_by)
VALUES ($1, $2, 'STS', $3, $4,
        $5, NULL, 'HMAC-SHA256', $6::jsonb, $7::cidr[],
        $8, $9, $10, '1', '0', $11, NULLIF($12, ''))
RETURNING idn`,
		parent.ClientIDN, credentialNo, sak, strings.TrimSuffix(secret.PrefixTempAK, "LIVE_"),
		cipherText, scopeJSON, cidrText,
		expireAt, TokenFingerprint(token), parent.IDN, "STS 临时凭据（2 小时，由兑换凭证派生）", operator,
	).Scan(&idn)
	if err != nil {
		return StsIssued{}, fmt.Errorf("写入 STS 凭证失败: %w", err)
	}
	if err := insertScopeRows(ctx, tx, idn, parent.Scope); err != nil {
		return StsIssued{}, fmt.Errorf("写入 STS 授权范围失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return StsIssued{}, fmt.Errorf("提交 STS 凭证失败: %w", err)
	}

	// 顺带清理过期很久的 STS 行（失败不影响签发）
	if _, purgeErr := s.PurgeExpired(ctx); purgeErr != nil {
		// 不返回错误：清理是维护动作
		_ = purgeErr
	}

	return StsIssued{
		CredentialIDN: idn,
		CredentialNo:  credentialNo,
		AccessKeyID:   sak,
		SecretKey:     ssk,
		SecurityToken: token,
		ExpireAt:      expireAt,
	}, nil
}

// insertScopeRows 把父凭证的授权范围复制给新签发的临时凭据（需求 11228）。
//
// 语义与父凭证一致：0 行 = 不限接口；`*` 表任意方法。
func insertScopeRows(ctx context.Context, tx pgx.Tx, credentialIDN int64, rules []ScopeRule) error {
	if len(rules) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		if method == "" {
			method = "*"
		}
		path := strings.TrimSpace(rule.Path)
		if !strings.HasPrefix(path, "/") {
			continue
		}
		batch.Queue(`
INSERT INTO open.open_api_credential_scope (credential_idn, method, path_pattern, create_by)
VALUES ($1, $2, $3, 'sts')
ON CONFLICT (credential_idn, method, path_pattern) DO NOTHING`, credentialIDN, method, path)
	}
	if batch.Len() == 0 {
		return nil
	}
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// PurgeExpired 物理删除过期超过保留期的 STS **临时凭据**行。
//
// 条件三重收敛：`auth_mode='STS'` + `token_fp IS NOT NULL`（排除兑换凭证）+ `dt_expire < now()-7天`。
// 兑换凭证是长期凭证（`token_fp` 为空），过期/停用由 `Active()` 判定，绝不在这里删。
func (s *StsStore) PurgeExpired(ctx context.Context) (int64, error) {
	if s.pool == nil {
		return 0, nil
	}
	// 需求 11228：先删授权范围明细、再删凭证行，同一事务——
	// 否则明细会变成孤儿行（只增不减），且接口清单页的「已授权凭证」会虚高。
	interval := fmt.Sprintf("%d hours", int(stsPurgeKeep.Hours()))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
DELETE FROM open.open_api_credential_scope
 WHERE credential_idn IN (
       SELECT idn FROM open.open_api_credential
        WHERE auth_mode = 'STS'
          AND token_fp IS NOT NULL
          AND dt_expire IS NOT NULL
          AND dt_expire < now() - $1::interval)`, interval); err != nil {
		return 0, err
	}

	tag, err := tx.Exec(ctx, `
DELETE FROM open.open_api_credential
 WHERE auth_mode = 'STS'
   AND token_fp IS NOT NULL
   AND dt_expire IS NOT NULL
   AND dt_expire < now() - $1::interval`, interval)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// stsParentUsable 父凭证是否仍可用于已派生的临时凭据（需求 11197）。
//
// 父凭证状态随主查询 LEFT JOIN 带出（ParentFound / ParentFlagEnable / ParentFlagRevoke / ParentExp），
// 判定口径与 Credential.Active 的「吊销优先」一致：吊销 → REVOKE，其余 → EXPIRE。
func stsParentUsable(credential *Credential, now time.Time) (bool, string) {
	if credential == nil || credential.ParentIDN <= 0 || !credential.ParentFound {
		return false, ReasonExpire
	}
	if credential.ParentFlagRevoke {
		return false, ReasonRevoke
	}
	if !credential.ParentFlagEnable {
		return false, ReasonExpire
	}
	if credential.ParentExp != nil && !now.Before(*credential.ParentExp) {
		return false, ReasonExpire
	}
	return true, ""
}

// authenticateSTS 校验 STS 临时凭据：查库 → token → 状态/过期 → 父凭证 → IP → 时间窗 → nonce → 签名。
func (m *Middleware) authenticateSTS(settings Settings, req *http.Request, body []byte,
	invocation *Invocation, algorithm, accessKeyID, signature string) (*Credential, int, string) {

	invocation.AuthMode = AuthModeSTS
	invocation.KFP = FingerprintStaticKey(accessKeyID)
	token := strings.TrimSpace(req.Header.Get(HeaderToken))
	if token == "" {
		return nil, CodeKeyInvalid, "STS 临时凭据必须携带 " + HeaderToken
	}
	invocation.TFP = TokenFingerprint(token)

	credential, err := m.source.Lookup(req.Context(), accessKeyID)
	if errors.Is(err, ErrCredentialNotFound) {
		return nil, CodeKeyInvalid, "STS 临时凭据不存在或已过期"
	}
	if err != nil {
		return nil, CodeAuthNotReady, "查询 STS 临时凭据失败: " + err.Error()
	}
	if credential.AuthMode != AuthModeSTS {
		return nil, CodeKeyInvalid, "该公开标识不是 STS 临时凭据"
	}
	if !MatchToken(credential.TokenFP, token) {
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

	// 父凭证吊销/停用/过期 → 派生凭据立即失效（需求 11197）。
	// 父凭证状态由主查询 LEFT JOIN 一并带出，不再二次查表；页面直发的 STS 没有父凭证。
	if credential.ParentIDN > 0 {
		if usable, reason := stsParentUsable(credential, m.now()); !usable {
			return credential, CodeCredentialStop, "父凭证已不可用（" + reason + "），临时凭据随之失效"
		}
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
	plainSK, err := secret.Decrypt(credential.SecretCipher, key)
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

// scopeToJSONText 把 ScopeRule 列表转成 scope_str 期望的 JSON 文本。
func scopeToJSONText(rules []ScopeRule) string {
	if len(rules) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		method := strings.TrimSpace(rule.Method)
		path := strings.TrimSpace(rule.Path)
		if path == "" {
			continue
		}
		if method == "" {
			method = "*"
		}
		parts = append(parts, fmt.Sprintf(`{"method":%q,"path":%q}`, method, path))
	}
	if len(parts) == 0 {
		return "[]"
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ipNetsToText 把 *net.IPNet 列表转成 CIDR 文本。
func ipNetsToText(nets []*net.IPNet) []string {
	out := make([]string, 0, len(nets))
	for _, item := range nets {
		if item != nil {
			out = append(out, item.String())
		}
	}
	return out
}
