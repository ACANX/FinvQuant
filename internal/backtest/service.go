package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxConcurrentRuns 并发回测任务上限（初始版本 4）。
const maxConcurrentRuns = 4

// newUUID 生成全大写的 UUID 字符串（run_id/template_id/env_id/strategy_id/account_id 等主键标识）。
// 约定：禁止生成含小写英文字母的 UUID（统一大写，避免大小写混用导致关联/查询歧义）。
func newUUID() string {
	return strings.ToUpper(uuid.NewString())
}

// Service 回测服务：策略/账户/任务 CRUD + 异步回测执行调度。
type Service struct {
	pool  *pgxpool.Pool
	mu    sync.Mutex
	tasks map[string]context.CancelFunc // run_id → 取消函数
	sem   chan struct{}                 // 并发执行信号量
}

// NewService 创建回测服务，并在启动时执行悬挂任务恢复（进程重启后
// RUNNING/PENDING 任务无法继续执行，统一标记 FAILED 并登记原因，避免永久卡住）。
func NewService(pool *pgxpool.Pool) *Service {
	s := &Service{
		pool:  pool,
		tasks: map[string]context.CancelFunc{},
		sem:   make(chan struct{}, maxConcurrentRuns),
	}
	s.recoverHangingRuns()
	return s
}

// recoverHangingRuns 启动时把 RUNNING/PENDING 任务标记为 FAILED（进程重启恢复机制）。
func (s *Service) recoverHangingRuns() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
UPDATE finv_quant_backtest_run
SET status = 'FAILED', error_message = '进程重启，任务中断（启动时自动标记失败，请重新发起回测）', finished_at = now()
WHERE status IN ('PENDING','RUNNING')`)
	if err != nil {
		// 启动期恢复失败不阻塞服务（记录日志由上层处理）
		fmt.Printf("[backtest] 悬挂任务恢复失败: %v\n", err)
	}
}

// ---------------------------------------------------------------------
// 策略 CRUD
// ---------------------------------------------------------------------

// SaveStrategy 新增或更新策略（strategy_id 为空时自动生成）。
func (s *Service) SaveStrategy(ctx context.Context, st *Strategy) (string, error) {
	if err := validateStrategy(st); err != nil {
		return "", err
	}
	if st.StrategyID == "" {
		st.StrategyID = newUUID()
	}
	if st.StrategyType == "" {
		st.StrategyType = StrategyTypeRuleBased
	}
	if st.DataPeriod == "" {
		st.DataPeriod = st.Definition.Data.Period
	}
	if st.DataPeriod == "" {
		st.DataPeriod = PeriodMin
	}
	if st.SecuCode == "" && len(st.Definition.Universe.Securities) > 0 {
		st.SecuCode = st.Definition.Universe.Securities[0]
	}
	if st.AllowBacktest == "" {
		st.AllowBacktest = FlagOn
	}
	if st.Status == "" {
		st.Status = StatusEnabled
	}
	if st.DefinitionVersion == 0 {
		st.DefinitionVersion = 1
	}
	if st.Definition.Version == "" {
		st.Definition.Version = "1"
	}
	defJSON, err := json.Marshal(st.Definition)
	if err != nil {
		return "", fmt.Errorf("策略定义序列化失败: %w", err)
	}
	if st.UserID == "" {
		st.UserID = "default"
	}

	_, err = s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_strategy
    (strategy_id, strategy_code, strategy_name, strategy_type, description,
     definition, definition_version, data_period, secu_code, user_id, template_id, allow_backtest, status, created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (strategy_id) DO UPDATE SET
    strategy_code = EXCLUDED.strategy_code,
    strategy_name = EXCLUDED.strategy_name,
    strategy_type = EXCLUDED.strategy_type,
    description = EXCLUDED.description,
    definition = EXCLUDED.definition,
    definition_version = EXCLUDED.definition_version,
    data_period = EXCLUDED.data_period,
    secu_code = EXCLUDED.secu_code,
    user_id = EXCLUDED.user_id,
    template_id = EXCLUDED.template_id,
    allow_backtest = EXCLUDED.allow_backtest,
    status = EXCLUDED.status,
    created_by = EXCLUDED.created_by`,
		st.StrategyID, st.StrategyCode, st.StrategyName, st.StrategyType, st.Description,
		defJSON, st.DefinitionVersion, st.DataPeriod, st.SecuCode, st.UserID, st.TemplateID,
		st.AllowBacktest, st.Status, st.CreatedBy)
	if err != nil {
		return "", err
	}
	return st.StrategyID, nil
}

// ListStrategies 分页查询策略（按 user_id 隔离）。
func (s *Service) ListStrategies(ctx context.Context, pager Pager, keyword, allowBacktest, userID string) ([]Strategy, int, error) {
	pager.Normalize()
	if userID == "" {
		userID = "default"
	}
	where := []string{"user_id = $1"}
	args := []any{userID}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			strategy_code ILIKE $%d OR strategy_name ILIKE $%d OR description ILIKE $%d OR secu_code ILIKE $%d
		)`, len(args), len(args), len(args), len(args)))
	}
	if allowBacktest == FlagOn || allowBacktest == FlagOff {
		args = append(args, allowBacktest)
		where = append(where, fmt.Sprintf("allow_backtest = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_backtest_strategy WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT strategy_id, strategy_code, strategy_name, strategy_type, description,
       definition, definition_version, data_period, secu_code, user_id, template_id, allow_backtest, status, created_by, gmt_update
FROM finv_quant_backtest_strategy
WHERE `+cond+`
ORDER BY gmt_update DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Strategy{}
	for rows.Next() {
		st, err := scanStrategy(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, st)
	}
	return list, total, rows.Err()
}

// GetStrategy 查询单个策略（多用户归属校验）。
func (s *Service) GetStrategy(ctx context.Context, strategyID, userID string) (*Strategy, error) {
	if userID == "" {
		userID = "default"
	}
	row := s.pool.QueryRow(ctx, `
SELECT strategy_id, strategy_code, strategy_name, strategy_type, description,
       definition, definition_version, data_period, secu_code, user_id, template_id, allow_backtest, status, created_by, gmt_update
FROM finv_quant_backtest_strategy WHERE strategy_id = $1 AND user_id = $2`, strategyID, userID)
	st, err := scanStrategy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("策略不存在: %s", strategyID)
		}
		return nil, err
	}
	return &st, nil
}

// ToggleStrategy 切换策略回测开关（多用户归属校验）。
func (s *Service) ToggleStrategy(ctx context.Context, strategyID, allowBacktest, userID string) error {
	if allowBacktest != FlagOn && allowBacktest != FlagOff {
		return fmt.Errorf("allow_backtest 仅支持 0/1")
	}
	if userID == "" {
		userID = "default"
	}
	tag, err := s.pool.Exec(ctx, `UPDATE finv_quant_backtest_strategy SET allow_backtest=$2 WHERE strategy_id=$1 AND user_id=$3`, strategyID, allowBacktest, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("策略不存在: %s", strategyID)
	}
	return nil
}

// DeleteStrategy 删除策略（存在关联回测任务时拒绝；多用户归属校验）。
func (s *Service) DeleteStrategy(ctx context.Context, strategyID, userID string) error {
	if userID == "" {
		userID = "default"
	}
	var runCount int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_run WHERE strategy_id = $1`, strategyID).Scan(&runCount); err != nil {
		return err
	}
	if runCount > 0 {
		return fmt.Errorf("策略已关联 %d 个回测任务，禁止删除（可改为禁用）", runCount)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM finv_quant_backtest_strategy WHERE strategy_id = $1 AND user_id = $2`, strategyID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("策略不存在: %s", strategyID)
	}
	return nil
}

// ---------------------------------------------------------------------
// 账户 CRUD
// ---------------------------------------------------------------------

// SaveAccount 新增或更新回测账户。
func (s *Service) SaveAccount(ctx context.Context, acc *Account) (string, error) {
	if err := validateAccount(acc); err != nil {
		return "", err
	}
	if acc.AccountID == "" {
		acc.AccountID = newUUID()
	}
	if acc.CurrencyType == "" {
		acc.CurrencyType = "USD"
	}
	if acc.MarginMode == "" {
		acc.MarginMode = "FULL"
	}
	if acc.MarginRate <= 0 || acc.MarginRate > 1 {
		acc.MarginRate = 1
	}
	if acc.UserID == "" {
		acc.UserID = "default"
	}
	if acc.AllowBacktest == "" {
		acc.AllowBacktest = FlagOn
	}
	if acc.Status == "" {
		acc.Status = StatusEnabled
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_account
    (account_id, account_code, account_name, user_id, group_id, env_id, initial_capital, currency_type,
     commission_rate, slippage_pct, margin_mode, margin_rate, allow_backtest, status, remark, created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT (account_id) DO UPDATE SET
    account_code = EXCLUDED.account_code,
    account_name = EXCLUDED.account_name,
    user_id = EXCLUDED.user_id,
    group_id = EXCLUDED.group_id,
    env_id = EXCLUDED.env_id,
    initial_capital = EXCLUDED.initial_capital,
    currency_type = EXCLUDED.currency_type,
    commission_rate = EXCLUDED.commission_rate,
    slippage_pct = EXCLUDED.slippage_pct,
    margin_mode = EXCLUDED.margin_mode,
    margin_rate = EXCLUDED.margin_rate,
    allow_backtest = EXCLUDED.allow_backtest,
    status = EXCLUDED.status,
    remark = EXCLUDED.remark,
    created_by = EXCLUDED.created_by`,
		acc.AccountID, acc.AccountCode, acc.AccountName, acc.UserID, acc.GroupID, acc.EnvID,
		acc.InitialCapital, acc.CurrencyType, acc.CommissionRate, acc.SlippagePct, acc.MarginMode,
		acc.MarginRate, acc.AllowBacktest, acc.Status, acc.Remark, acc.CreatedBy)
	if err != nil {
		return "", err
	}
	return acc.AccountID, nil
}

