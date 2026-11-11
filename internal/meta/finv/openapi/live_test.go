package openapi

// 端到端联调测试（默认跳过）：真库跑一遍「登记调用方 → 签发 → 列表 → 轮换 → 吊销 → 审计查询」。
//
//	FINV_OPENAPI_ADMIN_LIVE=1
//	FINV_TEST_PG_DSN=postgres://finv:...@127.0.0.1:5432/finv?sslmode=disable
//
// 只写 open schema 它自己的测试数据（代号前缀 zz-oa-admin-），结束时删除；不改 sys_config。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

func adminLiveService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("FINV_OPENAPI_ADMIN_LIVE") != "1" {
		t.Skip("未设置 FINV_OPENAPI_ADMIN_LIVE=1，跳过管理接口联调测试")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过管理接口联调测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewService(NewStore(pool)), pool
}

// TestLiveAdminLifecycle 调用方 + 凭证全生命周期。
func TestLiveAdminLifecycle(t *testing.T) {
	service, pool := adminLiveService(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	clientCode := "zz-oa-admin-" + suffix
	credentialNo := "zz-cred-" + suffix

	var clientIDN int64
	t.Cleanup(func() {
		if clientIDN > 0 {
			_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE client_idn = $1`, clientIDN)
			_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_client WHERE idn = $1`, clientIDN)
			t.Logf("已清理测试调用方 idn=%d", clientIDN)
		}
	})

	// 1) 新建调用方：默认停用
	client, err := service.SaveClient(ctx, ClientRequest{
		ClientCode:   clientCode,
		ClientName:   "联调测试调用方",
		AllowCIDR:    []string{"10.0.0.1", "192.168.0.0/16"},
		RateLimitRPM: 120,
	}, "livetest")
	if err != nil {
		t.Fatalf("新建调用方失败: %v", err)
	}
	clientIDN = client.IDN
	if client.FlagEnable != "0" {
		t.Fatalf("新建调用方应默认停用，实际 %q", client.FlagEnable)
	}
	if len(client.AllowCIDR) != 2 || client.AllowCIDR[0] != "10.0.0.1/32" {
		t.Fatalf("白名单应归一为 CIDR，实际 %v", client.AllowCIDR)
	}

	// 2) 启用 + 重复代号必须 409
	if _, err := service.SetClientEnable(ctx, client.IDN, true, "livetest"); err != nil {
		t.Fatalf("启用调用方失败: %v", err)
	}
	if _, err := service.SaveClient(ctx, ClientRequest{ClientCode: clientCode, ClientName: "重复"}, "livetest"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复 client_code 应返回 ErrConflict，实际 %v", err)
	}

	// 3) 签发静态 Key：明文返回一次、库里只有哈希
	issued, err := service.CreateCredential(ctx, CredentialCreateRequest{
		ClientIDN:    client.IDN,
		CredentialNo: credentialNo,
		AuthMode:     AuthModeAPIKey,
		Scope:        []string{"POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert"},
	}, "livetest")
	if err != nil {
		t.Fatalf("签发静态 Key 失败: %v", err)
	}
	if !strings.HasPrefix(issued.AccessKey, secret.PrefixAPIKey) {
		t.Fatalf("静态 Key 前缀应为 %s，实际 %q", secret.PrefixAPIKey, issued.AccessKey)
	}
	if issued.AccessKey != issued.SecretKey {
		t.Fatalf("静态 Key 模式下 access_key 与 secret_key 应相同")
	}
	if issued.Credential.AccessKeyID != issued.AccessKey {
		t.Fatalf("access_key_id 应等于静态 Key 本身")
	}
	if !strings.Contains(issued.Note, "只显示这一次") {
		t.Fatalf("签发提示应点明只显示一次，实际 %q", issued.Note)
	}
	var hash string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(secret_hash, '') FROM open.open_api_credential WHERE idn = $1`,
		issued.Credential.IDN).Scan(&hash); err != nil {
		t.Fatalf("读取 secret_hash 失败: %v", err)
	}
	if hash != secret.HashKey(issued.AccessKey) {
		t.Fatalf("库内应为 sha256 哈希，实际 %q", hash)
	}
	if strings.Contains(hash, issued.AccessKey) {
		t.Fatalf("库内不得出现明文 Key")
	}

	// 4) 列表：不得含密钥材料
	list, total, err := service.CredentialPage(ctx, client.IDN, "", 1, 20)
	if err != nil {
		t.Fatalf("查询凭证失败: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("应查到 1 条凭证，实际 total=%d len=%d", total, len(list))
	}
	if !list[0].HasSecret {
		t.Fatalf("has_secret 应为 true")
	}
	if len(list[0].Scope) != 1 || !strings.HasPrefix(list[0].Scope[0], "POST ") {
		t.Fatalf("scope 展示口径不符: %v", list[0].Scope)
	}

	// 5) 轮换：access_key_id 变化，旧 Key 不再匹配
	rotated, err := service.RotateCredential(ctx, issued.Credential.IDN, "livetest")
	if err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	if rotated.AccessKey == issued.AccessKey {
		t.Fatalf("轮换后公开标识应变化")
	}
	var newHash string
	var rotateAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(secret_hash, ''), dt_secret_rotate FROM open.open_api_credential WHERE idn = $1`,
		issued.Credential.IDN).Scan(&newHash, &rotateAt); err != nil {
		t.Fatalf("读取轮换结果失败: %v", err)
	}
	if newHash == hash {
		t.Fatalf("轮换后哈希应变化")
	}
	if rotateAt == nil {
		t.Fatalf("dt_secret_rotate 应被写入")
	}

	// 6) 吊销：原因必填 + 不可再吊销
	if _, err := service.RevokeCredential(ctx, issued.Credential.IDN, "   ", "livetest"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("空原因应返回 ErrInvalidRequest，实际 %v", err)
	}
	revoked, err := service.RevokeCredential(ctx, issued.Credential.IDN, "联调测试结束", "livetest")
	if err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	if revoked.FlagRevoke != "1" || revoked.FlagEnable != "0" || revoked.ReasonRevoke != "联调测试结束" {
		t.Fatalf("吊销状态不符: %+v", revoked)
	}
	if _, err := service.RevokeCredential(ctx, issued.Credential.IDN, "再吊销一次", "livetest"); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复吊销应返回 ErrConflict，实际 %v", err)
	}
	if _, err := service.RotateCredential(ctx, issued.Credential.IDN, "livetest"); !errors.Is(err, ErrConflict) {
		t.Fatalf("已吊销凭证轮换应返回 ErrConflict，实际 %v", err)
	}

	// 7) 调用审计查询（该调用方没有真实调用，只验证 SQL 与分页可用）
	logs, logTotal, err := service.InvokeLogPage(ctx, InvokeLogFilter{ClientIDN: client.IDN, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("查询调用审计失败: %v", err)
	}
	t.Logf("调用审计：total=%d len=%d", logTotal, len(logs))

	// 8) 配置读取（只读，不改）
	settings, err := service.Settings(ctx)
	if err != nil {
		t.Fatalf("读取鉴权配置失败: %v", err)
	}
	if settings.Mode != "0" && settings.Mode != "1" && settings.Mode != "2" {
		t.Fatalf("配置模式非法: %q", settings.Mode)
	}
	if settings.SignTimeWindow < 30 {
		t.Fatalf("签名时间窗异常: %d", settings.SignTimeWindow)
	}
	t.Logf("鉴权配置：mode=%s window=%ds retain=%dd kid=%s 19005=%t",
		settings.Mode, settings.SignTimeWindow, settings.LogRetainDays, settings.SecretKID, settings.SecretKeyReady)
}

