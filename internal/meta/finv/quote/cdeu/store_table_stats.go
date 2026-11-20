package cdeu

import (
	"context"
	"fmt"
)

// 表规模统计（需求 11285）：给「行情采集 → 状态总览」页提供关键表的行数指标。
//
// 口径与边界：
//   - 统计的是**当前行数**（`count(*)`），不做增量、不落库、不缓存；
//   - 表名只来自本文件的常量白名单，**不接受请求参数拼表名**；
//   - 表不存在时返回 `exists=false`（rows=0），不报错——状态总览页要能在部分表缺失的环境照常渲染。

// TableRowCount 单表行数统计结果。
type TableRowCount struct {
	// Table 表名（schema 全限定）。
	Table string `json:"table"`
	// Rows 当前行数（表不存在时恒为 0）。
	Rows int64 `json:"rows"`
	// Exists 表是否存在（false = 该环境还没有这张表）。
	Exists bool `json:"exists"`
}

// tableStatsTargets 状态总览统计的表（顺序即返回顺序）。
//
// 前两张是采集链路的「一行一证券」主表，行数应基本相当，差值可判断覆盖缺口：
//   - cdeu.finv_quote_secu_indicator：证券指标表（一行一证券，132 列）；
//   - futu.finv_futu_quote_collect：富途采集参数镜像表（一行一证券）。
//
// 其余为字典/映射表与三张行情快照表（需求 11341），规模对照可快速判断各链路是否在写：
//   - futu.finv_futu_secu_key：采集键表（含 status，DONE 后采集端跳过）；
//   - public.finv_security：证券主数据（录入/补全后的目标表）；
//   - futu.finv_futu_usc_map：usc 映射表（ACTIVE 行数≈可用证券数）；
//   - cdeu.finv_quote_secu_realtime：证券行情快照（股票/ETF/指数/板块/外汇等）；
//   - cdeu.finv_quote_secu_realtime_crypto：加密货币行情快照；
//   - cdeu.finv_quote_secu_realtime_future：期货行情快照。
var tableStatsTargets = []string{
	"cdeu.finv_quote_secu_indicator",
	"futu.finv_futu_quote_collect",
	"futu.finv_futu_secu_key",
	"public.finv_security",
	"futu.finv_futu_usc_map",
	"cdeu.finv_quote_secu_realtime",
	"cdeu.finv_quote_secu_realtime_crypto",
	"cdeu.finv_quote_secu_realtime_future",
}

// tableStatsAllowed 白名单集合（防拼表名注入；表名常量硬编码，不来自请求）。
var tableStatsAllowed = func() map[string]struct{} {
	out := make(map[string]struct{}, len(tableStatsTargets))
	for _, table := range tableStatsTargets {
		out[table] = struct{}{}
	}
	return out
}()

// TableStatsTargets 返回统计白名单的副本（handler 用，避免调用方改动内部切片）。
func TableStatsTargets() []string {
	out := make([]string, len(tableStatsTargets))
	copy(out, tableStatsTargets)
	return out
}

// CountTableRows 统计给定表的当前行数（表名必须在白名单内）。
//
// 每张表两次往返（存在性 + 计数）：表数量固定为 8，且都在本地库，
// 不做批量拼 SQL 的优化以免引入表名拼接面。表不存在时返回 exists=false、rows=0。
func (s *Store) CountTableRows(ctx context.Context, tables []string) ([]TableRowCount, error) {
	if len(tables) == 0 {
		tables = TableStatsTargets()
	}
	out := make([]TableRowCount, 0, len(tables))
	for _, table := range tables {
		if _, ok := tableStatsAllowed[table]; !ok {
			return nil, fmt.Errorf("不支持统计的表：%s（仅限状态总览白名单）", table)
		}
		var exists bool
		if err := s.pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			return nil, fmt.Errorf("判断表是否存在失败（%s）: %w", table, err)
		}
		item := TableRowCount{Table: table, Exists: exists}
		if exists {
			// 表名来自白名单常量，不来自调用方输入。
			if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&item.Rows); err != nil {
				return nil, fmt.Errorf("统计表行数失败（%s）: %w", table, err)
			}
		}
		out = append(out, item)
	}
	return out, nil
}