// ListAccounts 分页查询账户（按 user_id 隔离）。
func (s *Service) ListAccounts(ctx context.Context, pager Pager, keyword, allowBacktest, userID string) ([]Account, int, error) {
	pager.Normalize()
	if userID == "" {
		userID = "default"
	}
	where := []string{"user_id = $1"}
	args := []any{userID}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			account_code ILIKE $%d OR account_name ILIKE $%d OR remark ILIKE $%d
		)`, len(args), len(args), len(args)))
	}
	if allowBacktest == FlagOn || allowBacktest == FlagOff {
		args = append(args, allowBacktest)
		where = append(where, fmt.Sprintf("allow_backtest = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_backtest_account WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT account_id, account_code, account_name, user_id, group_id, env_id, initial_capital, currency_type,
       commission_rate, slippage_pct, margin_mode, margin_rate, allow_backtest, status, remark, created_by, gmt_update
FROM finv_quant_backtest_account
WHERE `+cond+`
ORDER BY gmt_update DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Account{}
	for rows.Next() {
		var a Account
		// remark / created_by 列可空，用指针接收避免 NULL 扫描失败
		var remark, createdBy *string
		if err := rows.Scan(&a.AccountID, &a.AccountCode, &a.AccountName, &a.UserID, &a.GroupID, &a.EnvID,
			&a.InitialCapital, &a.CurrencyType, &a.CommissionRate, &a.SlippagePct, &a.MarginMode, &a.MarginRate,
			&a.AllowBacktest, &a.Status, &remark, &createdBy, &a.GMTUpdate); err != nil {
			return nil, 0, err
		}
		if remark != nil {
			a.Remark = *remark
		}
		if createdBy != nil {
			a.CreatedBy = *createdBy
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

// GetAccount 查询单个账户（多用户归属校验）。
func (s *Service) GetAccount(ctx context.Context, accountID, userID string) (*Account, error) {
	if userID == "" {
		userID = "default"
	}
	row := s.pool.QueryRow(ctx, `
SELECT account_id, account_code, account_name, user_id, group_id, env_id, initial_capital, currency_type,
       commission_rate, slippage_pct, margin_mode, margin_rate, allow_backtest, status, remark, created_by, gmt_update
FROM finv_quant_backtest_account WHERE account_id = $1 AND user_id = $2`, accountID, userID)
	var a Account
	// remark / created_by 列可空，用指针接收避免 NULL 扫描失败
	var remark, createdBy *string
	if err := row.Scan(&a.AccountID, &a.AccountCode, &a.AccountName, &a.UserID, &a.GroupID, &a.EnvID,
		&a.InitialCapital, &a.CurrencyType, &a.CommissionRate, &a.SlippagePct, &a.MarginMode, &a.MarginRate,
		&a.AllowBacktest, &a.Status, &remark, &createdBy, &a.GMTUpdate); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("账户不存在: %s", accountID)
		}
		return nil, err
	}
	if remark != nil {
		a.Remark = *remark
	}
	if createdBy != nil {
		a.CreatedBy = *createdBy
	}
	return &a, nil
}

// ToggleAccount 切换账户回测开关（多用户归属校验）。
func (s *Service) ToggleAccount(ctx context.Context, accountID, allowBacktest, userID string) error {
	if allowBacktest != FlagOn && allowBacktest != FlagOff {
		return fmt.Errorf("allow_backtest 仅支持 0/1")
	}
	if userID == "" {
		userID = "default"
	}
	tag, err := s.pool.Exec(ctx, `UPDATE finv_quant_backtest_account SET allow_backtest=$2 WHERE account_id=$1 AND user_id=$3`, accountID, allowBacktest, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("账户不存在: %s", accountID)
	}
	return nil
}

// DeleteAccount 删除账户（存在关联回测任务时拒绝；多用户归属校验）。
func (s *Service) DeleteAccount(ctx context.Context, accountID, userID string) error {
	if userID == "" {
		userID = "default"
	}
	var runCount int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_run WHERE account_id = $1`, accountID).Scan(&runCount); err != nil {
		return err
	}
	if runCount > 0 {
		return fmt.Errorf("账户已关联 %d 个回测任务，禁止删除（可改为禁用）", runCount)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM finv_quant_backtest_account WHERE account_id = $1 AND user_id = $2`, accountID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("账户不存在: %s", accountID)
	}
	return nil
}

// ---------------------------------------------------------------------
// 回测任务
// ---------------------------------------------------------------------

// CreateRun 创建并启动回测任务（多用户归属校验：策略/账户必须属于当前用户）。
func (s *Service) CreateRun(ctx context.Context, req CreateRunRequest) (*Run, error) {
	if req.StrategyID == "" || req.AccountID == "" {
		return nil, fmt.Errorf("strategy_id 与 account_id 必填")
	}
	userID := req.UserID
	if userID == "" {
		userID = "default"
	}
	st, err := s.GetStrategy(ctx, req.StrategyID, userID)
	if err != nil {
		return nil, err
	}
	if st.AllowBacktest != FlagOn {
		return nil, fmt.Errorf("策略「%s」回测开关已关闭，请先在策略管理启用", st.StrategyName)
	}
	acc, err := s.GetAccount(ctx, req.AccountID, userID)
	if err != nil {
		return nil, err
	}
	if acc.AllowBacktest != FlagOn {
		return nil, fmt.Errorf("账户「%s」回测开关已关闭，请先在账户管理启用", acc.AccountName)
	}
	if !req.Options.EnableBacktest {
		return nil, fmt.Errorf("回测开关未启用，已拒绝创建回测任务")
	}

	// 标的：请求参数 > 策略 universe 首个证券
	secuCode := strings.TrimSpace(req.SecuCode)
	if secuCode == "" && len(st.Definition.Universe.Securities) > 0 {
		secuCode = st.Definition.Universe.Securities[0]
	}
	if secuCode == "" {
		return nil, fmt.Errorf("未指定回测标的（secu_code 或策略 universe.securities）")
	}

	// 周期与报告精度
	period := req.Period
	if period == "" {
		period = st.Definition.Data.Period
	}
	if period == "" {
		period = PeriodMin
	}
	if period != PeriodMin && period != PeriodHour && period != PeriodDay {
		return nil, fmt.Errorf("不支持的周期 %q", period)
	}
	precision := req.ReportPrecision
	if precision == "" {
		precision = PeriodDay
	}
	if precision != PeriodMin && precision != PeriodHour && precision != PeriodDay {
		return nil, fmt.Errorf("不支持的报告精度 %q", precision)
	}

	// 时间区间（date 维度）
	startDate, endDate, err := s.resolveDateRange(ctx, secuCode, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	startTS := dateToTS(startDate)
	endTS := dateToTS(endDate) + 86400 - 1

	// 快照
	defJSON, _ := json.Marshal(st.Definition)
	accSnapshot := AccountSnapshot{
		AccountID:      acc.AccountID,
		AccountCode:    acc.AccountCode,
		AccountName:    acc.AccountName,
		UserID:         acc.UserID,
		GroupID:        acc.GroupID,
		InitialCapital: acc.InitialCapital,
		CurrencyType:   acc.CurrencyType,
		CommissionRate: acc.CommissionRate,
		SlippagePct:    acc.SlippagePct,
		MarginMode:     acc.MarginMode,
		MarginRate:     acc.MarginRate,
	}
	accJSON, _ := json.Marshal(accSnapshot)
	optsJSON, _ := json.Marshal(req.Options)

	// 环境解析：请求 env_id > 账户 env_id > 默认环境（BACKTEST）
	env, err := s.resolveEnvironment(ctx, req.EnvID, acc.EnvID, userID)
	if err != nil {
		return nil, err
	}
	envJSON := []byte("null")
	envID := ""
	if env != nil {
		// 币种一致性校验（FR-14 环境 currency 生效）：环境币种与账户币种不一致时拒绝启动
		if envCur := env.Config.Currency; envCur != "" && envCur != acc.CurrencyType {
			return nil, fmt.Errorf("环境「%s」计价币种 %s 与账户「%s」币种 %s 不一致，请选择匹配的环境或调整账户币种",
				env.EnvName, envCur, acc.AccountName, acc.CurrencyType)
		}
		envJSON, _ = json.Marshal(env)
		envID = env.EnvID
	}

	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = "console"
	}

	marketCode := 0
	_ = s.pool.QueryRow(ctx,
		`SELECT COALESCE(market_code, 0) FROM finv_security WHERE usc = $1 OR security_code = $1 LIMIT 1`, secuCode).Scan(&marketCode)

	runID := newUUID()
	_, err = s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_run
    (run_id, user_id, strategy_id, strategy_code, strategy_name, strategy_snapshot,
     account_id, account_code, account_name, account_snapshot,
     env_id, env_snapshot,
     secu_code, market_code, period, report_precision,
     start_ts, end_ts, start_date, end_date, options, status, progress, created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,'PENDING',0,$22)`,
		runID, userID, st.StrategyID, st.StrategyCode, st.StrategyName, defJSON,
		acc.AccountID, acc.AccountCode, acc.AccountName, accJSON,
		envID, envJSON,
		secuCode, marketCode, period, precision,
		startTS, endTS, startDate, endDate, optsJSON, createdBy)
	if err != nil {
		return nil, err
	}

	// 异步执行
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.tasks[runID] = cancel
	s.mu.Unlock()
	go s.executeRun(runCtx, runID)

	run, err := s.GetRun(ctx, runID, userID)
	if err != nil {
		return nil, err
	}
	return run, nil
}

