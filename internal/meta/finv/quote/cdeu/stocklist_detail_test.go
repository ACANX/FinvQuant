package cdeu

import (
	"strings"
	"testing"
)

// TestStockListFormatDetail 校验逐页明细日志的排版规则（需求 10984 / 10988）：
//   - 每行 10 个证券；
//   - 每格形如 `[I] 02400` / `[U] 02238`，只含代码、不含名称；
//   - 每格固定 stockListDetailCellWidth（11）显示列，多行严格对齐。
func TestStockListFormatDetail(t *testing.T) {
	rows := []StockListSavedRow{
		{StockID: 1, StockCode: "02400", Name: "XD INC", Inserted: true},
		{StockID: 2, StockCode: "02238", Name: "GAC GROUP", Inserted: true},
		{StockID: 3, StockCode: "06680", Name: "JLMAG", Inserted: true},
		{StockID: 4, StockCode: "02676", Name: "NOVOSENSE", Inserted: true},
		{StockID: 5, StockCode: "09899", Name: "NETEASE MUSI", Inserted: true},
		{StockID: 6, StockCode: "01440", Name: "STAR SHINE H", Inserted: true},
		{StockID: 7, StockCode: "03317", Name: "XUNCE"},
		{StockID: 8, StockCode: "01456", Name: "GLMS SEC"},
		{StockID: 9, StockCode: "01686", Name: "SUNEVISION"},
		{StockID: 10, StockCode: "00013", Name: "HUTCHMED"},
		{StockID: 11, StockCode: "00700", Name: "TENCENT"},
		{StockID: 12, StockCode: "09988", Name: "BABA-W"},
	}

	out := stockListFormatDetail(rows)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("明细行数 = %d，期望 2（12 条 ÷ 每行 10 个）:\n%s", len(lines), out)
	}

	first, second := lines[0], lines[1]
	if got := strings.Count(first, "[I]") + strings.Count(first, "[U]"); got != stockListDetailPerLine {
		t.Errorf("第一行格数 = %d，期望 %d:\n%s", got, stockListDetailPerLine, first)
	}
	if got := strings.Count(second, "[I]") + strings.Count(second, "[U]"); got != 2 {
		t.Errorf("第二行格数 = %d，期望 2:\n%s", got, second)
	}

	// 不含名称：任一 Name 片段都不应出现在输出里
	for _, row := range rows {
		if row.Name != "" && strings.Contains(out, row.Name) {
			t.Errorf("明细不应包含名称 %q:\n%s", row.Name, out)
		}
	}

	// 标记正确：前 6 条为 [I]，其余为 [U]
	if !strings.HasPrefix(first, "[I] 02400") {
		t.Errorf("首格格式应为 `[I] 02400`，实际:\n%s", first)
	}
	if strings.Count(out, "[I]") != 6 {
		t.Errorf("[I] 数量 = %d，期望 6:\n%s", strings.Count(out, "[I]"), out)
	}

	// 对齐：每行显示宽度应一致（2 格 × 11 与 10 格 × 11 + 9 个分隔空格）
	if w := stockListDisplayWidth(first); w != stockListDetailPerLine*stockListDetailCellWidth+(stockListDetailPerLine-1) {
		t.Errorf("第一行显示宽度 = %d，期望 %d:\n%s", w, stockListDetailPerLine*stockListDetailCellWidth+stockListDetailPerLine-1, first)
	}
	if w := stockListDisplayWidth(second); w != 2*stockListDetailCellWidth+1 {
		t.Errorf("第二行显示宽度 = %d，期望 %d:\n%s", w, 2*stockListDetailCellWidth+1, second)
	}
}

// TestStockListDetailCellWidth 校验格子宽度常量与代码缺失回退。
func TestStockListDetailCellWidth(t *testing.T) {
	cell := stockListDetailCell(StockListSavedRow{StockID: 148739012639708, StockCode: ""})
	if !strings.HasPrefix(cell, "[U] ") {
		t.Errorf("代码缺失时应带标记，实际 %q", cell)
	}
	if w := stockListDisplayWidth(cell); w != stockListDetailCellWidth {
		t.Errorf("格子显示宽度 = %d，期望 %d（%q）", w, stockListDetailCellWidth, cell)
	}
	if stockListDetailCellWidth != 11 {
		t.Errorf("格子宽度常量 = %d，期望 11（只放 `[I] 代码`）", stockListDetailCellWidth)
	}
}
