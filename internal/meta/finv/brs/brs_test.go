package brs

import "testing"

func TestCompressSpec(t *testing.T) {
	cases := map[string]string{
		"none": "none",
		"gzip": "gzip:6",
		"zstd": "zstd:3",
		"":     "zstd:3",
	}
	for input, want := range cases {
		if got := compressSpec(input); got != want {
			t.Fatalf("compressSpec(%q)=%q，期望 %q", input, got, want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	if got := humanSize(1024); got != "1.0 KiB" {
		t.Fatalf("humanSize(1024)=%q", got)
	}
}
