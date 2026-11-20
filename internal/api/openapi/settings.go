package openapi

// 外部 OpenAPI 鉴权的运行期配置（来源 sys.sys_config 19xxx 号段，需求 11167/11168）。
//
// 与 syscfg 包分开读取：syscfg.Values 的 SELECT 列表是固定的 20000 段配置，
// 往那里加 19xxx 会牵动既有测试与语义；这里自建一个短 TTL 缓存读取器即可。

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 鉴权模式（sys_config 19001 的 char_value）。
const (
	// ModeOff 关闭鉴权：全部放行，等价早期过渡实现（也是默认值）。
	ModeOff = "0"
	// ModeOn 启用鉴权：校验失败即拒绝。
	ModeOn = "1"
	// ModePermissive 宽松：照常校验并写审计日志，但校验失败仍放行（联调灰度用）。
	ModePermissive = "2"
)

// DefaultSettings 配置缺省值（与 V300183 的种子保持一致）。
var DefaultSettings = Settings{
	Mode:            ModeOff,
	SignTimeWindow:  300 * time.Second,
	LogRetainDays:   90,
	CredCacheTTL:    10 * time.Second,
	SecretKID:       "k1",
	NonceKeepWindow: 600 * time.Second, // 2 倍签名时间窗
}

// Settings 鉴权配置快照。
type Settings struct {
	// Mode 19001：0 关闭 / 1 启用 / 2 宽松；非法值按「关闭」处理（宁可放行也不能误锁外部调用方）。
	Mode string
	// SignTimeWindow 19002：签名时间戳允许偏差。
	SignTimeWindow time.Duration
	// LogRetainDays 19003：审计日志保留天数（0 表示不清理）。
	LogRetainDays int
	// SecretKID 19004：当前加密密钥版本。
	SecretKID string
	// SecretKeyBase64 19005：AES-256-GCM 密钥（Base64 32 字节）；空串表示未配置。
	SecretKeyBase64 string
	// FallbackKey 19006：兼容期兜底静态密钥；空串表示不启用。
	FallbackKey string
	// CredCacheTTL 19007：读凭证的 Redis 缓存时长（需求 11214/11215）。
	// 0 表示关闭缓存（每次查表）；UI/接口写入范围 0~600 秒，读取侧再收敛一次。
	CredCacheTTL time.Duration
	// NonceKeepWindow nonce 去重保留时长（2 倍签名时间窗）。
	NonceKeepWindow time.Duration
}

// AuthEnabled 是否启用鉴权（1 或 2）。
func (s Settings) AuthEnabled() bool { return s.Mode == ModeOn || s.Mode == ModePermissive }

// Enforce 是否为强制模式（仅 1）。
func (s Settings) Enforce() bool { return s.Mode == ModeOn }

// SecretKeyBytes 解码 19005；未配置或格式非法返回错误（调用方按「鉴权未就绪」处理）。
func (s Settings) SecretKeyBytes() ([]byte, error) {
	raw := strings.TrimSpace(s.SecretKeyBase64)
	if raw == "" {
		return nil, errors.New("sys_config 19005（OpenAPI 凭证加密密钥）未配置")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("sys_config 19005 不是合法的 Base64")
	}
	if len(key) != 32 {
		return nil, errors.New("sys_config 19005 必须是 32 字节（AES-256）密钥")
	}
	return key, nil
}

// SettingsLoader 读取配置；实现须并发安全并自行缓存。
type SettingsLoader interface {
	Load(ctx context.Context) Settings
}

// configTTL 配置缓存时长：改配置后最多 10 秒生效，不必重启。
const configTTL = 10 * time.Second

const selectSettingsSQL = `
SELECT config_no, COALESCE(char_value, ''), COALESCE(str_value, ''), COALESCE(int_value, 0)
  FROM sys.sys_config
 WHERE config_no BETWEEN 19001 AND 19007`