// CancelRun 取消运行中的回测任务（多用户归属校验）。
func (s *Service) CancelRun(ctx context.Context, runID, userID string) error {
	if userID == "" {
		userID = "default"
	}
	var owner string
	if err := s.pool.QueryRow(ctx, `SELECT user_id FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(&owner); err != nil {
		return fmt.Errorf("任务不存在: %s", runID)
	}
	if owner != userID {
		return fmt.Errorf("无权操作其他用户的任务: %s", runID)
	}
	s.mu.Lock()
	cancel, ok := s.tasks[runID]
	s.mu.Unlock()
	if !ok {
		// 非运行中任务：检查状态，仅 PENDING/RUNNING 可取消
		var status string
		if err := s.pool.QueryRow(ctx, `SELECT status FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(&status); err != nil {
			return fmt.Errorf("任务不存在: %s", runID)
		}
		if status == RunPending || status == RunRunning {
			return fmt.Errorf("任务不在本进程调度器中（可能已重启），无法取消")
		}
		return fmt.Errorf("任务已结束（%s），无需取消", status)
	}
	cancel()
	return nil
}

// ListRuns 分页查询回测任务（按 user_id 隔离）。
func (s *Service) ListRuns(ctx context.Context, q RunListQuery, userID string) ([]Run, int, error) {
	q.Pager.Normalize()
	if userID == "" {
		userID = "default"
	}
	where := []string{"user_id = $1"}
	args := []any{userID}
	if q.Status != "" {
		args = append(args, q.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if q.SecuCode != "" {
		args = append(args, q.SecuCode)
		where = append(where, fmt.Sprintf("secu_code = $%d", len(args)))
	}
	if q.StrategyID != "" {
		args = append(args, q.StrategyID)
		where = append(where, fmt.Sprintf("strategy_id = $%d", len(args)))
	}
	if q.Keyword = strings.TrimSpace(q.Keyword); q.Keyword != "" {
		args = append(args, "%"+q.Keyword+"%")
		where = append(where, fmt.Sprintf(`(
			strategy_name ILIKE $%d OR account_name ILIKE $%d OR secu_code ILIKE $%d OR CAST(run_no AS TEXT) ILIKE $%d
		)`, len(args), len(args), len(args), len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_backtest_run WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT run_id, run_no, user_id, strategy_id, strategy_code, strategy_name, strategy_snapshot,
       account_id, account_code, account_name, account_snapshot,
       env_id, env_snapshot,
       secu_code, market_code, period, report_precision,
       start_ts, end_ts, start_date, end_date, options, status, progress, error_message, report,
       started_at, finished_at, created_by, gmt_update
FROM finv_quant_backtest_run
WHERE `+cond+`
ORDER BY run_no DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, rows.Err()
}

// GetRun 查询单个回测任务（含报告；多用户归属校验）。
func (s *Service) GetRun(ctx context.Context, runID, userID string) (*Run, error) {
	if userID == "" {
		userID = "default"
	}
	row := s.pool.QueryRow(ctx, `
SELECT run_id, run_no, user_id, strategy_id, strategy_code, strategy_name, strategy_snapshot,
       account_id, account_code, account_name, account_snapshot,
       env_id, env_snapshot,
       secu_code, market_code, period, report_precision,
       start_ts, end_ts, start_date, end_date, options, status, progress, error_message, report,
       started_at, finished_at, created_by, gmt_update
FROM finv_quant_backtest_run WHERE run_id = $1 AND user_id = $2`, runID, userID)
	r, err := scanRun(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("回测任务不存在: %s", runID)
		}
		return nil, err
	}
	return &r, nil
}

// GetReport 查询回测报告（汇总指标；多用户归属校验）。
func (s *Service) GetReport(ctx context.Context, runID, userID string) (*RunReport, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, err
	}
	var reportJSON []byte
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT status, COALESCE(report, '{}') FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(&status, &reportJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("回测任务不存在: %s", runID)
		}
		return nil, err
	}
	if status != RunSucceeded {
		return nil, fmt.Errorf("任务尚未成功完成（当前状态 %s），暂无报告", status)
	}
	var report RunReport
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return nil, fmt.Errorf("报告数据解析失败: %w", err)
	}
	return &report, nil
}

// checkRunOwner 校验任务归属（多用户隔离：他人任务不可读明细/报告）。
func (s *Service) checkRunOwner(ctx context.Context, runID, userID string) error {
	var owner string
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(&owner)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("回测任务不存在: %s", runID)
		}
		return err
	}
	if owner != userID {
		return fmt.Errorf("无权访问其他用户的任务: %s", runID)
	}
	return nil
}

// loadRunForExecution 任务执行时加载（内部方法，绕过用户隔离——执行器仅限本进程任务）。
func (s *Service) loadRunForExecution(ctx context.Context, runID string) (*Run, error) {
	row := s.pool.QueryRow(ctx, `
SELECT run_id, run_no, user_id, strategy_id, strategy_code, strategy_name, strategy_snapshot,
       account_id, account_code, account_name, account_snapshot,
       env_id, env_snapshot,
       secu_code, market_code, period, report_precision,
       start_ts, end_ts, start_date, end_date, options, status, progress, error_message, report,
       started_at, finished_at, created_by, gmt_update
FROM finv_quant_backtest_run WHERE run_id = $1`, runID)
	r, err := scanRun(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("回测任务不存在: %s", runID)
		}
		return nil, err
	}
	return &r, nil
}

// ListEquity 分页查询净值曲线（按报告精度；多用户归属校验）。
func (s *Service) ListEquity(ctx context.Context, runID, userID string, pager Pager) ([]EquityPoint, int, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, 0, err
	}
	pager = normalizeLargePager(pager)
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_equity WHERE run_id=$1`, runID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT seq, ts, COALESCE(date, 0), COALESCE("time", 0), equity, cash, position_value, position_qty, profit, roi, drawdown
FROM finv_quant_backtest_equity WHERE run_id=$1
ORDER BY seq ASC
LIMIT $2 OFFSET $3`, runID, pager.PageSize, (pager.Page-1)*pager.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []EquityPoint{}
	for rows.Next() {
		var p EquityPoint
		if err := rows.Scan(&p.Seq, &p.TS, &p.Date, &p.Time, &p.Equity, &p.Cash,
			&p.PositionValue, &p.PositionQty, &p.Profit, &p.ROI, &p.Drawdown); err != nil {
			return nil, 0, err
		}
		list = append(list, p)
	}
	return list, total, rows.Err()
}

// ListTrades 分页查询成交记录（多用户归属校验）。
func (s *Service) ListTrades(ctx context.Context, runID, userID string, pager Pager) ([]Trade, int, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, 0, err
	}
	pager = normalizeLargePager(pager)
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_trade WHERE run_id=$1`, runID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT trade_id, run_id, seq, ts, COALESCE(date, 0), COALESCE("time", 0), action, price, qty, amount, fee, COALESCE(profit, 0), position_after, cash_after, COALESCE(signal, ''), COALESCE(remark, '')
FROM finv_quant_backtest_trade WHERE run_id=$1
ORDER BY ts ASC
LIMIT $2 OFFSET $3`, runID, pager.PageSize, (pager.Page-1)*pager.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Trade{}
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.TradeID, &t.RunID, &t.Seq, &t.TS, &t.Date, &t.Time, &t.Action, &t.Price,
			&t.Qty, &t.Amount, &t.Fee, &t.Profit, &t.PositionAfter, &t.CashAfter, &t.Signal, &t.Remark); err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, rows.Err()
}

// ListCashflows 分页查询资金流水明细（需求⑨-1；多用户归属校验）。
func (s *Service) ListCashflows(ctx context.Context, runID, userID string, pager Pager) ([]Cashflow, int, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, 0, err
	}
	pager = normalizeLargePager(pager)
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_cashflow WHERE run_id=$1`, runID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT cashflow_id, run_id, seq, ts, COALESCE(date, 0), COALESCE("time", 0), flow_type, amount, cash_before, cash_after, COALESCE(trade_id, 0), COALESCE(remark, '')
FROM finv_quant_backtest_cashflow WHERE run_id=$1
ORDER BY seq ASC
LIMIT $2 OFFSET $3`, runID, pager.PageSize, (pager.Page-1)*pager.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Cashflow{}
	for rows.Next() {
		var cf Cashflow
		if err := rows.Scan(&cf.CashflowID, &cf.RunID, &cf.Seq, &cf.TS, &cf.Date, &cf.Time,
			&cf.FlowType, &cf.Amount, &cf.CashBefore, &cf.CashAfter, &cf.TradeID, &cf.Remark); err != nil {
			return nil, 0, err
		}
		list = append(list, cf)
	}
	return list, total, rows.Err()
}

