package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/kline"
	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newTestRouterWithConfig(t, &config.Config{
		KLineMaxBatchRows: 5000, KLineMaxListSize: 5000, KLineTSUnit: "s",
	})
}

// newTestRouterWithConfig 用指定配置构造路由，便于覆盖外部 OpenAPI 等开关。
func newTestRouterWithConfig(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 惰性连接池：不会真正连上数据库，用于构造路由（请求数据库的端点会返回 503）。
	pool, err := pgxpool.New(context.Background(),
		"postgres://test:test@127.0.0.1:1/test?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("创建测试连接池失败: %v", err)
	}
	t.Cleanup(pool.Close)
	archiveService, err := dedup.NewService(pool, dedup.Options{UploadMode: dedup.UploadModeNone})
	if err != nil {
		t.Fatalf("创建归档服务失败: %v", err)
	}
	return NewRouter(&Deps{
		Pool:          pool,
		Redis:         nil,
		Version:       "test",
		Commit:        "test",
		Server:        "finvquant",
		Config:        cfg,
		ExportArchive: archiveService,
	})
}

// TestExportArchiveRoutesRegistered 校验行情月归档运维接口均已注册。
func TestExportArchiveRoutesRegistered(t *testing.T) {
	router := newTestRouter(t)
	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	expected := []string{
		"GET /API/V1/Meta/Finv/Quote/DeDup/Status",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Settings",
		"PUT /API/V1/Meta/Finv/Quote/DeDup/Settings/Window",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Job/Runtime",
		"GET /API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview",
		"POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run",
		"POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Terminate",
		"POST /API/V1/Meta/Finv/Quote/DeDup/CDEU/Process/Run",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Job/List",
		"GET /API/V1/Meta/Finv/Quote/DeDup/File/List",
		"POST /API/V1/Meta/Finv/Quote/DeDup/Job/Retry",
		"POST /API/V1/Meta/Finv/Quote/DeDup/File/Retry",
		"POST /API/V1/Meta/Finv/Quote/DeDup/Job/Run",
	}
	for _, route := range expected {
		if !registered[route] {
			t.Fatalf("路由未注册: %s", route)
		}
	}
}

// TestKLineRoutesRegistered 校验新前缀、旧前缀过渡别名与运维端点均已注册。
func TestKLineRoutesRegistered(t *testing.T) {
	router := newTestRouter(t)
	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	expected := []string{
		"POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Add",
		"POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch",
		"POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert",
		"GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Page",
		"GET /API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni",
		"POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Query",
		"PATCH /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch",
		"DELETE /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch",
		"GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Count",
		"GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Stats",
		"GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Bounds",
		// 运维端点
		"GET /Health",
		"GET /healthz",
		"GET /",
	}
	for _, route := range expected {
		if !registered[route] {
			t.Fatalf("路由未注册: %s", route)
		}
	}
	// 需求 10782：全部 K 线端点已迁到 CDEU，新/旧 KLine 前缀均不得再注册
	for _, gone := range []string{
		"POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Add",
		"POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch",
		"POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Upsert",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Page",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/PageUni",
		"POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query",
		"PATCH /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch",
		"DELETE /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Count",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats",
		"GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Bounds",
		"POST /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Add",
		"POST /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Batch",
		"POST /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Upsert",
		"GET /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Page",
		"GET /API/V1/Meta/Finv/Quote/Secu/KLine/Min/PageUni",
		"POST /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Query",
		"PATCH /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Batch",
		"DELETE /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Batch",
		"GET /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Count",
		"GET /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Stats",
		"GET /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Bounds",
	} {
		if registered[gone] {
			t.Fatalf("旧路径不应再注册: %s", gone)
		}
	}
	// 同一路径不同方法必须共存（Gin 不允许重复注册同一 方法+路径）。
	if !registered["POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch"] ||
		!registered["PATCH /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch"] ||
		!registered["DELETE /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch"] {
		t.Fatal("POST/PATCH/DELETE /Batch 未全部注册")
	}
}

