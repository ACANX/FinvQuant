// Package logging 配置服务端日志输出。
//
// Native（systemd）方式通过 FINV_LOG_DIR 把标准库日志与 gin 访问日志
// 同时写入日志文件（便于长期留存/轮转）；容器方式留空，仅输出到
// stdout/stderr 由容器运行时收集。
package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultFile 默认日志文件名。
const DefaultFile = "finvquant.log"

// Setup 把标准库日志与 gin 访问日志同时写入标准输出和 dir/name。
//
// dir 为空时不做任何事（返回 nil, nil），保持仅标准输出。
// 目录不存在会自动创建；文件以追加方式打开（0640）。
func Setup(dir, name string) (io.Closer, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	if strings.TrimSpace(name) == "" {
		name = DefaultFile
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	gin.DefaultWriter = io.MultiWriter(os.Stdout, file)
	gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, file)
	return file, nil
}