// ListPositionLogs 分页查询持仓变化明细（需求⑨-2；多用户归属校验）。
func (s *Service) ListPositionLogs(ctx context.Context, runID, userID string, pager Pager) ([]PositionLog, int, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, 0, err
	}
	pager = normalizeLargePager(pager)
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_position_log WHERE run_id=$1`, runID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT log_id, run_id, seq, ts, COALESCE(date, 0), COALESCE("time", 0), action, price, qty,
       position_before, position_after, avg_cost_before, avg_cost_after, COALESCE(trade_id, 0), COALESCE(remark, '')
FROM finv_quant_backtest_position_log WHERE run_id=$1
ORDER BY seq ASC
LIMIT $2 OFFSET $3`, runID, pager.PageSize, (pager.Page-1)*pager.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []PositionLog{}
	for rows.Next() {
		var pl PositionLog
		if err := rows.Scan(&pl.LogID, &pl.RunID, &pl.Seq, &pl.TS, &pl.Date, &pl.Time, &pl.Action,
			&pl.Price, &pl.Qty, &pl.PositionBefore, &pl.PositionAfter, &pl.AvgCostBefore,
			&pl.AvgCostAfter, &pl.TradeID, &pl.Remark); err != nil {
			return nil, 0, err
		}
		list = append(list, pl)
	}
	return list, total, rows.Err()
}

// ListEventTraces 分页查询交易事件追踪（需求⑨-3；多用户归属校验）。
func (s *Service) ListEventTraces(ctx context.Context, runID, userID string, pager Pager) ([]EventTrace, int, error) {
	if userID == "" {
		userID = "default"
	}
	if err := s.checkRunOwner(ctx, runID, userID); err != nil {
		return nil, 0, err
	}
	pager = normalizeLargePager(pager)
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_event_trace WHERE run_id=$1`, runID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT event_id, run_id, seq, action, trigger_reason, trigger_ts, trigger_date, trigger_time,
       order_ts, order_date, order_time,
       exec_status, exec_ts, exec_date, exec_time, latency_bars, latency_sec, alive_sec,
       COALESCE(reject_reason, ''), price, qty, COALESCE(trade_id, 0)
FROM finv_quant_backtest_event_trace WHERE run_id=$1
ORDER BY seq ASC
LIMIT $2 OFFSET $3`, runID, pager.PageSize, (pager.Page-1)*pager.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []EventTrace{}
	for rows.Next() {
		var ev EventTrace
		if err := rows.Scan(&ev.EventID, &ev.RunID, &ev.Seq, &ev.Action, &ev.TriggerReason,
			&ev.TriggerTS, &ev.TriggerDate, &ev.TriggerTime,
			&ev.OrderTS, &ev.OrderDate, &ev.OrderTime,
			&ev.ExecStatus, &ev.ExecTS, &ev.ExecDate, &ev.ExecTime, &ev.LatencyBars, &ev.LatencySec, &ev.AliveSec,
			&ev.RejectReason, &ev.Price, &ev.Qty, &ev.TradeID); err != nil {
			return nil, 0, err
		}
		list = append(list, ev)
	}
	return list, total, rows.Err()
}

// ---------------------------------------------------------------------
// 任务执行
// ---------------------------------------------------------------------

// executeRun 执行回测任务（goroutine 内运行）。
func (s *Service) executeRun(ctx context.Context, runID string) {
	defer func() {
		s.mu.Lock()
		delete(s.tasks, runID)
		s.mu.Unlock()
	}()

	// 并发信号量（排队等待）
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		s.markRun(ctx, runID, RunCancelled, 0, "任务已取消", nil)
		return
	}

	run, err := s.loadRunForExecution(ctx, runID)
	if err != nil {
		s.markRun(context.Background(), runID, RunFailed, 0, "加载任务失败: "+err.Error(), nil)
		return
	}

	now := time.Now().UTC()
	_, _ = s.pool.Exec(ctx, `UPDATE finv_quant_backtest_run SET status='RUNNING', started_at=$2 WHERE run_id=$1`, runID, now)

	// 加载行情（按 date 范围）
	bars, err := s.loadBars(ctx, run.SecuCode, run.StartDate, run.EndDate)
	if err != nil {
		s.markRun(context.Background(), runID, RunFailed, 0, "加载行情失败: "+err.Error(), nil)
		return
	}
	bars, err = AggregateBars(bars, run.Period)
	if err != nil {
		s.markRun(context.Background(), runID, RunFailed, 0, "行情聚合失败: "+err.Error(), nil)
		return
	}

	// 组装引擎配置
	cfg := EngineConfig{
		Definition:      run.Definition(),
		Account:         run.AccountSnapshot,
		Environment:     run.EnvironmentSnapshot,
		SecuCode:        run.SecuCode,
		Period:          run.Period,
		StartTS:         run.StartTS,
		EndTS:           run.EndTS,
		ReportPrecision: run.ReportPrecision,
		Options:         run.RunOptions(),
	}
	lastProgress := -1
	engine := NewEngine(cfg, func(processed, total int) {
		pct := processed * 100 / total
		if pct != lastProgress {
			lastProgress = pct
			_, _ = s.pool.Exec(ctx, `UPDATE finv_quant_backtest_run SET progress=$2 WHERE run_id=$1`, runID, pct)
		}
	})

	result, err := engine.Run(ctx, bars)
	if err != nil {
		if ctx.Err() != nil {
			s.markRun(context.Background(), runID, RunCancelled, 0, "任务已取消", nil)
			return
		}
		s.markRun(context.Background(), runID, RunFailed, 0, "回测执行失败: "+err.Error(), nil)
		return
	}

	// 持久化曲线/成交/报告
	if err := s.persistResult(ctx, runID, result); err != nil {
		s.markRun(context.Background(), runID, RunFailed, 0, "结果持久化失败: "+err.Error(), nil)
		return
	}
	s.markRun(context.Background(), runID, RunSucceeded, 100, "", result.Report)
}

