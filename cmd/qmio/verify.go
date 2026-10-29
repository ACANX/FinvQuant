package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/acanx/finvquant/internal/meta/finv/quote/mvsv"
)

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	skipChecksum := fs.Bool("skip-checksum", false, "跳过 .sha256 校验")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: qmio verify FILE")
		return 2
	}
	path := fs.Arg(0)

	if !*skipChecksum {
		if err := verifyChecksum(path); err != nil {
			fmt.Fprintf(os.Stderr, "校验失败: %v\n", err)
			return 1
		}
	}
	reader, closer, err := openMaybeGzip(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		return 1
	}
	defer closer.Close()

	header, err := mvsv.ReadAll(reader, path, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
		return 1
	}
	fmt.Printf("校验通过：%s\n", path)
	fmt.Printf("  Format : %s\n", header.Format)
	fmt.Printf("  Count  : %d\n", header.Count)
	if len(header.Columns) > 0 {
		fmt.Printf("  Field  : %s\n", header.Values["Field"])
	}
	return 0
}
