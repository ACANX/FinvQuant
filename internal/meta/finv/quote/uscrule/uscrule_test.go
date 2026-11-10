package uscrule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestNormalizeEnv 空/空白环境一律按 * 处理（与条目层 scope_env 约定一致）。
func TestNormalizeEnv(t *testing.T) {
	cases := map[string]string{
		"":      DefaultEnv,
		"   ":   DefaultEnv,
		"prod":  "prod",
		" dev ": "dev",
	}
	for in, want := range cases {
		if got := normalizeEnv(in); got != want {
			t.Errorf("normalizeEnv(%q)=%q，期望 %q", in, got, want)
		}
	}
}

// TestNormalizePager 分页兜底：页码/页大小非法时回落，页大小上限 200。
func TestNormalizePager(t *testing.T) {
	cases := []struct{ page, size, wantPage, wantSize int }{
		{0, 0, 1, 20},
		{-1, -5, 1, 20},
		{3, 50, 3, 50},
		{1, 9999, 1, 200},
	}
	for _, c := range cases {
		page, size := normalizePager(c.page, c.size)
		if page != c.wantPage || size != c.wantSize {
			t.Errorf("normalizePager(%d,%d)=(%d,%d)，期望 (%d,%d)",
				c.page, c.size, page, size, c.wantPage, c.wantSize)
		}
	}
}

// TestNextRun 6 段 cron 的下一次执行时间；空串与非法表达式返回空。
func TestNextRun(t *testing.T) {
	from := time.Date(2026, 11, 10, 4, 0, 0, 0, time.Local)
	if got := NextRun("0 30 3 * * *", from); got != "2026-11-11T03:30:00+08:00" && got != "2026-11-11T03:30:00+09:00" {
		// 时区随服务器，只校验「日期是次日、时间为 03:30」。
		parsed, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Fatalf("NextRun 返回非法时间 %q: %v", got, err)
		}
		if parsed.Hour() != 3 || parsed.Minute() != 30 || parsed.Day() != 11 {
			t.Errorf("NextRun 期望次日 03:30，实际 %q", got)
		}
	}
	if got := NextRun("", from); got != "" {
		t.Errorf("空表达式应返回空串，实际 %q", got)
	}
	if got := NextRun("不是cron", from); got != "" {
		t.Errorf("非法表达式应返回空串，实际 %q", got)
	}
	if ParseSchedule("0 30 3 * * *") == nil {
		t.Error("合法表达式应解析成功")
	}
}

