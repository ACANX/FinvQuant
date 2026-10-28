// Package config 加载服务端配置（环境变量驱动，默认值适配 Docker Compose）。
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config 服务端运行配置。
type Config struct {
	// Server 监听地址与端口（默认 0.0.0.0:16001）
	Host string
	Port string

	// PostgreSQL 18
	PgHost     string
	PgPort     string
	PgUser     string
	PgPassword string
	PgDatabase string

	// Redis 8
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Gin 运行模式：debug / release
	Mode string

	// 分钟行情 API（融合 FinvQuoteDataDeDup）上限
	KLineMaxBatchRows int // POST /Batch、POST /Cdeu/Min/Upsert 的 items 上限（默认 5000）
	KLineMaxListSize  int // keys/uscs/PATCH 条目上限（默认 5000）

	// KLineTSUnit 入参 ts 单位：s=秒级（默认，与库内一致）；ms=接受 13 位毫秒并换算为秒。
	KLineTSUnit string

	// OpenAPIKey 外部 OpenAPI（/OpenAPI/V1）静态访问密钥（需求 10787，过渡方案）。
	// 为空时不注册外部路由；后续 API Key 体系落地后由 DB 凭证替换。
	OpenAPIKey string

	// 错误详情分级（DC-06）：
	//   safe  = 只输出安全文案（默认）
	//   admin = 允许高权限主体（调试令牌/内网 CIDR）获取 debug 字段
	ErrorDetailMode         string
	ErrorDebugToken         string
	ErrorDetailAllowedCIDRs []string

	// 证券行情月归档导出与上传（详见 Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md）
	QuoteDeDupEnabled         bool   // 功能开关，默认关闭；开启后由进程内调度器自动跑 10 个 Job
	QuoteDeDupTimeZone        string // 业务时区，默认 Asia/Shanghai
	QuoteDeDupOutputDir       string // 本地导出根目录
	QuoteDeDupWorkerID        string // worker 标识，空则用 hostname-pid
	QuoteDeDupPageSize        int    // LOAD 单页行数（5000~20000）
	QuoteDeDupLeaseSeconds    int    // 任务锁租约（秒）
	QuoteDeDupMaxAttempts     int    // 上传最大尝试次数
	QuoteDeDupUploadMode      string // none / dir / http
	QuoteDeDupUploadDir       string // dir 模式：独享存储本地挂载根目录
	QuoteDeDupUploadHTTPBase  string // http 模式：PUT 根地址
	QuoteDeDupUploadHTTPToken string // http 模式：Bearer 令牌（可选）
	QuoteDeDupUploadPrefix    string // 远端路径前缀（可选）
	// QuoteDeDupUploadLayout 远端目录布局：by_security（默认，{region}_{market}/{usc}/文件）
	// 或 flat（文件，兼容旧行为）。分层段与文件名同属命名契约，故不由 sys_config 20004 描述。
	QuoteDeDupUploadLayout string
	// QuoteDeDupUploadGitHubAPIBase GitHub API 端点（可选，默认 https://api.github.com；
	// 仅当 sys_config 20002='1' 走 Contents API 时才使用，GitHub Enterprise 可覆盖）。
	QuoteDeDupUploadGitHubAPIBase string
	QuoteDeDupLocalRetentionDay   int     // 本地文件保留天数
	QuoteDeDupMaturityThreshold   float64 // 成熟度阈值（默认 0.95）
	QuoteDeDupPatchBudgetRatio    float64 // Patch 独立预算占日预算比例（默认 0.10）
	QuoteDeDupCompression         string  // none / gzip
	QuoteDeDupFetchLimit          int     // 单轮候选生成上限
	QuoteDeDupMaxJobsPerRun       int     // 单轮 RUN_DUE_EXPORT_JOBS 最多处理任务数
}

