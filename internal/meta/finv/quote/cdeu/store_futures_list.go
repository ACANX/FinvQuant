package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件是「期货列表快照同步」在本项目侧的落地（需求 11273/11274）：
//   - 配置读写（FutureListConfigStore）：读 cdeu.finv_quote_cdeu_future_sync_config、
//     回写每轮 count_exists / total_available；
//   - 逐页落库（FutureListSink）：按主键 stock_id upsert 到 cdeu.finv_quote_realtime_future，
//     并在整轮成功后把「未再出现」的合约标记下架。
//
// 采集流程本身在 quote_list_futures_snapshot_sync.go（单文件、可整体迁往 MetaExt 第三方包）；
// 这里只做与 FinvQuant 存储相关的事。
//
// 字段口径（接口 fields 是按字段编号的位置化字典，本仓库只收录已验证的 5 个）：
//
//	fields["1"]  → contract_code   合约代码（GCmain）
//	fields["2"]  → name            合约名称
//	fields["3"]  → price_nominal   最新价
//	fields["4"]  → change_ratio    涨跌幅（去 % 后按百分点数值入库）
//	fields["19"] → market_label    市场标签（US / HK）
//
// 其余编号原样进 fields_json（另有 field_list_json 留档字段定义），
// 待确认含义后再补提列，不需要重新采集。

const (
	// tableFutureList 期货合约实时快照表（V300216）。
	//
	// 表名以 finv_quote_ 开头但位于 cdeu 域，属 DbSchemaSpec §1 的刻意例外
	// （用户 2026-11-20 指定），由 schema_qualify_test.go 的 exactTableSchema 固定为 cdeu.。
	tableFutureList = "cdeu.finv_quote_realtime_future"
	// tableFutureSyncConfig 期货列表采集配置表（V300216）。
	tableFutureSyncConfig = "cdeu.finv_quote_cdeu_future_sync_config"
)

// 期货接口已验证的字段编号。
const (
	futureFieldContractCode = "1"
	futureFieldName         = "2"
	futureFieldPrice        = "3"
	futureFieldChangeRatio  = "4"
	futureFieldMarketLabel  = "19"
)

// FutureListProvider 写入 provider 列的固定值（期货当前只有 moomoo 一个来源）。
const FutureListProvider = "FTMM"

// FutureListRow 是 cdeu.finv_quote_realtime_future 的一行。
//
// 数值列用 *json.Number：与既有快照落库口径一致，由 pgx 以文本传，
// 避免 float64 二次格式化丢精度；nil 表示该字段无来源，写 NULL。
type FutureListRow struct {
	StockID       int64
	USC           *string
	FuturesType   string
	MarketType    *string
	MarketLabel   *string
	ContractCode  *string
	Name          *string
	PriceNominal  *json.Number
	ChangeRatio   *json.Number
	PriceDirect   *string
	FieldsJSON    string
	FieldListJSON string
	Page          *int64
	DtUpdate      *int64
	DtLastSeen    *int64
	FlagOnline    string
	Provider      string
	Remark        *string
}

