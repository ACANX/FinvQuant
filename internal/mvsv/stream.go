package mvsv

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReadAll 流式读取 MVSV 文件（V1/V2），逐行回调，避免大文件整体读入内存。
//
// 返回头部信息；文件实际行数与头部 Count 不一致时返回错误。
func ReadAll(r io.Reader, sourceName string, fn func(*Header, Row) error) (*Header, error) {
	reader := bufio.NewReaderSize(r, 64*1024)
	values := map[string]string{}
	lineNumber := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("%s: 读取头部失败: %w", sourceName, err)
		}
		lineNumber++
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			if len(values) == 0 {
				return nil, fmt.Errorf("%s: 头部为空", sourceName)
			}
			break
		}
		key, value, ok := parseHeaderLine(trimmed)
		if !ok {
			return nil, fmt.Errorf("%s: 第 %d 行头部格式必须为 '# Key : Value'", sourceName, lineNumber)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("%s: 第 %d 行存在重复头部键: %s", sourceName, lineNumber, key)
		}
		values[key] = value
		if err == io.EOF {
			return nil, fmt.Errorf("%s: 未找到头部与数据的空行分隔", sourceName)
		}
	}
	header, err := buildHeader(values)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sourceName, err)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		row, err := parseRow(line, header, lineNumber)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sourceName, err)
		}
		if fn != nil {
			if err := fn(header, row); err != nil {
				return nil, err
			}
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: 读取数据失败: %w", sourceName, err)
	}
	if count != header.Count {
		return nil, fmt.Errorf("%s: Count=%d，实际记录数=%d", sourceName, header.Count, count)
	}
	return header, nil
}
