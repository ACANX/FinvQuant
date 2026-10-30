package jobmeta

var RefreshQuoteMaturity = Runtime{
	Kind: "REFRESH_QUOTE_MATURITY", DailyAt: "00:05", Schedule: "每日 00:05",
	Name:     "行情成熟度扫描",
	Summary:  "增量刷新证券日成熟度并自动发现新 usc，是证券月能否进入导出候选的前置条件。",
	Flow:     "读取扫描水位 → 增量扫描主表 → 自动发现新 usc → 按模板补全 day_expect_rows / 优先级默认值 → 按 day_expect_rows 阈值判定 IMMATURE / PARTIAL / COMPLETE → 写 cdeu.finv_quote_cdeu_quote_maturity_mon。",
	Details:  "水位首次建立或每月 1 日会全量校准最近 3 个月，防止水位漏行造成长期偏差。",
	Cautions: "day_expect_rows 按 cdeu.finv_quote_cdeu_usc_day_incr_row_tpl 模板自动补全（已配置值不覆盖），模板未命中且未人工配置的证券仍会保持 PARTIAL、永远不会导出；水位异常时用 PUT Watermark 重置。",
}
