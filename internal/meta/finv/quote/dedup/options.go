package dedup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/syscfg"
)

// Options 归档导出运行参数（由 FINV_QUOTE_DEDUB_* 环境变量经 config 派生）。
type Options struct {
	Enabled         bool
	Location        *time.Location
	TimeZone        string // 业务时区名（传给 PostgreSQL AT TIME ZONE）
	OutputDir       string
	WorkerID        string
	PageSize        int
	WindowStart     int // 低峰窗口开始（自 0 点起的**秒**数）；仅作兜底默认，运行时以 sys_config 20006 为准
	WindowEnd       int // 低峰窗口结束（自 0 点起的**秒**数）；仅作兜底默认，运行时以 sys_config 20006 为准
	Lease           time.Duration
	MaxAttempts     int
	UploadMode      string
	UploadDir       string
	UploadHTTPBase  string
	UploadHTTPToken string
	UploadPrefix    string
	// UploadLayout 远端目录布局：by_security（默认，{region}_{market}/{usc}/文件）
	// 或 flat（文件，兼容旧行为）。
	UploadLayout string
	// UploadGitHubAPIBase GitHub API 端点（空表示 api.github.com）；
	// 仅在 sys_config 20002='1' 走 Contents API 时使用。
	UploadGitHubAPIBase string
	LocalRetention      time.Duration
	MaturityThreshold   float64
	PatchBudgetRatio    float64
	Compression         string
	FetchLimit          int
	MaxJobsPerRun       int
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
		Enabled:             cfg.QuoteDeDupEnabled,
		Location:            location,
		TimeZone:            location.String(),
		OutputDir:           strings.TrimSpace(cfg.QuoteDeDupOutputDir),
		WorkerID:            strings.TrimSpace(cfg.QuoteDeDupWorkerID),
		PageSize:            cfg.QuoteDeDupPageSize,
		WindowStart:         defaultWindow().Start,
		WindowEnd:           defaultWindow().End,
		Lease:               time.Duration(cfg.QuoteDeDupLeaseSeconds) * time.Second,
		MaxAttempts:         cfg.QuoteDeDupMaxAttempts,
		UploadMode:          strings.ToLower(strings.TrimSpace(cfg.QuoteDeDupUploadMode)),
		UploadDir:           strings.TrimSpace(cfg.QuoteDeDupUploadDir),
		UploadHTTPBase:      strings.TrimSpace(cfg.QuoteDeDupUploadHTTPBase),
		UploadHTTPToken:     strings.TrimSpace(cfg.QuoteDeDupUploadHTTPToken),
		UploadPrefix:        strings.Trim(strings.TrimSpace(cfg.QuoteDeDupUploadPrefix), "/"),
		UploadLayout:        normalizedLayout(cfg.QuoteDeDupUploadLayout),
		UploadGitHubAPIBase: strings.TrimSpace(cfg.QuoteDeDupUploadGitHubAPIBase),
		LocalRetention:      time.Duration(cfg.QuoteDeDupLocalRetentionDay) * 24 * time.Hour,
		MaturityThreshold:   cfg.QuoteDeDupMaturityThreshold,
		PatchBudgetRatio:    cfg.QuoteDeDupPatchBudgetRatio,
		Compression:         strings.ToLower(strings.TrimSpace(cfg.QuoteDeDupCompression)),
		FetchLimit:          cfg.QuoteDeDupFetchLimit,
		MaxJobsPerRun:       cfg.QuoteDeDupMaxJobsPerRun,
	}

	if options.OutputDir == "" {
		options.OutputDir = "/data/App/FinvQuant/Export"
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
//
// UploadModeGitHub 比较特殊：它不由环境变量选择，而是由 sys.sys_config 驱动：
//
//	20002 = '1'                        启用开关
//	20003 = <PAT>                      GitHub Personal Access Token
//	20004 = {owner}:{repo}:{branch}:{pre_path_key}
//
// 其余模式（none/dir/http）仍由 FINV_QUOTE_DEDUB_UPLOAD_MODE 选择。
const (
	UploadModeNone   = "none"
	UploadModeDir    = "dir"
	UploadModeHTTP   = "http"
	UploadModeGitHub = "github"
)

// 远端归档目录布局（决定上传目标是否按证券分层）。
//
// 为什么放在代码而不是 sys.sys_config 20004：
//   - 20004 的 pre_path_key 是**静态仓库前缀**（如 Data/Finv/SecuQuoteMon/FTMM），
//     而 {region}_{market}/{usc} 是**逐文件变化**的，不可能用固定字符串表达；
//   - 分层段与文件名 {region}_{market}_{usc}_... 出自**同一命名契约**，放在
//     一起（本包 naming.go）才不会随时间漂移；
//   - 布局属于产品级归档约定，不是部署差异，不宜让运维用一个字符串改出多样目录；
//   - 若确需回退旧行为，用环境变量 FINV_QUOTE_DEDUB_UPLOAD_LAYOUT=flat 切换，
//     而不是把结构描述塞进 20004。
const (
	// UploadLayoutBySecurity {region}_{market}/{usc}/{fileName}（默认）。
	UploadLayoutBySecurity = "by_security"
	// UploadLayoutFlat {fileName}（不分子目录，兼容旧行为）。
	UploadLayoutFlat = "flat"
)

// InWindow 判断给定时刻是否落在低峰窗口内（支持跨零点窗口）。
//
// 注意：这里用的是 Options 上的**兜底默认值**；运行时应使用 Service.InWindow，
// 它读取 sys_config 20006 的在线配置。本方法保留供单元测试与 Options 自洽使用。
func (o Options) InWindow(at time.Time) bool {
	return inWindowAt(at, o.Location, o.WindowStart, o.WindowEnd)
}

// defaultWindow 返回内置降级窗口（来自 syscfg.ExportWindowDefault，秒级）。
func defaultWindow() syscfg.ExportWindow {
	window, err := syscfg.ParseExportWindow(syscfg.ExportWindowDefault)
	if err != nil {
		// 常量自身非法属于开发期错误，保底 00:30:00 ~ 06:30:00。
		return syscfg.ExportWindow{Start: 30 * 60, End: 6*3600 + 30*60}
	}
	return window
}

// inWindowAt 低峰窗口判定（start == end 表示全天；start > end 表示跨零点）。
func inWindowAt(at time.Time, location *time.Location, start, end int) bool {
	if start == end {
		return true
	}
	local := at.In(location)
	second := local.Hour()*3600 + local.Minute()*60 + local.Second()
	// 00:00:00 ~ 23:59:59 视为全天。
	if start == 0 && end >= 23*3600+59*60+59 {
		return true
	}
	if start < end {
		return second >= start && second < end
	}
	return second >= start || second < end
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
//
// 仅返回「前缀 + 文件名」，不含按证券分层的目录；需要归档分层时用 ArchiveTargetPath。
func (o Options) TargetPath(fileName string) string {
	if o.UploadPrefix == "" {
		return fileName
	}
	return o.UploadPrefix + "/" + fileName
}

// ArchiveTargetPath 返回归档文件的上传目标相对路径（不含 sys_config 20004 的 pre_path_key）。
//
// 布局（由 UploadLayout 决定；零值或非法值均按 by_security 处理）：
//
//	by_security : {UploadPrefix}/{region}_{market}/{usc}/{fileName}
//	flat        : {UploadPrefix}/{fileName}
//
// 例：region=USA、market=US-NSDQ、usc=NVDA、fileName=USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv
//
//	→ USA_US-NSDQ/NVDA/USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv
//
// 最终远端位置 = 20004 的 pre_path_key + 本函数返回值，例如：
//
//	Data/Finv/SecuQuoteMon/FTMM/USA_US-NSDQ/NVDA/USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv
//
// 目录段与文件名同样经 sanitizeFilePart 清洗，保证段内不含路径分隔符。
func (o Options) ArchiveTargetPath(region, market, usc, fileName string) string {
	parts := make([]string, 0, 4)
	if o.UploadPrefix != "" {
		parts = append(parts, o.UploadPrefix)
	}
	// 除显式 flat 外一律分层：Options 零值（如单测直接构造）也能得到默认布局。
	if o.UploadLayout != UploadLayoutFlat {
		parts = append(parts,
			safeDirSegment(sanitizeFilePart(region)+"_"+sanitizeFilePart(market)),
			safeDirSegment(sanitizeFilePart(usc)))
	}
	parts = append(parts, fileName)
	return strings.Join(parts, "/")
}

// safeDirSegment 确保单个目录段安全：空段或 '.'/'..' 替换为 UNKNOWN，避免路径越界。
func safeDirSegment(segment string) string {
	switch strings.TrimSpace(segment) {
	case "", ".", "..":
		return "UNKNOWN"
	default:
		return segment
	}
}

// normalizedLayout 收敛目录布局取值：仅认 by_security / flat，其余一律按默认分层。
func normalizedLayout(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UploadLayoutFlat:
		return UploadLayoutFlat
	default:
		return UploadLayoutBySecurity
	}
}

// ParseClock 解析 HH:MM 或 HH:MM:SS（24 小时制）为自 0 点起的**秒数**。
//
// 供在线修改窗口等场景校验；秒级窗口要求调用方传入 HH:MM:SS，
// 传 HH:MM 时秒部分按 0 处理（向后兼容）。
func ParseClock(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("时间格式应为 HH:MM 或 HH:MM:SS")
	}
	hour, errHour := strconv.Atoi(strings.TrimSpace(parts[0]))
	minute, errMinute := strconv.Atoi(strings.TrimSpace(parts[1]))
	second := 0
	var errSecond error
	if len(parts) == 3 {
		second, errSecond = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	if errHour != nil || errMinute != nil || errSecond != nil ||
		hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, fmt.Errorf("时间格式应为 HH:MM 或 HH:MM:SS（00:00:00 ~ 23:59:59）")
	}
	// 窗口判定为秒级；hour/minute/second 均已校验在合法范围内。
	return hour*3600 + minute*60 + second, nil
}
