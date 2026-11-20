package openapi

import (
	"testing"
	"time"
)

// TestParseScopeItems 授权范围字符串 → 明细表条目（需求 11228）。
//
// 写路径把「方法 路径」拆成 (method, path) 落 open_api_credential_scope，
// 这里的归一化口径必须与 scopeToJSON 一致，否则会写出鉴权侧匹配不上的行。
func TestParseScopeItems(t *testing.T) {
	cases := []struct {
		name  string
		items []string
		want  []ScopePair
	}{
		{
			name:  "标准两条",
			items: []string{"GET /OpenAPI/V1/A", "POST /OpenAPI/V1/B"},
			want:  []ScopePair{{"GET", "/OpenAPI/V1/A"}, {"POST", "/OpenAPI/V1/B"}},
		},
		{
			name:  "方法小写归一为大写",
			items: []string{"get /OpenAPI/V1/A"},
			want:  []ScopePair{{"GET", "/OpenAPI/V1/A"}},
		},
		{
			name:  "通配方法保留",
			items: []string{"* /OpenAPI/V1/A"},
			want:  []ScopePair{{"*", "/OpenAPI/V1/A"}},
		},
		{
			name:  "路径通配保留",
			items: []string{"GET /OpenAPI/V1/Meta/*"},
			want:  []ScopePair{{"GET", "/OpenAPI/V1/Meta/*"}},
		},
		{
			name:  "段数不对的跳过",
			items: []string{"GET", "GET /A /B", "", "   "},
			want:  []ScopePair{},
		},
		{
			name:  "path 不以斜杠开头跳过",
			items: []string{"GET OpenAPI/V1/A", "GET /OK"},
			want:  []ScopePair{{"GET", "/OK"}},
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			got := ParseScopeItems(item.items)
			if len(got) != len(item.want) {
				t.Fatalf("条目数不符：want %d, got %d (%+v)", len(item.want), len(got), got)
			}
			for i := range item.want {
				if got[i] != item.want[i] {
					t.Fatalf("第 %d 条不符：want %+v, got %+v", i, item.want[i], got[i])
				}
			}
		})
	}
}

// TestScopeToJSONThenParse 写库前的 JSON 与写表的条目必须同口径（需求 11228）。
//
// 两处若不一致，会出现「scope_str 有 2 条、明细表只有 1 条」的静默漂移，
// 而鉴权读明细表 → 少一条就是丢了权限。
func TestScopeToJSONThenParse(t *testing.T) {
	items := []string{"GET /OpenAPI/V1/A", "post /OpenAPI/V1/B", "GET OpenAPI/V1/BAD"}
	pairs := ParseScopeItems(items)
	if len(pairs) != 2 {
		t.Fatalf("非法条目应被跳过，实际 %d 条: %+v", len(pairs), pairs)
	}
	raw := scopeToJSON(items)
	back := scopeToText(raw)
	if len(back) != 2 {
		t.Fatalf("scope_str 与写表条目数不一致：写表 %d 条、JSON %d 条 (%s)", len(pairs), len(back), raw)
	}
}

// TestValidateExpireForMode 有效期口径（需求 11188 / 11246）：
// 静态 Key 必填且不超过 90 天；AK/SK 与 STS 兑换凭证允许长期。
func TestValidateExpireForMode(t *testing.T) {
	now := time.Now()
	day := 24 * time.Hour
	cases := []struct {
		name    string
		mode    string
		expire  *time.Time
		wantErr bool
	}{
		{name: "静态Key留空", mode: AuthModeAPIKey, expire: nil, wantErr: true},
		{name: "静态Key超90天", mode: AuthModeAPIKey, expire: ptrTime(now.Add(91 * day)), wantErr: true},
		{name: "静态Key 30天", mode: AuthModeAPIKey, expire: ptrTime(now.Add(30 * day)), wantErr: false},
		{name: "AKSK留空", mode: AuthModeAKSK, expire: nil, wantErr: false},
		{name: "AKSK超90天", mode: AuthModeAKSK, expire: ptrTime(now.Add(365 * day)), wantErr: false},
		{name: "STS留空", mode: AuthModeSTS, expire: nil, wantErr: false},
		{name: "STS长期", mode: AuthModeSTS, expire: ptrTime(now.Add(730 * day)), wantErr: false},
	}
	for _, c := range cases {
		if err := validateExpireForMode(c.mode, c.expire); (err != nil) != c.wantErr {
			t.Errorf("%s: 期望报错=%v，实际 err=%v", c.name, c.wantErr, err)
		}
	}
}

func ptrTime(v time.Time) *time.Time { return &v }
