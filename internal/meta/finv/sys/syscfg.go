// Package syscfg 提供 sys.sys_config 系统配置的读写访问（带短 TTL 缓存）。
//
// 为什么需要它：归档上传的目标（开关 / PAT / 仓库定位）、CDEU 运行模式、
// 导出低峰窗口不再依赖环境变量，而是落到数据库配置表，运维在线修改后无需
// 重启进程即可生效。
//
// 约定：读操作带 TTL 缓存；写操作仅限单个配置项的 upsert（WriteString）。
// 所有 SQL 显式使用 sys. 前缀（仓库规范：融合后 schemas 可能存在同名表，
// 禁止依赖 search_path 解析）。
package syscfg

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sys_config 中与「月归档上传 Git 仓库」相关的配置编号（见 V100034/V100035 种子迁移）。
const (
	// KeyArchiveUpload 启用月归档数据自动上传 Git 仓库标志：'0'-关闭（默认） '1'-启用。
	KeyArchiveUpload = 20002
	// KeyArchiveUploadToken 归档数据上传 GitHub 仓库的 PAT 值（char_value 仅 1 字符，
	// 故令牌存放在 str_value；无默认值，由用户自行配置）。
	KeyArchiveUploadToken = 20003
	// KeyArchiveRepoSpec 文件上传仓库配置，格式 {owner}:{repo}:{branch}:{pre_path_key}。
	KeyArchiveRepoSpec = 20004
	// KeyCDEUMode CDEU 工作流运行模式：Manual-手动（默认，仅手动触发）/ Auto-自动（按定时任务时间跑）。
	// char_value 仅 1 字符，故模式串存放在 str_value（data_type='3'）。
	KeyCDEUMode = 20005
	// KeyExportWindow 导出低峰窗口，格式 HH:MM:SS|HH:MM:SS（精确到秒，半角竖线分隔）。
	// 例：00:30:00|06:30:00；支持跨零点；start == end 视为全天放行。
	KeyExportWindow = 20006
	// KeyPurgeAfterExport 导出成功后是否立即删除主表行情数据（char_value：'1' 删除 / '0' 保留）。
	KeyPurgeAfterExport = 20007
	// KeyDynamicJobRetentionDays 历史运行动态 JOB 实例的保留天数（int_value，默认 30）。
	// 自 V200045 起 cdeu_job_dynamic 每次运行各一条，超过该天数的历史实例由
	// PURGE_STALE_DYNAMIC（归档复核阶段）清理。
	KeyDynamicJobRetentionDays = 20008
	// KeyHistoryPurgePolicy 历史流程运行数据的清理策略（str_value）：
	//   Off   不清理
	//   Days  删除超过 N 天的 workflow_run（N 取 20010）
	//   Count 只保留最近 N 个 workflow_run（N 取 20010）
	// 默认 Days。由 PURGE_HISTORY_RUNS（归档复核阶段）执行，见需求 10723。
	KeyHistoryPurgePolicy = 20009
	// KeyHistoryPurgeKeep 历史流程运行数据的保留阈值（int_value，默认 30）：
	// 20009=Days 时为天数，=Count 时为保留个数，=Off 时忽略。
	KeyHistoryPurgeKeep = 20010
	// KeyPurgeLocalExportFiles 归档复核阶段是否清理本地导出的 .mvsv / Manifest 文件
	// （char_value：'1' 清理（默认） / '0' 不清理）。
	// 由 PHASE_REVIEW 的 PROCESS_PURGE / JOB_PURGE_LOCAL_FILES 执行。
	KeyPurgeLocalExportFiles = 20011
	// KeyAuthSessionTTLMinutes 登录会话有效期（int_value，分钟，默认 480）。
	// 会话存放于 Redis；每次命中且剩余时间不足一半时滑动续期，
	// 因此该值同时是「空闲超时」的上界。见需求 10761。
	KeyAuthSessionTTLMinutes = 20012
)

