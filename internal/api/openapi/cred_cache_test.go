package openapi

// 凭证缓存单测（需求 11212）：命中/回源、版本号失效、Redis 故障降级。

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// countingSource 记录回源次数。
type countingSource struct {
	credential *Credential
	hits       int
	err        error
}

func (c *countingSource) Lookup(_ context.Context, accessKeyID string) (*Credential, error) {
	c.hits++
	if c.err != nil {
		return nil, c.err
	}
	if c.credential == nil || c.credential.AccessKeyID != accessKeyID {
		return nil, ErrCredentialNotFound
	}
	return c.credential, nil
}

// TestCredentialCacheRoundTrip 缓存编解码必须还原鉴权用到的全部字段。
func TestCredentialCacheRoundTrip(t *testing.T) {
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	clientExp := exp.Add(24 * time.Hour)
	original := &Credential{
		IDN: 33, CredentialNo: "STS-47-X", AuthMode: AuthModeSTS,
		AccessKeyID: "SAK_LIVE_ABC", SecretCipher: "cipher", SignAlgorithm: "HMAC-SHA256",
		TokenFP: "ABCDEF0123456789", ParentIDN: 31,
		ParentFound: true, ParentFlagEnable: true,
		Scope:         []ScopeRule{{Method: "GET", Path: "/OpenAPI/V1/X"}},
		CredentialIPs: parseCIDRs([]string{"10.0.0.0/8"}),
		ClientIDN:     47, ClientCode: "DEMO", ClientIPs: parseCIDRs([]string{"192.168.0.0/16"}),
		RateLimitRPM: 600, QuotaDay: 100000,
		FlagEnable: true, ClientFlagEnable: true,
		CredentialExp: &exp, ClientExp: &clientExp,
	}

	data, ok := encodeCachedCredential(original)
	if !ok {
		t.Fatal("编码失败")
	}
	decoded, ok := decodeCachedCredential(data)
	if !ok {
		t.Fatal("解码失败")
	}
	if decoded.IDN != original.IDN || decoded.AuthMode != original.AuthMode ||
		decoded.AccessKeyID != original.AccessKeyID || decoded.SecretCipher != original.SecretCipher {
		t.Fatalf("基本字段不一致: %+v", decoded)
	}
	if decoded.TokenFP != original.TokenFP || decoded.ParentIDN != original.ParentIDN ||
		!decoded.ParentFound || !decoded.ParentFlagEnable {
		t.Fatalf("STS/父凭证字段不一致: %+v", decoded)
	}
	if len(decoded.Scope) != 1 || decoded.Scope[0].Path != "/OpenAPI/V1/X" {
		t.Fatalf("授权范围不一致: %+v", decoded.Scope)
	}
	if len(decoded.CredentialIPs) != 1 || decoded.CredentialIPs[0].String() != "10.0.0.0/8" {
		t.Fatalf("凭证白名单不一致: %+v", decoded.CredentialIPs)
	}
	if len(decoded.ClientIPs) != 1 || !decoded.ClientEnable() {
		t.Fatalf("调用方白名单/启用标志不一致: %+v", decoded)
	}
	if decoded.CredentialExp == nil || !decoded.CredentialExp.Equal(exp) {
		t.Fatalf("到期时间不一致: %v", decoded.CredentialExp)
	}
	if decoded.ClientExp == nil || !decoded.ClientExp.Equal(clientExp) {
		t.Fatalf("调用方到期时间不一致: %v", decoded.ClientExp)
	}

	// 损坏数据必须判失败（由调用方回源），不能还原出半个凭证
	if _, ok := decodeCachedCredential([]byte("{不是 JSON")); ok {
		t.Fatal("损坏数据应解码失败")
	}
	if _, ok := decodeCachedCredential([]byte(`{"idn":0}`)); ok {
		t.Fatal("缺少 idn/auth_mode 应解码失败")
	}
}

