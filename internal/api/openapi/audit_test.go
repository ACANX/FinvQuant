package openapi

// 凭证使用统计（需求 11199）的契约测试。

import (
	"strings"
	"testing"
)

// TestUpdateCredentialUsageSQL 统计 SQL 必须只按主键自增，且用数据库时间（不信任客户端时钟）。
func TestUpdateCredentialUsageSQL(t *testing.T) {
	for _, want := range []string{"dt_last_use = now()", "count_use = count_use + 1", "WHERE idn = $1"} {
		if !strings.Contains(updateCredentialUsageSQL, want) {
			t.Fatalf("统计 SQL 缺少 %q：%s", want, updateCredentialUsageSQL)
		}
	}
}

// TestCountUsage 只有「放行 + 能定位到凭证」才计数。
func TestCountUsage(t *testing.T) {
	cases := []struct {
		name   string
		pass   bool
		credID int64
		want   bool
	}{
		{"放行且有凭证", true, 12, true},
		{"放行但兜底密钥", true, 0, false},
		{"拒绝且有凭证", false, 12, false},
		{"拒绝且无凭证", false, 0, false},
	}
	for _, c := range cases {
		if got := countUsageOf(c.pass, c.credID); got != c.want {
			t.Fatalf("%s：期望 %t，实际 %t", c.name, c.want, got)
		}
	}
}
