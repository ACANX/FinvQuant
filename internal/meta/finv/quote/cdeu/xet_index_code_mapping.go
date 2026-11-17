package cdeu

import (
	_ "embed"
	"bufio"
	"fmt"
	"strings"
	"sync"
)

// 本文件是「沪深京指数」采集的代码映射层（需求 11294）：
//   - 把中信 XetApp `GetQuoteSnapshotCNAIndex` 返回的**站点内部码**（1A0001 / 2A01 / H50001…）
//     还原成交易所或官方代码，并给出编码域与市场归属；
//   - 映射数据来自上游映射表（`XYTApp Docs/OpenAPI/Citics/XetApp/IndexCodeMapping.md`，
//     抓取 2026-11-18，四池 879 只 + 附录 360 只 = 1239 只），以 TSV 嵌入；
//   - 纯函数，便于单测；采集器只负责取数与落库。
//
// 为什么需要本地映射表：上游 Go 包（MetaExt v0.1.3）没有导出 `ResolveIndexCode`，
// 且内部码不能直接当交易所代码（`1B0861` 这类与深市个股撞号，站点会优先解析成个股）。

// 沪深京指数 Tab 的品种符号（前端 `instrument_symbol`）。
const (
	// XetIndexSymbol 沪深京指数（前端 Tab cnindex）。
	XetIndexSymbol = "cnindex"
	// XetIndexProvider 写入 cdeu.finv_quote_secu_realtime.provider 的固定值。
	XetIndexProvider = "ZXET"
	// XetIndexInstrumentType 写入 instrument_type 的值（需求 11294：与 moomoo ETF 的 4 同口径）。
	XetIndexInstrumentType = "4"
	// XetIndexMarketType 写入 market_type 的值（CN A 股，与 BDFS 同口径）。
	XetIndexMarketType = "4"
)

// xetIndexMappingTSV 上游映射表（go:embed，1239 行，列：接口代码 / 名称 / 交易所或官方代码 / 编码域 / 成分市场 / 市场号 / 校验）。
//
//go:embed xet_index_code_mapping.tsv
var xetIndexMappingTSV string

// XetIndexCodeEntry 一条指数代码映射。
type XetIndexCodeEntry struct {
	// Code 接口内部码（Row.Code，如 1A0001）。
	Code string
	// Name 指数名称。
	Name string
	// SecuCode 交易所代码或官方编码（如 000001 / 399001 / 899050 / 931743 / CESCPD）。
	SecuCode string
	// Domain 编码域：SH / SZ / BJ / NEEQ / CSI / CNI / CES / MSCI。
	Domain string
	// ComponentMarket 成分市场（上游映射表的「成分市场」列，如 沪 / 沪+深 / 港）。
	ComponentMarket string
	// MarketNo 站点市场号（字段 223，如 1100 / 1200 / 1400 / 7100）。
	MarketNo string
	// Check 上游校验方式（回环+价 / 价 / 回环 / 码），仅用于排障。
	Check string
}

var (
	xetIndexMappingOnce sync.Once
	xetIndexMappingByID map[string]XetIndexCodeEntry
	xetIndexMappingErr  error
)

// XetIndexCodeMapping 返回接口内部码 → 映射条目（进程内只解析一次）。
func XetIndexCodeMapping() (map[string]XetIndexCodeEntry, error) {
	xetIndexMappingOnce.Do(func() {
		xetIndexMappingByID, xetIndexMappingErr = parseXetIndexMapping(xetIndexMappingTSV)
	})
	return xetIndexMappingByID, xetIndexMappingErr
}

// parseXetIndexMapping 解析嵌入的 TSV（`#` 开头为注释行）。
func parseXetIndexMapping(text string) (map[string]XetIndexCodeEntry, error) {
	out := make(map[string]XetIndexCodeEntry, 1300)
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return nil, fmt.Errorf("xet 指数映射第 %d 行列数不符：期望 7，实际 %d", lineNo, len(fields))
		}
		entry := XetIndexCodeEntry{
			Code:            strings.TrimSpace(fields[0]),
			Name:            strings.TrimSpace(fields[1]),
			SecuCode:        strings.TrimSpace(fields[2]),
			Domain:          strings.ToUpper(strings.TrimSpace(fields[3])),
			ComponentMarket: strings.TrimSpace(fields[4]),
			MarketNo:        strings.TrimSpace(fields[5]),
			Check:           strings.TrimSpace(fields[6]),
		}
		if entry.Code == "" || entry.SecuCode == "" || entry.Domain == "" {
			return nil, fmt.Errorf("xet 指数映射第 %d 行关键列为空", lineNo)
		}
		if _, ok := out[entry.Code]; ok {
			return nil, fmt.Errorf("xet 指数映射第 %d 行内部码重复：%s", lineNo, entry.Code)
		}
		out[entry.Code] = entry
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 xet 指数映射失败: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("xet 指数映射为空")
	}
	return out, nil
}

// XetIndexMarketLabel 把站点市场号（字段 223，映射表「市场号」列）映射为 market_label。
//
// 规则（需求 11294，用户 2026-11-18 确认）：以**市场号**为准，不再看成分市场——
//
//	1100 → SH（沪市，含中证 93xxxx / H* / 95xxxx 与跨市场的沪深300、中证2000 等）
//	1200 → SZ（深市，含国证 98xxxx、新三板 899xxx）
//	1400 → BJ（北交所，899050 / 899601）
//	其它（7100 国证非标等，无对应交易所）→ CN
 //
// 注：成分市场（沪+深+京 等）只写进 remark，不参与归属判定。
func XetIndexMarketLabel(marketNo string) string {
	switch strings.TrimSpace(marketNo) {
	case "1100":
		return "SH"
	case "1200":
		return "SZ"
	case "1400":
		return "BJ"
	default:
		return "CN"
	}
}

// XetIndexUSC 按编码域构造 usc（不查 `finv_security`，规则即口径）。
//
// 规则（需求 11294，用户 2026-11-18 确认）：
//
//	SH   → SH + 4 位（000001 → SH0001，V300074 的 6 位口径）
//	BJ   → 899050 固定 BSE50（用户指定），其它原样（899601）
//	其它（SZ / NEEQ / CSI / CNI / CES / MSCI）→ 官方码原样
//	      （399001 / 899001 / 931743 / 980028 / CES300 / 716567）
//
// 为什么中证/国证/新三板不加域前缀：库内 `public.finv_security` 已用裸码登记
// （如 931743），`93/95/98/899` 段不属于任何交易所代码段，不会与个股撞号；
// 加前缀反而让同一指数出现两套 usc（需求 11294 联调时发现并纠正）。
func XetIndexUSC(entry XetIndexCodeEntry) string {
	code := entry.SecuCode
	switch entry.Domain {
	case "SH":
		if len(code) == 6 {
			code = code[2:] // 000001 → 0001
		}
		return "SH" + code
	case "BJ":
		if code == "899050" {
			return "BSE50"
		}
		return code
	default:
		return code
	}
}

// xetIndexCurrency 按名称后缀推断币种（需求 11294）：含 HKD → HKD、含 USD → USD，其余 CNY。
func xetIndexCurrency(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.Contains(upper, "HKD"):
		return "HKD"
	case strings.Contains(upper, "USD"):
		return "USD"
	default:
		return "CNY"
	}
}
