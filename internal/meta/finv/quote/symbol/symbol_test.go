package symbol

import "testing"

// TestNormalize 校验 ftmm_symbol 入库形态：英文字母大写、非字母原样、去前后空白。
func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"US.AAPL":                          "US.AAPL",
		"us.aapl":                          "US.AAPL",
		"  Gl.quote-forex-Asia-Pacific  ":  "GL.QUOTE-FOREX-ASIA-PACIFIC",
		"AU.quote-etf-au-index-etfs-s&amp": "AU.QUOTE-ETF-AU-INDEX-ETFS-S&AMP",
		"US.SBT,OLD":                       "US.SBT,OLD",
		"US.ASAI*":                         "US.ASAI*",
		"CC.XRP":                           "CC.XRP",
		"":                                 "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestNormalizeAll 校验批量归一原地生效。
func TestNormalizeAll(t *testing.T) {
	got := NormalizeAll([]string{"us.aapl", "gl.quote", ""})
	if got[0] != "US.AAPL" || got[1] != "GL.QUOTE" || got[2] != "" {
		t.Fatalf("NormalizeAll = %v", got)
	}
}
