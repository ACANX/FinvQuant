package jobmeta

var RefreshQuoteMaturity = Runtime{
	Kind: "REFRESH_QUOTE_MATURITY", DailyAt: "00:05", Schedule: "每日 00:05",
	Name:     "行情成熟度扫描",
	Summary:  "增量刷新证券日成熟度并自动发现新 usc，是证券月能否进入导出候选的前置条件。",
	Flow:     "读取扫描水位 → 增量扫描主表 → 自动发现新 usc → 补全证券元数据（finv_security 不限启用 + 富途镜像兜底 + 按区域/前缀推导）与 day_expect_rows 模板值 / 优先级默认值 → 按 day_expect_rows 阈值判定 IMMATURE / PARTIAL / COMPLETE → 写 cdeu.finv_quote_cdeu_quote_maturity_mon。",
	Details:  "水位首次建立或每月 1 日会全量校准最近 3 个月，防止水位漏行造成长期偏差。补全结果（补了多少、仍缺哪些字段、多少个证券）写进 JOB 日志与 Details，剩余未命中通常是 finv_security 与富途镜像表都未收录的品种（如部分美股、加密货币、历史 SH#### 前缀指数）。",
	Cautions: "补全只填空、不覆盖人工值（region/market/currency/name/timezone/category 与 day_expect_rows 均如此）；若某证券 region/market 仍为空则不进入导出候选（需人工补元数据或补 finv_market.market_symbol）；水位异常时用 PUT Watermark 重置。",
}
