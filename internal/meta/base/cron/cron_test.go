package cron

import (
	"testing"
	"time"
)

func TestParseAndMatches(t *testing.T) {
	schedule, err := Parse("0 30 2 * * *")
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	hit := time.Date(2026, 10, 29, 2, 30, 0, 0, time.Local)
	if !schedule.Matches(hit) {
		t.Fatalf("应命中 %v", hit)
	}
	if schedule.Matches(hit.Add(time.Second)) {
		t.Fatal("秒不为 0 时不应命中")
	}
}

func TestStepAndList(t *testing.T) {
	schedule, err := Parse("*/15 * * * * *")
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	for _, second := range []int{0, 15, 30, 45} {
		if !schedule.Matches(time.Date(2026, 10, 29, 1, 0, second, 0, time.Local)) {
			t.Fatalf("秒 %d 应命中", second)
		}
	}
	if schedule.Matches(time.Date(2026, 10, 29, 1, 0, 7, 0, time.Local)) {
		t.Fatal("秒 7 不应命中")
	}

	list, err := Parse("0 0 3 ? * 1,3")
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if !list.Matches(time.Date(2026, 10, 26, 3, 0, 0, 0, time.Local)) { // 周一
		t.Fatal("周一应命中")
	}
	if list.Matches(time.Date(2026, 10, 27, 3, 0, 0, 0, time.Local)) { // 周二
		t.Fatal("周二不应命中")
	}
}

func TestNext(t *testing.T) {
	schedule, err := Parse("0 0 3 * * *")
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	start := time.Date(2026, 10, 28, 4, 0, 0, 0, time.Local)
	next := schedule.Next(start)
	want := time.Date(2026, 10, 29, 3, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("Next 期望 %v，实际 %v", want, next)
	}
}

func TestInvalid(t *testing.T) {
	for _, expr := range []string{
		"0 3 * * *",
		"60 * * * * *",
		"0 0 24 * * *",
		"0 0 3 32 * *",
		"* * * * * */0",
	} {
		if _, err := Parse(expr); err == nil {
			t.Fatalf("表达式应报错: %q", expr)
		}
	}
}