// SettingsStore 基于 pgx 的配置读取器（短 TTL 缓存）。
type SettingsStore struct {
	pool *pgxpool.Pool
	now  func() time.Time

	mu       sync.Mutex
	cached   Settings
	cachedAt time.Time
	hasCache bool
}

// NewSettingsStore 创建配置读取器。
func NewSettingsStore(pool *pgxpool.Pool) *SettingsStore {
	return &SettingsStore{pool: pool, now: time.Now}
}

// Load 读取配置；数据库不可用时返回最近一次成功快照（无快照则返回缺省值）。
func (s *SettingsStore) Load(ctx context.Context) Settings {
	s.mu.Lock()
	if s.hasCache && s.now().Sub(s.cachedAt) < configTTL {
		cached := s.cached
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()

	values, err := s.query(ctx)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.hasCache {
			return s.cached
		}
		return DefaultSettings
	}

	s.mu.Lock()
	s.cached, s.cachedAt, s.hasCache = values, s.now(), true
	s.mu.Unlock()
	return values
}

// Invalidate 失效缓存（配置保存后调用）。
func (s *SettingsStore) Invalidate() {
	s.mu.Lock()
	s.hasCache = false
	s.mu.Unlock()
}

func (s *SettingsStore) query(ctx context.Context) (Settings, error) {
	if s.pool == nil {
		return DefaultSettings, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, selectSettingsSQL)
	if err != nil {
		return DefaultSettings, err
	}
	defer rows.Close()

	settings := DefaultSettings
	for rows.Next() {
		var (
			configNo int
			charVal  string
			strVal   string
			intVal   int
		)
		if err := rows.Scan(&configNo, &charVal, &strVal, &intVal); err != nil {
			return DefaultSettings, err
		}
		switch configNo {
		case 19001:
			switch strings.TrimSpace(charVal) {
			case ModeOn, ModePermissive, ModeOff:
				settings.Mode = strings.TrimSpace(charVal)
			}
		case 19002:
			if secs := intVal; secs >= 30 && secs <= 3600 {
				settings.SignTimeWindow = time.Duration(secs) * time.Second
			}
		case 19003:
			if intVal >= 0 {
				settings.LogRetainDays = intVal
			}
		case 19004:
			if v := strings.TrimSpace(strVal); v != "" {
				settings.SecretKID = v
			}
		case 19005:
			settings.SecretKeyBase64 = strings.TrimSpace(strVal)
		case 19006:
			settings.FallbackKey = strings.TrimSpace(strVal)
		case 19007:
			// 0 合法（关闭缓存）；负数按 0、超过上限按上限收敛，避免误配把直连改库的生效时间拖到几小时
			settings.CredCacheTTL = clampCredCacheTTL(intVal)
		}
	}
	if err := rows.Err(); err != nil {
		return DefaultSettings, err
	}
	settings.NonceKeepWindow = 2 * settings.SignTimeWindow
	return settings, nil
}

// clampCredCacheTTL 把 19007 的秒数收敛到 [0, MaxCredCacheTTL]。
func clampCredCacheTTL(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	value := time.Duration(seconds) * time.Second
	if value > MaxCredCacheTTL {
		return MaxCredCacheTTL
	}
	return value
}

// staticSettings 固定配置（单测与不吃库的场景使用）。
type staticSettings struct{ settings Settings }

func (s staticSettings) Load(context.Context) Settings { return s.settings }

// FixedSettings 返回一个恒定配置的加载器（供测试与显式注入）。
func FixedSettings(settings Settings) SettingsLoader { return staticSettings{settings: settings} }

// parseDurationSeconds 解析「秒」字符串（保留给配置扩展使用）。
func parseDurationSeconds(raw string) (time.Duration, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0, false
	}
	return time.Duration(value) * time.Second, true
}

// encodeBase64 把字节编码为 Base64（配置与测试共用）。
func encodeBase64(raw []byte) string { return base64.StdEncoding.EncodeToString(raw) }
