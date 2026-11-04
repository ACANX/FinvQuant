package brs

import "testing"

// TestValidEnvName 校验运行环境标识作为对象 key 一级目录的合法性。
func TestValidEnvName(t *testing.T) {
	ok := []string{"dev", "test", "uat", "prod", "Prod-2", "loc_al", "ab"}
	for _, v := range ok {
		if !validEnvName(v) {
			t.Errorf("%q 应判为合法", v)
		}
	}
	bad := []string{"", "a", " prod", "prod/", "prod/x", "prod x", "生产", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	for _, v := range bad {
		if validEnvName(v) {
			t.Errorf("%q 应判为非法", v)
		}
	}
}