// TestPreviewLive 对**真实库**做只读预演（不写任何表）。
//
// 默认跳过；需要只读验证时：
//
//	FINV_USC_LIVE_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test ./internal/meta/finv/quote/uscrule -run TestPreviewLive -v
//
// 断言口径与设计文档一致：预演行数 = 键表行数、冲突为空、令牌非空、分类计数之和 = 总行数。
func TestPreviewLive(t *testing.T) {
	dsn := os.Getenv("FINV_USC_LIVE_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_LIVE_DSN，跳过只读预演测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	var keyRows int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_secu_key`).Scan(&keyRows); err != nil {
		t.Fatalf("统计键表失败: %v", err)
	}

	service := NewService(pool)
	preview, err := service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("预演失败: %v", err)
	}
	t.Logf("预演：total=%d insert=%d update=%d retire=%d reuse=%d keep=%d escalate=%d key_update=%d token=%s",
		preview.Total, preview.Insert, preview.Update, preview.Retire, preview.Reuse,
		preview.Keep, preview.Escalate, preview.KeyUpdate, preview.PreviewToken)

	if preview.Total != keyRows {
		t.Errorf("预演行数 %d 与键表行数 %d 不一致", preview.Total, keyRows)
	}
	if preview.Insert+preview.Update+preview.Keep != preview.Total {
		t.Errorf("分类计数之和 %d 与总行数 %d 不一致",
			preview.Insert+preview.Update+preview.Keep, preview.Total)
	}
	if len(preview.Conflicts) != 0 {
		t.Errorf("预演报告了 usc 冲突: %v", preview.Conflicts)
	}
	if preview.PreviewToken == "" {
		t.Error("预演令牌为空")
	}
	if len(preview.Samples) > MaxPreviewSamples {
		t.Errorf("预演明细 %d 行超过上限 %d", len(preview.Samples), MaxPreviewSamples)
	}
	if preview.RuleVersion == "" {
		t.Error("规则版本为空")
	}

	// 完整明细分页：预演只回前 200 行，Diff/Page 必须能看到后面的行。
	rows, total, token, err := service.DiffPage("", "", "", 1, 50)
	if err != nil {
		t.Fatalf("读取预演明细分页失败: %v", err)
	}
	if token != preview.PreviewToken {
		t.Errorf("分页令牌 %q 与预演令牌 %q 不一致", token, preview.PreviewToken)
	}
	// 明细 = 结果表要改的行（新增/更新/退市/复用）+ 键表漂移行（key_update）
	expectTotal := int(preview.Insert + preview.Update + preview.Retire + preview.Reuse + preview.KeyUpdate)
	if total != expectTotal {
		t.Errorf("明细分页 total=%d，期望 %d（新增+更新+退市+复用+键表回写）", total, expectTotal)
	}
	if len(rows) != 50 && total >= 50 {
		t.Errorf("第 1 页应返回 50 行，实际 %d", len(rows))
	}

	// 第 2 页必须与第 1 页不重复（证明能翻到 200 行之后）
	rows2, _, _, err := service.DiffPage("", "", "", 2, 50)
	if err != nil {
		t.Fatalf("读取第 2 页失败: %v", err)
	}
	if len(rows2) > 0 && len(rows) > 0 && rows[0].FtmmSymbol == rows2[0].FtmmSymbol {
		t.Errorf("第 2 页首行与第 1 页相同（%s），分页没生效", rows2[0].FtmmSymbol)
	}

	// 关键字检索：按 ftmm_symbol / code / usc 都能命中同一只证券（不必翻页）
	if total > 0 && len(rows) > 0 {
		probe := rows[0]
		// 结果表随时查询（不依赖预演缓存/令牌）：按 ftmm_symbol 关键字应能直接定位这一条
		mapRows, mapTotal, err := service.MapPage(context.Background(),
			MapFilter{Keyword: probe.FtmmSymbol, Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("查询结果表失败: %v", err)
		}
		if mapTotal < 1 || len(mapRows) == 0 {
			t.Errorf("结果表按 %s 查询应至少命中 1 行，实际 %d", probe.FtmmSymbol, mapTotal)
		}
		t.Logf("结果表检索：%s 命中 %d 行", probe.FtmmSymbol, mapTotal)
		for _, kw := range []string{probe.FtmmSymbol, probe.Code} {
			if kw == "" {
				continue
			}
			hits, hitsTotal, _, err := service.DiffPage("", "", kw, 1, 50)
			if err != nil {
				t.Fatalf("关键字预演明细检索失败（%s）: %v", kw, err)
			}
			found := false
			for _, row := range hits {
				if row.FtmmSymbol == probe.FtmmSymbol {
					found = true
				}
			}
			if !found {
				t.Errorf("关键字 %q 未命中 %s（命中 %d 条 / 共 %d 条）",
					kw, probe.FtmmSymbol, len(hits), hitsTotal)
			}
		}
	}

	// 末页：翻过 200 行（预演响应上限）之外仍应有数据
	lastPage := (total + 199) / 200
	rowsLast, _, _, err := service.DiffPage("", "", "", lastPage, 200)
	if err != nil {
		t.Fatalf("读取末页失败: %v", err)
	}
	if total > MaxPreviewSamples && len(rowsLast) == 0 {
		t.Errorf("末页（第 %d 页）无数据，说明明细只缓存了前 %d 行", lastPage, MaxPreviewSamples)
	}
	t.Logf("明细分页：total=%d，第 1 页 %d 行，第 2 页 %d 行，末页(%d) %d 行",
		total, len(rows), len(rows2), lastPage, len(rowsLast))
}

