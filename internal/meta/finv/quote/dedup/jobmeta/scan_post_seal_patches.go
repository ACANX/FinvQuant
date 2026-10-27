package jobmeta

var ScanPostSealPatches = Runtime{
	Kind: "SCAN_POST_SEAL_PATCHES", DailyAt: "03:00", Schedule: "每日 03:00",
	Name:     "收口后补丁扫描",
	Summary:  "扫描收口后新到达或更正的数据，按 L1_HOT / L2_WARM / L3_COLD 生成补丁或重写 base。",
	Flow:     "读取补丁候选 → 按水位增量统计 delta → 无变化则推进水位 → 按级别重写 base 或生成 PATCH 文件 → 登记文件等待上传。",
	Details:  "单轮最多 100 个证券月；仅 delta > 0 时创建 INCR 动态 JOB；补丁预算受 FINV_QUOTE_DEDUB_PATCH_BUDGET_RATIO 限制。",
	Cautions: "base 尚未上传成功时会走 L1 重写 base（版本 +1）；过早收口会产生大量补丁，需先排查上游延迟。",
}
