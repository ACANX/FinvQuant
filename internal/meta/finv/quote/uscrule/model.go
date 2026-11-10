// Package uscrule 实现 USC 映射规则（孪生数据表）的维护、预演与应用（需求 11072）。
//
// 数据对象（迁移 V300157）：
//
//	futu.finv_futu_usc_rule         规则层（9 条：P10 闸门 / P12 P14 前处理 / P20-P22 市场默认 / P40 P50 例外 / P60 兜底）
//	futu.finv_futu_usc_rule_item    条目层（689 条；新增上市、退市、规则修正只动这张表）
//	futu.finv_futu_usc_map          结果表（ftmm_symbol → usc；ACTIVE 行大小写不敏感唯一，退市行允许被复用）
//	futu.finv_futu_usc_rule_log     审计日志
//	archive.finv_usc_reuse_snapshot 复用归档（usc 被复用时下游各表历史行整行快照）
//
// 计算口径：先由 futu.f_finv_futu_usc_calc(env) 做前处理（P12 别名 / P14 期货后缀）得到 canon_code 与
// 大小写不敏感计数 canon_cnt；canon_cnt=1 即 P10 闸门命中，直接定稿；否则按「精确 code → 市场+品种正则 →
// 市场默认 → 兜底」取第一条命中条目；候选仍与其它行撞名时升级为 ftmm_symbol；升级后仍撞名则整次中止。
package uscrule

// 表名常量（全部限定 schema，禁止裸表名）。
const (
	TableRule    = "futu.finv_futu_usc_rule"
	TableItem    = "futu.finv_futu_usc_rule_item"
	TableMap     = "futu.finv_futu_usc_map"
	TableLog     = "futu.finv_futu_usc_rule_log"
	TableArchive = "archive.finv_usc_reuse_snapshot"
	TableKey     = "futu.finv_futu_secu_key"
)

// ConfigCronNo 每日同步 cron 的系统配置编号（20015；20014 已被「Supabase 证券键源表名」占用）。
const ConfigCronNo = 20015

// DefaultEnv 默认环境标识（* 表示全部环境，与条目层 scope_env 的约定一致）。
const DefaultEnv = "*"

// MaxPreviewSamples 预演最多返回的明细行数（避免 7 万行全量回传）。
const MaxPreviewSamples = 200

// 规则类型（与 finv_futu_usc_rule.rule_type 的 CHECK 一致）。
const (
	RuleTypeGate          = "GATE"
	RuleTypeNormalize     = "NORMALIZE"
	RuleTypeMarketDefault = "MARKET_DEFAULT"
	RuleTypeException     = "EXCEPTION"
	RuleTypeFallback      = "FALLBACK"
)

// 条目动作（与 finv_futu_usc_rule_item.action 的 CHECK 一致）。
const (
	ActionCode    = "CODE"
	ActionFtmm    = "FTMM"
	ActionLiteral = "LITERAL"
	ActionRewrite = "REWRITE"
	ActionSH4     = "SH4"
)

// diff 分类。
const (
	DiffInsert = "insert"
	DiffUpdate = "update"
	DiffRetire = "retire"
	DiffKeep   = "keep"
)

// Rule 规则层一行。
type Rule struct {
	RuleNo        string  `json:"rule_no"`
	RuleName      string  `json:"rule_name"`
	RuleType      string  `json:"rule_type"`
	Priority      int     `json:"priority"`
	MatchScope    string  `json:"match_scope"`
	DefaultAction *string `json:"default_action"`
	FlagEnable    string  `json:"flag_enable"`
	RuleDesc      string  `json:"rule_desc"`
	Remark        string  `json:"remark"`
	ItemCount     int     `json:"item_count"`
	GmtUpdate     string  `json:"gmt_update"`
}

// Item 条目层一行（JSON 字段与库列名一致）。
type Item struct {
	IDN           int64   `json:"idn"`
	RuleNo        string  `json:"rule_no"`
	Market        *string `json:"market"`
	Instrument    *string `json:"instrument"`
	Code          *string `json:"code"`
	CodeRegex     *string `json:"code_regex"`
	Action        *string `json:"action"`
	USCLiteral    *string `json:"usc_literal"`
	RewriteTpl    *string `json:"rewrite_tpl"`
	Status        string  `json:"status"`
	EffectiveFrom *string `json:"effective_from"`
	EffectiveTo   *string `json:"effective_to"`
	Priority      int     `json:"priority"`
	ScopeEnv      string  `json:"scope_env"`
	ItemDesc      string  `json:"item_desc"`
	SourceRef     string  `json:"source_ref"`
	Remark        string  `json:"remark"`
	GmtUpdate     string  `json:"gmt_update"`
}