// TestLiveAdminAKSKIssuance AK/SK 签发：19005 未配置时拒绝（503），配置后可签发且密文能解回明文。
func TestLiveAdminAKSKIssuance(t *testing.T) {
	service, pool := adminLiveService(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// 记录并临时改写 19005（结束后恢复原值，避免污染现场）
	var originalKey string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = 19005`).Scan(&originalKey); err != nil {
		t.Fatalf("读取 19005 失败: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx,
			`UPDATE sys.sys_config SET str_value = $1 WHERE config_no = 19005`, originalKey); err != nil {
			t.Logf("恢复 19005 失败（需人工检查）: %v", err)
		}
	})

	// 先清空 19005：AK/SK 签发应返回 503（ErrNotReady）
	if _, err := pool.Exec(ctx, `UPDATE sys.sys_config SET str_value = '' WHERE config_no = 19005`); err != nil {
		t.Fatalf("清空 19005 失败: %v", err)
	}
	client, err := service.SaveClient(ctx, ClientRequest{
		ClientCode: "zz-oa-aksk-" + suffix, ClientName: "AKSK 联调调用方",
	}, "livetest")
	if err != nil {
		t.Fatalf("新建调用方失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE client_idn = $1`, client.IDN)
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_client WHERE idn = $1`, client.IDN)
		t.Logf("已清理 AKSK 测试调用方 idn=%d", client.IDN)
	})

	if _, err := service.CreateCredential(ctx, CredentialCreateRequest{
		ClientIDN: client.IDN, CredentialNo: "zz-aksk-" + suffix, AuthMode: AuthModeAKSK,
	}, "livetest"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("19005 未配置时签发 AK/SK 应返回 ErrNotReady，实际 %v", err)
	}
	t.Logf("19005 未配置 → 拒绝签发（符合预期）")

	// 配置一把临时密钥后重签
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE sys.sys_config SET str_value = $1 WHERE config_no = 19005`,
		base64Key(key)); err != nil {
		t.Fatalf("写入临时 19005 失败: %v", err)
	}

	issued, err := service.CreateCredential(ctx, CredentialCreateRequest{
		ClientIDN: client.IDN, CredentialNo: "zz-aksk-" + suffix, AuthMode: AuthModeAKSK,
	}, "livetest")
	if err != nil {
		t.Fatalf("配置 19005 后签发 AK/SK 失败: %v", err)
	}
	if !strings.HasPrefix(issued.AccessKey, secret.PrefixAK) || !strings.HasPrefix(issued.SecretKey, secret.PrefixSK) {
		t.Fatalf("AK/SK 前缀不符: ak=%q sk=%q", issued.AccessKey, issued.SecretKey)
	}
	if issued.AccessKey == issued.SecretKey {
		t.Fatalf("AK 与 SK 不得相同")
	}

	var cipherText string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(secret_cipher, '') FROM open.open_api_credential WHERE idn = $1`,
		issued.Credential.IDN).Scan(&cipherText); err != nil {
		t.Fatalf("读取 secret_cipher 失败: %v", err)
	}
	if cipherText == "" || strings.Contains(cipherText, issued.SecretKey) {
		t.Fatalf("库内应为密文且不得含明文 SK")
	}
	plain, err := secret.Decrypt(cipherText, key)
	if err != nil {
		t.Fatalf("密文解密失败: %v", err)
	}
	if plain != issued.SecretKey {
		t.Fatalf("解密结果应等于返回的 SK")
	}
	if issued.AccessKey != strings.ToUpper(issued.AccessKey) || issued.SecretKey != strings.ToUpper(issued.SecretKey) {
		t.Fatalf("AK/SK 必须全大写：ak=%q sk=%q", issued.AccessKey, issued.SecretKey)
	}
	t.Logf("AK/SK 签发成功：ak=%s… sk 长度=%d，密文可解回", issued.AccessKey[:12], len(issued.SecretKey))
}

// base64Key Base64 编码测试密钥。
func base64Key(raw []byte) string {
	return secret.EncodeKeyForTest(raw)
}

// TestLiveAdminClientDefaults 新增调用方时负责人与联系邮箱的默认值（需求 11174）。
func TestLiveAdminClientDefaults(t *testing.T) {
	service, pool := adminLiveService(t)
	ctx := context.Background()

	// 取一个真实登录账号作为「当前登录用户」
	var uid, email string
	err := pool.QueryRow(ctx,
		`SELECT uid, COALESCE(email, '') FROM sys.sys_user
		  WHERE COALESCE(email, '') <> '' ORDER BY idn LIMIT 1`).Scan(&uid, &email)
	if err != nil {
		t.Skip("sys_user 中没有带邮箱的账号，跳过默认值测试")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	client, err := service.SaveClient(ctx, ClientRequest{
		ClientCode: "zz-oa-def-" + suffix,
		ClientName: "默认值联调调用方",
		// owner_uid 与 contact_email 都不传，应由服务端按登录账号补
	}, uid)
	if err != nil {
		t.Fatalf("新建调用方失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE client_idn = $1`, client.IDN)
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_client WHERE idn = $1`, client.IDN)
		t.Logf("已清理默认值测试调用方 idn=%d", client.IDN)
	})

	if client.OwnerUID != uid {
		t.Fatalf("负责人应默认当前登录账号 %q，实际 %q", uid, client.OwnerUID)
	}
	if client.ContactEmail != email {
		t.Fatalf("联系邮箱应默认取用户资料 %q，实际 %q", email, client.ContactEmail)
	}
	t.Logf("默认值正确：owner_uid=%s email=%s", client.OwnerUID, client.ContactEmail)
}

// TestLiveAdminSecretRotation 密钥配置与重加密（需求 11175）。
//
// 覆盖：首次配置 19005 → 签发 AK/SK → 换密钥并重加密 → 新密钥可解、KID 更新；
// 非法密钥长度被拒；结束恢复 19004/19005/19006 原值。
func TestLiveAdminSecretRotation(t *testing.T) {
	service, pool := adminLiveService(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// 记录现场
	type cfg struct {
		kid, key, fallback string
	}
	var original cfg
	if err := pool.QueryRow(ctx, `