// TestCachedSourceHitsAndInvalidates 缓存命中、版本号失效、直通分支。
func TestCachedSourceHitsAndInvalidates(t *testing.T) {
	ctx := context.Background()
	credential := &Credential{IDN: 1, AuthMode: AuthModeAPIKey, AccessKeyID: "SA_LIVE_X", FlagEnable: true}
	next := &countingSource{credential: credential}
	source := newCachedSource(next, nil, nil)

	// rdb 为 nil：每次直通，无缓存
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("直通模式查询失败: %v", err)
	}
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("直通模式查询失败: %v", err)
	}
	if next.hits != 2 {
		t.Fatalf("rdb=nil 应每次回源，实际 %d 次", next.hits)
	}

	// 回源错误原样返回，不吞
	failing := newCachedSource(&countingSource{err: errors.New("db down")}, nil, nil)
	if _, err := failing.Lookup(ctx, "SA_LIVE_X"); err == nil {
		t.Fatal("回源错误应向上传递")
	}
}

// TestCacheKeyShapes 键形状：版本号参与键名，版本变化后旧键再也读不到。
func TestCacheKeyShapes(t *testing.T) {
	if !strings.HasPrefix(credCacheKeyBase, "finv:openapi:cred:") {
		t.Fatalf("缓存键前缀不符合约定: %s", credCacheKeyBase)
	}
	if credVersionKey != "finv:openapi:credver" {
		t.Fatalf("版本号键名不符合约定: %s", credVersionKey)
	}
	if credCacheTTL <= 0 {
		t.Fatal("TTL 必须为正")
	}
}

// fakeRedis 极简 Redis 替身：只实现缓存用到的 GET/SET/INCR。
type fakeRedis struct {
	mu     sync.Mutex
	values map[string]string
	fail   bool
}

func newFakeRedis() *fakeRedis { return &fakeRedis{values: map[string]string{}} }

func (f *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewStringCmd(ctx, "get", key)
	if f.fail {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	if value, ok := f.values[key]; ok {
		cmd.SetVal(value)
		return cmd
	}
	cmd.SetErr(redis.Nil)
	return cmd
}

func (f *fakeRedis) Set(ctx context.Context, key string, value any, _ time.Duration) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewStatusCmd(ctx, "set", key)
	if f.fail {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	f.values[key] = toString(value)
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Incr(ctx context.Context, key string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewIntCmd(ctx, "incr", key)
	if f.fail {
		cmd.SetErr(errors.New("redis down"))
		return cmd
	}
	current, _ := strconv.Atoi(f.values[key])
	current++
	f.values[key] = strconv.Itoa(current)
	cmd.SetVal(int64(current))
	return cmd
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	}
	return ""
}

// TestCachedSourceRedisBehaviour 命中缓存不回源、版本号变化后重新回源、Redis 故障回源。
func TestCachedSourceRedisBehaviour(t *testing.T) {
	ctx := context.Background()
	credential := &Credential{IDN: 1, AuthMode: AuthModeAPIKey, AccessKeyID: "SA_LIVE_X", FlagEnable: true}
	next := &countingSource{credential: credential}
	fake := newFakeRedis()
	source := newCachedSource(next, fake, nil)

	// 第一次：miss → 回源 → 写缓存
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("首次查询失败: %v", err)
	}
	if next.hits != 1 {
		t.Fatalf("首次应回源 1 次，实际 %d", next.hits)
	}
	// 第二次：命中缓存，不再回源
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("缓存命中查询失败: %v", err)
	}
	if next.hits != 1 {
		t.Fatalf("缓存命中不应回源，实际回源 %d 次", next.hits)
	}

	// 版本号 +1（等价于吊销/停用/轮换）→ 旧键失效 → 重新回源
	invalidator := &CacheInvalidator{rdb: fake}
	if err := invalidator.InvalidateCredentials(ctx); err != nil {
		t.Fatalf("失效失败: %v", err)
	}
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("版本变化后查询失败: %v", err)
	}
	if next.hits != 2 {
		t.Fatalf("版本变化后应重新回源，实际回源 %d 次", next.hits)
	}

	// Redis 故障 → 直接回源，不影响鉴权
	fake.fail = true
	if _, err := source.Lookup(ctx, "SA_LIVE_X"); err != nil {
		t.Fatalf("Redis 故障应降级回源，实际报错: %v", err)
	}
	if next.hits != 3 {
		t.Fatalf("Redis 故障应回源，实际回源 %d 次", next.hits)
	}
	if err := invalidator.InvalidateCredentials(ctx); err == nil {
		t.Fatal("Redis 故障时失效器应返回错误（由调用方忽略）")
	}
}
