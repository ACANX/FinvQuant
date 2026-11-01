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
