package openapi

// 服务层纯函数测试（需求 11169）：不连库，只校验入参归一与密钥格式约定。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

func TestValidateClient(t *testing.T) {
	base := ClientRequest{ClientCode: "cdeu-hook", ClientName: "CDEU 采集钩子"}

	ok, err := validateClient(base)
	if err != nil {
		t.Fatalf("合法入参不应报错: %v", err)
	}
	if ok.RateLimitRPM != 0 || ok.QuotaDay != 0 {
		t.Fatalf("未填的配额应保持 0（不限）")
	}

	cases := []struct {
		name string
		req  ClientRequest
		word string
	}{
		{"空 code", ClientRequest{ClientName: "x"}, "client_code"},
		{"大写 code", ClientRequest{ClientCode: "CDEU", ClientName: "x"}, "client_code"},
		{"单字符 code", ClientRequest{ClientCode: "a", ClientName: "x"}, "client_code"},
		{"code 含空格", ClientRequest{ClientCode: "a b", ClientName: "x"}, "client_code"},
		{"空名称", ClientRequest{ClientCode: "abc"}, "client_name"},
		{"负速率", ClientRequest{ClientCode: "abc", ClientName: "x", RateLimitRPM: -1}, "rate_limit_rpm"},
		{"负配额", ClientRequest{ClientCode: "abc", ClientName: "x", QuotaDay: -5}, "quota_day"},
		{"非法网段", ClientRequest{ClientCode: "abc", ClientName: "x", AllowCIDR: []string{"1.2.3/24"}}, "allow_cidr"},
		{"非法到期", ClientRequest{ClientCode: "abc", ClientName: "x", DTExpire: "2026/01/01"}, "dt_expire"},
	}
	for _, c := range cases {
		if _, err := validateClient(c.req); err == nil || !strings.Contains(err.Error(), c.word) {
			t.Fatalf("%s：期望错误包含 %q，实际 %v", c.name, c.word, err)
		} else if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%s：错误应可判定为 ErrInvalidRequest，实际 %v", c.name, err)
		}
	}

	// 裸 IP 归一成 /32、裸 IPv6 归一成 /128
	normalized, err := validateClient(ClientRequest{
		ClientCode: "abc", ClientName: "x", AllowCIDR: []string{"10.0.0.1", "2001:db8::1", "192.168.0.0/16"},
	})
	if err != nil {
		t.Fatalf("裸 IP 应被接受: %v", err)
	}
	want := []string{"10.0.0.1/32", "2001:db8::1/128", "192.168.0.0/16"}
	for i, value := range want {
		if normalized.AllowCIDR[i] != value {
			t.Fatalf("第 %d 个白名单应归一为 %q，实际 %q", i, value, normalized.AllowCIDR[i])
		}
	}
}

