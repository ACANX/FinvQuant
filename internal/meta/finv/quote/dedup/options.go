package dedup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/config"
)

// Options 归档导出运行参数（由 FINV_QUOTE_DEDUB_* 环境变量经 config 派生）。
type Options struct {
	Enabled           bool
	Location          *time.Location
	TimeZone          string // 业务时区名（传给 PostgreSQL AT TIME ZONE）
	OutputDir         string
	WorkerID          string
	PageSize          int
	WindowStart       int // 低峰窗口开始（自 0 点起的分钟数）
	WindowEnd         int // 低峰窗口结束（自 0 点起的分钟数）
	Lease             time.Duration
	MaxAttempts       int
	UploadMode        string
	UploadDir         string
	UploadHTTPBase    string
	UploadHTTPToken   string
	UploadPrefix      string
	LocalRetention    time.Duration
	PurgeMainEnabled  bool
	PurgeMainMonths   int
	MaturityThreshold float64
	PatchBudgetRatio  float64
	Compression       string
	FetchLimit        int
	MaxJobsPerRun     int
}

// OptionsFromConfig 由全局配置派生归档选项，并对取值做安全收敛。
func OptionsFromConfig(cfg *config.Config) Options {
	if cfg == nil {
		cfg = config.Load()
	}
	location, err := time.LoadLocation(strings.TrimSpace(cfg.QuoteDeDupTimeZone))
	if err != nil || location == nil {
		location = time.Local
	}

	options := Options{
		Enabled:           cfg.QuoteDeDupEnabled,
		Location:          location,
		TimeZone:          location.String(),
		OutputDir:         strings.TrimSpace(cfg.QuoteDeDupOutputDir),
		WorkerID:          strings.TrimSpace(cfg.QuoteDeDupWorkerID),
		PageSize:          cfg.QuoteDeDupPageSize,
		WindowStart:       parseClockMinutes(cfg.QuoteDeDupWindowStart, 30),
		WindowEnd:         parseClockMinutes(cfg.QuoteDeDupWindowEnd, 6*60+30),
		Lease:             time.Duration(cfg.QuoteDeDupLeaseSeconds) * time.Second,
		MaxAttempts:       cfg.QuoteDeDupMaxAttempts,
		UploadMode:        strings.ToLower(strings.TrimSpace(cfg.QuoteDeDupUploadMode)),
		UploadDir:         strings.TrimSpace(cfg.QuoteDeDupUploadDir),
		UploadHTTPBase:    strings.TrimSpace(cfg.QuoteDeDupUploadHTTPBase),
		UploadHTTPToken:   strings.TrimSpace(cfg.QuoteDeDupUploadHTTPToken),
		UploadPrefix:      strings.Trim(strings.TrimSpace(cfg.QuoteDeDupUploadPrefix), "/"),
		LocalRetention:    time.Duration(cfg.QuoteDeDupLocalRetentionDay) * 24 * time.Hour,
		PurgeMainEnabled:  cfg.QuoteDeDupPurgeMainEnabled,
		PurgeMainMonths:   cfg.QuoteDeDupPurgeMainMonths,
		MaturityThreshold: cfg.QuoteDeDupMaturityThreshold,
		PatchBudgetRatio:  cfg.QuoteDeDupPatchBudgetRatio,
		Compression:       strings.ToLower(strings.TrimSpace(cfg.QuoteDeDupCompression)),
		FetchLimit:        cfg.QuoteDeDupFetchLimit,
		MaxJobsPerRun:     cfg.QuoteDeDupMaxJobsPerRun,
	}

	if options.OutputDir == "" {
		options.OutputDir = "Data/ExportArchive"
	}
	if _, err := time.LoadLocation(options.TimeZone); err != nil || options.TimeZone == "Local" {
		options.TimeZone = "Asia/Shanghai"
	}
	if options.PageSize < 1000 {
		options.PageSize = 1000
	}
	if options.PageSize > 20000 {
		options.PageSize = 20000
	}
	if options.Lease <= 0 {
		options.Lease = 5 * time.Minute
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 8
	}
	if options.LocalRetention < 0 {
		options.LocalRetention = 0
	}
	if options.PurgeMainMonths <= 0 {
		options.PurgeMainMonths = 2
	}
	if options.MaturityThreshold <= 0 || options.MaturityThreshold > 1 {
		options.MaturityThreshold = 0.95
	}
	if options.PatchBudgetRatio <= 0 || options.PatchBudgetRatio > 0.5 {
		options.PatchBudgetRatio = 0.10
	}
	if options.Compression != CompressionGzip {
		options.Compression = CompressionNone
	}
	if options.FetchLimit <= 0 {
		options.FetchLimit = 2000
	}
	if options.MaxJobsPerRun <= 0 {
		options.MaxJobsPerRun = 200
	}
	switch options.UploadMode {
	case UploadModeDir, UploadModeHTTP, UploadModeNone:
	default:
		options.UploadMode = UploadModeNone
	}
	if options.WorkerID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "finvquant"
		}
		options.WorkerID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return options
}

// 上传实现模式。
const (
	UploadModeNone = "none"
	UploadModeDir  = "dir"
	UploadModeHTTP = "http"
)

// InWindow 判断给定时刻是否落在导出低峰窗口内（支持跨零点窗口）。
func (o Options) InWindow(at time.Time) bool {
	if o.WindowStart == o.WindowEnd {
		return true
	}
	local := at.In(o.Location)
	minute := local.Hour()*60 + local.Minute()
	if o.WindowStart < o.WindowEnd {
		return minute >= o.WindowStart && minute < o.WindowEnd
	}
	return minute >= o.WindowStart || minute < o.WindowEnd
}

// Now 返回业务时区的当前时间。
func (o Options) Now() time.Time {
	return time.Now().In(o.Location)
}

// Today 返回业务时区的当天日期（00:00）。
func (o Options) Today() time.Time {
	now := o.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, o.Location)
}

// CurrentMonth 返回业务时区的当前月份 yyyyMM。
func (o Options) CurrentMonth() int {
	now := o.Now()
	return now.Year()*100 + int(now.Month())
}

// TargetPath 返回上传目标相对路径（prefix/文件名），分隔符统一为 `/`。
func (o Options) TargetPath(fileName string) string {
	if o.UploadPrefix == "" {
		return fileName
	}
	return o.UploadPrefix + "/" + fileName
}

// parseClockMinutes 解析 HH:MM 为自 0 点起的分钟数；非法值回退默认值。
func parseClockMinutes(value string, fallback int) int {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return fallback
	}
	hour, errHour := strconv.Atoi(strings.TrimSpace(parts[0]))
	minute, errMinute := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fallback
	}
	return hour*60 + minute
}