// BuildFutureListRows 把一页接口条目转换成落库行（纯函数，便于单测覆盖字段口径）。
//
// uscByStockID：stock_id → usc（未解析到的不出现在 map 中，列写 NULL）；
// uscByFtmm：派生键 upper(市场标签.合约代码) → usc（stock_id 没命中时的兜底）。
func BuildFutureListRows(cfg FutureListCollectConfig, page int, items []quoteapiv2.FuturesListItem,
	uscByStockID map[int64]string, uscByFtmm map[string]string, dtMs int64) []FutureListRow {
	rows := make([]FutureListRow, 0, len(items))
	if page <= 0 {
		page = 1
	}
	pageNo := int64(page)
	dtUpdate := dtMs
	dtSeen := dtMs
	marketType := cfg.MarketType
	if strings.TrimSpace(marketType) == "" {
		marketType = FutureListDefaultMarketType
	}
	for _, item := range items {
		if item.StockID <= 0 {
			continue // 主键缺失的条目无法入库，直接跳过（调用方按条数差统计）
		}
		marketLabel := futureListItemText(item, futureFieldMarketLabel)
		contractCode := futureListItemText(item, futureFieldContractCode)

		// usc 解析顺序：stock_id 关联采集参数表 → 派生键关联证券键表；都查不到留 NULL。
		// 刻意不用合约代码冒充 usc（同码可能跨市场重复，且期货合约多数还没进主数据）。
		usc := ""
		if value, ok := uscByStockID[item.StockID]; ok {
			usc = value
		} else if value, ok := uscByFtmm[futureListFtmmKey(marketLabel, contractCode)]; ok {
			usc = value
		}

		row := FutureListRow{
			StockID:       item.StockID,
			FuturesType:   cfg.FuturesType,
			MarketType:    futureListText(marketType),
			MarketLabel:   futureListText(marketLabel),
			ContractCode:  futureListText(contractCode),
			Name:          futureListText(futureListItemText(item, futureFieldName)),
			PriceNominal:  futureListNumber(futureListItemText(item, futureFieldPrice)),
			ChangeRatio:   futureListNumber(futureListItemText(item, futureFieldChangeRatio)),
			PriceDirect:   futureListPriceDirect(item.PriceDirect),
			FieldsJSON:    futureListJSON(item.Fields),
			FieldListJSON: futureListJSON(item.FieldList),
			Page:          &pageNo,
			DtUpdate:      &dtUpdate,
			DtLastSeen:    &dtSeen,
			FlagOnline:    "1",
			Provider:      FutureListProvider,
		}
		if usc != "" {
			row.USC = &usc
		}
		if strings.TrimSpace(item.Fields[futureFieldPrice]) == "" && row.PriceNominal == nil {
			// 价格字段缺失（该分类可能不返回价格）：记一条备注便于事后排查，不猜值。
			remark := "接口未返回价格字段（fields[\"3\"]）"
			row.Remark = &remark
		}
		rows = append(rows, row)
	}
	return rows
}

// futureListItemText 取条目某个字段编号的展示值（去空白）。
func futureListItemText(item quoteapiv2.FuturesListItem, fieldID string) string {
	return strings.TrimSpace(item.Fields[fieldID])
}

// futureListText 文本字段：空串按未提供处理（写 NULL）。
func futureListText(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// futureListNumber 展示值 → json.Number：只做文本清洗（去千分位/百分号/正号），
// 解析失败返回 nil（写 NULL，不猜值）。
func futureListNumber(raw string) *json.Number {
	plain, ok := stockListPlainNumber(raw)
	if !ok || plain == "" {
		return nil
	}
	number := json.Number(plain)
	return &number
}

// futureListPriceDirect 涨跌方向：up/+ → `+`、down/- → `-`、flat/0 及其它 → `=`。
//
// 与 cdeu.finv_quote_secu_realtime 同口径（持平用 `=`，参考表 finv_cdeu_secu_realtime_stock 用 `0`）。
func futureListPriceDirect(raw string) *string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "up", "+":
		direct := "+"
		return &direct
	case "down", "-":
		direct := "-"
		return &direct
	case "":
		return nil
	default:
		direct := "="
		return &direct
	}
}

// futureListJSON 把任意结构序列化成 jsonb 文本；失败返回空串（列写 NULL）。
func futureListJSON(value any) string {
	if value == nil {
		return ""
	}
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	if string(body) == "{}" || string(body) == "[]" || string(body) == "null" {
		return ""
	}
	return string(body)
}

// futureListFtmmKey 派生键：upper(市场标签.合约代码)，与 futu.finv_futu_secu_key 口径一致。
func futureListFtmmKey(marketLabel, contractCode string) string {
	market := strings.ToUpper(strings.TrimSpace(marketLabel))
	code := strings.TrimSpace(contractCode)
	if market == "" || code == "" {
		return ""
	}
	return market + "." + code
}