// TestApplyOnClone 在**克隆库**上端到端跑一次应用（真实写结果表 + 回写键表 + 记账）。
//
// 不连生产库：用 `pg_dump -n futu -n archive` 克隆到临时库后执行。
//
//	FINV_USC_APPLY_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv_usctest?sslmode=disable" \
//	  go test ./internal/meta/finv/quote/uscrule -run TestApplyOnClone -v
//
// 覆盖点：预演 → 应用（令牌校验）→ 结果表行数、键表与结果表一致、抽样口径、二次应用幂等。
func TestApplyOnClone(t *testing.T) {
	dsn := os.Getenv("FINV_USC_APPLY_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_APPLY_DSN，跳过应用集成测试（会写库，必须指向克隆库）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	var keyRows int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_secu_key`).Scan(&keyRows); err != nil {
		t.Fatalf("统计键表失败: %v", err)
	}
	service := NewService(pool)

	preview, err := service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("预演失败: %v", err)
	}
	t.Logf("预演：total=%d insert=%d update=%d retire=%d escalate=%d",
		preview.Total, preview.Insert, preview.Update, preview.Retire, preview.Escalate)

	// 令牌错误必须被拒绝
	if _, err := service.Apply(ctx, DefaultEnv, "bogus-token", "test", "错误令牌应被拒绝"); err == nil {
		t.Fatal("应用应拒绝错误的预演令牌")
	}

	result, err := service.Apply(ctx, DefaultEnv, preview.PreviewToken, "test", "集成测试应用")
	if err != nil {
		t.Fatalf("应用失败: %v", err)
	}
	t.Logf("应用：insert=%d update=%d retire=%d reuse=%d keep=%d escalate=%d key_update=%d archive=%d elapsed=%dms",
		result.Insert, result.Update, result.Retire, result.Reuse, result.Keep,
		result.Escalate, result.KeyUpdate, result.Archive, result.DurationMs)

	if result.Insert != preview.Insert || result.Update != preview.Update || result.Retire != preview.Retire {
		t.Errorf("应用统计与预演不一致：应用(insert %d/update %d/retire %d) vs 预演(%d/%d/%d)",
			result.Insert, result.Update, result.Retire, preview.Insert, preview.Update, preview.Retire)
	}

	var mapRows int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_usc_map WHERE status = 'ACTIVE'`).Scan(&mapRows); err != nil {
		t.Fatalf("统计结果表失败: %v", err)
	}
	if mapRows != keyRows {
		t.Errorf("结果表在册行数 %d 与键表行数 %d 不一致", mapRows, keyRows)
	}

	// 键表 usc 必须与结果表完全一致
	var mismatch int64
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM futu.finv_futu_secu_key k
  JOIN futu.finv_futu_usc_map m ON m.ftmm_symbol = k.ftmm_symbol AND m.status = 'ACTIVE'
 WHERE lower(k.usc) IS DISTINCT FROM m.usc_lower`).Scan(&mismatch); err != nil {
		t.Fatalf("校验键表与结果表一致性失败: %v", err)
	}
	if mismatch != 0 {
		t.Errorf("键表 usc 与结果表不一致 %d 行", mismatch)
	}

	// 抽样口径
	samples := map[string]string{
		"US.AESG":   "AESG",     // P50 已停用 → 裸码
		"SH.000300": "SH0300",   // P22 前处理
		"US.XRP":    "US.XRP",   // P40 单点例外：美股 XRP ETF 用富途全码（需求 11097）
		"US..BFLY":  "IDX.BFLY", // P40 指数撞名例外
		"AU..XFL":   "XFL",      // P16 去前导点
	}
	for ftmm, want := range samples {
		var got string
		if err := pool.QueryRow(ctx, `SELECT usc FROM futu.finv_futu_usc_map WHERE ftmm_symbol = $1`, ftmm).Scan(&got); err != nil {
			t.Logf("抽样 %s 未在结果表中：%v", ftmm, err)
			continue
		}
		if got != want {
			t.Errorf("抽样 %s 的 usc=%q，期望 %q", ftmm, got, want)
		}
	}

	// 二次应用必须幂等：全部 keep、不再回写键表
	preview2, err := service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("二次预演失败: %v", err)
	}
	if preview2.Insert != 0 || preview2.Update != 0 {
		t.Errorf("二次预演应无变化，实际 insert=%d update=%d", preview2.Insert, preview2.Update)
	}
	result2, err := service.Apply(ctx, DefaultEnv, preview2.PreviewToken, "test", "幂等复跑")
	if err != nil {
		t.Fatalf("二次应用失败: %v", err)
	}
	if result2.Insert != 0 || result2.Update != 0 || result2.KeyUpdate != 0 {
		t.Errorf("二次应用应幂等，实际 insert=%d update=%d key_update=%d",
			result2.Insert, result2.Update, result2.KeyUpdate)
	}
}