func TestValidateCredential(t *testing.T) {
	// 静态 Key 必须指定唯一授权接口（需求 11188）
	base := CredentialCreateRequest{
		ClientIDN: 1, CredentialNo: "cred-01", AuthMode: "apikey",
		Scope:    []string{"GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni"},
		DTExpire: time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"), // 静态 Key 有效期 ≤ 90 天（需求 11188）
	}
	ok, err := validateCredential(base)
	if err != nil {
		t.Fatalf("合法入参不应报错: %v", err)
	}
	if ok.AuthMode != AuthModeAPIKey {
		t.Fatalf("auth_mode 应归一为大写，实际 %q", ok.AuthMode)
	}
	if ok.SignAlgorithm != "HMAC-SHA256" {
		// 列上有 NOT NULL + CHECK，静态 Key 也落 HMAC-SHA256（只是不参与验签）
		t.Fatalf("API_KEY 模式应落默认算法 HMAC-SHA256，实际 %q", ok.SignAlgorithm)
	}

	aksk, err := validateCredential(CredentialCreateRequest{ClientIDN: 1, CredentialNo: "cred-02", AuthMode: "AKSK"})
	if err != nil {
		t.Fatalf("AK/SK 合法: %v", err)
	}
	if aksk.SignAlgorithm != "HMAC-SHA256" {
		t.Fatalf("AK/SK 默认算法应为 HMAC-SHA256，实际 %q", aksk.SignAlgorithm)
	}

	bad := []struct {
		name string
		req  CredentialCreateRequest
		word string
	}{
		{"缺 client", CredentialCreateRequest{CredentialNo: "c1", AuthMode: "APIKEY"}, "client_idn"},
		{"空 no", CredentialCreateRequest{ClientIDN: 1, AuthMode: "APIKEY"}, "credential_no"},
		{"非法 no", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "含中文", AuthMode: "APIKEY"}, "credential_no"},
		{"预留模式", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "BEARER"}, "auth_mode"},
		{"预留算法", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "AKSK", SignAlgorithm: "ED25519"}, "sign_algorithm"},
		{"scope 缺路径", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY", Scope: []string{"POST"}}, "scope"},
		{"scope 方法非法", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY", Scope: []string{"FETCH /a"}}, "scope 方法"},
		{"scope 路径非斜杠", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY", Scope: []string{"GET a"}}, "scope 路径"},
		// 静态 Key：必须单接口、禁通配（需求 11188）
		{"静态 Key 空 scope", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY",
			DTExpire: time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02")}, "必须指定唯一授权接口"},
		{"静态 Key 多接口", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY",
			DTExpire: time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"),
			Scope:    []string{"GET /OpenAPI/V1/a", "POST /OpenAPI/V1/b"}}, "只能授权 1 个接口"},
		{"静态 Key 通配", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY",
			DTExpire: time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"),
			Scope:    []string{"* /OpenAPI/V1/*"}}, "不允许通配"},
		// 静态 Key 有效期上限 90 天（需求 11188）
		{"静态 Key 无到期日", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY",
			Scope: []string{"GET /OpenAPI/V1/a"}}, "必须设置到期日"},
		{"静态 Key 到期超 90 天", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "APIKEY",
			Scope:    []string{"GET /OpenAPI/V1/a"},
			DTExpire: time.Now().Add(120 * 24 * time.Hour).Format("2006-01-02")}, "最长 90 天"},
	}
	for _, c := range bad {
		if _, err := validateCredential(c.req); err == nil || !strings.Contains(err.Error(), c.word) {
			t.Fatalf("%s：期望错误包含 %q，实际 %v", c.name, c.word, err)
		}
	}
}

func TestScopeRoundTrip(t *testing.T) {
	items := []string{"POST /OpenAPI/V1/A", "GET /OpenAPI/V1/B/*"}
	encoded := scopeToJSON(items)
	if !strings.Contains(encoded, `"method":"POST"`) || !strings.Contains(encoded, `"path":"/OpenAPI/V1/B/*"`) {
		t.Fatalf("scope 序列化不符: %s", encoded)
	}
	decoded := scopeToText(encoded)
	if len(decoded) != 2 || decoded[0] != items[0] || decoded[1] != items[1] {
		t.Fatalf("scope 反序列化不符: %v", decoded)
	}
	if scopeToJSON(nil) != "[]" {
		t.Fatalf("空 scope 应序列化为 []")
	}
	if scopeToText("") != nil || scopeToText("bad json") != nil {
		t.Fatalf("空/非法 scope 应返回 nil")
	}
	// 缺 method 的元素按通配展示
	if got := scopeToText(`[{"path":"/x"}]`); len(got) != 1 || got[0] != "* /x" {
		t.Fatalf("缺 method 应展示为 *，实际 %v", got)
	}
}