// ---------------------------------------------------------------------
// 配置读写（FutureListConfigStore）
// ---------------------------------------------------------------------

// LoadFuturesConfigs 读取启用中的采集配置，按分类、市场排序（决定采集顺序）。
func (s *Store) LoadFuturesConfigs(ctx context.Context) ([]FutureListCollectConfig, error) {
	rows, err := s.pool.Query(ctx, `
SELECT market_type, futures_type, COALESCE(futures_type_name, '') AS futures_type_name,
       count_plan,
       COALESCE(count_exists, 0)    AS count_exists,
       COALESCE(total_available, 0) AS total_available
  FROM `+tableFutureSyncConfig+`
 WHERE flag_enable = '1'
 ORDER BY futures_type, market_type`)
	if err != nil {
		return nil, fmt.Errorf("读取期货采集配置失败: %w", err)
	}
	defer rows.Close()

	configs := make([]FutureListCollectConfig, 0, 8)
	for rows.Next() {
		var cfg FutureListCollectConfig
		if err := rows.Scan(&cfg.MarketType, &cfg.FuturesType, &cfg.FuturesTypeName,
			&cfg.CountPlan, &cfg.CountExists, &cfg.TotalAvailable); err != nil {
			return nil, fmt.Errorf("解析期货采集配置失败: %w", err)
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取期货采集配置失败: %w", err)
	}
	return configs, nil
}

// SaveFuturesProgress 回写采集进度：count_exists / total_available 只在拿到正数时覆盖，
// 避免接口异常返回 0 把已观测到的分类总数抹掉。
func (s *Store) SaveFuturesProgress(ctx context.Context, cfg FutureListCollectConfig, progress FutureListProgress) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableFutureSyncConfig+`
   SET count_exists    = CASE WHEN $3 > 0 THEN $3 ELSE count_exists END,
       total_available = CASE WHEN $4 > 0 THEN $4 ELSE total_available END,
       gmt_update      = now()
 WHERE market_type = $1
   AND futures_type = $2`,
		cfg.MarketType, cfg.FuturesType, progress.CountExists, progress.TotalAvailable)
	if err != nil {
		return fmt.Errorf("回写期货采集进度失败（market_type=%s futures_type=%s）: %w",
			cfg.MarketType, cfg.FuturesType, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("回写期货采集进度失败：配置不存在（market_type=%s futures_type=%s）",
			cfg.MarketType, cfg.FuturesType)
	}
	return nil
}

// ---------------------------------------------------------------------
// 逐页落库（FutureListSink）
// ---------------------------------------------------------------------

