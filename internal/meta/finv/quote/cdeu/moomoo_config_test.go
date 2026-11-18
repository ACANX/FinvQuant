package cdeu

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMoomooConfigConfigured 两项都非空才算已配置。
func TestMoomooConfigConfigured(t *testing.T) {
	cases := []struct {
		name string
		cfg  MoomooConfig
		want bool
	}{
		{"两项都空", MoomooConfig{}, false},
		{"只有 AppKey", MoomooConfig{AppKey: "k"}, false},
		{"只有私钥", MoomooConfig{PrivateKeyPEM: "p"}, false},
		{"两项都有", MoomooConfig{AppKey: "k", PrivateKeyPEM: "p"}, true},
		{"空白视为空", MoomooConfig{AppKey: "  ", PrivateKeyPEM: "p"}, false},
	}
	for _, tc := range cases {
		if got := tc.cfg.Configured(); got != tc.want {
			t.Fatalf("%s: Configured() = %v，期望 %v", tc.name, got, tc.want)
		}
	}
}

// TestMoomooConfigMaskedPrivateKey 私钥掩码不泄露任何字符。
func TestMoomooConfigMaskedPrivateKey(t *testing.T) {
	cfg := MoomooConfig{PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\nABCDEFGHIJKLMNOP\n-----END PRIVATE KEY-----"}
	masked := cfg.MaskedPrivateKey()
	if masked != "****" {
		t.Fatalf("掩码不符: %q", masked)
	}
	if strings.Contains(masked, "ABCDEFGH") || strings.Contains(masked, "PRIVATE") {
		t.Fatalf("掩码泄露了私钥内容: %q", masked)
	}
	if got := (MoomooConfig{}).MaskedPrivateKey(); got != "" {
		t.Fatalf("未配置时应返回空串，实际 %q", got)
	}
	if got := (MoomooConfig{PrivateKeyPEM: "abc"}).MaskedPrivateKey(); got != "****" {
		t.Fatalf("短私钥应返回 ****，实际 %q", got)
	}
}

// TestLiveMoomooConfigSaveLoad 真实库读写 sys_config 20016/20017（默认跳过）。
//
// 会临时覆盖凭据配置，结束后恢复原值（含「原本没有该行」的情况）。
//
//	FINV_CRYPTO_RUN_LIVE=1 FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveMoomooConfigSaveLoad -v ./internal/meta/finv/quote/cdeu/
func TestLiveMoomooConfigSaveLoad(t *testing.T) {
	if os.Getenv("FINV_CRYPTO_RUN_LIVE") != "1" {
		t.Skip("未设置 FINV_CRYPTO_RUN_LIVE=1，跳过 moomoo 配置真实库测试")
	}
	pool := testPool(t)
	store := NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 记录原值以便恢复
	before, err := store.LoadMoomooConfig(ctx)
	if err != nil {
		t.Fatalf("读取原配置失败: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(),
			`UPDATE sys.sys_config SET str_value = $2 WHERE config_no = $1`,
			SysConfigMoomooCredential, marshalMoomooCredential(before.AppKey, before.PrivateKeyPEM))
	}()

	// 动态生成一把合法 Ed25519 私钥（不使用任何真实凭据）
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试私钥失败: %v", err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	if err := store.SaveMoomooConfig(ctx, "test-app-key", keyPEM); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	cfg, err := store.LoadMoomooConfig(ctx)
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}
	if cfg.AppKey != "test-app-key" || !cfg.Configured() {
		t.Fatalf("保存后配置不符: %+v", cfg)
	}
	if cfg.MaskedPrivateKey() != "****" {
		t.Fatalf("掩码不符: %q", cfg.MaskedPrivateKey())
	}

	// 客户端可从配置构造（私钥合法）
	if _, err := store.MoomooClientFromConfig(ctx); err != nil {
		t.Fatalf("从配置构造客户端失败: %v", err)
	}

	// 私钥留空 = 保留原值
	if err := store.SaveMoomooConfig(ctx, "test-app-key-2", ""); err != nil {
		t.Fatalf("留空保存失败: %v", err)
	}
	if cfg, _ = store.LoadMoomooConfig(ctx); !cfg.Configured() || cfg.AppKey != "test-app-key-2" {
		t.Fatalf("留空不应清空私钥: %+v", cfg)
	}

	// 非法私钥应被拒绝（不写库）
	if err := store.SaveMoomooConfig(ctx, "test-app-key-2", "not-a-pem"); err == nil {
		t.Fatal("非法私钥应报错")
	}

	// __CLEAR__ 清空私钥
	if err := store.SaveMoomooConfig(ctx, "test-app-key-2", "__CLEAR__"); err != nil {
		t.Fatalf("清空私钥失败: %v", err)
	}
	if cfg, _ = store.LoadMoomooConfig(ctx); cfg.Configured() {
		t.Fatalf("清空后应视为未配置: %+v", cfg)
	}
	t.Logf("验证通过：sys_config %d 单配置项读写、留空保留、非法拒绝、清空均符合预期",
		SysConfigMoomooCredential)
}
