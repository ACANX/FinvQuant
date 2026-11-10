package uscrule

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/base/cron"
)

// previewCacheTTL 预演明细缓存有效期（超过后 Diff/Page 提示重新预演）。
const previewCacheTTL = 30 * time.Minute

// Service USC 映射规则业务层：页面查询、预演、应用与调度配置读取。
type Service struct {
	store *Store

	// 最近一次预演的完整明细（内存缓存，供 Diff/Page 分页浏览）。
	// 预演本身不落库，明细也没法从库里再查，所以缓存一次；重新预演会整体替换。
	mu       sync.Mutex
	cached   []DiffRow
	cacheAt  time.Time
	cacheEnv string
	cacheTok string
}

// NewService 创建服务。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{store: NewStore(pool)}
}

// Store 暴露底层读写器（运行器与测试用）。
func (s *Service) Store() *Store { return s.store }

// Status 页面快照。
func (s *Service) Status(ctx context.Context, env string) (Status, error) {
	return s.store.Status(ctx, env)
}

// Preview 预演（不落库），并把完整明细缓存在内存里供分页浏览。
func (s *Service) Preview(ctx context.Context, env string) (Preview, error) {
	preview, rows, err := s.store.Preview(ctx, env)
	if err != nil {
		return Preview{}, err
	}
	s.mu.Lock()
	s.cached = rows
	s.cacheAt = time.Now()
	s.cacheEnv = preview.Env
	s.cacheTok = preview.PreviewToken
	s.mu.Unlock()
	return preview, nil
}

// DiffPage 分页读取最近一次预演的明细。
//
// diff 为空表示全部（不含 keep）；token 非空时必须与缓存令牌一致，避免「预演 → 改了规则 → 还在看旧明细」。
func (s *Service) DiffPage(token, diff string, page, pageSize int) ([]DiffRow, int, string, error) {
	s.mu.Lock()
	rows := s.cached
	at := s.cacheAt
	tok := s.cacheTok
	s.mu.Unlock()

	if rows == nil || at.IsZero() {
		return nil, 0, "", fmt.Errorf("还没有预演结果，请先点「预演差异」")
	}
	if time.Since(at) > previewCacheTTL {
		return nil, 0, "", fmt.Errorf("预演明细已超过 %d 分钟，请重新预演", int(previewCacheTTL.Minutes()))
	}
	if token != "" && token != tok {
		return nil, 0, "", fmt.Errorf("预演令牌不匹配（可能是重新预演或服务重启），请刷新后重试")
	}

	filtered := rows
	if diff != "" {
		filtered = make([]DiffRow, 0, len(rows))
		for _, row := range rows {
			if row.Diff == diff {
				filtered = append(filtered, row)
			}
		}
	}
	page, pageSize = normalizePager(page, pageSize)
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return []DiffRow{}, total, tok, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total, tok, nil
}

// Apply 应用一次映射（落库 + 归档 + 回写键表 + 记账）。
func (s *Service) Apply(ctx context.Context, env, previewToken, operator, reason string) (Result, error) {
	if strings.TrimSpace(reason) == "" {
		reason = "应用 USC 映射规则（定时任务或维护页手动触发）"
	}
	return s.store.Apply(ctx, env, previewToken, operator, reason)
}

// ListRules 规则列表。
func (s *Service) ListRules(ctx context.Context) ([]Rule, error) {
	return s.store.ListRules(ctx)
}

// ListItems 条目分页。
func (s *Service) ListItems(ctx context.Context, filter ItemFilter) ([]Item, int64, error) {
	return s.store.ListItems(ctx, filter)
}

// SaveItem 保存条目。
func (s *Service) SaveItem(ctx context.Context, item Item, operator string) (int64, error) {
	return s.store.SaveItem(ctx, item, operator)
}

// RetireItem 停用条目。
func (s *Service) RetireItem(ctx context.Context, idn int64, reason, operator string) error {
	return s.store.RetireItem(ctx, idn, reason, operator)
}

// ListLogs 审计日志分页。
func (s *Service) ListLogs(ctx context.Context, page, pageSize int) ([]LogRow, int64, error) {
	return s.store.ListLogs(ctx, page, pageSize)
}

// NextRun 按 6 段 cron 计算下一次执行时间（表达式为空或非法时返回空串）。
func NextRun(expr string, from time.Time) string {
	schedule := ParseSchedule(expr)
	if schedule == nil {
		return ""
	}
	return schedule.Next(from).Format(time.RFC3339)
}

// ParseSchedule 解析 6 段 cron（空串/非法返回 nil）。
func ParseSchedule(expr string) *cron.Schedule {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	parsed, err := cron.Parse(expr)
	if err != nil {
		return nil
	}
	return parsed
}

// CronExpr 读取系统配置 20015 的 cron 表达式。
func (s *Service) CronExpr(ctx context.Context) (string, error) {
	var expr *string
	if err := s.store.pool.QueryRow(ctx, `SELECT str_value FROM sys.sys_config WHERE config_no = $1`, ConfigCronNo).Scan(&expr); err != nil {
		return "", fmt.Errorf("读取同步 cron 配置（%d）失败: %w", ConfigCronNo, err)
	}
	return strings.TrimSpace(derefString(expr)), nil
}