// 20012 登录会话有效期取值约束。
const (
	// AuthSessionTTLDefaultMinutes 缺失或非法时的内置默认值（8 小时）。
	AuthSessionTTLDefaultMinutes = 480
	// AuthSessionTTLMinMinutes 允许的最小值。
	AuthSessionTTLMinMinutes = 5
	// AuthSessionTTLMaxMinutes 允许的最大值（约 30 天）。
	AuthSessionTTLMaxMinutes = 43200
)

// NormalizeAuthSessionTTLMinutes 把 20012 的原始取值归一化到合法区间。
//
// 越界或非法（<= 0）时回退默认 480 分钟，保证会话有效期始终有意义。
func NormalizeAuthSessionTTLMinutes(raw int) int {
	if raw < AuthSessionTTLMinMinutes || raw > AuthSessionTTLMaxMinutes {
		return AuthSessionTTLDefaultMinutes
	}
	return raw
}

// PurgeLocalExportFilesDelay 本地文件在上传成功后的最短保留时长（需求 10744）。
//
// 保留 24 小时是为了给「刚上传完就发现异常」留出人工干预窗口，
// 同时避开上传与清理在同一轮流程里相邻执行造成的时间耦合。
const PurgeLocalExportFilesDelay = 24 * time.Hour

// 20009 历史流程运行数据清理策略取值。
const (
	// HistoryPurgeOff 不清理历史流程运行数据。
	HistoryPurgeOff = "Off"
	// HistoryPurgeDays 按天数清理（默认）。
	HistoryPurgeDays = "Days"
	// HistoryPurgeCount 按保留个数清理。
	HistoryPurgeCount = "Count"
)

// HistoryPurgeKeepDefault 20010 缺失或取值非法时的内置默认阈值。
const HistoryPurgeKeepDefault = 30

// HistoryPurgeKeepMax 20010 允许的最大阈值（约 10 年 / 10 年量级的运行数）。
const HistoryPurgeKeepMax = 3650

// DynamicJobRetentionDefault 20008 缺失或取值非法时的内置默认保留天数。
const DynamicJobRetentionDefault = 30

// DynamicJobRetentionMax 20008 允许的最大保留天数（约 10 年）。
const DynamicJobRetentionMax = 3650

// ExportWindowDefault 20006 缺失或取值非法时的内置降级默认值（00:30:00 ~ 06:30:00）。
const ExportWindowDefault = "00:30:00|06:30:00"

// ExportWindow 20006 的解析结果（自 0 点起的秒数，0 ~ 86399）。
type ExportWindow struct {
	Start int // 窗口开始（秒）
	End   int // 窗口结束（秒）
}

// ParseExportWindow 解析 20006 的取值：HH:MM:SS|HH:MM:SS。
//
// 严格校验：必须两段、必须带秒、必须落在 00:00:00 ~ 23:59:59。
func ParseExportWindow(raw string) (ExportWindow, error) {
	parts := strings.Split(strings.TrimSpace(raw), "|")
	if len(parts) != 2 {
		return ExportWindow{}, fmt.Errorf("导出低峰窗口格式应为 HH:MM:SS|HH:MM:SS（当前 %q）", raw)
	}
	start, err := ParseClockSeconds(parts[0])
	if err != nil {
		return ExportWindow{}, fmt.Errorf("窗口开始时间非法: %w", err)
	}
	end, err := ParseClockSeconds(parts[1])
	if err != nil {
		return ExportWindow{}, fmt.Errorf("窗口结束时间非法: %w", err)
	}
	return ExportWindow{Start: start, End: end}, nil
}

// ParseClockSeconds 解析 HH:MM:SS 为自 0 点起的秒数；拒绝 HH:MM（必须精确到秒）。
func ParseClockSeconds(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("%q 应为 HH:MM:SS", value)
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	second, errSecond := strconv.Atoi(parts[2])
	if errHour != nil || errMinute != nil || errSecond != nil ||
		hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, fmt.Errorf("%q 超出范围（00:00:00 ~ 23:59:59）", value)
	}
	return hour*3600 + minute*60 + second, nil
}

