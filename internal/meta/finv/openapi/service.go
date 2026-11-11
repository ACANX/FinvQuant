package openapi

// 服务层：在 Store 之上补业务规则（签发密钥、状态流转、参数校验）。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

// Service 「OpenAPI 调用方管理」服务。
type Service struct {
	store *Store
}

// NewService 创建服务。
func NewService(store *Store) *Service { return &Service{store: store} }

// Settings 读鉴权配置（不含密钥明文）。
func (s *Service) Settings(ctx context.Context) (SettingsView, error) {
	return s.store.LoadSettings(ctx)
}

// SaveSettings 保存 19001～19003；模式与数值都做范围校验。
func (s *Service) SaveSettings(ctx context.Context, req SettingsRequest) (SettingsView, error) {
	mode, err := normalizeMode(req.Mode)
	if err != nil {
		return SettingsView{}, err
	}
	if req.SignTimeWindow < 30 || req.SignTimeWindow > 3600 {
		return SettingsView{}, invalid("sign_time_window 需在 30~3600 秒之间，实际 %d", req.SignTimeWindow)
	}
	if req.LogRetainDays < 0 || req.LogRetainDays > 3650 {
		return SettingsView{}, invalid("log_retain_days 需在 0~3650 天之间（0 表示不清理），实际 %d", req.LogRetainDays)
	}
	if err := s.store.SaveSettings(ctx, SettingsRequest{
		Mode: mode, SignTimeWindow: req.SignTimeWindow, LogRetainDays: req.LogRetainDays,
	}); err != nil {
		return SettingsView{}, err
	}
	return s.store.LoadSettings(ctx)
}

// ClientPage 分页查询调用方。
func (s *Service) ClientPage(ctx context.Context, keyword string, page, pageSize int) ([]Client, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.store.ClientPage(ctx, keyword, page, pageSize)
}

// ClientDetail 单个调用方。
func (s *Service) ClientDetail(ctx context.Context, idn int64) (Client, error) {
	if idn <= 0 {
		return Client{}, invalid("idn 必须为正整数")
	}
	client, err := s.store.ClientByID(ctx, idn)
	if errors.Is(err, ErrNotFound) {
		return Client{}, fmt.Errorf("%w: 调用方 %d 不存在", ErrNotFound, idn)
	}
	return client, err
}

// SaveClient 新建（idn=0）或更新调用方。
//
// 新建默认 `flag_enable='0'`（停用）：先登记、核对白名单与配额，再显式启用。
func (s *Service) SaveClient(ctx context.Context, req ClientRequest, operator string) (Client, error) {
	// 新增时「负责人账号」默认当前登录账号、「联系邮箱」默认取该账号的资料，
	// 免得每个调用方都要手填（需求 11174）；调用方显式传值时以传入值优先。
	if strings.TrimSpace(req.OwnerUID) == "" {
		req.OwnerUID = operator
	}
	if strings.TrimSpace(req.ContactEmail) == "" && strings.TrimSpace(req.OwnerUID) != "" {
		if email, err := s.store.UserEmailByUID(ctx, req.OwnerUID); err == nil {
			req.ContactEmail = email
		}
	}

	validated, err := validateClient(req)
	if err != nil {
		return Client{}, err
	}
	if validated.IDN <= 0 {
		client, err := s.store.InsertClient(ctx, validated, operator)
		if err != nil {
			if isUniqueViolation(err, "uk_open_api_client_code") {
				return Client{}, fmt.Errorf("%w: client_code=%s 已存在", ErrConflict, validated.ClientCode)
			}
			return Client{}, err
		}
		return client, nil
	}
	client, err := s.store.UpdateClient(ctx, validated, operator)
	if errors.Is(err, ErrNotFound) {
		return Client{}, fmt.Errorf("%w: 调用方 %d 不存在", ErrNotFound, validated.IDN)
	}
	if err != nil {
		if isUniqueViolation(err, "uk_open_api_client_code") {
			return Client{}, fmt.Errorf("%w: client_code=%s 已被其它调用方占用", ErrConflict, validated.ClientCode)
		}
		return Client{}, err
	}
	return client, nil
}

