package kline

import "testing"

func strPtr(value string) *string { return &value }

func TestRatioText(t *testing.T) {
	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil 保持 null", nil, nil},
		{"正数两位", strPtr("2"), strPtr("2.00")},
		{"负数半值进位", strPtr("-0.005"), strPtr("-0.01")},
		{"负数不进位", strPtr("-0.004"), strPtr("-0.00")},
		{"负数零值保留负号", strPtr("-0.000000"), strPtr("-0.00")},
		{"正数半值进位", strPtr("1.005"), strPtr("1.01")},
		{"正数不进位", strPtr("1.004"), strPtr("1.00")},
		{"去尾零", strPtr("11.480000"), strPtr("11.48")},
		{"科学计数法", strPtr("1E+1"), strPtr("10.00")},
		{"非法值返回 null", strPtr("abc"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RatioText(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("期望 nil，实际 %q", *got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("期望 %q，实际 %v", *tc.want, got)
			}
		})
	}
}

func TestCellText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"4507048.00000000", "4507048"},
		{"11.480000", "11.48"},
		{"-0.000000", "0"},
		{"0.000000", "0"},
		{"392600", "392600"},
		{"-12.340000", "-12.34"},
		{"1E+3", "1000"},
	}
	for _, tc := range cases {
		got := CellText(strPtr(tc.in))
		if got == nil || *got != tc.want {
			t.Fatalf("CellText(%q) = %v，期望 %q", tc.in, got, tc.want)
		}
	}
	if CellText(nil) != nil {
		t.Fatal("nil 应返回 nil")
	}
}

func TestPageNumber(t *testing.T) {
	cases := []struct {
		offset, limit, want int
	}{
		{0, 50, 1},
		{50, 50, 2},
		{120, 50, 3},
		{0, 0, 1},
	}
	for _, tc := range cases {
		if got := PageNumber(tc.offset, tc.limit); got != tc.want {
			t.Fatalf("PageNumber(%d,%d)=%d，期望 %d", tc.offset, tc.limit, got, tc.want)
		}
	}
}
