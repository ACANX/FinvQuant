package openapi

// 凭证读取缓存（需求 11212）。
//
// 问题：每个外部请求都要查一次 `open.open_api_credential`（JOIN 调用方与父凭证），
// 高频调用时压力全落在 PG 上，而凭证行是「写少读多」的典型。
//
// 方案：Redis 缓存整条鉴权所需的凭证快照，键里带**全局版本号**：
//
//	finv:openapi:credver               → 版本号（整数，缺失按 0）
//	finv:openapi:cred:<ver>:<AK>       → 凭证快照 JSON，TTL 5 分钟
//
// 任何凭证/调用方写操作后把版本号 `INCR`，旧版本的键再也读不到 → 吊销、停用、轮换、
// 改额度/白名单都能**立即生效**，不依赖 TTL 到期，也不需要枚举删除。
//
// 失败姿态：Redis 不可用时直接回源查库（缓存是性能优化，不是正确性依赖）；
// 回源结果写缓存失败也只记日志，不影响本次鉴权。

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// 凭证缓存键与默认 TTL。
const (
	credVersionKey   = "finv:openapi:credver"
	credCacheKeyBase = "finv:openapi:cred:"
	// DefaultCredCacheTTL 凭证快照的默认存活时间（需求 11214）。
	//
	// 实际取值由 FINV_OPENAPI_CRED_CACHE_TTL 配置（秒），0 表示关闭缓存。
	// 平台侧吊销/停用/轮换会 `INCR` 版本号立即失效；TTL 只兜底「绕过平台直接改库」的生效延迟，
	// 同时决定稳态查库频率（每个活跃凭证每 TTL 最多 1 次）。
	DefaultCredCacheTTL = 10 * time.Second
	// MaxCredCacheTTL 配置上限（600 秒），避免误配成几小时导致手工改库长时间不生效。
	MaxCredCacheTTL = 600 * time.Second
)

// credentialCacheClient 凭证缓存用到的 Redis 能力（*redis.Client 满足；测试可注入替身）。
type credentialCacheClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
}

// CachedSource 给 CredentialSource 套一层 Redis 缓存。
type CachedSource struct {
	next CredentialSource
	rdb  credentialCacheClient
	ttl  time.Duration
	logf func(format string, args ...any)
	// group 合并同一 AK 的并发回源：TTL 刚到期时避免 N 个并发请求同时查库（缓存击穿）。
	group singleflight.Group
}

// NewCachedSource 创建带缓存的凭证来源；rdb 为 nil 时退化为直通（不缓存）。
//
// 注意：这里收 `*redis.Client` 而不是接口——未注入 Redis 时（单测/本地裸跑）传进来的
// typed nil 若直接装进接口会变成「非 nil 接口」，调用方法会 panic。
func NewCachedSource(next CredentialSource, rdb *redis.Client, ttl time.Duration, logf func(format string, args ...any)) *CachedSource {
	var client credentialCacheClient
	if rdb != nil && ttl > 0 {
		client = rdb
	}
	return newCachedSource(next, client, ttl, logf)
}

// newCachedSource 内部构造（测试可直接注入替身）。
func newCachedSource(next CredentialSource, client credentialCacheClient, ttl time.Duration, logf func(format string, args ...any)) *CachedSource {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if ttl <= 0 {
		ttl = DefaultCredCacheTTL
	}
	return &CachedSource{next: next, rdb: client, ttl: ttl, logf: logf}
}

// Lookup 先查缓存，未命中回源并写入缓存。
//
// 两种结果都缓存 10 秒：命中凭证、以及「AK 不存在」——后者避免有人拿乱码 AK 刷接口
// 把每个请求都打成一次全表查询。
func (s *CachedSource) Lookup(ctx context.Context, accessKeyID string) (*Credential, error) {
	accessKeyID = strings.TrimSpace(accessKeyID)
	if s.rdb == nil || accessKeyID == "" {
		return s.next.Lookup(ctx, accessKeyID)
	}

	// 版本号读取失败（Redis 故障）→ 回源，不把缓存故障变成鉴权故障
	version, ok := s.version(ctx)
	if !ok {
		return s.next.Lookup(ctx, accessKeyID)
	}
	key := credCacheKeyBase + version + ":" + accessKeyID
	if data, err := s.rdb.Get(ctx, key).Bytes(); err == nil {
		if credential, found, decodeOK := decodeCachedCredential(data); decodeOK {
			if !found {
				return nil, ErrCredentialNotFound
			}
			return credential, nil
		}
		s.logf("[openapi] 凭证缓存解码失败，丢弃并回源: %s", key)
	}

	// 同一 AK 的并发回源只放一个进去，其余等结果（缓存击穿保护）
	value, err, _ := s.group.Do(key, func() (any, error) {
		credential, lookupErr := s.next.Lookup(ctx, accessKeyID)
		switch {
		case errors.Is(lookupErr, ErrCredentialNotFound):
			s.store(ctx, key, nil)
			return nil, ErrCredentialNotFound
		case lookupErr != nil:
			return nil, lookupErr
		}
		s.store(ctx, key, credential)
		return credential, nil
	})
	if err != nil {
		return nil, err
	}
	credential, ok := value.(*Credential)
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return credential, nil
}