// SetClientEnable 启用/停用调用方。
func (s *Service) SetClientEnable(ctx context.Context, idn int64, enable bool, operator string) (Client, error) {
	if idn <= 0 {
		return Client{}, invalid("idn 必须为正整数")
	}
	client, err := s.store.SetClientEnable(ctx, idn, enable, operator)
	if errors.Is(err, ErrNotFound) {
		return Client{}, fmt.Errorf("%w: 调用方 %d 不存在", ErrNotFound, idn)
	}
	return client, err
}

// CredentialPage 分页查询凭证。
func (s *Service) CredentialPage(ctx context.Context, clientIDN int64, keyword string, page, pageSize int) ([]Credential, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.store.CredentialPage(ctx, clientIDN, keyword, page, pageSize)
}

// CreateCredential 签发凭证。
//
// 返回体里的密钥明文**只出现这一次**：库里只存哈希（静态 Key）或密文（SK）。
// 静态 Key 的明文同时就是 `access_key_id`（调用方要用它标识自己）。
func (s *Service) CreateCredential(ctx context.Context, req CredentialCreateRequest, operator string) (IssuedSecret, error) {
	validated, err := validateCredential(req)
	if err != nil {
		return IssuedSecret{}, err
	}
	client, err := s.store.ClientByID(ctx, validated.ClientIDN)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return IssuedSecret{}, fmt.Errorf("%w: 调用方 %d 不存在", ErrNotFound, validated.ClientIDN)
		}
		return IssuedSecret{}, err
	}
	// 静态 Key 的调用额度必须受限（需求 11188）：未配 rate_limit_rpm 也未配 quota_day 时拒绝签发，
	// 避免「一把无签名、无上限的 Key」。AK_SK 不受此限制（可签名、可精确授权到接口）。
	if validated.AuthMode == AuthModeAPIKey && client.RateLimitRPM <= 0 && client.QuotaDay <= 0 {
		return IssuedSecret{}, invalid("静态 Key 模式要求调用方先配置调用额度（rate_limit_rpm 或 quota_day 至少一个 > 0）")
	}

	issued, err := s.buildSecret(ctx, validated.AuthMode)
	if err != nil {
		return IssuedSecret{}, err
	}
	validated.AccessKey = issued.AccessKey

	idn, err := s.store.InsertCredential(ctx, validated, issued.SecretHash, issued.SecretCipher,
		issued.KeyPrefix, issued.KID, operator)
	if err != nil {
		if isUniqueViolation(err, "uk_open_api_credential_no") {
			return IssuedSecret{}, fmt.Errorf("%w: credential_no=%s 已存在", ErrConflict, validated.CredentialNo)
		}
		if isUniqueViolation(err, "uk_open_api_credential_access_key") {
			return IssuedSecret{}, fmt.Errorf("%w: 生成的公开标识与已有凭证冲突，请重试", ErrConflict)
		}
		return IssuedSecret{}, err
	}

	credential, err := s.store.CredentialByID(ctx, idn)
	if err != nil {
		return IssuedSecret{}, err
	}
	return IssuedSecret{
		Credential: credential,
		AccessKey:  issued.AccessKey,
		SecretKey:  issued.SecretKey,
		Note:       issuedNote(validated.AuthMode),
	}, nil
}