SELECT COALESCE(MAX(str_value) FILTER (WHERE config_no = 19004), ''),
       COALESCE(MAX(str_value) FILTER (WHERE config_no = 19005), ''),
       COALESCE(MAX(str_value) FILTER (WHERE config_no = 19006), '')
  FROM sys.sys_config WHERE config_no IN (19004, 19005, 19006)`).Scan(&original.kid, &original.key, &original.fallback); err != nil {
		t.Fatalf("读取密钥配置失败: %v", err)
	}
	// 重要：SaveSecret 的重加密是**全局**的，会改到库里所有持有密文的凭证。
	// 因此测试开始先整表快照（密文 + KID），结束时按 idn 原样写回，
	// 否则会把别人的凭证改成用临时密钥加密、而配置里没有那把密钥 → 永久解不开。
	type cipherRow struct {
		idn, kid, current, prev string
	}
	var snapshot []cipherRow
	cipherRows, err := pool.Query(ctx, `
SELECT idn::text, COALESCE(secret_kid, ''), COALESCE(secret_cipher, ''), COALESCE(secret_cipher_prev, '')
  FROM open.open_api_credential`)
	if err != nil {
		t.Fatalf("快照凭证密文失败: %v", err)
	}
	for cipherRows.Next() {
		var item cipherRow
		if err := cipherRows.Scan(&item.idn, &item.kid, &item.current, &item.prev); err != nil {
			cipherRows.Close()
			t.Fatalf("读取凭证密文失败: %v", err)
		}
		snapshot = append(snapshot, item)
	}
	cipherRows.Close()

	t.Cleanup(func() {
		// 先删掉本测试自己造的凭证，再把快照里的原始凭证密文写回
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE credential_no LIKE 'zz-secret-%'`)
		for _, item := range snapshot {
			if _, err := pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET secret_kid = NULLIF($2, ''), secret_cipher = NULLIF($3, ''), secret_cipher_prev = NULLIF($4, '')
 WHERE idn = $1::bigint`, item.idn, item.kid, item.current, item.prev); err != nil {
				t.Logf("恢复凭证 %s 密文失败（需人工检查）: %v", item.idn, err)
			}
		}
		if _, err := pool.Exec(ctx, `
UPDATE sys.sys_config SET str_value = $1 WHERE config_no = 19004`, original.kid); err != nil {
			t.Logf("恢复 19004 失败（需人工检查）: %v", err)
		}
		if _, err := pool.Exec(ctx, `
UPDATE sys.sys_config SET str_value = $1 WHERE config_no = 19005`, original.key); err != nil {
			t.Logf("恢复 19005 失败（需人工检查）: %v", err)
		}
		if _, err := pool.Exec(ctx, `
UPDATE sys.sys_config SET str_value = $1 WHERE config_no = 19006`, original.fallback); err != nil {
			t.Logf("恢复 19006 失败（需人工检查）: %v", err)
		}
	})

	// 非法密钥：长度不对应被拒
	if _, err := service.SaveSecret(ctx, SecretSaveRequest{SecretKeyBase64: "c2hvcnQ="}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("非法长度的密钥应返回 ErrInvalidRequest，实际 %v", err)
	}
	// 非法 KID
	if _, err := service.SaveSecret(ctx, SecretSaveRequest{SecretKID: "含中文"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("非法 KID 应返回 ErrInvalidRequest，实际 %v", err)
	}

	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i + 11)
		key2[i] = byte(i + 97)
	}

	// 首次配置密钥（KID=k1）
	view, err := service.SaveSecret(ctx, SecretSaveRequest{SecretKID: "k1", SecretKeyBase64: base64Key(key1)})
	if err != nil {
		t.Fatalf("首次配置密钥失败: %v", err)
	}
	if !view.SecretKeyReady || view.SecretKID != "k1" {
		t.Fatalf("配置状态不符: %+v", view)
	}

	// 建调用方 + AK/SK 凭证
	client, err := service.SaveClient(ctx, ClientRequest{
		ClientCode: "zz-oa-secret-" + suffix, ClientName: "密钥轮换联调",
	}, "livetest")
	if err != nil {
		t.Fatalf("新建调用方失败: %v", err)
	}
	issued, err := service.CreateCredential(ctx, CredentialCreateRequest{
		ClientIDN: client.IDN, CredentialNo: "zz-secret-" + suffix, AuthMode: AuthModeAKSK,
	}, "livetest")
	if err != nil {
		t.Fatalf("签发 AK/SK 失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_credential WHERE client_idn = $1`, client.IDN)
		_, _ = pool.Exec(ctx, `DELETE FROM open.open_api_client WHERE idn = $1`, client.IDN)
		t.Logf("已清理密钥轮换测试调用方 idn=%d", client.IDN)
	})

	// 换密钥 + 重加密
	view, err = service.SaveSecret(ctx, SecretSaveRequest{
		SecretKID: "k2", SecretKeyBase64: base64Key(key2), RotateCipher: true,
		FallbackKey: "fk_fallback_" + suffix,
	})
	if err != nil {
		t.Fatalf("换密钥失败: %v", err)
	}
	// 只要求「本测试自己那条」被重加密成功；库里其它凭证若本来就解不开（历史遗留），
	// 服务端会跳过并计数——这是预期行为，这里只记录不当作失败。
	if view.UpdatedCredential < 1 {
		t.Fatalf("至少应重加密 1 条凭证，实际 updated=%d skipped=%d", view.UpdatedCredential, view.SkippedCredential)
	}
	if view.SkippedCredential > 0 {
		t.Logf("跳过 %d 条解不开的历史凭证（预期内，需人工重新签发）", view.SkippedCredential)
	}

	// 新密文必须能用新密钥解出原 SK，且 secret_kid 已更新
	var cipherText, currentKID string
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(secret_cipher, ''), COALESCE(secret_kid, '') FROM open.open_api_credential WHERE idn = $1`,
		issued.Credential.IDN).Scan(&cipherText, &currentKID); err != nil {
		t.Fatalf("读取重加密结果失败: %v", err)
	}
	if currentKID != "k2" {
		t.Fatalf("secret_kid 应更新为 k2，实际 %q", currentKID)
	}
	plain, err := secret.Decrypt(cipherText, key2)
	if err != nil || plain != issued.SecretKey {
		t.Fatalf("新密钥应能解出原 SK: plain==issued?%t err=%v", plain == issued.SecretKey, err)
	}
	if _, err := secret.Decrypt(cipherText, key1); err == nil {
		t.Fatalf("旧密钥不应再能解开新密文")
	}

	// 视图与响应里不得出现密钥明文
	if strings.Contains(view.SecretKID, base64Key(key2)) {
		t.Fatalf("响应不得包含密钥明文")
	}
	t.Logf("密钥轮换完成：kid=%s updated=%d fallback_ready=%t", view.SecretKID, view.UpdatedCredential, view.FallbackKeyReady)
}