// FormatExportWindow 将起止秒数格式化为 HH:MM:SS|HH:MM:SS（用于回写与展示）。
func FormatExportWindow(window ExportWindow) string {
	return FormatClockSeconds(window.Start) + "|" + FormatClockSeconds(window.End)
}

// FormatClockSeconds 把自 0 点起的秒数格式化为 HH:MM:SS。
func FormatClockSeconds(seconds int) string {
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
}

// CDEU 工作流运行模式取值（20005）。
const (
	// CDEUModeManual 手动模式（默认）：只有手动触发（Job/Run）才执行流程，
	// 进程内调度器不自动触发任何 CDEU Job。
	CDEUModeManual = "Manual"
	// CDEUModeAuto 自动模式：每天按定时任务设定的时间自动跑流程。
	CDEUModeAuto = "Auto"
)

// NormalizeCDEUMode 归一化 20005 取值：大小写不敏感，仅 'auto' 视为自动模式，
// 其余（空值、未知值、未配置）一律返回 Manual——安全侧默认：不自动跑归档。
func NormalizeCDEUMode(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), CDEUModeAuto) {
		return CDEUModeAuto
	}
	return CDEUModeManual
}

// DefaultTTL 配置缓存有效期：在线修改最迟 TTL 后生效，避免每次上传都查库。
const DefaultTTL = 30 * time.Second

// 配置层面的可识别错误（调用方可用 errors.Is 判断并给出运维提示）。
var (
	// ErrRepoSpecEmpty 20004 未配置。
	ErrRepoSpecEmpty = errors.New("系统配置 20004（文件上传仓库配置）未配置")
	// ErrSwitchOff 20002 未启用。
	ErrSwitchOff = errors.New("系统配置 20002（启用月归档数据自动上传Git仓库标志）未启用，跳过上传")
	// ErrTokenMissing 20003 未配置。
	ErrTokenMissing = errors.New("系统配置 20003（归档数据上传GitHub仓库的PAT值）未配置")
	// ErrExportWindowEmpty 20006 未配置（调用方回退内置默认窗口）。
	ErrExportWindowEmpty = errors.New("系统配置 20006（导出低峰窗口）未配置，已回退默认 " + ExportWindowDefault)
)

// RepoSpec 是 20004 配置值的解析结果：{owner}:{repo}:{branch}:{pre_path_key}。
type RepoSpec struct {
	Owner      string // 仓库所有者
	Repo       string // 仓库名
	Branch     string // 分支名
	PathPrefix string // 文件保存的目录前缀（已去掉首尾 '/'，可为空表示仓库根）
	Raw        string // 原始配置值（便于日志排查，不含敏感信息）
}

// ParseRepoSpec 解析并校验 20004 的取值。
func ParseRepoSpec(raw string) (RepoSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return RepoSpec{}, ErrRepoSpecEmpty
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) != 4 {
		return RepoSpec{}, fmt.Errorf(
			"文件上传仓库配置应为 {owner}:{repo}:{branch}:{pre_path_key}（半角冒号分隔 4 段），当前 %d 段",
			len(parts))
	}
	spec := RepoSpec{
		Owner:      strings.TrimSpace(parts[0]),
		Repo:       strings.TrimSpace(parts[1]),
		Branch:     strings.TrimSpace(parts[2]),
		PathPrefix: strings.Trim(strings.TrimSpace(parts[3]), "/"),
		Raw:        trimmed,
	}
	for _, segment := range []struct {
		name  string
		value string
	}{
		{"owner", spec.Owner},
		{"repo", spec.Repo},
		{"branch", spec.Branch},
	} {
		if err := validateNameSegment(segment.name, segment.value); err != nil {
			return RepoSpec{}, err
		}
	}
	if err := validatePathPrefix(spec.PathPrefix); err != nil {
		return RepoSpec{}, err
	}
	return spec, nil
}