// loadBars 加载分钟行情（date 范围，时间升序）。
func (s *Service) loadBars(ctx context.Context, secuCode string, startDate, endDate int) ([]Bar, error) {
	rows, err := s.pool.Query(ctx, `
SELECT ts, COALESCE(date, 0), COALESCE("time", 0),
       COALESCE(open, close), COALESCE(high, close), COALESCE(low, close), COALESCE(close, open),
       COALESCE(volume, 0), COALESCE(turnover, 0)
FROM finv_quote_secu_kline_min
WHERE secu_code = $1 AND date BETWEEN $2 AND $3
ORDER BY ts ASC`, secuCode, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bars := []Bar{}
	for rows.Next() {
		var b Bar
		if err := rows.Scan(&b.TS, &b.Date, &b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume, &b.Turnover); err != nil {
			return nil, err
		}
		bars = append(bars, b)
	}
	return bars, rows.Err()
}

// persistResult 批量落库曲线/成交/资金流水/持仓明细/事件追踪/报告。
// 成交记录先落库（RETURNING trade_id, seq），明细表通过 seq 映射关联 trade_id。
func (s *Service) persistResult(ctx context.Context, runID string, result *EngineResult) error {
	// Step 1: 插入成交记录，建立 seq → trade_id 映射
	tradeBatch := &pgx.Batch{}
	for _, t := range result.Trades {
		tradeBatch.Queue(`
INSERT INTO finv_quant_backtest_trade
    (run_id, seq, ts, date, "time", action, price, qty, amount, fee, profit, position_after, cash_after, signal, remark)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING trade_id, seq`,
			runID, t.Seq, t.TS, t.Date, t.Time, t.Action, t.Price, t.Qty, t.Amount,
			t.Fee, t.Profit, t.PositionAfter, t.CashAfter, t.Signal, t.Remark)
	}
	tradeIDBySeq := map[int]int64{}
	if tradeBatch.Len() > 0 {
		tradeResults := s.pool.SendBatch(ctx, tradeBatch)
		for i := 0; i < tradeBatch.Len(); i++ {
			rows, err := tradeResults.Query()
			if err != nil {
				_ = tradeResults.Close()
				return err
			}
			for rows.Next() {
				var tradeID int64
				var seq int
				if err := rows.Scan(&tradeID, &seq); err != nil {
					rows.Close()
					_ = tradeResults.Close()
					return err
				}
				tradeIDBySeq[seq] = tradeID
			}
			rows.Close()
		}
		if err := tradeResults.Close(); err != nil {
			return err
		}
	}

	// Step 2: 曲线 + 明细 + 报告
	batch := &pgx.Batch{}
	for _, p := range result.EquityPoints {
		batch.Queue(`
INSERT INTO finv_quant_backtest_equity (run_id, seq, ts, date, "time", equity, cash, position_value, position_qty, profit, roi, drawdown)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (run_id, ts) DO UPDATE SET
  equity=EXCLUDED.equity, cash=EXCLUDED.cash, position_value=EXCLUDED.position_value,
  position_qty=EXCLUDED.position_qty, profit=EXCLUDED.profit, roi=EXCLUDED.roi, drawdown=EXCLUDED.drawdown`,
			runID, p.Seq, p.TS, p.Date, p.Time, p.Equity, p.Cash, p.PositionValue,
			p.PositionQty, p.Profit, p.ROI, p.Drawdown)
	}
	for _, cf := range result.Cashflows {
		batch.Queue(`
INSERT INTO finv_quant_backtest_cashflow
    (run_id, seq, ts, date, "time", flow_type, amount, cash_before, cash_after, trade_id, remark)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			runID, cf.Seq, cf.TS, cf.Date, cf.Time, cf.FlowType, cf.Amount,
			cf.CashBefore, cf.CashAfter, tradeIDBySeq[cf.TradeSeq], cf.Remark)
	}
	for _, pl := range result.PositionLogs {
		batch.Queue(`
INSERT INTO finv_quant_backtest_position_log
    (run_id, seq, ts, date, "time", action, price, qty, position_before, position_after, avg_cost_before, avg_cost_after, trade_id, remark)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			runID, pl.Seq, pl.TS, pl.Date, pl.Time, pl.Action, pl.Price, pl.Qty,
			pl.PositionBefore, pl.PositionAfter, pl.AvgCostBefore, pl.AvgCostAfter,
			tradeIDBySeq[pl.TradeSeq], pl.Remark)
	}
	for _, ev := range result.EventTraces {
		batch.Queue(`
INSERT INTO finv_quant_backtest_event_trace
    (run_id, seq, action, trigger_reason, trigger_ts, trigger_date, trigger_time,
     order_ts, order_date, order_time,
     exec_status, exec_ts, exec_date, exec_time, latency_bars, latency_sec, alive_sec,
     reject_reason, price, qty, trade_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			runID, ev.Seq, ev.Action, ev.TriggerReason, ev.TriggerTS, ev.TriggerDate, ev.TriggerTime,
			ev.OrderTS, ev.OrderDate, ev.OrderTime,
			ev.ExecStatus, ev.ExecTS, ev.ExecDate, ev.ExecTime, ev.LatencyBars, ev.LatencySec, ev.AliveSec,
			ev.RejectReason, ev.Price, ev.Qty, tradeIDBySeq[ev.TradeSeq])
	}
	reportJSON, _ := json.Marshal(result.Report)
	batch.Queue(`UPDATE finv_quant_backtest_run SET report=$2 WHERE run_id=$1`, runID, reportJSON)

	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// normalizeLargePager 大页分页归一化（曲线/成交查询用，单页上限 5000）。
func normalizeLargePager(p Pager) Pager {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 5000
	}
	if p.PageSize > 5000 {
		p.PageSize = 5000
	}
	return p
}

// resolveEnvironment 解析回测环境：请求 env_id > 账户 env_id > 默认环境（BACKTEST）。
// 返回 nil 表示不启用环境（引擎按策略/账户配置运行）。
func (s *Service) resolveEnvironment(ctx context.Context, reqEnvID string, accEnvID *string, userID string) (*Environment, error) {
	envID := reqEnvID
	if envID == "" && accEnvID != nil {
		envID = *accEnvID
	}
	if envID != "" {
		env, err := s.GetEnvironment(ctx, envID, userID)
		if err != nil {
			return nil, fmt.Errorf("加载环境失败: %w", err)
		}
		if env.AllowBacktest != FlagOn {
			return nil, fmt.Errorf("环境「%s」回测开关已关闭", env.EnvName)
		}
		return env, nil
	}
	// 回退：用户默认环境（BACKTEST 类型 is_default='1'）
	env, err := s.defaultEnvironment(ctx)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return env, nil
}

// defaultEnvironment 查询默认回测环境。
func (s *Service) defaultEnvironment(ctx context.Context) (*Environment, error) {
	row := s.pool.QueryRow(ctx, `
SELECT env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by, gmt_update
FROM finv_quant_environment
WHERE env_type='BACKTEST' AND is_default='1' AND allow_backtest='1' AND status='ENABLED'
ORDER BY gmt_update DESC LIMIT 1`)
	return scanEnvironment(row)
}

// ---------------------------------------------------------------------
// 环境 CRUD
// ---------------------------------------------------------------------

// SaveEnvironment 新增或更新环境。
func (s *Service) SaveEnvironment(ctx context.Context, env *Environment) (string, error) {
	if err := validateEnvironment(env); err != nil {
		return "", err
	}
	if env.EnvID == "" {
		env.EnvID = newUUID()
	}
	if env.EnvType == "" {
		env.EnvType = "BACKTEST"
	}
	if env.UserID == "" {
		env.UserID = "default"
	}
	if env.IsDefault == "" {
		env.IsDefault = FlagOff
	}
	if env.AllowBacktest == "" {
		env.AllowBacktest = FlagOn
	}
	if env.Status == "" {
		env.Status = StatusEnabled
	}
	configJSON, err := json.Marshal(env.Config)
	if err != nil {
		return "", fmt.Errorf("环境配置序列化失败: %w", err)
	}
	// 设为默认环境时，同用户 + 同类型仅保留一个默认（配合 V28 部分唯一索引）
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if env.IsDefault == FlagOn {
		if _, err := tx.Exec(ctx,
			`UPDATE finv_quant_environment SET is_default='0' WHERE user_id=$1 AND env_type=$2 AND env_id <> $3 AND is_default='1'`,
			env.UserID, env.EnvType, env.EnvID); err != nil {
			return "", err
		}
	}
	_, err = tx.Exec(ctx, `
INSERT INTO finv_quant_environment
    (env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (env_id) DO UPDATE SET
    env_code = EXCLUDED.env_code, env_name = EXCLUDED.env_name, env_type = EXCLUDED.env_type,
    region = EXCLUDED.region, market_code = EXCLUDED.market_code, config = EXCLUDED.config,
    user_id = EXCLUDED.user_id, is_default = EXCLUDED.is_default, allow_backtest = EXCLUDED.allow_backtest,
    status = EXCLUDED.status, description = EXCLUDED.description, created_by = EXCLUDED.created_by`,
		env.EnvID, env.EnvCode, env.EnvName, env.EnvType, env.Region, env.MarketCode, configJSON,
		env.UserID, env.IsDefault, env.AllowBacktest, env.Status, env.Description, env.CreatedBy)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return env.EnvID, nil
}

// ListEnvironments 分页查询环境（user_id 隔离：system + 指定用户）。
func (s *Service) ListEnvironments(ctx context.Context, pager Pager, userID, envType, keyword string) ([]Environment, int, error) {
	pager.Normalize()
	where := []string{"(user_id = 'system' OR user_id = $1)"}
	args := []any{}
	if userID == "" {
		userID = "default"
	}
	args = append(args, userID)
	if envType != "" {
		args = append(args, envType)
		where = append(where, fmt.Sprintf("env_type = $%d", len(args)))
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			env_code ILIKE $%d OR env_name ILIKE $%d OR description ILIKE $%d OR region ILIKE $%d
		)`, len(args), len(args), len(args), len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_environment WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by, gmt_update
FROM finv_quant_environment
WHERE `+cond+`
ORDER BY is_default DESC, gmt_update DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Environment{}
	for rows.Next() {
		env, err := scanEnvironment(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *env)
	}
	return list, total, rows.Err()
}

// GetEnvironment 查询单个环境（多用户归属校验：system 全局可见）。
func (s *Service) GetEnvironment(ctx context.Context, envID, userID string) (*Environment, error) {
	if userID == "" {
		userID = "default"
	}
	row := s.pool.QueryRow(ctx, `
SELECT env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by, gmt_update
FROM finv_quant_environment WHERE env_id = $1 AND (user_id = 'system' OR user_id = $2)`, envID, userID)
	env, err := scanEnvironment(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("环境不存在: %s", envID)
		}
		return nil, err
	}
	return env, nil
}

// ToggleEnvironment 切换环境回测开关（多用户归属校验）。
func (s *Service) ToggleEnvironment(ctx context.Context, envID, allowBacktest, userID string) error {
	if allowBacktest != FlagOn && allowBacktest != FlagOff {
		return fmt.Errorf("allow_backtest 仅支持 0/1")
	}
	if userID == "" {
		userID = "default"
	}
	tag, err := s.pool.Exec(ctx, `UPDATE finv_quant_environment SET allow_backtest=$2 WHERE env_id=$1 AND (user_id='system' OR user_id=$3)`, envID, allowBacktest, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("环境不存在: %s", envID)
	}
	return nil
}

// DeleteEnvironment 删除环境（存在关联回测任务时拒绝；多用户归属校验）。
func (s *Service) DeleteEnvironment(ctx context.Context, envID, userID string) error {
	if userID == "" {
		userID = "default"
	}
	var runCount int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_run WHERE env_id = $1`, envID).Scan(&runCount); err != nil {
		return err
	}
	if runCount > 0 {
		return fmt.Errorf("环境已关联 %d 个回测任务，禁止删除（可改为禁用）", runCount)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM finv_quant_environment WHERE env_id = $1 AND user_id = $2`, envID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("环境不存在: %s", envID)
	}
	return nil
}

// scanEnvironment 扫描环境行。
func scanEnvironment(row rowScanner) (*Environment, error) {
	var env Environment
	var configJSON []byte
	// region / description / created_by 列可空（region 无 NOT NULL），用指针接收避免 NULL 扫描失败
	var region, desc, createdBy *string
	err := row.Scan(&env.EnvID, &env.EnvCode, &env.EnvName, &env.EnvType, &region, &env.MarketCode,
		&configJSON, &env.UserID, &env.IsDefault, &env.AllowBacktest, &env.Status,
		&desc, &createdBy, &env.GMTUpdate)
	if err != nil {
		return nil, err
	}
	if region != nil {
		env.Region = *region
	}
	if desc != nil {
		env.Description = *desc
	}
	if createdBy != nil {
		env.CreatedBy = *createdBy
	}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &env.Config); err != nil {
			return nil, fmt.Errorf("环境配置解析失败: %w", err)
		}
	}
	return &env, nil
}