// Load 从环境变量加载配置，缺失时使用默认值。
func Load() *Config {
	return &Config{
		Host:          envOr("FINV_SERVER_HOST", "0.0.0.0"),
		Port:          envOr("FINV_SERVER_PORT", "16001"),
		PgHost:        envOr("FINV_PG_HOST", "localhost"),
		PgPort:        envOr("FINV_PG_PORT", "5432"),
		PgUser:        envOr("FINV_PG_USER", "finv"),
		PgPassword:    envOr("FINV_PG_PASSWORD", "finvquant"),
		PgDatabase:    envOr("FINV_PG_DATABASE", "finv"),
		RedisAddr:     envOr("FINV_REDIS_ADDR", "localhost:6379"),
		RedisPassword: envOr("FINV_REDIS_PASSWORD", ""),
		RedisDB:       envIntOr("FINV_REDIS_DB", 0),
		Mode:          envOr("FINV_MODE", "release"),

		KLineMaxBatchRows: envIntOr("FINV_KLINE_MAX_BATCH_ROWS", 5000),
		KLineMaxListSize:  envIntOr("FINV_KLINE_MAX_LIST_SIZE", 5000),
		KLineTSUnit:       envLowerOr("FINV_KLINE_TS_UNIT", "s"),
		OpenAPIKey:        envOr("FINV_OPENAPI_KEY", ""),

		ErrorDetailMode:         envLowerOr("FINV_ERROR_DETAIL_MODE", "safe"),
		ErrorDebugToken:         os.Getenv("FINV_ERROR_DEBUG_TOKEN"),
		ErrorDetailAllowedCIDRs: envListOr("FINV_ERROR_DETAIL_ALLOWED_CIDRS"),

		QuoteDeDupEnabled:             envBoolOr("FINV_QUOTE_DEDUB_ENABLED", false),
		QuoteDeDupTimeZone:            envOr("FINV_QUOTE_DEDUB_TIMEZONE", "Asia/Shanghai"),
		QuoteDeDupOutputDir:           envOr("FINV_QUOTE_DEDUB_OUTPUT_DIR", "/data/App/FinvQuant/Export"),
		QuoteDeDupWorkerID:            os.Getenv("FINV_QUOTE_DEDUB_WORKER_ID"),
		QuoteDeDupPageSize:            envIntOr("FINV_QUOTE_DEDUB_PAGE_SIZE", 10000),
		QuoteDeDupLeaseSeconds:        envIntOr("FINV_QUOTE_DEDUB_LEASE_SECONDS", 300),
		QuoteDeDupMaxAttempts:         envIntOr("FINV_QUOTE_DEDUB_MAX_ATTEMPTS", 8),
		QuoteDeDupUploadMode:          envLowerOr("FINV_QUOTE_DEDUB_UPLOAD_MODE", "none"),
		QuoteDeDupUploadDir:           envOr("FINV_QUOTE_DEDUB_UPLOAD_DIR", "/data/App/FinvQuant/Export/Upload"),
		QuoteDeDupUploadHTTPBase:      os.Getenv("FINV_QUOTE_DEDUB_UPLOAD_HTTP_BASE"),
		QuoteDeDupUploadHTTPToken:     os.Getenv("FINV_QUOTE_DEDUB_UPLOAD_HTTP_TOKEN"),
		QuoteDeDupUploadPrefix:        os.Getenv("FINV_QUOTE_DEDUB_UPLOAD_PREFIX"),
		QuoteDeDupUploadLayout:        envLowerOr("FINV_QUOTE_DEDUB_UPLOAD_LAYOUT", "by_security"),
		QuoteDeDupUploadGitHubAPIBase: os.Getenv("FINV_QUOTE_DEDUB_UPLOAD_GITHUB_API_BASE"),
		QuoteDeDupLocalRetentionDay:   envIntOr("FINV_QUOTE_DEDUB_LOCAL_RETENTION_DAYS", 30),
		QuoteDeDupMaturityThreshold:   envFloatOr("FINV_QUOTE_DEDUB_MATURITY_THRESHOLD", 0.95),
		QuoteDeDupPatchBudgetRatio:    envFloatOr("FINV_QUOTE_DEDUB_PATCH_BUDGET_RATIO", 0.10),
		QuoteDeDupCompression:         envLowerOr("FINV_QUOTE_DEDUB_COMPRESSION", "none"),
		QuoteDeDupFetchLimit:          envIntOr("FINV_QUOTE_DEDUB_FETCH_LIMIT", 2000),
		QuoteDeDupMaxJobsPerRun:       envIntOr("FINV_QUOTE_DEDUB_MAX_JOBS_PER_RUN", 200),
	}
}

// ListenAddr 返回服务端监听地址。
func (c *Config) ListenAddr() string {
	return c.Host + ":" + c.Port
}

// WebListenAddr 返回前端监听地址（默认 0.0.0.0:16002）。
func (c *Config) WebListenAddr() string {
	return c.Host + ":" + envOr("FINV_WEB_PORT", "16002")
}

// MigrationsDir 返回数据库迁移目录（默认 Deploy/Migrations）。
func (c *Config) MigrationsDir() string {
	return envOr("FINV_MIGRATIONS_DIR", "Deploy/Migrations")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// envLowerOr 读取环境变量并统一转小写（用于枚举型配置）。
func envLowerOr(key, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value
}

// envListOr 读取逗号分隔的列表，忽略空白项。
func envListOr(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	items := make([]string, 0, 4)
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// envBoolOr 读取布尔环境变量（1/true/yes/on 为真，0/false/no/off 为假）。
func envBoolOr(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// envFloatOr 读取浮点环境变量（非法值回退默认值）。
func envFloatOr(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envIntOr(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		parsed = parsed*10 + int(r-'0')
	}
	return parsed
}