// validateNameSegment 校验 owner/repo/branch：非空、不含分隔符与空白。
func validateNameSegment(name, value string) error {
	if value == "" {
		return fmt.Errorf("文件上传仓库配置的 %s 段为空", name)
	}
	if strings.ContainsAny(value, ":/\\") || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("文件上传仓库配置的 %s 段非法（%q）：不能包含 ':'、'/'、'\\' 或空白", name, value)
	}
	return nil
}

// validatePathPrefix 校验目录前缀：允许分层，但禁止绝对路径与 '..' 越界。
func validatePathPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if strings.ContainsAny(prefix, " \t\r\n") {
		return fmt.Errorf("文件上传仓库配置的 pre_path_key 段非法（%q）：不能包含空白", prefix)
	}
	for _, segment := range strings.Split(prefix, "/") {
		switch segment {
		case "":
			return fmt.Errorf("文件上传仓库配置的 pre_path_key 段非法（%q）：存在空路径段", prefix)
		case ".", "..":
			return fmt.Errorf("文件上传仓库配置的 pre_path_key 段非法（%q）：不允许 '.' 或 '..'", prefix)
		}
	}
	return nil
}

// Join 拼接仓库内的相对路径：pre_path_key/targetPath（自动归一化分隔符）。
func (s RepoSpec) Join(targetPath string) string {
	cleaned := strings.TrimLeft(strings.TrimSpace(targetPath), "/")
	if cleaned == "" {
		return s.PathPrefix
	}
	if s.PathPrefix == "" {
		return cleaned
	}
	return s.PathPrefix + "/" + cleaned
}

// Values 一组归档上传相关的系统配置快照。
type Values struct {
	ArchiveUploadEnabled bool         // 20002 == '1'
	ArchiveToken         string       // 20003（去空白；未配置时为 ""）
	ArchiveRepo          RepoSpec     // 20004 解析结果
	ArchiveRepoErr       error        // 20004 解析错误（未配置 / 格式非法）
	CDEUMode             string       // 20005 归一化后的运行模式（Manual / Auto）
	CDEUModeRaw          string       // 20005 原始取值（便于诊断非法配置）
	ExportWindow         ExportWindow // 20006 解析结果（秒）
	ExportWindowRaw      string       // 20006 原始取值
	ExportWindowErr      error        // 20006 解析错误（缺失 / 格式非法），非 nil 时调用方回退默认值
	PurgeAfterExport     bool         // 20007 == '1'（默认 true：上传成功后删主表数据）
	// DynamicJobRetentionDays 20008 解析结果（天）；缺失或非法时为默认 30。
	DynamicJobRetentionDays int // 20008（天）
	DynamicJobRetentionRaw  int // 20008 原始取值（便于诊断非法配置）
	// HistoryPurgePolicy 20009 归一化后的清理策略（Off / Days / Count）。
	HistoryPurgePolicy string // 20009
	// HistoryPurgeKeep 20010 解析结果（天数或个数，取决于 20009）；缺失或非法时为默认 30。
	HistoryPurgeKeep int // 20010
	// PurgeLocalExportFiles 20011：归档复核阶段是否清理本地导出文件（默认 true）。
	PurgeLocalExportFiles bool // 20011 != '0'
	// AuthSessionTTLMinutes 20012：登录会话有效期（分钟，默认 480）。
	AuthSessionTTLMinutes int       // 20012（已归一化到 5~43200）
	LoadedAt              time.Time // 快照时间
}

// CDEUScheduleEnabled 是否允许进程内调度器自动触发 CDEU Job。
//
// 只有 20005 明确为 Auto 时才允许；Manual（默认）、空值、未配置一律不允许，
// 保证「默认不自动跑归档」的安全侧行为。
func (v Values) CDEUScheduleEnabled() bool {
	return NormalizeCDEUMode(v.CDEUModeRaw) == CDEUModeAuto
}

