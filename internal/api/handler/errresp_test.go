package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/kline"
)

func init() { gin.SetMode(gin.TestMode) }

var requestIDPattern = regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)

// newErrRouter 构建一个只用于错误响应测试的路由。
func newErrRouter(responder *ErrorResponder, err error) *gin.Engine {
	router := gin.New()
	router.GET("/boom", func(c *gin.Context) {
		responder.Respond(c, err)
	})
	return router
}

func doRequest(router *gin.Engine, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	request := httptest.NewRequest(http.MethodGet, "/boom", nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var body map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

func TestErrorResponderSafeByDefault(t *testing.T) {
	err := kline.New(kline.CodeDuplicateKey).
		WithDebug("constraint", "finv_quote_secu_kline_min_pkey").
		WithDebug("sqlstate", "23505")
	router := newErrRouter(NewErrorResponder(&config.Config{}), err)
	recorder, body := doRequest(router, nil)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("HTTP 期望 409，实际 %d", recorder.Code)
	}
	if body["code"].(float64) != kline.CodeDuplicateKey {
		t.Fatalf("code 期望 %d，实际 %v", kline.CodeDuplicateKey, body["code"])
	}
	if _, ok := body["debug"]; ok {
		t.Fatalf("safe 模式不得输出 debug 字段: %v", body["debug"])
	}
	if body["detail"] != body["message"] {
		t.Fatalf("safe 模式 detail 应等于安全文案: %v", body["detail"])
	}
	errorObject := body["error"].(map[string]any)
	if errorObject["catalog_version"] != kline.CatalogVersion {
		t.Fatalf("缺少 catalog_version: %v", errorObject)
	}
	requestID, _ := body["request_id"].(string)
	if !requestIDPattern.MatchString(requestID) {
		t.Fatalf("request_id 应为大写 UUID4，实际 %q", requestID)
	}
	if strings.ToUpper(requestID) != requestID {
		t.Fatalf("request_id 不应含小写字母: %q", requestID)
	}
	if strings.Contains(recorder.Body.String(), "pkey") {
		t.Fatal("safe 模式下响应体泄漏了约束名")
	}
}

func TestErrorResponderAdminWithToken(t *testing.T) {
	cfg := &config.Config{ErrorDetailMode: "admin", ErrorDebugToken: "s3cr3t"}
	err := kline.New(kline.CodeDuplicateKey).WithDebug("constraint", "pk_test").WithDebug("sqlstate", "23505")
	router := newErrRouter(NewErrorResponder(cfg), err)

	// 缺少令牌 → safe
	_, body := doRequest(router, nil)
	if _, ok := body["debug"]; ok {
		t.Fatal("缺少令牌时不得输出 debug")
	}

	// 错误令牌 → safe，且不得提示令牌错误
	_, body = doRequest(router, map[string]string{"X-Debug-Token": "wrong"})
	if _, ok := body["debug"]; ok {
		t.Fatal("错误令牌时不得输出 debug")
	}
	if strings.Contains(strings.ToLower(body["message"].(string)), "token") {
		t.Fatalf("不得提示令牌错误: %v", body["message"])
	}

	// 正确令牌 → 输出白名单 debug 字段
	_, body = doRequest(router, map[string]string{"X-Debug-Token": "s3cr3t"})
	debug, ok := body["debug"].(map[string]any)
	if !ok {
		t.Fatalf("admin 模式应输出 debug，实际 %v", body)
	}
	if debug["constraint"] != "pk_test" {
		t.Fatalf("debug.constraint 期望 pk_test，实际 %v", debug["constraint"])
	}
	if _, leaked := debug["internal"]; leaked {
		t.Fatal("debug 只允许白名单字段")
	}
}

func TestErrorResponderAdminRequiresMode(t *testing.T) {
	// 仅配置令牌但未开启 admin 模式 → safe
	cfg := &config.Config{ErrorDetailMode: "safe", ErrorDebugToken: "s3cr3t"}
	err := kline.New(kline.CodeDBError).WithDebug("err_class", "pgx.error")
	router := newErrRouter(NewErrorResponder(cfg), err)
	_, body := doRequest(router, map[string]string{"X-Debug-Token": "s3cr3t"})
	if _, ok := body["debug"]; ok {
		t.Fatal("未开启 admin 模式时不得输出 debug")
	}
}

func TestErrorResponderUnknownErrorMasked(t *testing.T) {
	router := newErrRouter(NewErrorResponder(&config.Config{}), assertErr{})
	recorder, body := doRequest(router, nil)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("未知错误应为 500，实际 %d", recorder.Code)
	}
	if body["message"] != "服务内部错误，请稍后重试" {
		t.Fatalf("安全文案不符: %v", body["message"])
	}
	if strings.Contains(recorder.Body.String(), "dsn") {
		t.Fatal("响应体泄漏内部细节")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "connect dsn=postgres://user:pass@host/db failed" }
