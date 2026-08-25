package domain

import (
	"math"
	"strings"
)

func validateRequiredText(field, value string, maxRunes int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Invalid(field, "该字段不能为空")
	}
	if len([]rune(trimmed)) > maxRunes {
		return Invalid(field, "字段内容超过允许长度")
	}
	return nil
}

func validateOptionalText(field, value string, maxRunes int) error {
	if len([]rune(strings.TrimSpace(value))) > maxRunes {
		return Invalid(field, "字段内容超过允许长度")
	}
	return nil
}

func validPositiveNumber(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validNonNegativeNumber(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
