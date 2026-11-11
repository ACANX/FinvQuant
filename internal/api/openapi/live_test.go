package openapi

// 端到端联调测试（默认跳过）：真库 + 真 Redis + 真 HTTP 服务。
//
// 前置：
//
//	FINV_OPENAPI_LIVE=1
//	FINV_TEST_PG_DSN=postgres://finv:...@127.0.0.1:5432/finv?sslmode=disable
//	FINV_OPENAPI_BASE=http://127.0.0.1:16001   （运行中的服务地址）
//
// 该测试会：
//   - 临时把 sys_config 19001 置为 '1'（强制鉴权）与 19005（临时密钥），结束后**恢复原值**；
//   - 写入它自己的 client/credential（编码前缀 `zz-oa-live-`），结束后删除；
//   - 只调用只读接口 `GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni`，不写业务表。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func liveEnv(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	if os.Getenv("FINV_OPENAPI_LIVE") != "1" {
		t.Skip("未设置 FINV_OPENAPI_LIVE=1，跳过端到端联调测试")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过端到端联调测试")
	}
	base := strings.TrimRight(os.Getenv("FINV_OPENAPI_BASE"), "/")
	if base == "" {
		base = "http://127.0.0.1:16001"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, base
}

// getConfig 读单个配置的原始值（用于恢复现场）。
func getConfig(t *testing.T, pool *pgxpool.Pool, configNo int) (charValue, strValue string, exists bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(char_value, ''), COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = $1`,
		configNo).Scan(&charValue, &strValue)
	if err != nil {
		return "", "", false
	}
	return charValue, strValue, true
}

func setConfig(t *testing.T, pool *pgxpool.Pool, configNo int, charValue, strValue string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE sys.sys_config SET char_value = $2, str_value = $3 WHERE config_no = $1`,
		configNo, charValue, strValue); err != nil {
		t.Fatalf("写入 sys_config %d 失败: %v", configNo, err)
	}
}

// waitSettingsReload 等配置缓存过期（10 秒 TTL，留 2 秒余量）。
func waitSettingsReload() { time.Sleep(12 * time.Second) }

