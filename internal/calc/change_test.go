package calc

import "testing"

func ptr(value string) *string { return &value }

func TestChangeRule(t *testing.T) {
	cases := []struct {
		name      string
		close     *string
		lastClose *string
		open      *string
		wantPrice *string
		wantRatio *string
	}{
		{
			name:      "优先用 last_close",
			close:     ptr("11"),
			lastClose: ptr("10"),
			open:      ptr("9"),
			wantPrice: ptr("1.000000"),
			wantRatio: ptr("10.000000"),
		},
		{
			name:      "last_close 缺失时用 open",
			close:     ptr("21"),
			lastClose: nil,
			open:      ptr("20"),
			wantPrice: ptr("1.000000"),
			wantRatio: ptr("5.000000"),
		},
		{
			name:      "last_close 为 0 时回退 open",
			close:     ptr("21"),
			lastClose: ptr("0"),
			open:      ptr("20"),
			wantPrice: ptr("1.000000"),
			wantRatio: ptr("5.000000"),
		},
		{
			name:      "两者都缺失则不计算",
			close:     ptr("21"),
			lastClose: nil,
			open:      nil,
			wantPrice: nil,
			wantRatio: nil,
		},
		{
			name:      "两者都为 0 则不计算",
			close:     ptr("21"),
			lastClose: ptr("0"),
			open:      ptr("0"),
			wantPrice: nil,
			wantRatio: nil,
		},
		{
			name:      "close 缺失则不计算",
			close:     nil,
			lastClose: ptr("10"),
			open:      ptr("10"),
			wantPrice: nil,
			wantRatio: nil,
		},
		{
			name:      "负涨跌",
			close:     ptr("9.5"),
			lastClose: ptr("10"),
			open:      nil,
			wantPrice: ptr("-0.500000"),
			wantRatio: ptr("-5.000000"),
		},
		{
			name:      "高精度保留 6 位",
			close:     ptr("11.48"),
			lastClose: ptr("11.5"),
			open:      nil,
			wantPrice: ptr("-0.020000"),
			wantRatio: ptr("-0.173913"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, ratio := Change(tc.close, tc.lastClose, tc.open)
			if tc.wantPrice == nil {
				if price != nil || ratio != nil {
					t.Fatalf("期望不计算，实际 price=%v ratio=%v", price, ratio)
				}
				return
			}
			if price == nil || *price != *tc.wantPrice {
				t.Fatalf("change_price=%v，期望 %s", price, *tc.wantPrice)
			}
			if ratio == nil || *ratio != *tc.wantRatio {
				t.Fatalf("change_ratio=%v，期望 %s", ratio, *tc.wantRatio)
			}
		})
	}
}
