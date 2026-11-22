package logic

import (
	"strings"
	"unicode"

	"hfjj/backend/proto/passportpb"
)

func normalizeAccount(account string) string {
	return strings.TrimSpace(account)
}

func normalizeNickname(nickname string) string {
	return strings.TrimSpace(nickname)
}

func containsWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !isASCIIDigit(r) {
			return false
		}
	}
	return true
}

func validatePlatformAccount(account string) (passportpb.PassportCode, string, bool) {
	if account == "" {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号不能为空", false
	}
	if containsWhitespace(account) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号不能包含空格", false
	}

	runes := []rune(account)
	if len(runes) > 10 {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号长度不能超过 10 位", false
	}
	if isAllDigits(account) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号不能为纯数字", false
	}

	for _, r := range runes {
		if r == '_' || isASCIILetter(r) || isASCIIDigit(r) || unicode.Is(unicode.Han, r) {
			continue
		}
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号仅支持中文、字母、数字和下划线", false
	}
	return passportpb.PassportCode_PASSPORT_CODE_OK, "", true
}

func validateLoginAccount(account string) (passportpb.PassportCode, string, bool) {
	if account == "" {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号或学号不能为空", false
	}
	if containsWhitespace(account) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "账号或学号不能包含空格", false
	}
	if isAllDigits(account) {
		return passportpb.PassportCode_PASSPORT_CODE_OK, "", true
	}
	return validatePlatformAccount(account)
}

func validatePlatformNickname(nickname string) (passportpb.PassportCode, string, bool) {
	if nickname == "" {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "昵称不能为空", false
	}
	if containsWhitespace(nickname) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "昵称不能包含空格", false
	}

	runes := []rune(nickname)
	if len(runes) < 2 || len(runes) > 16 {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "昵称长度需为 2 到 16 位", false
	}
	for _, r := range runes {
		if r == '_' || isASCIILetter(r) || isASCIIDigit(r) || unicode.Is(unicode.Han, r) {
			continue
		}
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "昵称仅支持中文、字母、数字和下划线", false
	}
	return passportpb.PassportCode_PASSPORT_CODE_OK, "", true
}

func validatePlatformPassword(password string) (passportpb.PassportCode, string, bool) {
	if password == "" {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "密码不能为空", false
	}
	if containsWhitespace(password) {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "密码不能包含空格", false
	}
	if len([]rune(password)) < 8 {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "密码至少需要 8 位", false
	}

	var hasLetter bool
	var hasDigit bool
	for _, r := range password {
		if isASCIILetter(r) {
			hasLetter = true
			continue
		}
		if isASCIIDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "密码需同时包含字母和数字", false
	}
	return passportpb.PassportCode_PASSPORT_CODE_OK, "", true
}

func isASCIILetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
