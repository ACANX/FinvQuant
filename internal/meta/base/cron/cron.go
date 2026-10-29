// Package cron 提供 6 段 cron 表达式的解析与匹配（秒 分 时 日 月 周）。
//
// 独立包、仅依赖标准库，后续可抽出为单独库维护。
//
// 表达式格式：
//
//	秒 分 时 日 月 周
//	0  0  3  *  *  *
//
// 支持：
//   - `*` 任意值；
//   - `?`（仅「日」「周」）表示不指定，等价于 `*`；
//   - 数字、范围 `a-b`、步长 `*/n` / `a-b/n` / `a/n`、列表 `a,b,c`；
//   - 周日为 0（0-6）。
//
// 日与周同时指定时按「同时满足」处理（通常用 `?` 忽略其中一个）。
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule 解析后的 6 段调度。
type Schedule struct {
	second, minute, hour, dayOfMonth, month, dayOfWeek fieldSet
}

type fieldSet struct {
	min, max int
	values   map[int]struct{}
}

func (f fieldSet) contains(value int) bool {
	_, ok := f.values[value]
	return ok
}

// Parse 解析 6 段 cron 表达式。
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 6 {
		return nil, fmt.Errorf("cron 表达式必须为 6 段（秒 分 时 日 月 周），实际 %d 段: %q", len(fields), expr)
	}
	specs := []struct {
		name     string
		min, max int
		allowAny bool
	}{
		{"秒", 0, 59, false},
		{"分", 0, 59, false},
		{"时", 0, 23, false},
		{"日", 1, 31, true},
		{"月", 1, 12, false},
		{"周", 0, 6, true},
	}
	sets := make([]fieldSet, 6)
	for i, spec := range specs {
		set, err := parseField(fields[i], spec.name, spec.min, spec.max, spec.allowAny)
		if err != nil {
			return nil, err
		}
		sets[i] = set
	}
	return &Schedule{
		second: sets[0], minute: sets[1], hour: sets[2],
		dayOfMonth: sets[3], month: sets[4], dayOfWeek: sets[5],
	}, nil
}

func parseField(raw, name string, min, max int, allowAny bool) (fieldSet, error) {
	set := fieldSet{min: min, max: max, values: map[int]struct{}{}}
	if raw == "?" {
		if !allowAny {
			return set, fmt.Errorf("cron %s不支持 ?（仅日/周可用）", name)
		}
		raw = "*"
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return set, fmt.Errorf("cron %s存在空字段", name)
		}
		step := 1
		base := part
		if index := strings.Index(part, "/"); index >= 0 {
			base = part[:index]
			stepText := part[index+1:]
			parsed, err := strconv.Atoi(stepText)
			if err != nil || parsed <= 0 {
				return set, fmt.Errorf("cron %s步长非法: %q", name, part)
			}
			step = parsed
		}
		start, end := min, max
		switch {
		case base == "*":
			// 保持 min..max
		case strings.Contains(base, "-"):
			pieces := strings.SplitN(base, "-", 2)
			a, err1 := strconv.Atoi(pieces[0])
			b, err2 := strconv.Atoi(pieces[1])
			if err1 != nil || err2 != nil {
				return set, fmt.Errorf("cron %s范围非法: %q", name, part)
			}
			start, end = a, b
		default:
			value, err := strconv.Atoi(base)
			if err != nil {
				return set, fmt.Errorf("cron %s取值非法: %q", name, part)
			}
			start, end = value, value
		}
		if start < min || end > max || start > end {
			return set, fmt.Errorf("cron %s超出范围 %d-%d: %q", name, min, max, part)
		}
		for value := start; value <= end; value += step {
			set.values[value] = struct{}{}
		}
	}
	if len(set.values) == 0 {
		return set, fmt.Errorf("cron %s没有有效取值", name)
	}
	return set, nil
}

// Matches 判断时间 t 是否命中调度（按 t 的时区/本地时间）。
func (s *Schedule) Matches(t time.Time) bool {
	return s.second.contains(t.Second()) &&
		s.minute.contains(t.Minute()) &&
		s.hour.contains(t.Hour()) &&
		s.dayOfMonth.contains(t.Day()) &&
		s.month.contains(int(t.Month())) &&
		s.dayOfWeek.contains(int(t.Weekday()))
}

// Next 返回 t 之后（不含 t）的下一个命中时间；最多向后搜索 5 年。
func (s *Schedule) Next(t time.Time) time.Time {
	candidate := t.Truncate(time.Second).Add(time.Second)
	limit := candidate.AddDate(5, 0, 0)
	for candidate.Before(limit) {
		if s.Matches(candidate) {
			return candidate
		}
		candidate = candidate.Add(time.Second)
	}
	return time.Time{}
}