// store 写缓存（credential 为 nil 时写「不存在」标记）；写失败只记日志。
func (s *CachedSource) store(ctx context.Context, key string, credential *Credential) {
	var (
		data []byte
		ok   bool
	)
	if credential == nil {
		data, ok = encodeNotFound()
	} else {
		data, ok = encodeCachedCredential(credential)
	}
	if !ok {
		return
	}
	if err := s.rdb.Set(ctx, key, data, s.ttl).Err(); err != nil {
		s.logf("[openapi] 写凭证缓存失败（忽略）: %v", err)
	}
}

// cacheEnvelope 缓存值外层封装：区分「凭证快照」与「AK 不存在」两种结果。
type cacheEnvelope struct {
	NotFound bool            `json:"not_found,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// encodeNotFound 生成「AK 不存在」缓存值。
func encodeNotFound() ([]byte, bool) {
	data, err := json.Marshal(cacheEnvelope{NotFound: true})
	if err != nil {
		return nil, false
	}
	return data, true
}

// version 读当前缓存版本号；键不存在按 0 处理。
func (s *CachedSource) version(ctx context.Context) (string, bool) {
	value, err := s.rdb.Get(ctx, credVersionKey).Result()
	switch {
	case err == nil:
		return value, true
	case errors.Is(err, redis.Nil):
		return "0", true
	default:
		s.logf("[openapi] 读取凭证缓存版本号失败（回源查库）: %v", err)
		return "", false
	}
}

// CacheInvalidator 让鉴权缓存立即失效（凭证/调用方发生写操作后调用）。
type CacheInvalidator struct {
	rdb credentialCacheClient
}

// NewCacheInvalidator 创建缓存失效器；rdb 为 nil 时所有调用都是空操作。
func NewCacheInvalidator(rdb *redis.Client) *CacheInvalidator {
	var client credentialCacheClient
	if rdb != nil {
		client = rdb
	}
	return &CacheInvalidator{rdb: client}
}

// InvalidateCredentials 版本号 +1，使所有已缓存的凭证快照失效。
//
// 不删除旧键：它们带旧版本前缀，TTL 到期后自然回收（凭证表本来就只有几十行）。
func (c *CacheInvalidator) InvalidateCredentials(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if err := c.rdb.Incr(ctx, credVersionKey).Err(); err != nil {
		return err
	}
	return nil
}

// cachedCredential 缓存用的序列化结构。
//
// 单独定义而不是直接 JSON 化 Credential：CIDR 是 `*net.IPNet`、调用方启用标志是未导出字段，
// 直接序列化会丢数据；这里统一转成字符串，读回时复用 parseCIDRs 的口径。
type cachedCredential struct {
	IDN           int64    `json:"idn"`
	CredentialNo  string   `json:"credential_no"`
	AuthMode      string   `json:"auth_mode"`
	AccessKeyID   string   `json:"access_key_id"`
	SecretHash    string   `json:"secret_hash"`
	SecretCipher  string   `json:"secret_cipher"`
	SecretCipherP string   `json:"secret_cipher_prev"`
	SecretKID     string   `json:"secret_kid"`
	SignAlgorithm string   `json:"sign_algorithm"`
	TokenFP       string   `json:"token_fp"`
	ParentIDN     int64    `json:"parent_idn"`
	ParentFound   bool     `json:"parent_found"`
	ParentEnable  bool     `json:"parent_enable"`
	ParentRevoke  bool     `json:"parent_revoke"`
	ParentExp     string   `json:"parent_exp,omitempty"`
	ScopeJSON     string   `json:"scope_json"`
	ScopeBroken   bool     `json:"scope_broken"`
	CredentialIPs []string `json:"credential_ips"`
	ClientIDN     int64    `json:"client_idn"`
	ClientCode    string   `json:"client_code"`
	ClientIPs     []string `json:"client_ips"`
	RateLimitRPM  int      `json:"rate_limit_rpm"`
	QuotaDay      int      `json:"quota_day"`
	FlagEnable    bool     `json:"flag_enable"`
	FlagRevoke    bool     `json:"flag_revoke"`
	ClientEnable  bool     `json:"client_enable"`
	CredentialExp string   `json:"credential_exp,omitempty"`
	ClientExp     string   `json:"client_exp,omitempty"`
}

// encodeCachedCredential 把凭证转成缓存字节。
func encodeCachedCredential(credential *Credential) ([]byte, bool) {
	if credential == nil {
		return nil, false
	}
	scopeJSON, err := json.Marshal(credential.Scope)
	if err != nil {
		return nil, false
	}
	item := cachedCredential{
		IDN: credential.IDN, CredentialNo: credential.CredentialNo, AuthMode: credential.AuthMode,
		AccessKeyID: credential.AccessKeyID, SecretHash: credential.SecretHash,
		SecretCipher: credential.SecretCipher, SecretCipherP: credential.SecretCipherP,
		SecretKID: credential.SecretKID, SignAlgorithm: credential.SignAlgorithm,
		TokenFP: credential.TokenFP, ParentIDN: credential.ParentIDN,
		ParentFound: credential.ParentFound, ParentEnable: credential.ParentFlagEnable,
		ParentRevoke: credential.ParentFlagRevoke, ParentExp: timeToText(credential.ParentExp),
		ScopeJSON: string(scopeJSON), ScopeBroken: credential.ScopeBroken,
		CredentialIPs: cidrsToText(credential.CredentialIPs),
		ClientIDN:     credential.ClientIDN, ClientCode: credential.ClientCode,
		ClientIPs:    cidrsToText(credential.ClientIPs),
		RateLimitRPM: credential.RateLimitRPM, QuotaDay: credential.QuotaDay,
		FlagEnable: credential.FlagEnable, FlagRevoke: credential.FlagRevoke,
		ClientEnable:  credential.ClientEnable(),
		CredentialExp: timeToText(credential.CredentialExp), ClientExp: timeToText(credential.ClientExp),
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return nil, false
	}
	data, err := json.Marshal(cacheEnvelope{Data: payload})
	if err != nil {
		return nil, false
	}
	return data, true
}

// decodeCachedCredential 从缓存字节还原凭证。
//
// 返回值：credential、是否命中凭证（false 表示缓存里是「AK 不存在」标记）、解码是否成功。
// 解码失败时由调用方回源（缓存数据损坏不能当成「不存在」把请求拒掉）。
func decodeCachedCredential(data []byte) (*Credential, bool, bool) {
	var envelope cacheEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false, false
	}
	if envelope.NotFound {
		return nil, false, true
	}
	var item cachedCredential
	if err := json.Unmarshal(envelope.Data, &item); err != nil {
		return nil, false, false
	}
	if item.IDN <= 0 || item.AuthMode == "" {
		return nil, false, false
	}
	scope, broken := parseScope(item.ScopeJSON)
	credential := &Credential{
		IDN: item.IDN, CredentialNo: item.CredentialNo, AuthMode: item.AuthMode,
		AccessKeyID: item.AccessKeyID, SecretHash: item.SecretHash,
		SecretCipher: item.SecretCipher, SecretCipherP: item.SecretCipherP,
		SecretKID: item.SecretKID, SignAlgorithm: item.SignAlgorithm,
		TokenFP: item.TokenFP, ParentIDN: item.ParentIDN,
		ParentFound: item.ParentFound, ParentFlagEnable: item.ParentEnable,
		ParentFlagRevoke: item.ParentRevoke, ParentExp: textToTime(item.ParentExp),
		Scope: scope, ScopeBroken: broken || item.ScopeBroken,
		CredentialIPs: parseCIDRs(item.CredentialIPs),
		ClientIDN:     item.ClientIDN, ClientCode: item.ClientCode,
		ClientIPs:    parseCIDRs(item.ClientIPs),
		RateLimitRPM: item.RateLimitRPM, QuotaDay: item.QuotaDay,
		FlagEnable: item.FlagEnable, FlagRevoke: item.FlagRevoke,
		ClientFlagEnable: item.ClientEnable,
		CredentialExp:    textToTime(item.CredentialExp), ClientExp: textToTime(item.ClientExp),
	}
	return credential, true, true
}

// cidrsToText 把 IPNet 列表转成文本（与 ipNetsToText 同口径）。
func cidrsToText(nets []*net.IPNet) []string { return ipNetsToText(nets) }

// timeToText 时间转 RFC3339 文本；nil 返回空串。
func timeToText(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// textToTime 文本转时间；空串或非法返回 nil。
func textToTime(text string) *time.Time {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return nil
	}
	return &parsed
}