func httpGet(t *testing.T, url, apiKey, authorization string, extra map[string]string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if apiKey != "" {
		request.Header.Set("X-API-Key", apiKey)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	for key, value := range extra {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}

// TestLiveOpenAPIAuthEndToEnd 静态 Key + AK/SK 两条链路走真服务验证。
func TestLiveOpenAPIAuthEndToEnd(t *testing.T) {
	pool, base := liveEnv(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	testStart := time.Now().Add(-time.Minute)
	clientCode := "zz-oa-live-" + suffix
	staticKey := "SA_LIVE_" + suffix
	ak := "AK_LIVE_" + suffix
	sk := "SK_LIVE_" + suffix

	// ---- 现场记录，defer 恢复 ----
	modeChar, modeStr, _ := getConfig(t, pool, 19001)
	kidChar, kidStr, _ := getConfig(t, pool, 19004)
	keyChar, keyStr, _ := getConfig(t, pool, 19005)
	key := testKey()

	var clientIDN, staticIDN, akskIDN int64
	t.Cleanup(func() {
		setConfig(t, pool, 19001, modeChar, modeStr)
		setConfig(t, pool, 19004, kidChar, kidStr)
		setConfig(t, pool, 19005, keyChar, keyStr)
		// 审计清理按「本次测试时间窗 + 本测试调用方（含未识别凭证的 0）」，
		// 因为「未携带凭证」「错 Key」这两类请求落库时没有 access_key_id，按 key 删不掉。
		_, _ = pool.Exec(ctx,
			`DELETE FROM open.open_api_invoke_log WHERE dt_request >= $1 AND (client_idn = $2 OR client_idn = 0)`,
			testStart, clientIDN)
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE client_idn = $1`, clientIDN)
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_client WHERE idn = $1`, clientIDN)
		t.Logf("已恢复 sys_config 19001/19004/19005 并清理测试数据")
	})

	if err := pool.QueryRow(ctx,
		`INSERT INTO open.open_api_client (client_code, client_name, flag_enable, rate_limit_rpm, remark)
		 VALUES ($1, '联调测试调用方', '1', 600, 'FINV_OPENAPI_LIVE 自动化测试，可删') RETURNING idn`,
		clientCode).Scan(&clientIDN); err != nil {
		t.Fatalf("插入测试调用方失败: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO open.open_api_credential (client_idn, credential_no, auth_mode, access_key_id, key_prefix, secret_hash, flag_enable)
		 VALUES ($1, $2, 'API_KEY', $3, $4, $5, '1') RETURNING idn`,
		clientIDN, "cred-static-"+suffix, staticKey, staticKey[:8], HashStaticKey(staticKey)).Scan(&staticIDN); err != nil {
		t.Fatalf("插入静态 Key 凭证失败: %v", err)
	}

	cipherText, err := EncryptSecret(sk, key)
	if err != nil {
		t.Fatalf("加密 SK 失败: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO open.open_api_credential (client_idn, credential_no, auth_mode, access_key_id, secret_cipher, secret_kid, sign_algorithm, flag_enable)
		 VALUES ($1, $2, 'AK_SK', $3, $4, 'k1', 'HMAC-SHA256', '1') RETURNING idn`,
		clientIDN, "cred-aksk-"+suffix, ak, cipherText).Scan(&akskIDN); err != nil {
		t.Fatalf("插入 AK/SK 凭证失败: %v", err)
	}

	// ---- 模式 1：强制鉴权 ----
	setConfig(t, pool, 19001, "1", "")
	setConfig(t, pool, 19005, "", encodeBase64(key))
	waitSettingsReload()

	target := base + "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni?column=usc&row=AAPL&limit=1"

	// 无凭证 → 19001
	status, body := httpGet(t, target, "", "", nil)
	if status != http.StatusUnauthorized || !strings.Contains(body, "19001") {
		t.Fatalf("无凭证应 401/19001，实际 %d %s", status, body)
	}
	t.Logf("无凭证 → %d %s", status, strings.TrimSpace(body))

	// 错 Key → 19002
	status, body = httpGet(t, target, "SA_LIVE_WRONG"+suffix, "", nil)
	if status != http.StatusUnauthorized || !strings.Contains(body, "19002") {
		t.Fatalf("错 Key 应 401/19002，实际 %d %s", status, body)
	}
	t.Logf("错 Key → %d %s", status, strings.TrimSpace(body))

	// 正确静态 Key → 200 且响应为 OpenAPI 信封
	status, body = httpGet(t, target, staticKey, "", nil)
	if status != http.StatusOK || !strings.Contains(body, `"code":0`) {
		t.Fatalf("正确静态 Key 应 200/信封 code=0，实际 %d %s", status, body)
	}
	t.Logf("静态 Key → %d %s", status, truncate(body, 120))

	// AK/SK 正确签名 → 200
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "live-" + suffix
	path := "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni"
	canonical := CanonicalString(SignRequest{
		Method: http.MethodGet, Path: path, RawQuery: "column=usc&row=AAPL&limit=1",
		BodyHash: BodyHash(nil), Timestamp: timestamp, Nonce: nonce,
	})
	signature, err := Sign("HMAC-SHA256", sk, canonical)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	status, body = httpGet(t, target, "", "META1-HMAC-SHA256 Credential="+ak+", Signature="+signature, map[string]string{
		HeaderTimestamp: timestamp,
		HeaderNonce:     nonce,
	})
	if status != http.StatusOK {
		t.Fatalf("AK/SK 正确签名应 200，实际 %d %s", status, body)
	}
	t.Logf("AK/SK → %d", status)

	// 重放同一 nonce → 19007
	status, body = httpGet(t, target, "", "META1-HMAC-SHA256 Credential="+ak+", Signature="+signature, map[string]string{
		HeaderTimestamp: timestamp,
		HeaderNonce:     nonce,
	})
	if status != http.StatusUnauthorized || !strings.Contains(body, "19007") {
		t.Fatalf("重放 nonce 应 401/19007，实际 %d %s", status, body)
	}
	t.Logf("nonce 重放 → %d %s", status, strings.TrimSpace(body))

	// 吊销静态凭证 → 19003
	if _, err := pool.Exec(ctx,
		`UPDATE open.open_api_credential SET flag_revoke = '1', dt_revoke = now(), reason_revoke = '联调测试' WHERE idn = $1`,
		staticIDN); err != nil {
		t.Fatalf("吊销测试凭证失败: %v", err)
	}
	status, body = httpGet(t, target, staticKey, "", nil)
	if status != http.StatusForbidden || !strings.Contains(body, "19003") {
		t.Fatalf("已吊销凭证应 403/19003，实际 %d %s", status, body)
	}
	t.Logf("已吊销 → %d %s", status, strings.TrimSpace(body))

	// 审计日志落库（放行与拒绝都要有）
	var passCount, denyCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE flag_pass = '1'), count(*) FILTER (WHERE flag_pass = '0')
		   FROM open.open_api_invoke_log WHERE access_key_id IN ($1, $2)`,
		staticKey, ak).Scan(&passCount, &denyCount); err != nil {
		t.Fatalf("查询审计日志失败: %v", err)
	}
	if passCount < 2 || denyCount < 1 {
		t.Fatalf("审计日志应既有放行也有拒绝，实际 pass=%d deny=%d", passCount, denyCount)
	}
	t.Logf("审计日志：pass=%d deny=%d", passCount, denyCount)

	// ---- 模式 0：关闭鉴权，无凭证也放行 ----
	setConfig(t, pool, 19001, "0", "")
	waitSettingsReload()
	status, body = httpGet(t, target, "", "", nil)
	if status != http.StatusOK {
		t.Fatalf("模式 0 应放行，实际 %d %s", status, body)
	}
	t.Logf("模式 0 无凭证 → %d（放行）", status)
	_ = clientIDN
	_ = akskIDN
	_ = net.IPv4len
	_ = json.Valid
}