// Target 归档上传目标：凭据 + 仓库定位。
type Target struct {
	Token string
	Repo  RepoSpec
}

// ArchiveTarget 校验配置完整性并返回上传目标；任一环节缺失都返回可读错误。
func (v Values) ArchiveTarget() (Target, error) {
	if !v.ArchiveUploadEnabled {
		return Target{}, ErrSwitchOff
	}
	if v.ArchiveRepoErr != nil {
		return Target{}, v.ArchiveRepoErr
	}
	if strings.TrimSpace(v.ArchiveToken) == "" {
		return Target{}, ErrTokenMissing
	}
	return Target{Token: strings.TrimSpace(v.ArchiveToken), Repo: v.ArchiveRepo}, nil
}

// selectConfigSQL 读取配置；char_value 承载 1/0 开关，str_value 承载 PAT、仓库定位、
// 运行模式与窗口，int_value 承载天数类整数配置（如 20008）。
const selectConfigSQL = `
SELECT config_no, char_value, str_value, int_value
  FROM sys.sys_config
 WHERE config_no IN ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

// Store sys.sys_config 读取器（短 TTL 缓存；并发安全）。
type Store struct {
	load  func(ctx context.Context) (Values, error)
	write func(ctx context.Context, configNo int, value string) error // nil 表示不支持写入
	ttl   time.Duration
	now   func() time.Time

	mu       sync.Mutex
	cached   Values
	hasCache bool
	cachedAt time.Time
}

// NewStore 创建基于 pgx 连接池的配置读写器。
func NewStore(pool *pgxpool.Pool) *Store {
	store := &Store{ttl: DefaultTTL, now: time.Now}
	if pool == nil {
		store.load = func(context.Context) (Values, error) {
			return Values{}, errors.New("数据库连接池未初始化，无法读取 sys.sys_config")
		}
		return store
	}
	store.load = func(ctx context.Context) (Values, error) {
		return loadFromPool(ctx, pool)
	}
	store.write = func(ctx context.Context, configNo int, value string) error {
		return writeStringToPool(ctx, pool, configNo, value)
	}
	return store
}

// WriteString 写入单个字符串配置项（data_type='3' 的 str_value），成功后失效缓存。
//
// 仅供用户主动修改配置（如在线调整窗口）使用，不属于种子发放，因此允许覆盖；
// 种子迁移仍必须遵守「不覆盖用户配置」红线。
func (s *Store) WriteString(ctx context.Context, configNo int, value string) error {
	if s.write == nil {
		return errors.New("当前配置读取器不支持写入")
	}
	if err := s.write(ctx, configNo, value); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

// writeStringToPool 将配置写入 sys.sys_config（仅更新 str_value 与修改标记）。
//
// 只更新已存在的行：新增配置必须走种子迁移，避免服务在运行期凭空造出配置项。
func writeStringToPool(ctx context.Context, pool *pgxpool.Pool, configNo int, value string) error {
	tag, err := pool.Exec(ctx, `
