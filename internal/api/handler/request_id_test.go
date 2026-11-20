package handler

// 追踪 ID 的大小写口径（需求 11181、ApiSpec §1.0）：内部接口一律大写 UUID4。

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestIDUppercaseUUID 请求 ID 必须是大写 UUID4，且同一请求内多次调用保持一致。
func TestRequestIDUppercaseUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	first := RequestID(c)
	if first == "" {
		t.Fatalf("RequestID 不应为空")
	}
	if first != strings.ToUpper(first) {
		t.Fatalf("请求 ID 必须全大写，实际 %q", first)
	}
	// UUID4 形态：8-4-4-4-12 共 36 字符
	parts := strings.Split(first, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[4]) != 12 {
		t.Fatalf("应为 UUID 形态（8-4-4-4-12），实际 %q", first)
	}
	if second := RequestID(c); second != first {
		t.Fatalf("同一请求内应复用同一个 ID：%q vs %q", first, second)
	}
	// 版本位固定为 4（UUID4）
	if !strings.HasPrefix(parts[2], "4") {
		t.Fatalf("应为 UUID4（第三段以 4 开头），实际 %q", parts[2])
	}
}
