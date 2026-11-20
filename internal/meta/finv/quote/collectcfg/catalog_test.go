package collectcfg

import (
	"strings"
	"testing"
)

// TestCatalogRange 目录只含 20013–20018 段里与实时行情采集相关的 5 项（需求 11343）。
func TestCatalogRange(t *testing.T) {
	items := Items()
	want := []int{
		ConfigNoMoomooCredential,     // 20016
		ConfigNoMoomooQuoteProxy,     // 20018
		ConfigNoSupabaseConn,         // 20013
		ConfigNoSupabaseSecuKeyTable, // 20014
		ConfigNoUscRuleCron,          // 20015
	}
	if len(items) != len(want) {
		t.Fatalf("目录项数 %d，期望 %d", len(items), len(want))
	}
	seen := map[int]bool{}
	for i, item := range items {
		if item.ConfigNo != want[i] {
			t.Errorf("第 %d 项编号 %d，期望 %d", i+1, item.ConfigNo, want[i])
		}
		if seen[item.ConfigNo] {
			t.Errorf("编号 %d 重复", item.ConfigNo)
		}
		seen[item.ConfigNo] = true
		if item.Name == "" || item.Module == "" || item.ValueFormat == "" || item.Description == "" {
			t.Errorf("编号 %d 元数据不完整: %+v", item.ConfigNo, item)
		}
	}
	// 20017 已被 V300228 合并进 20016 后删除，不得出现在目录里
	if _, ok := ItemByNo(20017); ok {
		t.Error("目录不应包含已删除的 20017")
	}
	// 20012（登录会话）属认证域，刻意排除
	if _, ok := ItemByNo(20012); ok {
		t.Error("目录不应包含认证域的 20012")
	}
	// 敏感项只有 20013 / 20016
	for _, item := range items {
		wantSensitive := item.ConfigNo == ConfigNoSupabaseConn || item.ConfigNo == ConfigNoMoomooCredential
		if item.Sensitive != wantSensitive {
			t.Errorf("编号 %d 的 Sensitive=%v，期望 %v", item.ConfigNo, item.Sensitive, wantSensitive)
		}
	}
}

// TestMask 敏感项掩码口径：20013 保留项目 ID，20016 全掩；未配置返回空串。
func TestMask(t *testing.T) {
	cases := []struct {
		name     string
		configNo int
		raw      string
		want     string
	}{
		{"20013 有值", ConfigNoSupabaseConn, "xvtunbplzpraqrogxsnt|sb_publishable_abcdef", "xvtunbplzpraqrogxsnt|****"},
		{"20013 只有项目 ID", ConfigNoSupabaseConn, "projid123", "****"},
		{"20013 未配置", ConfigNoSupabaseConn, "  ", ""},
		{"20016 有值", ConfigNoMoomooCredential, "0f0ba46cce34ca7f884e4e0fcdc309d5|MC4CAQAwBQYDK2Vw", "****|****"},
		{"20016 未配置", ConfigNoMoomooCredential, "", ""},
	}
	for _, tc := range cases {
		if got := Mask(tc.configNo, tc.raw); got != tc.want {
			t.Errorf("%s：期望 %q，实际 %q", tc.name, tc.want, got)
		}
	}
	// 掩码不得泄露 key 的任何字符
	if strings.Contains(Mask(ConfigNoSupabaseConn, "projid123|sb_secret_VALUE"), "sb_secret") {
		t.Error("20013 掩码泄露了 API Key")
	}
}

// TestValidateByCatalog 保存校验的口径（不连库：只测纯校验分支）。
func TestValidateByCatalog(t *testing.T) {
	cases := []struct {
		name     string
		configNo int
		value    string
		wantErr  bool
	}{
		{"20013 合法", ConfigNoSupabaseConn, "abcdef12|sb_publishable_x", false},
		{"20013 少一段", ConfigNoSupabaseConn, "abcdef12", true},
		{"20013 空 key", ConfigNoSupabaseConn, "abcdef12|", true},
		{"20013 清空允许", ConfigNoSupabaseConn, "", false},
		{"20014 合法", ConfigNoSupabaseSecuKeyTable, "finv_quote_secu_key", false},
		{"20014 合法两段", ConfigNoSupabaseSecuKeyTable, "public.finv_quote_secu_key", false},
		{"20014 非法字符", ConfigNoSupabaseSecuKeyTable, "bad-table", true},
		{"20014 空串回退默认", ConfigNoSupabaseSecuKeyTable, "", false},
		{"20015 合法 cron", ConfigNoUscRuleCron, "0 30 2 * * *", false},
		{"20015 空串关闭", ConfigNoUscRuleCron, "", false},
		{"20015 五段非法", ConfigNoUscRuleCron, "30 2 * * *", true},
		{"20018 空串直连", ConfigNoMoomooQuoteProxy, "", false},
		{"20018 启用有地址", ConfigNoMoomooQuoteProxy, "1|https://proxy.example.com/", false},
		{"20018 启用缺地址", ConfigNoMoomooQuoteProxy, "1|", true},
		{"20018 关闭", ConfigNoMoomooQuoteProxy, "0|", false},
	}
	for _, tc := range cases {
		got := validateValue(tc.configNo, tc.value)
		if tc.wantErr && got == nil {
			t.Errorf("%s：期望报错，实际通过", tc.name)
		}
		if !tc.wantErr && got != nil {
			t.Errorf("%s：期望通过，实际报错 %v", tc.name, got)
		}
	}
}

// TestSaveRejectsOutOfScope 不在目录内的编号一律拒绝（含 20012 / 20017 / 其它段）。
func TestSaveRejectsOutOfScope(t *testing.T) {
	store := NewStore(nil) // 编号校验在访问数据库之前完成
	for _, no := range []int{20012, 20017, 20000, 39001, 1} {
		if _, err := store.Save(nil, no, "x"); err == nil { //nolint:staticcheck // 早返回，不会用到 ctx
			t.Errorf("编号 %d 应被拒绝", no)
		}
	}
}