// ---------------------------------------------------------------------
// 模板 CRUD
// ---------------------------------------------------------------------

// SaveTemplate 新增或更新模板（内置模板禁止修改；用户保存强制 is_builtin='0'）。
func (s *Service) SaveTemplate(ctx context.Context, tmpl *Template) (string, error) {
	if err := validateTemplate(tmpl); err != nil {
		return "", err
	}
	if tmpl.TemplateID == "" {
		tmpl.TemplateID = newUUID()
	}
	if tmpl.UserID == "" {
		tmpl.UserID = "default"
	}
	if tmpl.Status == "" {
		tmpl.Status = StatusEnabled
	}
	// 内置模板保护：已存在且 is_builtin='1' 时拒绝更新（防篡改内置模板）
	if tmpl.TemplateID != "" {
		var builtin string
		err := s.pool.QueryRow(ctx, `SELECT is_builtin FROM finv_quant_template WHERE template_id=$1`, tmpl.TemplateID).Scan(&builtin)
		if err == nil && builtin == FlagOn {
			return "", fmt.Errorf("内置模板禁止修改（请复制为新模板后自定义）")
		}
	}
	// 用户保存的模板不允许伪造内置标识
	tmpl.IsBuiltin = FlagOff
	contentJSON, err := json.Marshal(tmpl.Content)
	if err != nil {
		return "", fmt.Errorf("模板内容序列化失败: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (template_id) DO UPDATE SET
    template_code = EXCLUDED.template_code, template_name = EXCLUDED.template_name,
    template_type = EXCLUDED.template_type, content = EXCLUDED.content,
    user_id = EXCLUDED.user_id, is_builtin = EXCLUDED.is_builtin, status = EXCLUDED.status,
    description = EXCLUDED.description, created_by = EXCLUDED.created_by`,
		tmpl.TemplateID, tmpl.TemplateCode, tmpl.TemplateName, tmpl.TemplateType, contentJSON,
		tmpl.UserID, tmpl.IsBuiltin, tmpl.Status, tmpl.Description, tmpl.CreatedBy)
	if err != nil {
		return "", err
	}
	return tmpl.TemplateID, nil
}

// ListTemplates 分页查询模板（user_id 隔离：system + 指定用户）。
func (s *Service) ListTemplates(ctx context.Context, pager Pager, userID, templateType, keyword string) ([]Template, int, error) {
	pager.Normalize()
	where := []string{"(user_id = 'system' OR user_id = $1)"}
	args := []any{}
	if userID == "" {
		userID = "default"
	}
	args = append(args, userID)
	if templateType != "" {
		args = append(args, templateType)
		where = append(where, fmt.Sprintf("template_type = $%d", len(args)))
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			template_code ILIKE $%d OR template_name ILIKE $%d OR description ILIKE $%d
		)`, len(args), len(args), len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_template WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by, gmt_update
FROM finv_quant_template
WHERE `+cond+`
ORDER BY is_builtin DESC, gmt_update DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Template{}
	for rows.Next() {
		var tmpl Template
		var contentJSON []byte
		// description / created_by 列可空，用指针接收避免 NULL 扫描失败
		var desc, createdBy *string
		if err := rows.Scan(&tmpl.TemplateID, &tmpl.TemplateCode, &tmpl.TemplateName, &tmpl.TemplateType,
			&contentJSON, &tmpl.UserID, &tmpl.IsBuiltin, &tmpl.Status, &desc, &createdBy, &tmpl.GMTUpdate); err != nil {
			return nil, 0, err
		}
		if desc != nil {
			tmpl.Description = *desc
		}
		if createdBy != nil {
			tmpl.CreatedBy = *createdBy
		}
		if len(contentJSON) > 0 {
			_ = json.Unmarshal(contentJSON, &tmpl.Content)
		}
		list = append(list, tmpl)
	}
	return list, total, rows.Err()
}

// GetTemplate 查询单个模板（多用户归属校验：system 全局可见）。
func (s *Service) GetTemplate(ctx context.Context, templateID, userID string) (*Template, error) {
	if userID == "" {
		userID = "default"
	}
	var tmpl Template
	var contentJSON []byte
	// description / created_by 列可空，用指针接收避免 NULL 扫描失败
	var desc, createdBy *string
	err := s.pool.QueryRow(ctx, `
SELECT template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by, gmt_update
FROM finv_quant_template WHERE template_id = $1 AND (user_id = 'system' OR user_id = $2)`, templateID, userID).Scan(
		&tmpl.TemplateID, &tmpl.TemplateCode, &tmpl.TemplateName, &tmpl.TemplateType,
		&contentJSON, &tmpl.UserID, &tmpl.IsBuiltin, &tmpl.Status, &desc, &createdBy, &tmpl.GMTUpdate)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("模板不存在: %s", templateID)
		}
		return nil, err
	}
	if desc != nil {
		tmpl.Description = *desc
	}
	if createdBy != nil {
		tmpl.CreatedBy = *createdBy
	}
	if len(contentJSON) > 0 {
		_ = json.Unmarshal(contentJSON, &tmpl.Content)
	}
	return &tmpl, nil
}

// DeleteTemplate 删除模板（内置模板禁止删除；多用户归属校验）。
func (s *Service) DeleteTemplate(ctx context.Context, templateID, userID string) error {
	if userID == "" {
		userID = "default"
	}
	var isBuiltin, owner string
	if err := s.pool.QueryRow(ctx,
		`SELECT is_builtin, user_id FROM finv_quant_template WHERE template_id = $1`, templateID).Scan(&isBuiltin, &owner); err != nil {
		return fmt.Errorf("模板不存在: %s", templateID)
	}
	if isBuiltin == FlagOn {
		return fmt.Errorf("内置模板禁止删除")
	}
	if owner != userID {
		return fmt.Errorf("无权删除其他用户的模板")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM finv_quant_template WHERE template_id = $1 AND user_id = $2`, templateID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("模板不存在: %s", templateID)
	}
	return nil
}

// markRun 更新任务状态/进度/错误/报告。
func (s *Service) markRun(ctx context.Context, runID, status string, progress int, errMsg string, report *RunReport) {
	var reportJSON any
	if report != nil {
		reportJSON = report
	} else {
		reportJSON = nil
	}
	_, _ = s.pool.Exec(ctx, `
UPDATE finv_quant_backtest_run
SET status=$2, progress=$3, error_message=$4, report=COALESCE($5, report), finished_at=now()
WHERE run_id=$1`, runID, status, progress, nullIfEmpty(errMsg), reportJSON)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// resolveDateRange 解析回测日期区间（缺省取行情最早/最晚日期）。
func (s *Service) resolveDateRange(ctx context.Context, secuCode string, startDate, endDate int) (int, int, error) {
	if startDate == 0 || endDate == 0 {
		var minDate, maxDate int
		err := s.pool.QueryRow(ctx, `
SELECT COALESCE(MIN(date), 0), COALESCE(MAX(date), 0)
FROM finv_quote_secu_kline_min WHERE secu_code = $1`, secuCode).Scan(&minDate, &maxDate)
		if err != nil {
			return 0, 0, err
		}
		if minDate == 0 {
			return 0, 0, fmt.Errorf("标的 %s 无任何行情数据", secuCode)
		}
		if startDate == 0 {
			startDate = minDate
		}
		if endDate == 0 {
			endDate = maxDate
		}
	}
	if startDate > endDate {
		return 0, 0, fmt.Errorf("开始日期（%d）不能晚于结束日期（%d）", startDate, endDate)
	}
	return startDate, endDate, nil
}

// dateToTS 将 yyyymmdd 转为 UTC 秒（当日 00:00）。
func dateToTS(date int) int64 {
	year := date / 10000
	month := (date / 100) % 100
	day := date % 100
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Unix()
}

// ---------------------------------------------------------------------
// 回测任务删除（异步批处理删除任务 + 审计日志留痕）
//
// 设计（ACANX 需求）：回测任务可能产生大量无参考价值记录（净值曲线/成交/
// 资金流水/持仓/事件追踪明细），按 run_id 删除任务及其关联记录；策略/账户/
// 环境/模板保留。删除以异步任务执行（复用 s.tasks/s.sem 并发模式），
// 全过程写 finv_quant_backtest_del_log 审计日志（创建/每表删除行数/成功/失败）。
// ---------------------------------------------------------------------

// deleteDetailTables 删除任务明细表的删除顺序（先子表后主表，避免中途失败遗留孤立数据）。
var deleteDetailTables = []string{
	"finv_quant_backtest_event_trace",
	"finv_quant_backtest_position_log",
	"finv_quant_backtest_cashflow",
	"finv_quant_backtest_trade",
	"finv_quant_backtest_equity",
}

// DeleteRun 提交一个回测任务删除任务（异步执行）。
// 校验：任务存在且属于当前用户、状态为已结束（SUCCEEDED/FAILED/CANCELLED）、
// 且无进行中的删除任务。返回删除任务 ID（del_task_id）。
func (s *Service) DeleteRun(ctx context.Context, runID, userID string) (string, error) {
	if userID == "" {
		userID = "default"
	}
	// 校验任务归属与状态
	var owner, status string
	if err := s.pool.QueryRow(ctx,
		`SELECT user_id, status FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(&owner, &status); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("回测任务不存在: %s", runID)
		}
		return "", err
	}
	if owner != userID {
		return "", fmt.Errorf("无权操作其他用户的任务: %s", runID)
	}
	if status == RunPending || status == RunRunning {
		return "", fmt.Errorf("任务正在执行（%s），仅已结束任务可删除", status)
	}

	// 防重复：存在该 run 的进行中删除任务时拒绝
	var activeCnt int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_quant_backtest_del_task
		 WHERE run_id=$1 AND status IN ('PENDING','RUNNING')`, runID).Scan(&activeCnt); err != nil {
		return "", err
	}
	if activeCnt > 0 {
		return "", fmt.Errorf("该任务已有进行中的删除任务，请勿重复提交")
	}

	// 创建删除任务（UUID 大写）
	delTaskID := newUUID()
	if _, err := s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_del_task (del_task_id, run_id, status, progress, created_by)
VALUES ($1,$2,$3,0,$4)`, delTaskID, runID, DelPending, userID); err != nil {
		return "", err
	}
	// 审计日志：任务创建
	s.appendDelLog(ctx, delTaskID, runID, "TASK_CREATED", "删除任务创建（run_id="+runID+"）")

	// 注册异步执行（复用 s.tasks/s.sem 并发模式）
	delCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.tasks["del:"+delTaskID] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.tasks, "del:"+delTaskID)
			s.mu.Unlock()
		}()
		s.executeRunDelete(delCtx, delTaskID, runID)
	}()

	return delTaskID, nil
}

