package cdeu

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestLiveStockListPageBase 实测 moomoo get-stock-list 的 page 起始值（0 还是 1）。
//
// 默认跳过（同其它 live 测试）。在可访问 moomoo 的环境执行：
//
//	FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListPageBase -v ./internal/meta/finv/quote/cdeu/
//
// 2026-11 实测结论（MetaExt v0.1.2，marketType=1、pageSize=5）：
//
//	page=0 → 00700 腾讯控股（pagination.page=0、total≈3337），即第一页
//	page=1 → 02328 中国财险（第二页，跳过市值前 pageSize 名）
//
// 即本接口 page 从 0 开始，采集侧 StockListDefaultPageStart=0 正确。
//
// 判定方法：分别请求 page=0/1/2，比较首条 stockId。
//   - page=0 与 page=1 首条不同：服务端起算为 0（当前结论）
//   - page=0 与 page=1 首条相同：服务端把 0 与 1 当同一页
func TestLiveStockListPageBase(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过真实接口测试")
	}
	client := NewStockListFetcher()

	heads := make(map[string]int64, 3)
	for _, page := range []string{"0", "1", "2"} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, err := client.GetStockList(ctx, quoteapiv2.GetStockListRequest{
			MarketType: "1",
			PlateType:  "1",
			RankType:   "3",
			Page:       page,
			PageSize:   "5",
		})
		cancel()
		if err != nil {
			t.Fatalf("page=%s 请求失败: %v", page, err)
		}
		head := ""
		if len(result.Items) > 0 {
			heads[page] = result.Items[0].StockID
			head = fmt.Sprintf("%d %s %s", result.Items[0].StockID, result.Items[0].Name, result.Items[0].MarketValue)
		}
		t.Logf("page=%s 返回 %d 条 | pagination.page=%d pageSize=%d pageCount=%d total=%d | 首条=%s",
			page, len(result.Items), result.Pagination.Page, result.Pagination.PageSize,
			result.Pagination.PageCount, result.Pagination.Total, head)
		if len(result.Items) > 1 {
			t.Logf("  第二条=%d %s", result.Items[1].StockID, result.Items[1].Name)
		}
	}
	if len(heads) == 3 && heads["0"] == heads["1"] {
		t.Logf("page=0 与 page=1 首条相同（%d），服务端把两者当同一页", heads["0"])
	}
}

// TestLiveStockListChineseDefault 验证 MetaExt v0.1.2 的默认请求口径即返回中文名称与中文单位。
//
// 默认跳过（同其它 live 测试）。执行：
//
//	FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListChineseDefault -v ./internal/meta/finv/quote/cdeu/
//
// 判定口径：
//   - 名称含中日韩统一表意文字（如「腾讯控股」），而非英文（如 TENCENT）；
//   - 总市值带中文单位「万亿/亿/万」，而非英文单位 T/B/M/K。
//
// 采集侧不再注入 referer / accept-language / 自定义 transport，因此本用例通过即说明
// 上游默认值已足够（需求 10992 的本地临时规避可以退场）。
func TestLiveStockListChineseDefault(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过真实接口测试")
	}
	client := NewStockListFetcher()
	markets := []struct {
		marketType string
		label      string
	}{
		{"1", "港股"},
		{"2", "美股"},
		{"4", "沪深"},
	}
	for _, m := range markets {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, err := client.GetStockList(ctx, quoteapiv2.GetStockListRequest{
			MarketType: m.marketType,
			PlateType:  "1",
			RankType:   "3",
			Page:       "0",
			PageSize:   "5",
		})
		cancel()
		if err != nil {
			t.Errorf("%s（marketType=%s）请求失败: %v", m.label, m.marketType, err)
			continue
		}
		if len(result.Items) == 0 {
			t.Errorf("%s（marketType=%s）返回 0 条", m.label, m.marketType)
			continue
		}
		head := result.Items[0]
		t.Logf("%s 首条：%d %s | 市值 %s | 页码 %d | 总数 %d",
			m.label, head.StockID, head.Name, head.MarketValue, result.Pagination.Page, result.Pagination.Total)
		if !stockListHasCJK(head.Name) {
			t.Errorf("%s 名称未返回中文：%q（英文降级口径）", m.label, head.Name)
		}
		if !stockListHasChineseUnit(head.MarketValue) {
			t.Errorf("%s 市值未使用中文单位：%q", m.label, head.MarketValue)
		}
	}
}

// stockListHasCJK 判断文本是否含中日韩统一表意文字。
func stockListHasCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// stockListHasChineseUnit 判断文本是否含中文数量单位（万亿/亿/万）。
func stockListHasChineseUnit(text string) bool {
	return strings.ContainsAny(text, "万亿")
}