// SaveFuturesListPage 把一页列表条目写入期货实时快照表，按行返回新增/更新标记。
//
// 冲突策略：主键单列 stock_id，整行覆盖（EXCLUDED 优先）。
// usc 例外：新值解析不到时保留旧值（COALESCE），避免一次解析失败把已补好的 usc 抹掉。
func (s *Store) SaveFuturesListPage(ctx context.Context, cfg FutureListCollectConfig, page int, items []quoteapiv2.FuturesListItem) ([]FutureListSavedRow, error) {
	if len(items) == 0 {
		return nil, nil
	}
	uscByStockID, uscByFtmm, err := s.loadFutureListUSC(ctx, items)
	if err != nil {
		return nil, err
	}

	nowMs := time.Now().UnixMilli()
	rows := BuildFutureListRows(cfg, page, items, uscByStockID, uscByFtmm, nowMs)
	if len(rows) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(futureListSourceRows(rows))
	if err != nil {
		return nil, fmt.Errorf("序列化期货落库行失败（futures_type=%s page=%d）: %w", cfg.FuturesType, page, err)
	}

	// 整批在一条语句内完成：jsonb_to_recordset 展开 → upsert → RETURNING 逐行新增/更新标记。
	sql := `
WITH src AS (
    SELECT (x.stock_id)::bigint                                AS stock_id,
           nullif(btrim(x.usc), '')                            AS usc,
           btrim(x.futures_type)                               AS futures_type,
           nullif(btrim(x.market_type), '')                     AS market_type,
           nullif(btrim(x.market_label), '')                    AS market_label,
           nullif(btrim(x.contract_code), '')                   AS contract_code,
           nullif(btrim(x.name), '')                           AS name,
           nullif(btrim(x.price_nominal), '')::numeric(20,6)   AS price_nominal,
           nullif(btrim(x.change_ratio), '')::numeric(20,6)    AS change_ratio,
           nullif(btrim(x.price_direct), '')                   AS price_direct,
           nullif(x.fields_json, '')::jsonb                    AS fields_json,
           nullif(x.field_list_json, '')::jsonb                AS field_list_json,
           (x.page)::integer                                   AS page,
           (x.dt_update)::bigint                               AS dt_update,
           (x.dt_last_seen)::bigint                            AS dt_last_seen,
           COALESCE(nullif(btrim(x.flag_online), ''), '1')      AS flag_online,
           COALESCE(nullif(btrim(x.provider), ''), 'FTMM')      AS provider,
           nullif(btrim(x.remark), '')                         AS remark
      FROM jsonb_to_recordset($1::jsonb) AS x(
             stock_id text, usc text, futures_type text, market_type text,
             market_label text, contract_code text, name text,
             price_nominal text, change_ratio text, price_direct text,
             fields_json text, field_list_json text,
             page bigint, dt_update bigint, dt_last_seen bigint,
             flag_online text, provider text, remark text)
), upserted AS (
    INSERT INTO ` + tableFutureList + ` AS t
        (stock_id, usc, futures_type, market_type, market_label, contract_code, name,
         price_nominal, change_ratio, price_direct, fields_json, field_list_json,
         page, dt_update, dt_last_seen, flag_online, provider, remark)
    SELECT src.stock_id, src.usc, src.futures_type, src.market_type, src.market_label,
           src.contract_code, src.name, src.price_nominal, src.change_ratio, src.price_direct,
           src.fields_json, src.field_list_json, src.page, src.dt_update, src.dt_last_seen,
           src.flag_online, src.provider, src.remark
      FROM src
    ON CONFLICT (stock_id) DO UPDATE SET
        usc             = COALESCE(EXCLUDED.usc, t.usc),
        futures_type    = EXCLUDED.futures_type,
        market_type     = EXCLUDED.market_type,
        market_label    = COALESCE(EXCLUDED.market_label, t.market_label),
        contract_code   = COALESCE(EXCLUDED.contract_code, t.contract_code),
        name            = COALESCE(EXCLUDED.name, t.name),
        price_nominal   = COALESCE(EXCLUDED.price_nominal, t.price_nominal),
        change_ratio    = COALESCE(EXCLUDED.change_ratio, t.change_ratio),
        price_direct    = COALESCE(EXCLUDED.price_direct, t.price_direct),
        fields_json     = COALESCE(EXCLUDED.fields_json, t.fields_json),
        field_list_json = COALESCE(EXCLUDED.field_list_json, t.field_list_json),
        page            = EXCLUDED.page,
        dt_update       = EXCLUDED.dt_update,
        dt_last_seen    = EXCLUDED.dt_last_seen,
        flag_online     = EXCLUDED.flag_online,
        provider        = EXCLUDED.provider,
        remark          = COALESCE(EXCLUDED.remark, t.remark)
    RETURNING t.stock_id, (xmax = 0) AS inserted
)
SELECT stock_id, inserted FROM upserted`

	resultRows, err := s.pool.Query(ctx, sql, string(payload))
	if err != nil {
		return nil, fmt.Errorf("写入 %s 失败（futures_type=%s page=%d）: %w",
			tableFutureList, cfg.FuturesType, page, err)
	}
	defer resultRows.Close()

	inserted := make(map[int64]bool, len(rows))
	for resultRows.Next() {
		var stockID int64
		var isInsert bool
		if err := resultRows.Scan(&stockID, &isInsert); err != nil {
			return nil, fmt.Errorf("解析期货写入结果失败: %w", err)
		}
		inserted[stockID] = isInsert
	}
	if err := resultRows.Err(); err != nil {
		return nil, fmt.Errorf("写入 %s 失败（futures_type=%s page=%d）: %w",
			tableFutureList, cfg.FuturesType, page, err)
	}

	saved := make([]FutureListSavedRow, 0, len(rows))
	unresolved := 0
	for _, row := range rows {
		if row.USC == nil {
			unresolved++
		}
		saved = append(saved, FutureListSavedRow{
			StockID:      row.StockID,
			USC:          futureListValue(row.USC),
			ContractCode: futureListValue(row.ContractCode),
			Name:         futureListValue(row.Name),
			Inserted:     inserted[row.StockID],
		})
	}
	if unresolved > 0 {
		log.Printf("[cdeu][futurelist] usc 未解析 %d/%d 行（futures_type=%s page=%d），"+
			"合约仍入库、usc 留空，待键表/主数据补齐后下一轮自动回填",
			unresolved, len(rows), cfg.FuturesType, page)
	}
	return saved, nil
}