// executeRunDelete 异步执行删除任务：按序删除 5 张明细表 + 主表，全程写审计日志。
func (s *Service) executeRunDelete(ctx context.Context, delTaskID, runID string) {
	// 并发信号量（排队等待，与回测任务共用上限）
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		s.failDelTask(context.Background(), delTaskID, runID, "删除任务已取消")
		return
	}

	// 状态置 RUNNING
	if _, err := s.pool.Exec(ctx,
		`UPDATE finv_quant_backtest_del_task SET status=$2, gmt_update=now() WHERE del_task_id=$1`,
		delTaskID, DelRunning); err != nil {
		s.failDelTask(context.Background(), delTaskID, runID, "更新删除任务状态失败: "+err.Error())
		return
	}

	// 先归档 run 元信息（"曾经存在的证明"）：失败/已结束任务的执行记录
	// 写入归档表留痕，再物理删除明细与主表（归档失败则中止删除，保证留痕不缺失）。
	if err := s.archiveRun(ctx, delTaskID, runID); err != nil {
		s.failDelTask(context.Background(), delTaskID, runID, "归档任务元信息失败: "+err.Error())
		return
	}

	counts := map[string]int{}
	tableCnt := len(deleteDetailTables) + 1 // 明细表 + 主表
	step := 0

	// 1. 按序删除明细表
	for _, tbl := range deleteDetailTables {
		if ctx.Err() != nil {
			s.failDelTask(context.Background(), delTaskID, runID, "删除任务已取消")
			return
		}
		tag, err := s.pool.Exec(ctx, `DELETE FROM `+tbl+` WHERE run_id=$1`, runID)
		if err != nil {
			s.failDelTask(context.Background(), delTaskID, runID, fmt.Sprintf("删除表 %s 失败: %v", tbl, err))
			return
		}
		n := int(tag.RowsAffected())
		counts[tbl] = n
		step++
		// 审计日志 + 进度
		s.appendDelLog(ctx, delTaskID, runID, "DELETING_TABLE",
			fmt.Sprintf("%s: 删除 %d 行", tbl, n))
		progress := step * 100 / tableCnt
		_, _ = s.pool.Exec(ctx,
			`UPDATE finv_quant_backtest_del_task SET progress=$2, gmt_update=now() WHERE del_task_id=$1`,
			delTaskID, progress)
	}

	// 2. 删除主表
	tag, err := s.pool.Exec(ctx, `DELETE FROM finv_quant_backtest_run WHERE run_id=$1`, runID)
	if err != nil {
		s.failDelTask(context.Background(), delTaskID, runID, "删除任务主表失败: "+err.Error())
		return
	}
	counts["finv_quant_backtest_run"] = int(tag.RowsAffected())
	step++

	// 3. 更新删除任务为 SUCCEEDED，登记各表删除行数
	countsJSON, _ := json.Marshal(counts)
	if _, err := s.pool.Exec(ctx,
		`UPDATE finv_quant_backtest_del_task SET status=$2, progress=100, deleted_counts=$3, gmt_update=now()
		 WHERE del_task_id=$1`, delTaskID, DelSucceeded, countsJSON); err != nil {
		s.failDelTask(context.Background(), delTaskID, runID, "更新删除任务结果失败: "+err.Error())
		return
	}
	// 审计日志：成功（含各表行数汇总）
	s.appendDelLog(ctx, delTaskID, runID, "TASK_SUCCEEDED",
		fmt.Sprintf("删除完成，共 %d 张表（%v）", step, counts))
}

// failDelTask 将删除任务标记为 FAILED 并写失败日志。
func (s *Service) failDelTask(ctx context.Context, delTaskID, runID, reason string) {
	_, _ = s.pool.Exec(ctx,
		`UPDATE finv_quant_backtest_del_task SET status=$2, error_message=$3, gmt_update=now()
		 WHERE del_task_id=$1`, delTaskID, DelFailed, nullIfEmpty(reason))
	s.appendDelLog(ctx, delTaskID, runID, "TASK_FAILED", reason)
}

// appendDelLog 追加删除审计日志（只追加，不可改）。
func (s *Service) appendDelLog(ctx context.Context, delTaskID, runID, action, detail string) {
	var seq int
	_ = s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0)+1 FROM finv_quant_backtest_del_log WHERE del_task_id=$1`,
		delTaskID).Scan(&seq)
	_, _ = s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_del_log (del_task_id, run_id, seq, action, detail)
VALUES ($1,$2,$3,$4,$5)`, delTaskID, runID, seq, action, detail)
}

