package mvsv

import "testing"

// TestParseChangeFields 校验 cp/cr 列被解析（历史实现会忽略），供导入服务「有值则不补全」使用。
func TestParseChangeFields(t *testing.T) {
	result, err := Parse([]byte(sampleMvsv), "TEST.mvsv")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(result.Rows) == 0 {
		t.Fatal("无解析结果")
	}
	first := result.Rows[0]
	if first.ChangePrice == nil || *first.ChangePrice != "0.001" {
		t.Fatalf("change_price 期望 0.001，实际 %v", first.ChangePrice)
	}
	if first.ChangeRatio == nil || *first.ChangeRatio != "0.000143" {
		t.Fatalf("change_ratio 期望 0.000143，实际 %v", first.ChangeRatio)
	}
}