// MarkFuturesOffline 把「最近一次出现早于 notSeenBeforeMs」的在售合约标记下架。
//
// 只在整轮全分类采集成功后调用（约束见 FutureListSink 接口注释）：
// 主键是单列 stock_id，按分类分批置下架会误标其它分类仍在售的合约。
func (s *Store) MarkFuturesOffline(ctx context.Context, notSeenBeforeMs int64) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableFutureList+`
   SET flag_online = '0'
 WHERE flag_online = '1'
   AND COALESCE(dt_last_seen, 0) < $1`, notSeenBeforeMs)
	if err != nil {
		return 0, fmt.Errorf("标记期货下架失败: %w", err)
	}
	return tag.RowsAffected(), nil
}

// loadFutureListUSC 批量解析 usc（需求 11273「解析 usc，字段可先为空」）。
//
// 关联顺序：
//  1. stock_id → futu.finv_futu_quote_collect.usc（最准确：富途 stockId 全局唯一）；
//  2. 派生键 upper(市场标签.合约代码) → futu.finv_futu_secu_key.usc（键表兜底）。
//
// 两级都不命中就留 NULL：期货合约多数还没进主数据，这个 NULL 是「尚未录入」的信号，
// 不得用合约代码冒充 usc。
func (s *Store) loadFutureListUSC(ctx context.Context, items []quoteapiv2.FuturesListItem) (map[int64]string, map[string]string, error) {
	byStockID := make(map[int64]string, len(items))
	byFtmm := make(map[string]string, len(items))
	if len(items) == 0 {
		return byStockID, byFtmm, nil
	}

	ids := make([]int64, 0, len(items))
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.StockID > 0 {
			ids = append(ids, item.StockID)
		}
		key := futureListFtmmKey(futureListItemText(item, futureFieldMarketLabel),
			futureListItemText(item, futureFieldContractCode))
		if key != "" {
			keys = append(keys, key)
		}
	}

	if len(ids) > 0 {
		rows, err := s.pool.Query(ctx, `
SELECT stock_id, btrim(usc)
  FROM futu.finv_futu_quote_collect
 WHERE stock_id = ANY($1::bigint[]) AND usc IS NOT NULL AND btrim(usc) <> ''`, ids)
		if err != nil {
			return nil, nil, fmt.Errorf("查询 stock_id→usc 映射失败: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var stockID int64
			var usc string
			if err := rows.Scan(&stockID, &usc); err != nil {
				return nil, nil, fmt.Errorf("解析 stock_id→usc 映射失败: %w", err)
			}
			byStockID[stockID] = usc
		}
		if err := rows.Err(); err != nil {
			return nil, nil, fmt.Errorf("查询 stock_id→usc 映射失败: %w", err)
		}
	}

	if len(keys) > 0 {
		rows, err := s.pool.Query(ctx, `
SELECT upper(btrim(ftmm_symbol)), btrim(usc)
  FROM futu.finv_futu_secu_key
 WHERE upper(btrim(ftmm_symbol)) = ANY($1::text[]) AND usc IS NOT NULL AND btrim(usc) <> ''`, keys)
		if err != nil {
			return nil, nil, fmt.Errorf("查询派生键→usc 映射失败: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var key string
			var usc string
			if err := rows.Scan(&key, &usc); err != nil {
				return nil, nil, fmt.Errorf("解析派生键→usc 映射失败: %w", err)
			}
			byFtmm[key] = usc
		}
		if err := rows.Err(); err != nil {
			return nil, nil, fmt.Errorf("查询派生键→usc 映射失败: %w", err)
		}
	}
	return byStockID, byFtmm, nil
}

// futureListSourceRow jsonb_to_recordset 的入参形状（字段名与 SQL 内 x(...) 列名一致）。
type futureListSourceRow struct {
	StockID       string `json:"stock_id"`
	USC           string `json:"usc"`
	FuturesType   string `json:"futures_type"`
	MarketType    string `json:"market_type"`
	MarketLabel   string `json:"market_label"`
	ContractCode  string `json:"contract_code"`
	Name          string `json:"name"`
	PriceNominal  string `json:"price_nominal"`
	ChangeRatio   string `json:"change_ratio"`
	PriceDirect   string `json:"price_direct"`
	FieldsJSON    string `json:"fields_json"`
	FieldListJSON string `json:"field_list_json"`
	Page          int64  `json:"page"`
	DtUpdate      int64  `json:"dt_update"`
	DtLastSeen    int64  `json:"dt_last_seen"`
	FlagOnline    string `json:"flag_online"`
	Provider      string `json:"provider"`
	Remark        string `json:"remark"`
}

// futureListSourceRows 把落库行转成 jsonb 入参（数值统一转文本，交给 SQL 侧的 NUMERIC 定点转换）。
func futureListSourceRows(rows []FutureListRow) []futureListSourceRow {
	src := make([]futureListSourceRow, 0, len(rows))
	for _, row := range rows {
		src = append(src, futureListSourceRow{
			StockID:       strconv.FormatInt(row.StockID, 10),
			USC:           futureListValue(row.USC),
			FuturesType:   row.FuturesType,
			MarketType:    futureListValue(row.MarketType),
			MarketLabel:   futureListValue(row.MarketLabel),
			ContractCode:  futureListValue(row.ContractCode),
			Name:          futureListValue(row.Name),
			PriceNominal:  futureListNumberValue(row.PriceNominal),
			ChangeRatio:   futureListNumberValue(row.ChangeRatio),
			PriceDirect:   futureListValue(row.PriceDirect),
			FieldsJSON:    row.FieldsJSON,
			FieldListJSON: row.FieldListJSON,
			Page:          futureListInt64Value(row.Page),
			DtUpdate:      futureListInt64Value(row.DtUpdate),
			DtLastSeen:    futureListInt64Value(row.DtLastSeen),
			FlagOnline:    row.FlagOnline,
			Provider:      row.Provider,
			Remark:        futureListValue(row.Remark),
		})
	}
	return src
}

// futureListValue 解引用可空文本（nil → 空串）。
func futureListValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// futureListNumberValue 解引用可空数值（nil → 空串，SQL 侧转 NULL）。
func futureListNumberValue(value *json.Number) string {
	if value == nil {
		return ""
	}
	return value.String()
}

// futureListInt64Value 解引用可空整数（nil → 0，调用方保证 dt/page 必有值）。
func futureListInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// 编译期断言：*Store 同时满足期货采集的配置读写与落库接口。
var (
	_ FutureListConfigStore = (*Store)(nil)
	_ FutureListSink        = (*Store)(nil)
)
