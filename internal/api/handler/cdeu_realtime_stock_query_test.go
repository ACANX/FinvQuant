package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newQueryCtx 构造一个带指定 query 的 gin 上下文，只用于参数解析测试。
func newQueryCtx(query string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?"+query, nil)
	return c
}

// TestRealtimeStockQueryInt64StockID 回归「预填失败：stockId 必须是 1..0 的整数」。
//
// 曾经用 realtimeStockQueryInt(name, 0, 1, 0) 解析 stockId：min=1、max=0 → 任何正值都判越界，
// 前端打开「录入证券元数据」对话框必然报错。stockId 是 bigint 主键，必须按 int64 且只校验下界。
func TestRealtimeStockQueryInt64StockID(t *testing.T) {
	ok := map[string]int64{
		"stockId=1":              1,
		"stockId=86002425217944": 86002425217944,
		"stockId=79083232831247": 79083232831247,
		// 允许前后空白
		"stockId=%2079037%20": 79037,
	}
	for query, want := range ok {
		got, err := realtimeStockQueryInt64(newQueryCtx(query), "stockId")
		if err != nil {
			t.Fatalf("%s 解析失败: %v", query, err)
		}
		if got != want {
			t.Fatalf("%s 解析为 %d，期望 %d", query, got, want)
		}
	}

	bad := []string{"", "stockId=", "stockId=0", "stockId=-5", "stockId=abc", "stockId=1.5"}
	for _, query := range bad {
		if _, err := realtimeStockQueryInt64(newQueryCtx(query), "stockId"); err == nil {
			t.Fatalf("%q 应被拒绝", query)
		}
	}
}

// TestRealtimeStockQueryIntRange 确保闭区间解析在正常 min<=max 下行为不变。
func TestRealtimeStockQueryIntRange(t *testing.T) {
	if got, err := realtimeStockQueryInt(newQueryCtx("pageSize=100"), "pageSize", 50, 1, 500); err != nil || got != 100 {
		t.Fatalf("pageSize=100 → %d/%v", got, err)
	}
	if got, err := realtimeStockQueryInt(newQueryCtx(""), "pageSize", 50, 1, 500); err != nil || got != 50 {
		t.Fatalf("缺省 → %d/%v", got, err)
	}
	for _, query := range []string{"pageSize=0", "pageSize=501", "pageSize=x"} {
		if _, err := realtimeStockQueryInt(newQueryCtx(query), "pageSize", 50, 1, 500); err == nil {
			t.Fatalf("%q 应被拒绝", query)
		}
	}
}
