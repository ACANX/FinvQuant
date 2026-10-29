// Package job 按单个 JOB 维护调度说明与可观测日志文本，不依赖 dedup 主包，避免循环引用。
package jobmeta

// Runtime JOB 的调度配置与中文说明。
type Runtime struct {
	Kind     string `json:"job_kind"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	Flow     string `json:"flow"`
	Details  string `json:"details"`
	Cautions string `json:"cautions"`
	DailyAt  string `json:"daily_at,omitempty"`
	Interval int    `json:"interval_seconds,omitempty"`
	Windowed bool   `json:"windowed"`
	Schedule string `json:"schedule"`
}