// RotateCredential 轮换密钥（旧 Key 立即失效；AK/SK 允许旧 SK 在窗口内并行验签）。
func (s *Service) RotateCredential(ctx context.Context, idn int64, operator string) (IssuedSecret, error) {
	if idn <= 0 {
		return IssuedSecret{}, invalid("idn 必须为正整数")
	}
	current, err := s.store.CredentialByID(ctx, idn)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return IssuedSecret{}, fmt.Errorf("%w: 凭证 %d 不存在", ErrNotFound, idn)
		}
		return IssuedSecret{}, err
	}
	if current.FlagRevoke == "1" {
		return IssuedSecret{}, fmt.Errorf("%w: 凭证已吊销，不能轮换（请新建凭证）", ErrConflict)
	}

	issued, err := s.buildSecret(ctx, current.AuthMode)
	if err != nil {
		return IssuedSecret{}, err
	}
	if err := s.store.RotateCredential(ctx, idn, current.AuthMode, issued.AccessKey,
		issued.SecretHash, issued.SecretCipher, issued.KeyPrefix, issued.KID, operator); err != nil {
		return IssuedSecret{}, err
	}
	credential, err := s.store.CredentialByID(ctx, idn)
	if err != nil {
		return IssuedSecret{}, err
	}
	// AK/SK 模式轮换只换 SK，AK 保持原值（它是身份标识）；API_KEY 模式的公开标识本身就是密钥，会变。
	// 因此这里回读库里的 access_key_id，而不是返回刚生成的那个——否则响应里的 AK 根本不存在于库中。
	return IssuedSecret{
		Credential: credential,
		AccessKey:  credential.AccessKeyID,
		SecretKey:  issued.SecretKey,
		Note:       "轮换完成：" + issuedNote(current.AuthMode),
	}, nil
}

// SetCredentialEnable 启用/停用凭证。
func (s *Service) SetCredentialEnable(ctx context.Context, idn int64, enable bool, operator string) (Credential, error) {
	if idn <= 0 {
		return Credential{}, invalid("idn 必须为正整数")
	}
	credential, err := s.store.SetCredentialEnable(ctx, idn, enable, operator)
	if errors.Is(err, ErrNotFound) {
		return Credential{}, fmt.Errorf("%w: 凭证 %d 不存在", ErrNotFound, idn)
	}
	return credential, err
}

// RevokeCredential 吊销凭证（原因必填，不可恢复）。
func (s *Service) RevokeCredential(ctx context.Context, idn int64, reason, operator string) (Credential, error) {
	if idn <= 0 {
		return Credential{}, invalid("idn 必须为正整数")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Credential{}, invalid("吊销必须填写原因")
	}
	if len([]rune(reason)) > 500 {
		return Credential{}, invalid("吊销原因不能超过 500 字符")
	}
	credential, err := s.store.RevokeCredential(ctx, idn, reason, operator)
	if errors.Is(err, ErrNotFound) {
		return Credential{}, fmt.Errorf("%w: 凭证 %d 不存在", ErrNotFound, idn)
	}
	return credential, err
}

// InvokeLogPage 分页查询调用审计。
func (s *Service) InvokeLogPage(ctx context.Context, filter InvokeLogFilter) ([]InvokeLog, int, error) {
	if filter.FlagPass != "" && filter.FlagPass != "0" && filter.FlagPass != "1" {
		return nil, 0, invalid("flag_pass 只能是 0（拒绝）或 1（放行）")
	}
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	return s.store.InvokeLogPage(ctx, filter)
}

// ---------------- 内部 ----------------

// issuedSecret 服务层内部的密钥材料（不含明文以外的展示信息）。
type issuedSecret struct {
	AccessKey    string
	SecretKey    string
	SecretHash   string
	SecretCipher string
	KeyPrefix    string
	KID          string
}

