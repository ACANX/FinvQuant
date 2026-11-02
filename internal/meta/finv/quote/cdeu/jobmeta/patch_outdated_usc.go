package jobmeta

// PatchOutdatedUsc 每日 05:00 按补丁规则把过时 usc 的行情迁移到新 usc（需求 10931）。
var PatchOutdatedUsc = Runtime{
	Kind:     "PATCH_OUTDATED_USC",
	DailyAt:  "05:00",
	Schedule: "每日 05:00",
	Name:     "过时 USC 补丁",
	Summary:  "按 cdeu.finv_quote_cdeu_usc_patch_rule 的生效规则，把旧 usc 的行情迁移到新 usc。",
	Flow:     "读生效规则 → 按 ts 区间分批 → upsert 到目标 usc（较新者胜）→ 删除旧 usc 行 → 汇总统计。",
	Details:  "分批上限 5000 行/批；冲突行按 gmt_update 较新者胜覆盖；无生效规则时直接返回。",
	Cautions: "规则启用前先用试算接口核对影响行数；上游映射未同步时旧 usc 会反复回流，重复迁移行数偏高说明上游待修。",
}
