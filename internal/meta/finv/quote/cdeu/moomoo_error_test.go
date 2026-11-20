package cdeu

import (
	"errors"
	"strings"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestDescribeMoomooErrorHTML 风控页（HTML）要给出「风控/限流」结论与单行片段（需求 11335）。
func TestDescribeMoomooErrorHTML(t *testing.T) {
	raw := &quoteapiv2.APIError{
		Path:    "/quote-api/quote-v2/get-stock-list",
		URL:     "https://www.moomoo.com/quote-api/quote-v2/get-stock-list?marketType=4",
		Message: "响应解析失败: invalid character '<' looking for beginning of value",
		Body:    "<!DOCTYPE html>\n<html lang=\"en\">\n<head><title>403 - Operations too frequent</title></head>",
	}
	got := describeMoomooError(raw)
	text := got.Error()
	for _, want := range []string{"HTML 页面", "风控", "Operations too frequent", "响应片段"} {
		if !strings.Contains(text, want) {
			t.Errorf("错误信息缺少 %q：%s", want, text)
		}
	}
	if strings.Contains(text, "\n") {
		t.Errorf("响应片段应压成单行，实际含换行：%q", text)
	}
	// 原始错误仍可用 errors.As 取出，上层分支（如重试判定）不受影响
	var apiErr *quoteapiv2.APIError
	if !errors.As(got, &apiErr) {
		t.Error("包装后应仍能 errors.As 出 *quoteapiv2.APIError")
	}
}

// TestDescribeMoomooErrorEmptyBody 空响应且无状态码 → 提示网关拦截。
func TestDescribeMoomooErrorEmptyBody(t *testing.T) {
	raw := &quoteapiv2.APIError{
		Path:    "/quote-api/quote-v2/get-stock-list",
		URL:     "https://www.moomoo.com/quote-api/quote-v2/get-stock-list",
		Message: "响应解析失败: unexpected end of JSON input",
	}
	text := describeMoomooError(raw).Error()
	if !strings.Contains(text, "空响应") {
		t.Errorf("空响应应提示网关/风控拦截：%s", text)
	}
}

// TestDescribeMoomooErrorPassthrough 业务码错误与普通错误原样返回，不改文案。
func TestDescribeMoomooErrorPassthrough(t *testing.T) {
	business := &quoteapiv2.APIError{
		Path: "/quote-api/quote-v2/get-stock-list",
		URL:  "https://www.moomoo.com/quote-api/quote-v2/get-stock-list",
		Code: -403, Message: "Params Error",
	}
	if got := describeMoomooError(business); got.Error() != business.Error() {
		t.Errorf("业务码错误不应改写：%s != %s", got.Error(), business.Error())
	}
	plain := errors.New("network unreachable")
	if got := describeMoomooError(plain); got != plain {
		t.Errorf("普通错误应原样返回，实际 %v", got)
	}
	if got := describeMoomooError(nil); got != nil {
		t.Errorf("nil 应返回 nil，实际 %v", got)
	}
}
