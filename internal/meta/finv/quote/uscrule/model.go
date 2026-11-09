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
	DiffAdded     = "added"
	DiffChanged   = "changed"
	DiffRemoved   = "removed"
	DiffUnchanged = "unchanged"
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

// Preview 预演结果（不落库）。
type Preview struct {
	Env          string    `json:"env"`
	RuleVersion  string    `json:"rule_version"`
	ComputedAt   string    `json:"computed_at"`
	Total        int64     `json:"total"`
	Added        int64     `json:"added"`
	Changed      int64     `json:"changed"`
	Removed      int64     `json:"removed"`
	Reused       int64     `json:"reused"`
	Unchanged    int64     `json:"unchanged"`
	Escalated    int64     `json:"escalated"`
	PreviewToken string    `json:"preview_token"`
	Samples      []DiffRow `json:"samples"`
	Conflicts    []string  `json:"conflicts"`
}

// Result 一次应用（同步）的结果。
type Result struct {
	Env         string `json:"env"`
	RuleVersion string `json:"rule_version"`
	Added       int64  `json:"added"`
	Changed     int64  `json:"changed"`
	Removed     int64  `json:"removed"`
	Reused      int64  `json:"reused"`
	Unchanged   int64  `json:"unchanged"`
	Escalated   int64  `json:"escalated"`
	Archived    int64  `json:"archived"`
	KeyUpdated  int64  `json:"key_updated"`
	ElapsedMs   int64  `json:"elapsed_ms"`
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
	MapRetired   int64    `json:"map_retired"`
	MapReused    int64    `json:"map_reused"`
	KeyTotal     int64    `json:"key_total"`
	KeyMatched   int64    `json:"key_matched"`
	KeyMismatch  int64    `json:"key_mismatch"`
	LastComputed string   `json:"last_computed_at"`
	ArchivedRows int64    `json:"archived_rows"`
	CronExpr     string   `json:"cron_expr"`
	CronNextRun  string   `json:"cron_next_run"`
	TargetTables []string `json:"target_tables"`
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