func TestRootAndHealth(t *testing.T) {
	router := newTestRouter(t)

	// 服务标识
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/ 期望 200，实际 %d", recorder.Code)
	}
	var root map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &root); err != nil {
		t.Fatalf("/ 响应不是 JSON: %v", err)
	}
	if root["prefix"] != kline.APIPrefix {
		t.Fatalf("prefix 期望 %s，实际 %v", kline.APIPrefix, root["prefix"])
	}

	// 探活：无数据库连接池时应返回 503，且响应体不含内部细节。
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/Health", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("/Health 期望 503，实际 %d", recorder.Code)
	}
	var health map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &health)
	if health["status"] != "degraded" {
		t.Fatalf("status 期望 degraded，实际 %v", health["status"])
	}
	// 过渡别名行为一致。
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("/healthz 期望 503，实际 %d", recorder.Code)
	}
}

// TestOpenAPIExampleRoute 校验外部示例端点（需求 10787）：
//   - 未配置 FINV_OPENAPI_KEY 时不注册；
//   - 配置后注册，且 Bearer / X-API-Key 两种方式都可通过静态密钥校验；
//   - 业务逻辑复用内部 handler（空 items 返回 400 / 211001，说明已进入 handler）。
func TestOpenAPIExampleRoute(t *testing.T) {
	const path = "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert"

	plain := newTestRouter(t)
	for _, route := range plain.Routes() {
		if route.Path == path {
			t.Fatalf("未配置 FINV_OPENAPI_KEY 时不应注册 %s", path)
		}
	}

	router := newTestRouterWithConfig(t, &config.Config{
		KLineMaxBatchRows: 5000, KLineMaxListSize: 5000, KLineTSUnit: "s",
		OpenAPIKey: "test-openapi-key",
	})
	registered := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost && route.Path == path {
			registered = true
		}
	}
	if !registered {
		t.Fatalf("配置 FINV_OPENAPI_KEY 后应注册 %s", path)
	}

	do := func(headers map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"items":[]}`))
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	if got := do(nil); got != http.StatusUnauthorized {
		t.Fatalf("无密钥期望 401，实际 %d", got)
	}
	if got := do(map[string]string{"Authorization": "Bearer wrong-key"}); got != http.StatusUnauthorized {
		t.Fatalf("错误密钥期望 401，实际 %d", got)
	}
	if got := do(map[string]string{"Authorization": "Bearer test-openapi-key"}); got != http.StatusBadRequest {
		t.Fatalf("Bearer 正确密钥应进入 handler（空 items → 400），实际 %d", got)
	}
	if got := do(map[string]string{"X-API-Key": "test-openapi-key"}); got != http.StatusBadRequest {
		t.Fatalf("X-API-Key 正确密钥应进入 handler（空 items → 400），实际 %d", got)
	}
}

// TestWrapOpenAPIResponseEnvelope 校验外部 OpenAPI 成功响应统一为
// {"code":0,"data":<原始 JSON>,"message":"OK"}（成功码默认 0）；非 2xx 原样透传；204 不包装。
func TestWrapOpenAPIResponseEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WrapOpenAPIResponse())
	router.POST("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"affected": 1, "mode": "update"})
	})
	router.POST("/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/err", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 211001, "message": "缺少必填参数"})
	})

	serve := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		return recorder
	}

	ok := serve("/ok")
	if ok.Code != http.StatusOK {
		t.Fatalf("ok 期望 200，实际 %d", ok.Code)
	}
	var envelope map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析信封失败: %v", err)
	}
	if envelope["code"] != float64(0) || envelope["message"] != "OK" {
		t.Fatalf("信封 code/message 不符合预期: %v", envelope)
	}
	data, isMap := envelope["data"].(map[string]any)
	if !isMap || data["affected"] != float64(1) || data["mode"] != "update" {
		t.Fatalf("data 节点不符合预期: %v", envelope["data"])
	}

	if noContent := serve("/empty"); noContent.Code != http.StatusNoContent || noContent.Body.Len() != 0 {
		t.Fatalf("204 应原样无 body，实际 %d %q", noContent.Code, noContent.Body.String())
	}

	bad := serve("/err")
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), `"code":211001`) {
		t.Fatalf("错误响应应原样透传，实际 %d %s", bad.Code, bad.Body.String())
	}
}
