package collectcfg

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLiveCollectConfigReadWrite 真实读写 20013–20018 五项（默认跳过）。
//
// 需要：
//
//	FINV_TEST_PG_DSN  PostgreSQL 连接串
//
// 运行：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveCollectConfig -v ./internal/meta/finv/quote/collectcfg/
//
// 安全：测试会备份每项原值并在结束时还原；不打印任何配置值。
func TestLiveCollectConfigReadWrite(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store := NewStore(pool)

	// 备份原值
	backup := map[int]string{}
	for _, item := range Items() {
		var raw string
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = $1`,
			item.ConfigNo).Scan(&raw); err != nil {
			t.Fatalf("读取 %d 原值失败: %v", item.ConfigNo, err)
		}
		backup[item.ConfigNo] = raw
	}
	t.Cleanup(func() {
		for no, raw := range backup {
			if _, err := pool.Exec(context.Background(),
				`UPDATE sys.sys_config SET str_value = $2 WHERE config_no = $1`, no, raw); err != nil {
				t.Logf("还原 %d 失败: %v", no, err)
			}
		}
	})

	views, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(views) != len(Items()) {
		t.Fatalf("List 返回 %d 项，期望 %d", len(views), len(Items()))
	}
	for _, view := range views {
		if !view.Sensitive {
			if view.Value != backup[view.ConfigNo] {
				t.Errorf("%d 非敏感项应回原值", view.ConfigNo)
			}
			continue
		}
		if !view.Masked {
			t.Errorf("%d 敏感项应标记 masked", view.ConfigNo)
		}
		want := Mask(view.ConfigNo, backup[view.ConfigNo])
		if view.Value != want {
			t.Errorf("%d 掩码不符: 期望 %q 实际 %q", view.ConfigNo, want, view.Value)
		}
		// 掩码不得包含原值的 key 段
		_, keyPart, _ := strings.Cut(strings.TrimSpace(backup[view.ConfigNo]), "|")
		if keyPart != "" && strings.Contains(view.Value, keyPart) {
			t.Errorf("%d 掩码泄露了 key 段", view.ConfigNo)
		}
	}
	t.Logf("List 返回 %d 项，敏感项掩码正常", len(views))

	// Save：20014 写合法值 → 列表反映；写非法值 → 报错且不落库
	if _, err := store.Save(ctx, ConfigNoSupabaseSecuKeyTable, "public.finv_quote_secu_key"); err != nil {
		t.Fatalf("保存 20014 合法值失败: %v", err)
	}
	if _, err := store.Save(ctx, ConfigNoSupabaseSecuKeyTable, "bad-table-name"); err == nil {
		t.Error("20014 非法值应报错")
	} else if _, ok := err.(*InputError); !ok {
		t.Errorf("20014 非法值应返回 InputError，实际 %T", err)
	}

	// SecretValue：敏感项可读、非敏感项被拒
	if raw, err := store.SecretValue(ctx, ConfigNoMoomooCredential); err != nil {
		t.Fatalf("读取 20016 明文失败: %v", err)
	} else if raw != backup[ConfigNoMoomooCredential] {
		t.Error("20016 明文与原值不一致")
	}
	if _, err := store.SecretValue(ctx, ConfigNoSupabaseSecuKeyTable); err == nil {
		t.Error("非敏感项 20014 调 SecretValue 应被拒绝")
	} else if _, ok := err.(*InputError); !ok {
		t.Errorf("应返回 InputError，实际 %T", err)
	}

	// 20016：AppKey 沿用原值 + 私钥留空 = 保留原私钥
	current, err := store.currentMoomooCredential(ctx)
	if err != nil {
		t.Fatalf("读取 20016 现有私钥失败: %v", err)
	}
	appKey := strings.TrimSpace(strings.SplitN(backup[ConfigNoMoomooCredential], "|", 2)[0])
	if appKey == "" {
		appKey = "testappkey"
	}
	if _, err := store.Save(ctx, ConfigNoMoomooCredential, appKey+"|"); err != nil {
		t.Fatalf("保存 20016（私钥留空）失败: %v", err)
	}
	after, err := store.currentMoomooCredential(ctx)
	if err != nil {
		t.Fatalf("读取 20016 私钥失败: %v", err)
	}
	if after != current {
		t.Error("20016 私钥留空时应保留原私钥")
	}
}
