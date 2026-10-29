// Package logging 配置服务端日志输出与轮转。
//
// Native（systemd）方式通过 FINV_LOG_DIR 把标准库日志与 gin 访问日志
// 同时写入日志文件；容器方式留空，仅输出到 stdout/stderr。
//
// 轮转规则（需求 10797）：
//   - 活动文件：<DIR>/FinvQuant.log，超过 FINV_LOG_MAX_MB（默认 10MB）自动切分；
//   - 切分文件：<DIR>/FinvQuant-yyyyMMdd-NN.log（NN 从 01 自增）；
//   - 跨日时把此前日期的切分文件按日打包为 <DIR>/FinvQuant-yyyyMMdd.tar.gz，
//     打包成功后删除对应的 .log 文件。
package logging

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultFile 默认活动日志文件名。
const DefaultFile = "FinvQuant.log"

// DefaultMaxBytes 默认单文件上限（10MB）。
const DefaultMaxBytes int64 = 10 << 20

// splitPattern 切分文件：FinvQuant-yyyyMMdd-NN.log。
var splitPattern = regexp.MustCompile(`^FinvQuant-(\d{8})-(\d{2})\.log$`)

// Options 日志写入配置。
type Options struct {
	Dir      string
	File     string
	MaxBytes int64
	// Now 便于测试注入时间；为空使用 time.Now。
	Now func() time.Time
}

// RotatingWriter 按大小切分、按日归档的日志 writer（并发安全）。
type RotatingWriter struct {
	dir      string
	base     string // 不含 .log 的文件名前缀，如 FinvQuant
	maxBytes int64
	now      func() time.Time

	mu   sync.Mutex
	file *os.File
	size int64
	day  string // 活动文件所属日期 yyyyMMdd
}

// NewWriter 创建日志 writer；目录不存在会自动创建。
func NewWriter(opts Options) (*RotatingWriter, error) {
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return nil, fmt.Errorf("日志目录不能为空")
	}
	name := strings.TrimSpace(opts.File)
	if name == "" {
		name = DefaultFile
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		base = strings.TrimSuffix(DefaultFile, ".log")
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	w := &RotatingWriter{
		dir:      dir,
		base:     base,
		maxBytes: maxBytes,
		now:      now,
	}
	if err := w.openLocked(); err != nil {
		return nil, err
	}
	// 启动时归档历史切分文件，避免服务停机跨日导致漏归档。
	w.mu.Lock()
	w.archiveBeforeLocked(w.day)
	w.mu.Unlock()
	return w, nil
}

// Write 写入日志；超过上限时切分，跨日时归档前一天。
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := w.now().Format("20060102")
	if w.day != today {
		if w.file != nil {
			_ = w.file.Close()
			w.file = nil
			if err := w.rotateLocked(w.day); err != nil {
				return 0, err
			}
		}
		w.archiveBeforeLocked(today)
		if err := w.openLocked(); err != nil {
			return 0, err
		}
	}

	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.file.Close(); err != nil {
			return 0, err
		}
		w.file = nil
		if err := w.rotateLocked(today); err != nil {
			return 0, err
		}
		if err := w.openLocked(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close 关闭活动文件。
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// ActivePath 当前活动文件路径。
func (w *RotatingWriter) ActivePath() string {
	return filepath.Join(w.dir, w.base+".log")
}

// openLocked 打开（或新建）活动文件并记录大小/日期。
func (w *RotatingWriter) openLocked() error {
	path := w.ActivePath()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	w.day = w.now().Format("20060102")
	return nil
}

// rotateLocked 把活动文件重命名为 <base>-<day>-NN.log（NN 从 01 自增）。
func (w *RotatingWriter) rotateLocked(day string) error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	active := w.ActivePath()
	if _, err := os.Stat(active); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	seq := w.nextSeqLocked(day)
	target := filepath.Join(w.dir, fmt.Sprintf("%s-%s-%02d.log", w.base, day, seq))
	if err := os.Rename(active, target); err != nil {
		return err
	}
	w.size = 0
	return nil
}

// nextSeqLocked 返回该日期下一个可用序号（从 01 开始）。
func (w *RotatingWriter) nextSeqLocked(day string) int {
	max := 0
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return 1
	}
	for _, entry := range entries {
		m := splitPattern.FindStringSubmatch(entry.Name())
		if m == nil || m[1] != day {
			continue
		}
		var seq int
		_, _ = fmt.Sscanf(m[2], "%d", &seq)
		if seq > max {
			max = seq
		}
	}
	return max + 1
}

// archiveBeforeLocked 归档所有早于 today 的切分文件。
func (w *RotatingWriter) archiveBeforeLocked(today string) {
	days := map[string]struct{}{}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		m := splitPattern.FindStringSubmatch(entry.Name())
		if m == nil || m[1] >= today {
			continue
		}
		days[m[1]] = struct{}{}
	}
	ordered := make([]string, 0, len(days))
	for day := range days {
		ordered = append(ordered, day)
	}
	sort.Strings(ordered)
	for _, day := range ordered {
		_ = w.archiveDayLocked(day)
	}
}

// archiveDayLocked 把某日的切分文件打包为 <base>-<day>.tar.gz 并删除源文件。
func (w *RotatingWriter) archiveDayLocked(day string) error {
	var files []string
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		m := splitPattern.FindStringSubmatch(entry.Name())
		if m == nil || m[1] != day {
			continue
		}
		files = append(files, filepath.Join(w.dir, entry.Name()))
	}
	if len(files) == 0 {
		return nil
	}
	sort.Strings(files)

	target := w.archivePathLocked(day)
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	for _, path := range files {
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		header := &tar.Header{
			Name:    filepath.Base(path),
			Mode:    0o640,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(header); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			_ = out.Close()
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			_ = tw.Close()
			_ = gz.Close()
			_ = out.Close()
			return readErr
		}
		if _, err := tw.Write(data); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			_ = out.Close()
			return err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = out.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	for _, path := range files {
		_ = os.Remove(path)
	}
	return nil
}

// archivePathLocked 返回不冲突的归档路径（同日重复归档追加 -2/-3…）。
func (w *RotatingWriter) archivePathLocked(day string) string {
	base := filepath.Join(w.dir, fmt.Sprintf("%s-%s.tar.gz", w.base, day))
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	for i := 2; ; i++ {
		candidate := filepath.Join(w.dir, fmt.Sprintf("%s-%s-%d.tar.gz", w.base, day, i))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// Setup 把标准库日志与 gin 访问日志同时写入标准输出和轮转文件。
//
// dir 为空时不做任何事（返回 nil, nil），保持仅标准输出。
func Setup(dir, name string, maxBytes int64) (io.Closer, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	writer, err := NewWriter(Options{Dir: dir, File: name, MaxBytes: maxBytes})
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, writer))
	gin.DefaultWriter = io.MultiWriter(os.Stdout, writer)
	gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, writer)
	return writer, nil
}
