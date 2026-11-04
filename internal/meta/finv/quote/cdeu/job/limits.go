package job

// 导出链路的批量上限常量。需求 10717 拆 JOB 后集中在本文件维护。
const (
	// stagingPurgeLimit 单轮清理的中转表残留行数上限。
	stagingPurgeLimit = 50000
	// localCleanupLimit 单轮清理的本地过期文件数上限。
	localCleanupLimit = 500
	// mainPurgeBatchLimit 主表按证券月删除时的单批行数上限。
	mainPurgeBatchLimit = 50000
	// settledPurgeBatch 单轮扫描的「已完整交付」证券月数量上限。
	settledPurgeBatch = 500
)

// boolToFlag 把开关转成配置口径的 1/0，便于进度日志与 20007 取值对照。
func boolToFlag(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}
