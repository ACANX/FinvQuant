package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/brs"
)

// TestParseCamelPager 校验分页参数按小驼峰读取（page / pageSize），默认 1 / 20。
func TestParseCamelPager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		query      string
		page, size int
	}{
		{"", 1, 20},
		{"page=3&pageSize=50", 3, 50},
		{"page=2&page_size=99", 2, 20}, // 旧名 page_size 不再被读取
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/x?"+c.query, nil)
		page, size := parseCamelPager(ctx)
		if page != c.page || size != c.size {
			t.Errorf("query %q：得到 (%d,%d)，期望 (%d,%d)", c.query, page, size, c.page, c.size)
		}
	}
}

// TestIsBrsInputError 校验只有 InputError 被判为 400，其余走 500。
func TestIsBrsInputError(t *testing.T) {
	if isBrsInputError(errors.New("普通错误")) {
		t.Error("普通错误不应判为 InputError")
	}
	// 非法 flagEnable 在参数校验阶段即返回 InputError（不访问数据库，pool 可为 nil）。
	reg := brs.NewRegistry(nil)
	_, _, err := reg.List(context.Background(), 1, 20, "", "2")
	if !isBrsInputError(err) {
		t.Errorf("非法 flagEnable 应返回 InputError，实际: %v", err)
	}
}