// ItemFilter 条目查询条件（空字符串表示不过滤）。
type ItemFilter struct {
	RuleNo   string
	Market   string
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// DiffRow 预演/应用的一条差异明细。
type DiffRow struct {
	FtmmSymbol string  `json:"ftmm_symbol"`
	Market     string  `json:"market"`
	Code       string  `json:"code"`
	Instrument string  `json:"instrument"`
	CurrentUSC *string `json:"current_usc"`
	NewUSC     *string `json:"new_usc"`
	RuleNo     string  `json:"rule_no"`
	Basis      string  `json:"basis"`
	Diff       string  `json:"diff"`
	ReuseOf    *string `json:"reuse_of_ftmm,omitempty"`
}

// MapRow 结果表（futu.finv_futu_usc_map）一行，供「预演与应用」页随时查询在册映射。
//
// 与预演明细无关：不依赖预演缓存与令牌，任何时刻都能按键/代码/usc 查。
type MapRow struct {
	FtmmSymbol   string  `json:"ftmm_symbol"`
	Market       string  `json:"market"`
	Code         string  `json:"code"`
	Instrument   string  `json:"instrument"`
	USC          string  `json:"usc"`
	RuleNo       string  `json:"rule_no"`
	Basis        string  `json:"basis"`
	Status       string  `json:"status"`
	ScopeEnv     string  `json:"scope_env"`
	RuleVersion  string  `json:"rule_version"`
	DtCompute    string  `json:"dt_compute"`
	DtRetire     *string `json:"dt_retire"`
	ReuseOfFtmm  *string `json:"reuse_of_ftmm"`
}

// MapFilter 结果表查询条件（关键字同时匹配 ftmm_symbol / code / usc）。
type MapFilter struct {
	Keyword  string
	Market   string
	Status   string
	Page     int
	PageSize int
}

// Preview 预演结果（不落库）。
type Preview struct {
	Env          string    `json:"env"`
	RuleVersion  string    `json:"rule_version"`
	ComputeAt    string    `json:"dt_compute"`
	Total        int64     `json:"total"`
	Insert       int64     `json:"insert"`
	Update       int64     `json:"update"`
	Retire       int64     `json:"retire"`
	Reuse        int64     `json:"reuse"`
	Keep         int64     `json:"keep"`
	Escalate     int64     `json:"escalate"`
	KeyUpdate    int64     `json:"key_update"`
	PreviewToken string    `json:"preview_token"`
	Samples      []DiffRow `json:"samples"`
	Conflicts    []string  `json:"conflicts"`
}

// Result 一次应用（同步）的结果。
type Result struct {
	Env         string `json:"env"`
	RuleVersion string `json:"rule_version"`
	Insert      int64  `json:"insert"`
	Update      int64  `json:"update"`
	Retire      int64  `json:"retire"`
	Reuse       int64  `json:"reuse"`
	Keep        int64  `json:"keep"`
	Escalate    int64  `json:"escalate"`
	Archive     int64  `json:"archive"`
	KeyUpdate   int64  `json:"key_update"`
	DurationMs  int64  `json:"duration_ms"`
	PreviewAt   string `json:"preview_at"`
}

// Status 页面只读快照。
type Status struct {
	Env          string   `json:"env"`
	RuleVersion  string   `json:"rule_version"`
	RuleCount    int64    `json:"rule_count"`
	ItemCount    int64    `json:"item_count"`
	ItemActive   int64    `json:"item_active"`
	MapActive    int64    `json:"map_active"`
	MapRetire    int64    `json:"map_retire"`
	MapReuse     int64    `json:"map_reuse"`
	KeyTotal     int64    `json:"key_total"`
	KeyMatch     int64    `json:"key_match"`
	KeyMiss      int64    `json:"key_miss"`
	LastCompute  string   `json:"last_compute_at"`
	ArchiveRows  int64    `json:"archive_rows"`
	CronExpr     string   `json:"cron_expr"`
	CronNextRun  string   `json:"cron_next_run"`
	TargetTables []string `json:"target_tables"`
}

// ScheduleConfig 每日同步的调度配置（sys_config 20015）。
type ScheduleConfig struct {
	CronExpr  string   `json:"cron_expr"`
	Enable    bool     `json:"enable"` // 表达式非空且合法
	Remark    string   `json:"remark"`
	GmtModify *string  `json:"gmt_modify"`
	NextRun   []string `json:"next_run"` // 未来 5 次执行时间（本地时区）
}

// LogRow 审计日志一行。
type LogRow struct {
	IDN       int64   `json:"idn"`
	SrcTable  string  `json:"src_table"`
	Action    string  `json:"action"`
	KeyText   string  `json:"key_text"`
	Before    *string `json:"before_json"`
	After     *string `json:"after_json"`
	Operator  string  `json:"operator"`
	Reason    string  `json:"reason"`
	GmtCreate string  `json:"gmt_create"`
}
