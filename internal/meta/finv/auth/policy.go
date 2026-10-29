package auth

import (
	"errors"
	"fmt"
	"unicode"
)

// 口令强度约束（需求 10763）。
//
// 为什么用「至少 3 类字符」而不是强制四类：强制四类会逼出 `Passw0rd!`
// 这类可预测变形，长度与字符多样性组合更能在可用性与强度间取得平衡。
const (
	// PasswordMinLength 口令最小长度。
	PasswordMinLength = 8
	// PasswordMaxLength 口令最大长度（防止超长输入拖慢 argon2）。
	PasswordMaxLength = 64
	// PasswordMinClasses 至少需要命中的字符类别数（大写/小写/数字/符号）。
	PasswordMinClasses = 3
)

// ErrPasswordTooWeak 表示新口令不满足强度要求。
var ErrPasswordTooWeak = errors.New("新密码不符合强度要求")

// ErrPasswordSameAsOld 表示新口令与当前口令相同。
var ErrPasswordSameAsOld = errors.New("新密码不能与当前密码相同")

// ErrPasswordMismatch 表示当前口令校验失败。
var ErrPasswordMismatch = errors.New("当前密码不正确")

// ValidatePasswordStrength 校验口令强度，不合格时返带回具体原因的错误。
//
// 返回的错误消息可直接展示给用户（不含任何敏感信息）。
func ValidatePasswordStrength(password string) error {
	runes := []rune(password)
	if len(runes) < PasswordMinLength {
		return fmt.Errorf("%w：长度至少 %d 位", ErrPasswordTooWeak, PasswordMinLength)
	}
	if len(runes) > PasswordMaxLength {
		return fmt.Errorf("%w：长度不能超过 %d 位", ErrPasswordTooWeak, PasswordMaxLength)
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range runes {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		default:
			// 空白与控制字符不计入类别，但仍允许出现在口令中
		}
	}

	classes := 0
	for _, ok := range []bool{hasUpper, hasLower, hasDigit, hasSymbol} {
		if ok {
			classes++
		}
	}
	if classes < PasswordMinClasses {
		return fmt.Errorf("%w：需包含大写字母、小写字母、数字、符号中的至少 %d 类",
			ErrPasswordTooWeak, PasswordMinClasses)
	}
	return nil
}
