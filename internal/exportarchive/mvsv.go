package exportarchive

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RowSource 数据行来源：逐行产出 12 列文本（空值用空串），由 WriteMVsv 负责拼装与落盘。
type RowSource func(yield func(cols [12]string) error) error

// WriteResult 文件写出结果。
type WriteResult struct {
	Path     string // 最终文件绝对路径
	Size     int64  // 文件字节数
	RowCount int64  // 实际写入的数据行数（不含 16 行文件头与 1 行空行）
	Checksum string // 文件内容 SHA256（小写十六进制）；gzip 时为压缩后内容的 SHA256
}

// WriteMVsv 流式写出 MVsv 文件。
//
// 顺序与契约（方案稿 B-01~B-11 / 4.3）：
//  1. 16 行 `# ` 文件头（含 Count = expectedRows）；
//  2. 1 行空行（不得省略）；
//  3. 数据行：12 列以 `|` 分隔，空值为空串（即出现 `||`）；
//  4. 先写同目录临时文件，校验实际行数与 expectedRows 一致后原子 rename；
//  5. 返回文件大小与 SHA256，供 dedup.finv_dedup_export_file 登记与上传校验。
//
// expectedRows 必须由调用方预统计（COUNT(*)），传负数视为 0。
func WriteMVsv(dest string, meta FileMeta, compression string, expectedRows int64, source RowSource) (WriteResult, error) {
	if source == nil {
		return WriteResult{}, errors.New("导出数据源为空")
	}
	if expectedRows < 0 {
		expectedRows = 0
	}
	if compression != CompressionGzip {
		compression = CompressionNone
	}

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("创建导出目录失败: %w", err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(dest)+".tmp-*")
	if err != nil {
		return WriteResult{}, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hash := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(file, hash)}
	buffered := bufio.NewWriterSize(counter, 256<<10)

	var sink io.Writer = buffered
	var gzipWriter *gzip.Writer
	if compression == CompressionGzip {
		gzipWriter = gzip.NewWriter(buffered)
		sink = gzipWriter
	}

	// 1) 16 行文件头
	for _, line := range meta.headerLines(expectedRows) {
		if _, err := io.WriteString(sink, line+"\n"); err != nil {
			return WriteResult{}, fmt.Errorf("写文件头失败: %w", err)
		}
	}
	// 2) 文件头与数据区之间的空行（B-05，不得省略）
	if _, err := io.WriteString(sink, "\n"); err != nil {
		return WriteResult{}, fmt.Errorf("写空行失败: %w", err)
	}

	// 3) 数据行
	var written int64
	writeErr := source(func(cols [12]string) error {
		if _, err := io.WriteString(sink, strings.Join(cols[:], MvsvDelimiter)); err != nil {
			return err
		}
		if _, err := io.WriteString(sink, "\n"); err != nil {
			return err
		}
		written++
		return nil
	})

	// 4) 收尾：先关 gzip / 刷缓冲 / fsync，保证落盘后再校验并 rename
	var closeErr error
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			closeErr = fmt.Errorf("关闭 gzip 写入器失败: %w", err)
		}
	}
	if err := buffered.Flush(); err != nil && closeErr == nil {
		closeErr = fmt.Errorf("刷新文件缓冲失败: %w", err)
	}
	if err := file.Sync(); err != nil && closeErr == nil {
		closeErr = fmt.Errorf("同步文件失败: %w", err)
	}
	if err := file.Close(); err != nil && closeErr == nil {
		closeErr = fmt.Errorf("关闭文件失败: %w", err)
	}
	if writeErr != nil {
		return WriteResult{}, fmt.Errorf("写数据行失败: %w", writeErr)
	}
	if closeErr != nil {
		return WriteResult{}, closeErr
	}

	if written != expectedRows {
		return WriteResult{}, fmt.Errorf("数据行数不一致：实际 %d，Count 预统计 %d", written, expectedRows)
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		return WriteResult{}, fmt.Errorf("原子重命名失败: %w", err)
	}
	committed = true

	return WriteResult{
		Path:     dest,
		Size:     counter.n,
		RowCount: written,
		Checksum: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// WriteFileAtomic 原子写小文件（Manifest JSON），返回大小与 SHA256。
func WriteFileAtomic(dest string, data []byte) (WriteResult, error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("创建目录失败: %w", err)
	}
	file, err := os.CreateTemp(dir, filepath.Base(dest)+".tmp-*")
	if err != nil {
		return WriteResult{}, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	if _, err := writer.Write(data); err != nil {
		return WriteResult{}, fmt.Errorf("写文件失败: %w", err)
	}
	if err := file.Sync(); err != nil {
		return WriteResult{}, fmt.Errorf("同步文件失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return WriteResult{}, fmt.Errorf("关闭文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return WriteResult{}, fmt.Errorf("原子重命名失败: %w", err)
	}
	committed = true
	return WriteResult{
		Path:     dest,
		Size:     int64(len(data)),
		RowCount: 0,
		Checksum: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// FileSHA256 计算文件内容 SHA256（小写十六进制）。
func FileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = file.Close() }()
	return sha256Of(file)
}

// sha256Of 计算 reader 全部内容的 SHA256 与字节数。
func sha256Of(reader io.Reader) (string, int64, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, reader)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// VerifyMVsv 校验 MVsv 文件结构（方案稿 5.1「文件结构」一行）：
//   - 前 16 行均以 `# ` 开头；
//   - 第 17 行为空行；
//   - `# Field` 行等于 ts|da|ti|o|h|l|c|v|t|cp|cr|lc；
//   - `# Count` 与实际数据行数一致（expectedRows >= 0 时同时与期望行数比较）。
func VerifyMVsv(path string, compression string, expectedRows int64) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var reader io.Reader = file
	if compression == CompressionGzip {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("解压失败: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)

	var headerCount int
	var countValue int64 = -1
	var fieldOK bool
	for line := 1; line <= MvsvHeaderLines; line++ {
		if !scanner.Scan() {
			return fmt.Errorf("文件头不足 %d 行（实际 %d 行）", MvsvHeaderLines, headerCount)
		}
		text := scanner.Text()
		if !strings.HasPrefix(text, "# ") {
			return fmt.Errorf("第 %d 行不是以 `# ` 开头的文件头: %q", line, text)
		}
		headerCount++
		switch {
		case strings.HasPrefix(text, "# Field : "):
			value := strings.Trim(strings.TrimPrefix(text, "# Field : "), `"`)
			if value != MvsvFieldLine {
				return fmt.Errorf("# Field 行不符：%q", value)
			}
			fieldOK = true
		case strings.HasPrefix(text, "# Count : "):
			parsed, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(text, "# Count : ")), 10, 64)
			if err != nil {
				return fmt.Errorf("# Count 行解析失败: %w", err)
			}
			countValue = parsed
		}
	}
	if !fieldOK {
		return errors.New("缺少 # Field 文件头")
	}

	if !scanner.Scan() {
		return errors.New("文件头之后缺少空行")
	}
	if strings.TrimSpace(scanner.Text()) != "" {
		return fmt.Errorf("第 17 行应为空行，实际为 %q", scanner.Text())
	}

	var rows int64
	for scanner.Scan() {
		rows++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if countValue >= 0 && countValue != rows {
		return fmt.Errorf("# Count 与实际数据行数不一致（%d != %d）", countValue, rows)
	}
	if expectedRows >= 0 && rows != expectedRows {
		return fmt.Errorf("数据行数与登记行数不一致（%d != %d）", rows, expectedRows)
	}
	return nil
}

// countingWriter 统计写入字节数。
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
