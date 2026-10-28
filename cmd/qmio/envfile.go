package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ensureEnvLoaded 在未显式提供 --pg-url 时，自动加载 finvquant.env。
//
// systemd 会通过 EnvironmentFile 注入配置，但手工执行 qmio 不会；
// 这里按以下顺序查找并加载（已存在的环境变量不覆盖）：
//  1. FINV_ENV_FILE
//  2. <qmio 所在目录>/finvquant.env        （qmio 与 env 同目录）
//  3. <qmio 所在目录>/../finvquant.env     （qmio 在 bin/ 下）
//  4. /data/App/FinvQuant/finvquant.env    （默认安装根目录）
func ensureEnvLoaded() {
	if os.Getenv("FINV_PG_PASSWORD") != "" || os.Getenv("FINV_PG_HOST") != "" {
		return
	}
	for _, path := range candidateEnvFiles() {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if err := loadEnvFile(path); err != nil {
			continue
		}
		return
	}
}

func candidateEnvFiles() []string {
	candidates := make([]string, 0, 4)
	if path := strings.TrimSpace(os.Getenv("FINV_ENV_FILE")); path != "" {
		candidates = append(candidates, path)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "finvquant.env"),
			filepath.Join(dir, "..", "finvquant.env"),
		)
	}
	candidates = append(candidates, "/data/App/FinvQuant/finvquant.env")
	return candidates
}

// loadEnvFile 解析 KEY=VALUE 格式的 env 文件（忽略注释/空行，去除简单引号）。
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
