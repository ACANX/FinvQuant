package webapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

// TestSigningText 签名原文的 5 段格式与 body 摘要。
func TestSigningText(t *testing.T) {
	body := []byte(`{"code_list":["CC.BTCUSD"]}`)
	sum := sha256.Sum256(body)
	want := "1782357937000\nPOST\n/api/v1.0/quote/snapshot\n\n" + hex.EncodeToString(sum[:])
	if got := SigningText("1782357937000", "POST", "/api/v1.0/quote/snapshot", "", body); got != want {
		t.Fatalf("签名原文不符:\n got=%q\nwant=%q", got, want)
	}

	// 无 body / 无 query：仍保留 5 段（末尾为空）
	got := SigningText("1", "GET", "/api/v1.0/server-time", "", nil)
	if got != "1\nGET\n/api/v1.0/server-time\n\n" {
		t.Fatalf("无 body 的签名原文不符: %q", got)
	}

	// 有 query：原样拼在第 4 段
	got = SigningText("2", "GET", "/api/v1.0/quote/trading-days", "market=HK&start=2025-12-22", nil)
	if !strings.Contains(got, "\nmarket=HK&start=2025-12-22\n") {
		t.Fatalf("query 段不符: %q", got)
	}
}

// TestParseEd25519PrivateKey 支持标准 PEM 与字面 \n 两种写法。
func TestParseEd25519PrivateKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	parsed, err := ParseEd25519PrivateKey(pemText)
	if err != nil {
		t.Fatalf("解析 PEM 失败: %v", err)
	}
	if !parsed.Equal(priv) {
		t.Fatal("解析出的私钥与原私钥不一致")
	}

	escaped := strings.ReplaceAll(strings.TrimSpace(pemText), "\n", `\n`)
	parsed, err = ParseEd25519PrivateKey(escaped)
	if err != nil {
		t.Fatalf("解析字面 \\n 私钥失败: %v", err)
	}
	if !parsed.Equal(priv) {
		t.Fatal("字面 \\n 解析结果不一致")
	}

	if _, err := ParseEd25519PrivateKey("not a pem"); err == nil {
		t.Fatal("非法 PEM 应报错")
	}
}

// TestNewClientValidation 缺少 AppKey / 私钥时的报错。
func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient(Config{PrivateKeyPEM: "x"}); err == nil {
		t.Fatal("缺少 AppKey 应报错")
	}
	if _, err := NewClient(Config{AppKey: "k", PrivateKeyPEM: "x"}); err == nil {
		t.Fatal("非法私钥应报错")
	}
}

// TestGetQuoteSnapshotsRejectsTooManyCodes 单次 code 数上限校验（不发网络请求）。
func TestGetQuoteSnapshotsRejectsTooManyCodes(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	c, err := NewClient(Config{
		AppKey:        "test",
		PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	})
	if err != nil {
		t.Fatalf("构造客户端失败: %v", err)
	}
	codes := make([]string, MaxCodes+1)
	if _, err := c.GetQuoteSnapshots(context.Background(), codes); err == nil {
		t.Fatal("超过 MaxCodes 应报错")
	}
	if list, err := c.GetQuoteSnapshots(context.Background(), nil); err != nil || list != nil {
		t.Fatalf("空 code 应返回 nil/nil，实际 %v/%v", list, err)
	}
}

// TestSnapshotUnmarshal 快照字段解析（用真实响应片段，只保留本包用到的键）。
func TestSnapshotUnmarshal(t *testing.T) {
	raw := `{"ret_code":0,"ret_msg":"success","data":{"snapshot_list":[{
		"code":"CC.BTCUSD","name":"BTC/USD","last_price":64372.58,"open_price":64856.94,
		"high_price":64927.52,"low_price":64063.7,"prev_close_price":64874.99,
		"amplitude":1.332,"volume":22062,"turnover":1420895.14,"bid_ask_ratio":-96.796,
		"highest52weeks_price":97776,"lowest52weeks_price":57832.68,
		"sec_status":"NORMAL","suspension":false,"data_date":"2026-11-17",
		"update_time":1794971178277}]}}`
	var envelope snapshotEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if envelope.RetCode != 0 || envelope.Data == nil || len(envelope.Data.SnapshotList) != 1 {
		t.Fatalf("信封解析不符: %+v", envelope)
	}
	s := envelope.Data.SnapshotList[0]
	if s.Code != "CC.BTCUSD" || s.LastPrice != 64372.58 || s.UpdateTime != 1794971178277 || s.Suspension {
		t.Fatalf("字段解析不符: %+v", s)
	}
}