// archiveRun 归档待删除任务的元信息（"曾经存在的证明"）：把 run 主表元信息
// （任务号/策略/账户/标的/区间/状态/错误信息/报告）写入归档表，再删除主表。
// 返回错误则中止删除流程（保证留痕不缺失）。
func (s *Service) archiveRun(ctx context.Context, delTaskID, runID string) error {
	var a RunArchive
	var reportJSON []byte
	var strategyID, strategyName, accountID, accountName, secu, period, errMsg *string
	var startDate, endDate *int
	if err := s.pool.QueryRow(ctx, `
SELECT run_id, run_no, strategy_id, strategy_name, account_id, account_name,
       secu_code, period, start_date, end_date, status, error_message, report
FROM finv_quant_backtest_run WHERE run_id=$1`, runID).Scan(
		&a.RunID, &a.RunNo,
		&strategyID, &strategyName, &accountID, &accountName,
		&secu, &period, &startDate, &endDate,
		&a.Status, &errMsg, &reportJSON); err != nil {
		return fmt.Errorf("读取任务元信息失败: %w", err)
	}
	if strategyID != nil {
		a.StrategyID = *strategyID
	}
	if strategyName != nil {
		a.StrategyName = *strategyName
	}
	if accountID != nil {
		a.AccountID = *accountID
	}
	if accountName != nil {
		a.AccountName = *accountName
	}
	if secu != nil {
		a.SecuCode = *secu
	}
	if period != nil {
		a.Period = *period
	}
	if startDate != nil {
		a.StartDate = *startDate
	}
	if endDate != nil {
		a.EndDate = *endDate
	}
	if errMsg != nil {
		a.ErrorMessage = *errMsg
	}
	if len(reportJSON) > 0 {
		a.ReportJSON = string(reportJSON)
	}

	a.ArchiveID = newUUID()
	// 操作者取删除任务创建人（DeleteRun 已做归属校验）
	deletedBy := "console"
	if err := s.pool.QueryRow(ctx,
		`SELECT created_by FROM finv_quant_backtest_del_task WHERE del_task_id=$1`, delTaskID).Scan(&deletedBy); err != nil {
		deletedBy = "console"
	}
	a.DeletedBy = deletedBy
	a.DelTaskID = delTaskID
	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_quant_backtest_run_archive
    (archive_id, run_id, run_no, strategy_id, strategy_name, account_id, account_name,
     secu_code, period, start_date, end_date, status, error_message, report_json,
     deleted_at, deleted_by, del_task_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),$15,$16)`,
		a.ArchiveID, a.RunID, a.RunNo, a.StrategyID, a.StrategyName, a.AccountID, a.AccountName,
		a.SecuCode, a.Period, a.StartDate, a.EndDate, a.Status, a.ErrorMessage, a.ReportJSON,
		a.DeletedBy, delTaskID)
	if err != nil {
		return err
	}
	// 审计日志：归档留痕
	s.appendDelLog(ctx, delTaskID, runID, "ARCHIVE_RUN",
		fmt.Sprintf("任务 #%d 元信息已归档（archive_id=%s）", a.RunNo, a.ArchiveID))
	return nil
}

// ListRunArchives 分页查询已删除任务归档（"曾经存在的证明"）。
func (s *Service) ListRunArchives(ctx context.Context, pager Pager, runID string) ([]RunArchive, int, error) {
	pager.Normalize()
	where := "1=1"
	args := []any{}
	if runID != "" {
		args = append(args, runID)
		where = "run_id = $1"
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_backtest_run_archive WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT archive_id, run_id, run_no, COALESCE(strategy_id,''), COALESCE(strategy_name,''),
       COALESCE(account_id,''), COALESCE(account_name,''), COALESCE(secu_code,''),
       COALESCE(period,''), COALESCE(start_date,0), COALESCE(end_date,0),
       status, COALESCE(error_message,''), COALESCE(report_json,''),
       deleted_at, deleted_by, del_task_id
FROM finv_quant_backtest_run_archive
WHERE `+where+`
ORDER BY deleted_at DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []RunArchive{}
	for rows.Next() {
		var a RunArchive
		if err := rows.Scan(&a.ArchiveID, &a.RunID, &a.RunNo, &a.StrategyID, &a.StrategyName,
			&a.AccountID, &a.AccountName, &a.SecuCode, &a.Period, &a.StartDate, &a.EndDate,
			&a.Status, &a.ErrorMessage, &a.ReportJSON, &a.DeletedAt, &a.DeletedBy, &a.DelTaskID); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

// ListRunDelTasks 分页查询删除任务（按 user_id 隔离：仅本人创建的删除任务）。
func (s *Service) ListRunDelTasks(ctx context.Context, q DelTaskListQuery, userID string) ([]RunDelTask, int, error) {
	q.Pager.Normalize()
	if userID == "" {
		userID = "default"
	}
	where := []string{"created_by = $1"}
	args := []any{userID}
	if q.Status != "" {
		args = append(args, q.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if q.RunID != "" {
		args = append(args, q.RunID)
		where = append(where, fmt.Sprintf("run_id = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM finv_quant_backtest_del_task WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT del_task_id, run_id, status, progress, COALESCE(error_message, ''), COALESCE(deleted_counts, '{}'::jsonb), created_by, gmt_update
FROM finv_quant_backtest_del_task
WHERE `+cond+`
ORDER BY gmt_update DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []RunDelTask{}
	for rows.Next() {
		var t RunDelTask
		var countsJSON []byte
		if err := rows.Scan(&t.DelTaskID, &t.RunID, &t.Status, &t.Progress, &t.ErrorMessage,
			&countsJSON, &t.CreatedBy, &t.GMTUpdate); err != nil {
			return nil, 0, err
		}
		if len(countsJSON) > 0 {
			_ = json.Unmarshal(countsJSON, &t.DeletedCounts)
		}
		list = append(list, t)
	}
	return list, total, rows.Err()
}

// ListRunDelLogs 按删除任务查询审计日志（留痕）。
func (s *Service) ListRunDelLogs(ctx context.Context, delTaskID, userID string) ([]RunDelLog, error) {
	if userID == "" {
		userID = "default"
	}
	// 归属校验：删除任务须属于当前用户
	var owner string
	if err := s.pool.QueryRow(ctx,
		`SELECT created_by FROM finv_quant_backtest_del_task WHERE del_task_id=$1`, delTaskID).Scan(&owner); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("删除任务不存在: %s", delTaskID)
		}
		return nil, err
	}
	if owner != userID {
		return nil, fmt.Errorf("无权查看其他用户的删除任务: %s", delTaskID)
	}
	rows, err := s.pool.Query(ctx, `
SELECT log_id, del_task_id, run_id, seq, action, COALESCE(detail, ''), created_at
FROM finv_quant_backtest_del_log WHERE del_task_id=$1 ORDER BY seq ASC`, delTaskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []RunDelLog{}
	for rows.Next() {
		var l RunDelLog
		if err := rows.Scan(&l.LogID, &l.DelTaskID, &l.RunID, &l.Seq, &l.Action, &l.Detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// RetryRunDelete 重试失败的删除任务（新建一个删除任务重新执行；原任务保留为 FAILED 留痕）。
func (s *Service) RetryRunDelete(ctx context.Context, delTaskID, userID string) (string, error) {
	if userID == "" {
		userID = "default"
	}
	var owner, status, runID string
	if err := s.pool.QueryRow(ctx,
		`SELECT created_by, status, run_id FROM finv_quant_backtest_del_task WHERE del_task_id=$1`,
		delTaskID).Scan(&owner, &status, &runID); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("删除任务不存在: %s", delTaskID)
		}
		return "", err
	}
	if owner != userID {
		return "", fmt.Errorf("无权操作其他用户的删除任务: %s", delTaskID)
	}
	if status != DelFailed {
		return "", fmt.Errorf("仅 FAILED 状态的删除任务可重试（当前 %s）", status)
	}
	return s.DeleteRun(ctx, runID, userID)
}

// ---------------------------------------------------------------------
// 行扫描与解析辅助
// ---------------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanStrategy(row rowScanner) (Strategy, error) {
	var st Strategy
	var defJSON []byte
	// description / secu_code / created_by 列可空（种子或历史数据可能为 NULL），用指针接收
	var desc, secuCode, createdBy *string
	err := row.Scan(&st.StrategyID, &st.StrategyCode, &st.StrategyName, &st.StrategyType,
		&desc, &defJSON, &st.DefinitionVersion, &st.DataPeriod, &secuCode,
		&st.UserID, &st.TemplateID, &st.AllowBacktest, &st.Status, &createdBy, &st.GMTUpdate)
	if err != nil {
		return st, err
	}
	if desc != nil {
		st.Description = *desc
	}
	if secuCode != nil {
		st.SecuCode = *secuCode
	}
	if createdBy != nil {
		st.CreatedBy = *createdBy
	}
	if err := json.Unmarshal(defJSON, &st.Definition); err != nil {
		return st, fmt.Errorf("策略定义解析失败: %w", err)
	}
	return st, nil
}

func scanRun(row rowScanner) (Run, error) {
	var r Run
	var defJSON, accJSON, envJSON, optsJSON, reportJSON []byte
	var startedAt, finishedAt *time.Time
	// error_message / env_id / created_by 列可空（markRun 空错误写 NULL），用指针接收避免 NULL 扫描失败
	var envID, errMsg, createdBy *string
	err := row.Scan(&r.RunID, &r.RunNo, &r.UserID, &r.StrategyID, &r.StrategyCode, &r.StrategyName, &defJSON,
		&r.AccountID, &r.AccountCode, &r.AccountName, &accJSON,
		&envID, &envJSON,
		&r.SecuCode, &r.MarketCode, &r.Period, &r.ReportPrecision,
		&r.StartTS, &r.EndTS, &r.StartDate, &r.EndDate, &optsJSON,
		&r.Status, &r.Progress, &errMsg, &reportJSON,
		&startedAt, &finishedAt, &createdBy, &r.GMTUpdate)
	if err != nil {
		return r, err
	}
	if envID != nil {
		r.EnvID = *envID
	}
	if errMsg != nil {
		r.ErrorMessage = *errMsg
	}
	if createdBy != nil {
		r.CreatedBy = *createdBy
	}
	if len(defJSON) > 0 {
		if err := json.Unmarshal(defJSON, &r.StrategySnapshot); err != nil {
			return r, fmt.Errorf("策略快照解析失败: %w", err)
		}
	}
	if len(accJSON) > 0 {
		if err := json.Unmarshal(accJSON, &r.AccountSnapshot); err != nil {
			return r, fmt.Errorf("账户快照解析失败: %w", err)
		}
	}
	if len(envJSON) > 0 && string(envJSON) != "null" {
		var env Environment
		if err := json.Unmarshal(envJSON, &env); err == nil {
			r.EnvironmentSnapshot = &env
		}
	}
	if len(optsJSON) > 0 {
		if err := json.Unmarshal(optsJSON, &r.Options); err != nil {
			return r, fmt.Errorf("任务配置解析失败: %w", err)
		}
	}
	if len(reportJSON) > 0 && string(reportJSON) != "null" && string(reportJSON) != "{}" {
		var report RunReport
		if err := json.Unmarshal(reportJSON, &report); err == nil {
			r.Report = &report
		}
	}
	r.StartedAt = startedAt
	r.FinishedAt = finishedAt
	return r, nil
}

// Definition 解析任务快照为策略定义。
func (r *Run) Definition() StrategyDefinition {
	var def StrategyDefinition
	raw, _ := json.Marshal(r.StrategySnapshot)
	_ = json.Unmarshal(raw, &def)
	return def
}

// RunOptions 解析任务配置。
func (r *Run) RunOptions() RunOptions {
	var opts RunOptions
	raw, _ := json.Marshal(r.Options)
	_ = json.Unmarshal(raw, &opts)
	return opts
}