UPDATE sys.sys_config
   SET str_value = $2, gmt_modify = CURRENT_TIMESTAMP(6), modify_by = 'finv'
 WHERE config_no = $1`, configNo, value)
	if err != nil {
		return fmt.Errorf("写入 sys.sys_config %d 失败: %w", configNo, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("系统配置 %d 不存在，请先执行对应的种子迁移", configNo)
	}
	return nil
}

// newStoreWithLoader 供测试注入加载函数。
func newStoreWithLoader(load func(ctx context.Context) (Values, error), ttl time.Duration) *Store {
	return &Store{load: load, ttl: ttl, now: time.Now}
}

// Load 返回配置快照；未过期时命中缓存，过期后重新读库。
//
// 读库失败时不做「粘性回退」：让调用方明确感知配置不可用，避免把文件
// 静默上传到环境变量兜底的目标而绕过运维意图。
func (s *Store) Load(ctx context.Context) (Values, error) {
	if values, ok := s.cachedValues(); ok {
		return values, nil
	}
	values, err := s.load(ctx)
	if err != nil {
		return Values{}, err
	}
	s.mu.Lock()
	s.cached, s.hasCache, s.cachedAt = values, true, s.now()
	s.mu.Unlock()
	return values, nil
}

// Invalidate 主动失效缓存（配置被在线修改后立即生效）。
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.hasCache = false
	s.mu.Unlock()
}

func (s *Store) cachedValues() (Values, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasCache {
		return Values{}, false
	}
	if s.ttl > 0 && s.now().Sub(s.cachedAt) >= s.ttl {
		return Values{}, false
	}
	return s.cached, true
}

// loadFromPool 从 sys.sys_config 读取配置并解析。
func loadFromPool(ctx context.Context, pool *pgxpool.Pool) (Values, error) {
	rows, err := pool.Query(ctx, selectConfigSQL, KeyArchiveUpload, KeyArchiveUploadToken, KeyArchiveRepoSpec, KeyCDEUMode, KeyExportWindow, KeyPurgeAfterExport, KeyDynamicJobRetentionDays, KeyHistoryPurgePolicy, KeyHistoryPurgeKeep, KeyPurgeLocalExportFiles, KeyAuthSessionTTLMinutes)
	if err != nil {
		return Values{}, fmt.Errorf("查询 sys.sys_config 失败: %w", err)
	}
	defer rows.Close()

	values := Values{LoadedAt: time.Now()}
	purgeSeen := false // 20007 是否命中（用于区分「缺行」与「显式 0」）
	retentionSeen := false
	policySeen := false
	keepSeen := false
	localSeen := false
	authTTLSeen := false
	for rows.Next() {
		var (
			configNo  int
			charValue *string
			strValue  *string
			intValue  *int32
		)
		if err := rows.Scan(&configNo, &charValue, &strValue, &intValue); err != nil {
			return Values{}, fmt.Errorf("读取 sys.sys_config 记录失败: %w", err)
		}
		switch configNo {
		case KeyArchiveUpload:
			values.ArchiveUploadEnabled = strings.TrimSpace(stringValue(charValue)) == "1"
		case KeyArchiveUploadToken:
			values.ArchiveToken = strings.TrimSpace(stringValue(strValue))
		case KeyArchiveRepoSpec:
			spec, specErr := ParseRepoSpec(stringValue(strValue))
			values.ArchiveRepo = spec
			if specErr != nil {
				values.ArchiveRepoErr = specErr
			}
		case KeyCDEUMode:
			values.CDEUModeRaw = strings.TrimSpace(stringValue(strValue))
			values.CDEUMode = NormalizeCDEUMode(values.CDEUModeRaw)
		case KeyExportWindow:
			values.ExportWindowRaw = strings.TrimSpace(stringValue(strValue))
			window, windowErr := ParseExportWindow(values.ExportWindowRaw)
			values.ExportWindow = window
			values.ExportWindowErr = windowErr
		case KeyPurgeAfterExport:
			// '0' 保留数据；其余（'1'、空）按立即删除。
			purgeSeen = true
			values.PurgeAfterExport = strings.TrimSpace(stringValue(charValue)) != "0"
		case KeyDynamicJobRetentionDays:
			retentionSeen = true
			raw := int(intValueValue(intValue))
			values.DynamicJobRetentionRaw = raw
			values.DynamicJobRetentionDays = NormalizeDynamicJobRetentionDays(raw)
		case KeyHistoryPurgePolicy:
			policySeen = true
			values.HistoryPurgePolicy = NormalizeHistoryPurgePolicy(stringValue(strValue))
		case KeyHistoryPurgeKeep:
			keepSeen = true
			values.HistoryPurgeKeep = NormalizeHistoryPurgeKeep(int(intValueValue(intValue)))
		case KeyPurgeLocalExportFiles:
			localSeen = true
			// '0' 不清理；其余（'1'、空）按清理——与 20007 同口径。
			values.PurgeLocalExportFiles = strings.TrimSpace(stringValue(charValue)) != "0"
		case KeyAuthSessionTTLMinutes:
			authTTLSeen = true
			values.AuthSessionTTLMinutes = NormalizeAuthSessionTTLMinutes(int(intValueValue(intValue)))
		}
	}
	if err := rows.Err(); err != nil {
		return Values{}, fmt.Errorf("遍历 sys.sys_config 结果失败: %w", err)
	}
	// 缺行时同样视为「未配置」，保持与 str_value 为空一致的处理。
	if values.ArchiveRepo.Raw == "" && values.ArchiveRepoErr == nil {
		values.ArchiveRepoErr = ErrRepoSpecEmpty
	}
	// 20005 缺行时按 Manual（安全侧：不自动跑）。
	if values.CDEUMode == "" {
		values.CDEUMode = CDEUModeManual
	}
	// 20007 缺行时按 '1'（立即删除）兜底，与种子默认值保持一致。
	if !purgeSeen {
		values.PurgeAfterExport = true
	}
	// 20008 缺行时按默认 30 天兜底。
	if !retentionSeen {
		values.DynamicJobRetentionDays = DynamicJobRetentionDefault
	}
	// 20009 缺行时按 Days 兜底（与服务默认行为一致）。
	if !policySeen || values.HistoryPurgePolicy == "" {
		values.HistoryPurgePolicy = HistoryPurgeDays
	}
	// 20010 缺行时按默认 30 兜底。
	if !keepSeen {
		values.HistoryPurgeKeep = HistoryPurgeKeepDefault
	}
	// 20011 缺行时按 '1'（清理）兜底，与种子默认值一致。
	if !localSeen {
		values.PurgeLocalExportFiles = true
	}
	// 20012 缺行时按默认 480 分钟（8 小时）兜底。
	if !authTTLSeen {
		values.AuthSessionTTLMinutes = AuthSessionTTLDefaultMinutes
	}
	// 20006 缺行或为空时按内置默认值降级（不再回读环境变量）。
	if values.ExportWindowRaw == "" {
		fallback, _ := ParseExportWindow(ExportWindowDefault)
		values.ExportWindow = fallback
		values.ExportWindowErr = ErrExportWindowEmpty
	}
	return values, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// intValueValue 安全读取可空的 int_value。
func intValueValue(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

// NormalizeDynamicJobRetentionDays 校验 20008 取值：
// 落在 1~DynamicJobRetentionMax 内直接采用，否则回退默认 30 天。
func NormalizeDynamicJobRetentionDays(days int) int {
	if days < 1 || days > DynamicJobRetentionMax {
		return DynamicJobRetentionDefault
	}
	return days
}

// NormalizeHistoryPurgePolicy 归一化 20009 清理策略：空值或非法值回退 Days。
func NormalizeHistoryPurgePolicy(raw string) string {
	switch strings.TrimSpace(raw) {
	case HistoryPurgeOff, "off", "OFF":
		return HistoryPurgeOff
	case HistoryPurgeCount, "count", "COUNT":
		return HistoryPurgeCount
	case HistoryPurgeDays, "days", "DAYS":
		return HistoryPurgeDays
	default:
		return HistoryPurgeDays
	}
}

// NormalizeHistoryPurgeKeep 归一化 20010 保留阈值：越界或非法回退默认 30。
func NormalizeHistoryPurgeKeep(keep int) int {
	if keep < 1 || keep > HistoryPurgeKeepMax {
		return HistoryPurgeKeepDefault
	}
	return keep
}