// buildSecret 生成密钥材料（英文字母一律大写，需求 11176）。
//
//   - API_KEY：access_key_id 即静态 Key（`SA_LIVE_` + 32 位十六进制大写），库里只存 sha256；
//   - AK_SK：AK 为 `AK_LIVE_` + 32 位十六进制大写，SK 为 `SK_LIVE_` + 48 位十六进制大写，
//     库里存 AES-256-GCM 密文。
func (s *Service) buildSecret(ctx context.Context, authMode string) (issuedSecret, error) {
	// 十六进制随机数本身是小写，统一转大写（需求 11176）
	random, err := secret.RandomToken(16)
	if err != nil {
		return issuedSecret{}, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	random = secret.NormalizeKey(random)

	if authMode == AuthModeAPIKey {
		key := secret.PrefixAPIKey + random
		return issuedSecret{
			AccessKey:  key,
			SecretKey:  key,
			SecretHash: secret.HashKey(key),
			KeyPrefix:  keyPrefixOf(key),
		}, nil
	}

	skToken, err := secret.RandomToken(24)
	if err != nil {
		return issuedSecret{}, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	sk := secret.PrefixSK + secret.NormalizeKey(skToken)

	base64Key, err := s.store.SecretKeyBase64(ctx)
	if err != nil {
		return issuedSecret{}, err
	}
	keyBytes, err := secret.DecodeKey(base64Key)
	if err != nil {
		return issuedSecret{}, fmt.Errorf("%w: %v（请先在系统配置里填写 19005）", ErrNotReady, err)
	}
	cipherText, err := secret.Encrypt(sk, keyBytes)
	if err != nil {
		return issuedSecret{}, fmt.Errorf("加密 SK 失败: %w", err)
	}
	kid, err := s.store.SecretKID(ctx)
	if err != nil {
		return issuedSecret{}, err
	}
	return issuedSecret{
		AccessKey:    secret.PrefixAK + random,
		SecretKey:    sk,
		SecretCipher: cipherText,
		KeyPrefix:    keyPrefixOf(secret.PrefixAK + random),
		KID:          kid,
	}, nil
}

// keyPrefixOf 取前 12 位作为展示前缀（列宽 varchar(12)）。
func keyPrefixOf(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

// issuedNote 签发提示语。
func issuedNote(authMode string) string {
	switch authMode {
	case AuthModeAPIKey:
		return "静态 Key 只显示这一次：请立即复制保存（库里不存明文，丢失只能轮换）"
	case AuthModeSTS:
		// STS 模式的 AK/SK 是「兑换券」：不能直接调业务接口，只能换 2 小时临时凭据（需求 11210）
		return "这对 AK/SK 是 STS 兑换凭证，只能调用 POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue 换取 2 小时有效的 SAK/SSK/SST，不能用它直接调业务接口。SK 只显示这一次，请立即保存"
	default:
		return "SK 只显示这一次：请立即复制保存（库里只存密文，丢失只能轮换）；请求用 Authorization: META1-<算法> Credential=<AK>, Signature=<签名> + X-META-TS + X-META-Nonce"
	}
}

// normalizePage 归一页码与页大小（1 ≤ page，1 ≤ pageSize ≤ 200）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

// isUniqueViolation 判断是否指定约束的唯一键冲突。
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// ---------------- 密钥配置（需求 11175）----------------

// SecretView 密钥配置视图（**不含任何密钥明文**）。
type SecretView struct {
	SecretKID         string `json:"secret_kid"`
	SecretKeyReady    bool   `json:"secret_key_ready"`
	FallbackKeyReady  bool   `json:"fallback_key_ready"`
	CountCredential   int    `json:"count_credential"`
	CountStaleKID     int    `json:"count_stale_kid"`
	UpdatedCredential int    `json:"updated_credential"`
	SkippedCredential int    `json:"skipped_credential"`
}

// SecretSaveRequest 保存密钥配置。
//
// 三个值都是「空 = 不改」；secret_key_base64 非空时表示要更换加密密钥，
// rotate_cipher=true 时会把现有凭证密文用新密钥重加密（否则那些凭证会解不开）。
type SecretSaveRequest struct {
	SecretKID        string `json:"secret_kid"`
	SecretKeyBase64  string `json:"secret_key_base64"`
	RotateCipher     bool   `json:"rotate_cipher"`
	FallbackKey      string `json:"fallback_key"`
	ClearFallbackKey bool   `json:"clear_fallback_key"`
}

const (
	// MinSecretKeyBytes / MaxSecretKeyBytes AES-256 固定 32 字节。
	secretKeyBytes = 32
	// maxFallbackKeyLen 兜底密钥长度上限（只是字符串比对，不参与密码学）。
	maxFallbackKeyLen = 128
)

// SecretStatus 读密钥配置现状。
func (s *Service) SecretStatus(ctx context.Context) (SecretView, error) {
	state, err := s.store.LoadSecretState(ctx)
	if err != nil {
		return SecretView{}, err
	}
	return SecretView{
		SecretKID:        state.SecretKID,
		SecretKeyReady:   strings.TrimSpace(state.SecretKeyBase64) != "",
		FallbackKeyReady: strings.TrimSpace(state.FallbackKey) != "",
		CountCredential:  state.CountCredential,
		CountStaleKID:    state.CountCredentialStale,
	}, nil
}

// SaveSecret 保存 19004 / 19005 / 19006，并在换密钥时按需重加密现有凭证密文。
func (s *Service) SaveSecret(ctx context.Context, req SecretSaveRequest) (SecretView, error) {
	state, err := s.store.LoadSecretState(ctx)
	if err != nil {
		return SecretView{}, err
	}

	// KID
	var kidPtr *string
	if kid := strings.TrimSpace(req.SecretKID); kid != "" {
		if len(kid) > 16 || !kidPattern.MatchString(kid) {
			return SecretView{}, invalid("secret_kid 只允许字母/数字/._-，1~16 位")
		}
		kidPtr = &kid
	}
	newKID := state.SecretKID
	if kidPtr != nil {
		newKID = *kidPtr
	}

	// 兜底密钥
	var fallbackPtr *string
	if req.ClearFallbackKey {
		empty := ""
		fallbackPtr = &empty
	} else if key := strings.TrimSpace(req.FallbackKey); key != "" {
		if len(key) > maxFallbackKeyLen {
			return SecretView{}, invalid("兜底密钥不能超过 %d 字符", maxFallbackKeyLen)
		}
		fallbackPtr = &key
	}

	// 加密密钥
	var (
		secretPtr *string
		newKey    []byte
		oldKey    []byte
	)
	if raw := strings.TrimSpace(req.SecretKeyBase64); raw != "" {
		decoded, err := secret.DecodeKey(raw)
		if err != nil {
			return SecretView{}, invalid("%v", err)
		}
		newKey = decoded
		secretPtr = &raw
		if old := strings.TrimSpace(state.SecretKeyBase64); old != "" {
			if decodedOld, err := secret.DecodeKey(old); err == nil {
				oldKey = decodedOld
			}
		}
	}

	// 「重加密密文」与「写入新密钥」必须原子完成：只换一半会造成密文与密钥不匹配的死局。
	updated, skipped := 0, 0
	tx, err := s.store.Begin(ctx)
	if err != nil {
		return SecretView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if newKey != nil && oldKey != nil && state.CountCredential > 0 {
		updated, skipped, err = s.store.ReencryptCredentials(ctx, tx, oldKey, newKey, newKID)
		if err != nil {
			return SecretView{}, err
		}
	}
	if err := s.store.SaveSecretSQL(ctx, tx, kidPtr, secretPtr, fallbackPtr); err != nil {
		return SecretView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SecretView{}, err
	}
	if newKey != nil && oldKey == nil && state.CountCredential > 0 {
		// 首次配置密钥（此前没有密钥）时不该已有密文；若有，说明数据异常，明确报出来让人工处理
		skipped = state.CountCredential
	}

	view, err := s.SecretStatus(ctx)
	if err != nil {
		return SecretView{}, err
	}
	view.UpdatedCredential = updated
	view.SkippedCredential = skipped
	return view, nil
}
