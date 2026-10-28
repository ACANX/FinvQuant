package mvsv

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func intPtr(value int) *int       { return &value }
func strPtr(value string) *string { return &value }

func sampleV2Row() Row {
	return Row{
		Usc: "GCMain", Ts: 1700000000,
		Date: intPtr(20231114), Time: intPtr(93000),
		Open: strPtr("3.51"), High: strPtr("3.62"), Low: strPtr("3.50"), Close: strPtr("3.60"),
		LastClose: strPtr("3.50"), Volume: strPtr("12.345678"), Turnover: strPtr("3560.00"),
		ChangePrice: strPtr("0.10"), ChangeRatio: strPtr("2.857143"), Paocd: strPtr("3.56"),
		Provider:  strPtr("FT"),
		GmtCreate: strPtr("2026-10-28T12:00:00+08:00"),
		GmtUpdate: strPtr("2026-10-28T12:00:00+08:00"),
	}
}

// TestWriteV2RoundTripGmt 校验含 gmt 的 19 列往返一致。
func TestWriteV2RoundTripGmt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kline.mvsv.gz")
	want := sampleV2Row()
	result, err := WriteV2(path, V2Options{Title: "test", IncludeGmt: true}, CompressionGzip, 1,
		func(yield func(Row) error) error { return yield(want) })
	if err != nil {
		t.Fatalf("WriteV2 失败: %v", err)
	}
	if result.RowCount != 1 || result.Checksum == "" {
		t.Fatalf("写出结果异常: %+v", result)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip 读取失败: %v", err)
	}
	defer gz.Close()

	var got Row
	header, err := ReadAll(gz, "kline.mvsv.gz", func(h *Header, row Row) error {
		if h.Format != FormatV2 {
			t.Fatalf("Format 期望 MVSV-2，实际 %s", h.Format)
		}
		got = row
		return nil
	})
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if header.Count != 1 || len(header.Columns) != 19 {
		t.Fatalf("头部异常: count=%d columns=%d", header.Count, len(header.Columns))
	}
	if got.Usc != want.Usc || got.Ts != want.Ts {
		t.Fatalf("键不一致: %+v", got)
	}
	if got.LastClose == nil || *got.LastClose != "3.50" {
		t.Fatalf("last_close 不一致: %v", got.LastClose)
	}
	if got.Volume == nil || *got.Volume != "12.345678" {
		t.Fatalf("volume 不一致: %v", got.Volume)
	}
	if got.GmtCreate == nil || *got.GmtCreate != "2026-10-28T12:00:00+08:00" {
		t.Fatalf("gmt_create 不一致: %v", got.GmtCreate)
	}
}

// TestWriteV2RoundTripNoGmt 校验 --no-gmt 的 17 列往返一致。
func TestWriteV2RoundTripNoGmt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kline_nogmt.mvsv")
	want := sampleV2Row()
	want.GmtCreate, want.GmtUpdate = nil, nil
	result, err := WriteV2(path, V2Options{Title: "test"}, CompressionNone, 1,
		func(yield func(Row) error) error { return yield(want) })
	if err != nil {
		t.Fatalf("WriteV2 失败: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("行数异常: %+v", result)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()
	header, err := ReadAll(file, "kline_nogmt.mvsv", func(h *Header, row Row) error {
		if row.GmtCreate != nil || row.GmtUpdate != nil {
			t.Fatal("不应含 gmt 列")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if len(header.Columns) != 17 {
		t.Fatalf("列数期望 17，实际 %d", len(header.Columns))
	}
}