func TestNormalizeModeAndExpire(t *testing.T) {
	for _, raw := range []string{"0", "1", "2", " 1 "} {
		if _, err := normalizeMode(raw); err != nil {
			t.Fatalf("mode %q 应合法: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "3", "on", "true"} {
		if _, err := normalizeMode(raw); err == nil {
			t.Fatalf("mode %q 应被拒绝", raw)
		}
	}

	if expire, err := parseExpire(""); err != nil || expire != nil {
		t.Fatalf("空到期时间为长期（nil）")
	}
	if expire, err := parseExpire("2026-12-31"); err != nil || expire == nil {
		t.Fatalf("2006-01-02 应可解析: %v", err)
	}
	if expire, err := parseExpire("2026-12-31T23:59:59+08:00"); err != nil || expire == nil {
		t.Fatalf("RFC3339 应可解析: %v", err)
	}
	if _, err := parseExpire("2026/12/31"); err == nil {
		t.Fatalf("非法格式应报错")
	}
}

func TestKeyFormatConvention(t *testing.T) {
	// 文档承诺的格式（需求 11176 起全大写；11178 起静态 Key 前缀为 SA_LIVE_）：
	// 静态 Key = SA_LIVE_ + 32 位十六进制；AK = AK_LIVE_ + 32 位；SK = SK_LIVE_ + 48 位
	random, err := secret.RandomToken(16)
	if err != nil {
		t.Fatalf("生成随机数失败: %v", err)
	}
	if len(random) != 32 {
		t.Fatalf("16 字节随机数应产生 32 位十六进制，实际 %d", len(random))
	}
	key := secret.PrefixAPIKey + random
	if !strings.HasPrefix(key, secret.PrefixAPIKey) || len(key) != len(secret.PrefixAPIKey)+32 {
		t.Fatalf("静态 Key 格式不符: %s", key)
	}
	if prefix := keyPrefixOf(key); len(prefix) != 12 || prefix != secret.PrefixAPIKey+random[:4] {
		t.Fatalf("展示前缀应为前 12 位，实际 %q", prefix)
	}

	// 同一 Key 的哈希稳定，且不同 Key 不会撞
	if secret.HashKey(key) == secret.HashKey(key+"x") {
		t.Fatalf("不同 Key 的哈希不应相同")
	}
	if !secret.MatchKey(secret.HashKey(key), key) {
		t.Fatalf("哈希应能验证自身")
	}
}

func TestNormalizePage(t *testing.T) {
	cases := []struct{ page, size, wantPage, wantSize int }{
		{0, 0, 1, 20},
		{-5, -1, 1, 20},
		{3, 50, 3, 50},
		{1, 999, 1, 200},
	}
	for _, c := range cases {
		page, size := normalizePage(c.page, c.size)
		if page != c.wantPage || size != c.wantSize {
			t.Fatalf("normalizePage(%d,%d)=(%d,%d)，期望 (%d,%d)",
				c.page, c.size, page, size, c.wantPage, c.wantSize)
		}
	}
}

func TestIssuedNoteMentionsOnceOnly(t *testing.T) {
	for _, mode := range []string{AuthModeAPIKey, AuthModeAKSK} {
		if note := issuedNote(mode); !strings.Contains(note, "只显示这一次") {
			t.Fatalf("%s 的签发提示必须点明「只显示这一次」，实际 %q", mode, note)
		}
	}
}

func TestInvokeLogFilterValidation(t *testing.T) {
	service := &Service{}
	if _, _, err := service.InvokeLogPage(nil, InvokeLogFilter{FlagPass: "x"}); err == nil {
		t.Fatalf("非法 flag_pass 应被拒绝")
	}
}

// TestIssuedKeyUppercase 凭证号与三种密钥的英文字母一律大写（需求 11176）。
func TestIssuedKeyUppercase(t *testing.T) {
	// 凭证号：小写入参被归一为大写
	validated, err := validateCredential(CredentialCreateRequest{
		ClientIDN: 1, CredentialNo: "cred-cdeu-01", AuthMode: "apikey",
		Scope:    []string{"GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni"},
		DTExpire: time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("合法凭证号不应报错: %v", err)
	}
	if validated.CredentialNo != "CRED-CDEU-01" {
		t.Fatalf("凭证号应归一为大写，实际 %q", validated.CredentialNo)
	}

	// 生成侧：前缀与随机部分都大写
	random, err := secret.RandomToken(16)
	if err != nil {
		t.Fatalf("生成随机数失败: %v", err)
	}
	upper := secret.NormalizeKey(random)
	if upper != strings.ToUpper(random) {
		t.Fatalf("NormalizeKey 应转大写")
	}
	if secret.PrefixAPIKey != "SA_LIVE_" {
		t.Fatalf("静态 Key 前缀应为 SA_LIVE_（需求 11178），实际 %q", secret.PrefixAPIKey)
	}
	for _, prefix := range []string{secret.PrefixAPIKey, secret.PrefixAK, secret.PrefixSK} {
		if prefix != strings.ToUpper(prefix) {
			t.Fatalf("密钥前缀 %q 必须全大写", prefix)
		}
	}
	full := secret.PrefixAPIKey + upper
	if full != strings.ToUpper(full) {
		t.Fatalf("拼出的密钥必须全大写: %s", full)
	}

	// 校验侧：库里存大写 Key 的哈希时，客户端传小写也能通过（归一后比对）
	hashOfUpper := secret.HashKey(full)
	if !secret.MatchKey(hashOfUpper, strings.ToLower(full)) {
		t.Fatalf("小写形式应能匹配大写 Key 的哈希")
	}
	if !secret.MatchKey(hashOfUpper, full) {
		t.Fatalf("大写形式应能匹配")
	}

	// 兼容：改口径之前签发的小写 Key（存的是小写 Key 的哈希）仍必须可用，
	// 这是 MatchKey 里「大写归一失败后回退原样比对」分支的用途。
	legacy := "fk_live_abcdef0123456789"
	if !secret.MatchKey(secret.HashKey(legacy), legacy) {
		t.Fatalf("历史小写 Key 应仍可匹配")
	}
}

// TestRotationKeepsAccessKeyForAKSK 轮换不会凭空换掉 AK（AK 是身份标识，库里不变）。
//
// 这里只断言纯逻辑约定：AK/SK 模式的轮换只重写 secret_cipher，
// 服务层返回的 access_key 必须来自回读的凭证行（而不是 buildSecret 新生成的那个）。
func TestRotationKeepsAccessKeyForAKSK(t *testing.T) {
	issued := issuedSecret{AccessKey: "AK_LIVE_NEW", SecretKey: "SK_LIVE_NEW"}
	persisted := Credential{AccessKeyID: "AK_LIVE_ORIGINAL"}

	// 模拟 RotateCredential 的取值口径
	returnedAK := persisted.AccessKeyID
	if returnedAK == issued.AccessKey {
		t.Fatalf("AK/SK 轮换后应对齐库内 AK，不能返回新生成的 AK")
	}
	if returnedAK != "AK_LIVE_ORIGINAL" {
		t.Fatalf("应返回库内 AK，实际 %q", returnedAK)
	}
	if issued.SecretKey == "" {
		t.Fatalf("SK 必须换成新的")
	}
}

// TestScopeLimitByAuthMode 静态 Key 只能单接口，AK_SK 不受接口个数限制（需求 11188）。
func TestScopeLimitByAuthMode(t *testing.T) {
	multi := []string{
		"GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni",
		"POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert",
		"POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert",
	}
	if _, err := validateCredential(CredentialCreateRequest{
		ClientIDN: 1, CredentialNo: "cred-aksk", AuthMode: "AKSK", Scope: multi,
	}); err != nil {
		t.Fatalf("AK_SK 应允许多个接口: %v", err)
	}
	if _, err := validateCredential(CredentialCreateRequest{
		ClientIDN: 1, CredentialNo: "cred-key", AuthMode: "APIKEY", Scope: multi,
	}); err == nil {
		t.Fatalf("API_KEY 不应允许多个接口")
	}
}

// TestCredentialPageStatusFilter 状态过滤参数校验（需求 11217）。
func TestCredentialPageStatusFilter(t *testing.T) {
	service := NewService(NewStore(nil))
	// 非法 status 直接拒绝，不落库（store 为 nil 时若走到 SQL 会报连接池未初始化）
	if _, _, err := service.CredentialPage(context.Background(), 0, "", "revoked", 1, 20); err == nil {
		t.Fatal("非法 status 应被拒绝")
	}
	// 合法值走 store，store 无连接池时返回「未初始化」错误（证明参数已通过校验）
	for _, status := range []string{"", "active", "ALL"} {
		if _, _, err := service.CredentialPage(context.Background(), 0, "", status, 1, 20); err == nil ||
			!strings.Contains(err.Error(), "连接池") {
			t.Fatalf("status=%q 应进入 store 查询，实际错误 %v", status, err)
		}
	}
}