// TestItemKeyText 校验冲突提示里的键描述（纯函数，不连库）。
func TestItemKeyText(t *testing.T) {
	str := func(s string) *string { return &s }
	item := Item{
		RuleNo:    "P40",
		Market:    str("CC"),
		Code:      str("XRP"),
		ScopeEnv:  DefaultEnv,
		Instrument: nil,
	}
	if got, want := itemKeyText(item), "P40 / market=CC / code=XRP"; got != want {
		t.Fatalf("itemKeyText = %q, want %q", got, want)
	}
	item2 := Item{RuleNo: "P60", ScopeEnv: "dev", CodeRegex: str("^0")}
	if got, want := itemKeyText(item2), "P60 / code_regex=^0 / scope_env=dev"; got != want {
		t.Fatalf("itemKeyText = %q, want %q", got, want)
	}
}

// TestSaveItemConflictOnClone 校验「自然键重复」返回可读的 ErrItemExists，而不是裸的 23505。
//
// 需要克隆库（会写条目表，但用例结束会删除自建行）：
//
//	FINV_USC_APPLY_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv_usctest?sslmode=disable" \
//	  go test -run TestSaveItemConflictOnClone -v ./internal/meta/finv/quote/uscrule/
func TestSaveItemConflictOnClone(t *testing.T) {
	dsn := os.Getenv("FINV_USC_APPLY_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_APPLY_DSN，跳过条目保存冲突用例（会写库，必须指向克隆库）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	service := NewService(pool)
	count := func(ruleNo string) int64 {
		var n int64
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM futu.finv_futu_usc_rule_item WHERE rule_no = $1 AND market = 'ZZ'
			   AND code = 'TSTCONFLICT'`, ruleNo).Scan(&n); err != nil {
			t.Fatalf("统计条目失败: %v", err)
		}
		return n
	}
	cleanup := func() {
		if _, err := pool.Exec(ctx,
			`DELETE FROM futu.finv_futu_usc_rule_item WHERE market = 'ZZ' AND code = 'TSTCONFLICT'`); err != nil {
			t.Fatalf("清理测试条目失败: %v", err)
		}
	}
	cleanup()
	defer cleanup()

	market, code, literal := "ZZ", "TSTCONFLICT", "ZZTEST"
	desc := "冲突用例：自然键重复必须返回可读错误"
	first, err := service.SaveItem(ctx, Item{
		RuleNo: "P40", Market: &market, Code: &code, Action: strPtr(ActionLiteral),
		USCLiteral: &literal, ItemDesc: desc,
	}, "test")
	if err != nil {
		t.Fatalf("首次保存应成功: %v", err)
	}
	if count("P40") != 1 {
		t.Fatalf("首次保存后条目数应为 1")
	}

	// 同自然键再新增（idn=0）：必须判为「条目已存在」，并且提示里带上已有 idn
	_, err = service.SaveItem(ctx, Item{
		RuleNo: "P40", Market: &market, Code: &code, Action: strPtr(ActionLiteral),
		USCLiteral: &literal, ItemDesc: desc,
	}, "test")
	if err == nil {
		t.Fatal("重复自然键应报错")
	}
	if !errors.Is(err, ErrItemExists) {
		t.Fatalf("期望 ErrItemExists，实际: %v", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("idn=%d", first)) {
		t.Fatalf("提示里应包含已有条目 idn=%d，实际: %v", first, err)
	}
	if count("P40") != 1 {
		t.Fatalf("冲突不应写入新行，条目数应仍为 1")
	}

	// 改走编辑（带 idn）应成功
	updated := "冲突用例：按 idn 编辑应成功"
	if _, err := service.SaveItem(ctx, Item{
		IDN: first, RuleNo: "P40", Market: &market, Code: &code,
		Action: strPtr(ActionLiteral), USCLiteral: &literal, ItemDesc: updated,
	}, "test"); err != nil {
		t.Fatalf("按 idn 编辑应成功: %v", err)
	}
}

func strPtr(s string) *string { return &s }

// TestKeyDriftOnClone 校验「手工改键表 usc」必须能被预演看见并在应用时回写。
//
// 缺陷背景：`_usc_diff` 只比「结果表 vs 本次计算」，手工改 `futu.finv_futu_secu_key.usc`
// 后结果表没变 → diff='keep' → 预演显示「无变更」，用户以为规则没生效。
//
//	FINV_USC_APPLY_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv_usctest?sslmode=disable" \
//	  go test -run TestKeyDriftOnClone -v ./internal/meta/finv/quote/uscrule/
func TestKeyDriftOnClone(t *testing.T) {
	dsn := os.Getenv("FINV_USC_APPLY_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_APPLY_DSN，跳过键表漂移用例（会写库，必须指向克隆库）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	service := NewService(pool)

	// 先让结果表与键表完全一致，确保后面的漂移是唯一变量
	preview, err := service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("预演失败: %v", err)
	}
	if _, err := service.Apply(ctx, DefaultEnv, preview.PreviewToken, "test", "键表漂移用例：先收敛"); err != nil {
		t.Fatalf("首次应用失败: %v", err)
	}

	// 挑一行已知映射，手工把键表 usc 改歪（模拟用户手工改值）
	const target = "US.AAPL"
	var expect string
	if err := pool.QueryRow(ctx,
		`SELECT usc FROM futu.finv_futu_usc_map WHERE ftmm_symbol = $1 AND status = 'ACTIVE'`, target).Scan(&expect); err != nil {
		t.Fatalf("读取 %s 的结果表 usc 失败: %v", target, err)
	}
	const drift = "ZZ_DRIFT_TMP"
	if _, err := pool.Exec(ctx,
		`UPDATE futu.finv_futu_secu_key SET usc = $2 WHERE ftmm_symbol = $1`, target, drift); err != nil {
		t.Fatalf("手工改键表失败: %v", err)
	}

	// 预演必须看见这行漂移
	preview, err = service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("漂移后预演失败: %v", err)
	}
	if preview.KeyUpdate < 1 {
		t.Fatalf("预演未识别键表漂移：key_update=%d（应 ≥1）", preview.KeyUpdate)
	}
	found := false
	for _, row := range preview.Samples {
		if row.FtmmSymbol == target && row.Diff == "key_update" {
			found = true
			gotCur, gotNew := "", ""
			if row.CurrentUSC != nil {
				gotCur = *row.CurrentUSC
			}
			if row.NewUSC != nil {
				gotNew = *row.NewUSC
			}
			if gotCur != drift || gotNew != expect {
				t.Fatalf("漂移行内容不符：current=%q new=%q（期望 %q → %q）", gotCur, gotNew, drift, expect)
			}
		}
	}
	if !found {
		t.Fatalf("预演明细里没有 %s 的 key_update 行", target)
	}

	// 应用后键表必须被回写回规则结果
	result, err := service.Apply(ctx, DefaultEnv, preview.PreviewToken, "test", "键表漂移用例：回写")
	if err != nil {
		t.Fatalf("应用失败: %v", err)
	}
	if result.KeyUpdate < 1 {
		t.Fatalf("应用未回写键表：key_update=%d（应 ≥1）", result.KeyUpdate)
	}
	var got string
	if err := pool.QueryRow(ctx,
		`SELECT usc FROM futu.finv_futu_secu_key WHERE ftmm_symbol = $1`, target).Scan(&got); err != nil {
		t.Fatalf("读取回写后的键表 usc 失败: %v", err)
	}
	if got != expect {
		t.Fatalf("键表未回写：got=%q want=%q", got, expect)
	}
}

// TestSaveRuleOnClone 规则层保存（启用开关 / 默认动作 / 说明 / 备注）会落库并写审计日志。
//
// 会写库，必须指向克隆库：
//
//	FINV_USC_APPLY_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv_usctest?sslmode=disable" \
//	  go test -run TestSaveRuleOnClone -v ./internal/meta/finv/quote/uscrule/
//
// 覆盖点：改 flag_enable/default_action/rule_desc/remark → 落库一致；审计日志出现 src_table='rule' 的一行；
// 非法 default_action 被拒；不存在的 rule_no 被拒；用例结束把 P50 恢复为原值。
func TestSaveRuleOnClone(t *testing.T) {
	dsn := os.Getenv("FINV_USC_APPLY_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_APPLY_DSN，跳过规则保存用例（会写库，必须指向克隆库）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	service := NewService(pool)

	// 记下原值，结束时还原（克隆库也要保持干净）
	var origFlag, origAction, origDesc, origRemark string
	if err := pool.QueryRow(ctx, `
SELECT flag_enable, COALESCE(default_action, ''), rule_desc, remark
  FROM futu.finv_futu_usc_rule WHERE rule_no = 'P50'`).Scan(&origFlag, &origAction, &origDesc, &origRemark); err != nil {
		t.Fatalf("读取 P50 原值失败: %v", err)
	}
	t.Cleanup(func() {
		var action *string
		if origAction != "" {
			action = &origAction
		}
		_ = service.SaveRule(context.Background(), RuleUpdate{
			RuleNo: "P50", FlagEnable: origFlag, DefaultAction: action,
			RuleDesc: origDesc, Remark: origRemark, Reason: "用例还原",
		}, "test")
	})

	flip := "0"
	if origFlag == "0" {
		flip = "1"
	}
	action := "FTMM"
	if err := service.SaveRule(ctx, RuleUpdate{
		RuleNo: "P50", FlagEnable: flip, DefaultAction: &action,
		RuleDesc: "用例：规则层可改", Remark: "用例：备注可改", Reason: "TestSaveRuleOnClone",
	}, "test"); err != nil {
		t.Fatalf("保存规则失败: %v", err)
	}

	var gotFlag, gotAction, gotDesc, gotRemark string
	if err := pool.QueryRow(ctx, `
SELECT flag_enable, COALESCE(default_action, ''), rule_desc, remark
  FROM futu.finv_futu_usc_rule WHERE rule_no = 'P50'`).Scan(&gotFlag, &gotAction, &gotDesc, &gotRemark); err != nil {
		t.Fatalf("读取保存后的规则失败: %v", err)
	}
	if gotFlag != flip || gotAction != action || gotDesc != "用例：规则层可改" || gotRemark != "用例：备注可改" {
		t.Fatalf("落库不一致：flag=%q action=%q desc=%q remark=%q", gotFlag, gotAction, gotDesc, gotRemark)
	}

	var logs int64
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM futu.finv_futu_usc_rule_log
 WHERE src_table = 'rule' AND key_text = 'P50' AND operator = 'test'`).Scan(&logs); err != nil {
		t.Fatalf("统计审计日志失败: %v", err)
	}
	if logs < 1 {
		t.Fatalf("规则保存未写审计日志（src_table='rule'）")
	}

	bad := "NOT_AN_ACTION"
	if err := service.SaveRule(ctx, RuleUpdate{RuleNo: "P50", FlagEnable: "1", DefaultAction: &bad}, "test"); err == nil {
		t.Fatalf("非法 default_action 应被拒绝")
	}
	if err := service.SaveRule(ctx, RuleUpdate{RuleNo: "P99", FlagEnable: "1"}, "test"); err == nil {
		t.Fatalf("不存在的 rule_no 应被拒绝")
	}
}
