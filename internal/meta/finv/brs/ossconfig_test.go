package brs

import (
	"encoding/json"
	"testing"
)

// TestOSSSettingsCompatibility 校验 39002 的凭证键兼容性：
// 新键 ak/sk 优先，V300082 之前的旧键 access_key/secret_key 兜底。
func TestOSSSettingsCompatibility(t *testing.T) {
	cases := []struct {
		name           string
		raw            string
		wantAK, wantSK string
	}{
		{"新键", `{"enabled":true,"ak":"AK1","sk":"SK1"}`, "AK1", "SK1"},
		{"旧键（未迁移）", `{"enabled":true,"access_key":"AK2","secret_key":"SK2"}`, "AK2", "SK2"},
		{"新旧并存以新键为准", `{"ak":"AK3","sk":"SK3","access_key":"OLD","secret_key":"OLD"}`, "AK3", "SK3"},
		{"新键为空时回退旧键", `{"ak":"","sk":"","access_key":"AK4","secret_key":"SK4"}`, "AK4", "SK4"},
	}
	for _, c := range cases {
		var s ossSettings
		if err := json.Unmarshal([]byte(c.raw), &s); err != nil {
			t.Fatalf("%s：解析失败 %v", c.name, err)
		}
		if got := s.accessKey(); got != c.wantAK {
			t.Errorf("%s：accessKey=%q，期望 %q", c.name, got, c.wantAK)
		}
		if got := s.secretKey(); got != c.wantSK {
			t.Errorf("%s：secretKey=%q，期望 %q", c.name, got, c.wantSK)
		}
	}
}

// TestOSSSettingsMarshalWritesNewKeys 校验保存时只写 ak/sk，不残留旧键。
func TestOSSSettingsMarshalWritesNewKeys(t *testing.T) {
	out, err := json.Marshal(ossSettings{AK: "AK5", SK: "SK5", Prefix: "brs/"})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if _, ok := m["access_key"]; ok {
		t.Errorf("序列化结果不应包含 access_key：%s", out)
	}
	if _, ok := m["secret_key"]; ok {
		t.Errorf("序列化结果不应包含 secret_key：%s", out)
	}
	if m["ak"] != "AK5" || m["sk"] != "SK5" {
		t.Errorf("ak/sk 未按新键写出：%s", out)
	}
}

// TestNormalizeOSS 校验协议/厂商归一化（需求 10912）。
func TestNormalizeOSS(t *testing.T) {
	cases := []struct {
		name                         string
		protocol, provider, endpoint string
		wantProtocol, wantProvider   string
		wantDriver                   string
		wantErr                      bool
	}{
		{"新格式 cloudflare-r2", "s3", "cloudflare-r2", "https://x.r2.cloudflarestorage.com", "s3", "cloudflare-r2", "s3", false},
		{"旧格式 provider=s3 + R2 endpoint", "", "s3", "https://x.r2.cloudflarestorage.com", "s3", "cloudflare-r2", "s3", false},
		{"旧格式 provider=s3 + 其它 endpoint", "", "s3", "https://s3.example.com", "s3", "aws-s3", "s3", false},
		{"协议空厂商空", "", "", "https://x.r2.cloudflarestorage.com", "s3", "cloudflare-r2", "s3", false},
		{"厂商为大写", "S3", "Cloudflare-R2", "https://x.r2.cloudflarestorage.com", "s3", "cloudflare-r2", "s3", false},
		{"local 协议", "local", "local", "/tmp/brs", "local", "local", "local", false},
		{"仅 provider=local", "", "local", "", "local", "local", "local", false},
		{"未知协议", "ftp", "", "", "", "", "", true},
	}
	for _, c := range cases {
		protocol, provider, driver, err := normalizeOSS(c.protocol, c.provider, c.endpoint)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s：应报错", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s：意外报错 %v", c.name, err)
			continue
		}
		if protocol != c.wantProtocol || provider != c.wantProvider || driver != c.wantDriver {
			t.Errorf("%s：得到 (%q,%q,%q)，期望 (%q,%q,%q)", c.name,
				protocol, provider, driver, c.wantProtocol, c.wantProvider, c.wantDriver)
		}
	}
}

// TestOSSSettingsProtocolCompatibility 校验 protocol 新键优先、误拼 protocal 兜底。
func TestOSSSettingsProtocolCompatibility(t *testing.T) {
	cases := []struct {
		name, raw, wantProtocol string
		wantEnable              bool
	}{
		{"新键 protocol", `{"enable":true,"protocol":"s3"}`, "s3", true},
		{"旧误拼 protocal", `{"enabled":true,"protocal":"s3"}`, "s3", true},
		{"新旧并存以新键为准", `{"enable":false,"enabled":true,"protocol":"local","protocal":"s3"}`, "local", false},
		{"全旧格式（10912 之前）", `{"enabled":true,"provider":"s3","endpoint":"https://x.r2.cloudflarestorage.com"}`, "", true},
	}
	for _, c := range cases {
		var s ossSettings
		if err := json.Unmarshal([]byte(c.raw), &s); err != nil {
			t.Fatalf("%s：解析失败 %v", c.name, err)
		}
		if got := s.protocolValue(); got != c.wantProtocol {
			t.Errorf("%s：protocol=%q，期望 %q", c.name, got, c.wantProtocol)
		}
		if got := s.enabled(); got != c.wantEnable {
			t.Errorf("%s：enable=%v，期望 %v", c.name, got, c.wantEnable)
		}
	}
}

// TestOSSSettingsMarshalWritesProtocolKeys 校验保存只写新键（enable/protocol），不残留 enabled/protocal。
func TestOSSSettingsMarshalWritesProtocolKeys(t *testing.T) {
	enable := true
	out, err := json.Marshal(ossSettings{
		Enable: &enable, Protocol: "s3", Provider: "cloudflare-r2",
		AK: "AK", SK: "SK", Prefix: "BRS/",
	})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	for _, bad := range []string{"enabled", "protocal", "access_key", "secret_key"} {
		if _, ok := m[bad]; ok {
			t.Errorf("序列化结果不应包含旧键 %q：%s", bad, out)
		}
	}
	if m["enable"] != true || m["protocol"] != "s3" || m["provider"] != "cloudflare-r2" {
		t.Errorf("新键未正确写出：%s", out)
	}
}
