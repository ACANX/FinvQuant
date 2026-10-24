package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/exportarchive"
	"github.com/acanx/finvquant/internal/kline"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 惰性连接池：不会真正连上数据库，用于构造路由（请求数据库的端点会返回 503）。
	pool, err := pgxpool.New(context.Background(),
		"postgres://test:test@127.0.0.1:1/test?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("创建测试连接池失败: %v", err)
	}
	t.Cleanup(pool.Close)
	archiveService, err := exportarchive.NewService(pool, exportarchive.Options{UploadMode: exportarchive.UploadModeNone})
	if err != nil {
		t.Fatalf("创建归档服务失败: %v", err)
	}
	return NewRouter(&Deps{
		Pool:          pool,
		Redis:         nil,
		Version:       "test",
		Commit:        "test",
		Server:        "finvquant",
		Config:        &config.Config{KLineMaxBatchRows: 5000, KLineMaxListSize: 5000, KLineTSUnit: "s"},
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
		// 旧前缀过渡别名
		"POST /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Add",
		"PATCH /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Batch",
		"DELETE /API/V1/Meta/Finv/Quote/Secu/KLine/Min/Batch",
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
	// 同一路径不同方法必须共存（Gin 不允许重复注册同一 方法+路径）。
	if !registered["POST /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch"] ||
		!registered["PATCH /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch"] ||
		!registered["DELETE /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Batch"] {
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
