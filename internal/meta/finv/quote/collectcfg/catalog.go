// Package collectcfg 负责「行情采集系统配置」页的数据访问（需求 11343）。
//
// 范围：`sys.sys_config` 里**与实时行情采集相关**的配置项，只做「查询 + 修改」，
// **不新增、不删除配置项**（配置项由迁移发放，服务运行期凭空造行会让其它模块的
// 「缺行回退默认值」语义失效）。
//
//	20013  Supabase 数据库连接配置（敏感）
//	20014  Supabase 证券键源表名
//	20015  USC 映射规则每日同步 cron（6 段）
//	20016  moomoo OpenAPI 凭据（敏感）
//	20018  moomoo 网页接口代理配置
//
// 20017 曾在 V300227 发放，V300228 已合并进 20016 并删除，因此不在本页展示。
//
// 刻意不含：20012（登录会话有效期，属认证域）、20002–20011（月归档上传与 CDEU
// 运行/清理策略，属行情数据归档域）——它们各有自己的维护页面或后续单独迁移。
//
// 安全约定：
//   - 敏感项在列表里只返回掩码（`Masked=true`），明文只能经 `SecretValue` 取出，
//     由 handler 以 **base64** 形式返回给已登录用户（不直接传明文，避免复制/日志中的明文残留）；
//   - 本包的日志/错误消息一律不包含配置值本身。
package collectcfg

import "time"

// 配置项编号（与 sys.sys_config 一致）。
const (
	// ConfigNoSupabaseConn 20013 Supabase 数据库连接配置，格式 `{PROJECT_ID}|{APIKEY}`。
	ConfigNoSupabaseConn = 20013
	// ConfigNoSupabaseSecuKeyTable 20014 Supabase 证券键源表名。
	ConfigNoSupabaseSecuKeyTable = 20014
	// ConfigNoUscRuleCron 20015 USC 映射规则每日同步 cron（6 段，空=关闭自动同步）。
	ConfigNoUscRuleCron = 20015
	// ConfigNoMoomooCredential 20016 moomoo OpenAPI 凭据，格式 `{AppKey}|{私钥 PEM}`。
	ConfigNoMoomooCredential = 20016
	// ConfigNoMoomooQuoteProxy 20018 moomoo 网页接口代理配置，格式 `{enable}|{base_url}`。
	ConfigNoMoomooQuoteProxy = 20018
)

// 模块分组（前端按此分区展示）。
const (
	ModuleMoomoo   = "moomoo 接入"
	ModuleSupabase = "Supabase 证券参数"
	ModuleUscRule  = "USC 映射规则"
)

// Item 配置项目录元数据（静态，不随库变化）。
type Item struct {
	// ConfigNo 配置编号（sys.sys_config.config_no）。
	ConfigNo int `json:"config_no"`
	// Name 配置项名称（沿用库里的 config_name 语义，页面展示用）。
	Name string `json:"name"`
	// Module 模块分组。
	Module string `json:"module"`
	// Sensitive 是否敏感：列表只回掩码，明文需单独取（base64）。
	Sensitive bool `json:"sensitive"`
	// ValueFormat 值格式说明（给运维填值用）。
	ValueFormat string `json:"value_format"`
	// Description 用途与生效时机。
	Description string `json:"description"`
	// EmptyHint 留空时的行为。
	EmptyHint string `json:"empty_hint"`
}

// catalog 目录（顺序即页面展示顺序：先 moomoo 接入，再证券参数，最后 USC 规则）。
var catalog = []Item{
	{
		ConfigNo:    ConfigNoMoomooCredential,
		Name:        "moomoo OpenAPI 凭据",
		Module:      ModuleMoomoo,
		Sensitive:   true,
		ValueFormat: "{AppKey}|{Ed25519 PKCS#8 私钥 PEM}",
		Description: "加密货币 / 美股港股指数快照采集用（webapi.moomoo.com，AppKey + Ed25519 签名）。" +
			"私钥可只填中间 base64 主体一行；留空私钥 = 保留原值，填 __CLEAR__ 只清私钥。保存后下一次采集生效。",
		EmptyHint: "空串 = 未配置，回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY",
	},
	{
		ConfigNo:    ConfigNoMoomooQuoteProxy,
		Name:        "moomoo 网页接口代理配置",
		Module:      ModuleMoomoo,
		Sensitive:   false,
		ValueFormat: "{enable}|{base_url}",
		Description: "网页行情接口（quote-api：列表 / K 线 / ETF / 期货 / 汇率 / 日 K）的中转代理。" +
			"enable 取 1/true/on/yes/y 启用；不作用于带凭据的 OpenAPI 通道。服务启动时读取一次，改后需重启。",
		EmptyHint: "空串 = 直连（不启用代理）",
	},
	{
		ConfigNo:    ConfigNoSupabaseConn,
		Name:        "Supabase 数据库连接配置",
		Module:      ModuleSupabase,
		Sensitive:   true,
		ValueFormat: "{PROJECT_ID}|{APIKEY}",
		Description: "证券参数同步从 Supabase 读取源数据的连接配置；" +
			"项目 ID 用于拼 https://{PROJECT_ID}.supabase.co。保存后下一次同步生效。",
		EmptyHint: "空串 = 未配置（证券参数同步直接报「Supabase 连接不可用」）",
	},
	{
		ConfigNo:    ConfigNoSupabaseSecuKeyTable,
		Name:        "Supabase 证券键源表名",
		Module:      ModuleSupabase,
		Sensitive:   false,
		ValueFormat: "table 或 schema.table",
		Description: "证券参数同步的源表名，最多两段；首段必须以字母或下划线开头、只允许字母数字下划线。",
		EmptyHint:   "空串 = 回退默认值 finv_quote_secu_key",
	},
	{
		ConfigNo:    ConfigNoUscRuleCron,
		Name:        "USC 映射规则每日同步 cron",
		Module:      ModuleUscRule,
		Sensitive:   false,
		ValueFormat: "6 段 cron（秒 分 时 日 月 周）",
		Description: "USC 映射规则每天自动「预演 → 应用」的调度表达式，由 USC 映射规则页的调度卡片共用同一配置项。" +
			"改后按新表达式重新排期。",
		EmptyHint: "空串 = 关闭自动同步（只能手动应用）",
	},
}

// Items 返回目录副本（调用方不要改动内部切片）。
func Items() []Item {
	out := make([]Item, len(catalog))
	copy(out, catalog)
	return out
}

// ItemByNo 按编号取目录项。
func ItemByNo(configNo int) (Item, bool) {
	for _, item := range catalog {
		if item.ConfigNo == configNo {
			return item, true
		}
	}
	return Item{}, false
}

// View 单条配置的当前状态（目录 + 库里的值）。
type View struct {
	Item
	// Value 当前值：敏感项为掩码（见 Masked），非敏感项为原值。
	Value string `json:"value"`
	// Masked true = Value 是掩码（原始值需经 SecretValue 取）。
	Masked bool `json:"masked"`
	// HasValue 库里是否已有非空值（敏感项判「是否已配置」用）。
	HasValue bool `json:"has_value"`
	// GmtModify 最近修改时间（库列 gmt_modify）。
	GmtModify *time.Time `json:"gmt_modify"`
}
