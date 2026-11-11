package openapi

// 服务层纯函数测试（需求 11169）：不连库，只校验入参归一与密钥格式约定。

import (
	"errors"
	"strings"
	"testing"

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
	base := CredentialCreateRequest{ClientIDN: 1, CredentialNo: "cred-01", AuthMode: "api_key"}
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

	aksk, err := validateCredential(CredentialCreateRequest{ClientIDN: 1, CredentialNo: "cred-02", AuthMode: "AK_SK"})
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
		{"缺 client", CredentialCreateRequest{CredentialNo: "c1", AuthMode: "API_KEY"}, "client_idn"},
		{"空 no", CredentialCreateRequest{ClientIDN: 1, AuthMode: "API_KEY"}, "credential_no"},
		{"非法 no", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "含中文", AuthMode: "API_KEY"}, "credential_no"},
		{"预留模式", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "BEARER"}, "auth_mode"},
		{"预留算法", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "AK_SK", SignAlgorithm: "ED25519"}, "sign_algorithm"},
		{"scope 缺路径", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "API_KEY", Scope: []string{"POST"}}, "scope"},
		{"scope 方法非法", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "API_KEY", Scope: []string{"FETCH /a"}}, "scope 方法"},
		{"scope 路径非斜杠", CredentialCreateRequest{ClientIDN: 1, CredentialNo: "c1", AuthMode: "API_KEY", Scope: []string{"GET a"}}, "scope 路径"},
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
	// 文档承诺的格式（需求 11176 起全大写）：静态 Key = FK_LIVE_ + 32 位十六进制；
	// AK = AK_LIVE_ + 32 位十六进制；SK = SK_LIVE_ + 48 位十六进制
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
		ClientIDN: 1, CredentialNo: "cred-cdeu-01", AuthMode: "api_key",
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
