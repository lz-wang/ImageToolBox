package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CLI 层参数校验：尽早报出参数错误（先于文件 IO 与领域层处理），
// 领域包中的同款校验保留作为防御性默认。

// enumValidator 生成字符串枚举校验器（大小写不敏感，允许空值走默认逻辑）。
func enumValidator(flag string, allowed ...string) func(string) error {
	return func(v string) error {
		if v == "" {
			return nil
		}
		for _, a := range allowed {
			if strings.EqualFold(v, a) {
				return nil
			}
		}
		return fmt.Errorf("--%s 仅支持: %s（当前: %s）", flag, strings.Join(allowed, "/"), v)
	}
}

// formatValidator 生成图片格式校验器，规则与 imageio.NormalizeFormat 一致
// （大小写不敏感，允许省略前导点）。
func formatValidator(flag string) func(string) error {
	return func(v string) error {
		normalized := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(v), "."))
		switch normalized {
		case "jpg", "jpeg", "png", "webp":
			return nil
		default:
			return fmt.Errorf("--%s 仅支持: jpg/jpeg/png/webp（当前: %s）", flag, v)
		}
	}
}

// intRangeValidator 生成整数范围校验器（闭区间）。
func intRangeValidator(flag string, min, max int) func(int) error {
	return func(v int) error {
		if v < min || v > max {
			return fmt.Errorf("--%s 必须在 %d-%d 范围内（当前: %d）", flag, min, max, v)
		}
		return nil
	}
}

// positiveIntValidator 生成正整数校验器；flag 未显式提供时值为零值，
// 不会触发校验，因此可安全用于可选数值 flag。
func positiveIntValidator(flag string) func(int) error {
	return func(v int) error {
		if v <= 0 {
			return fmt.Errorf("--%s 必须大于 0（当前: %d）", flag, v)
		}
		return nil
	}
}

// positiveInt64Validator 生成 int64 正数校验器。
func positiveInt64Validator(flag string) func(int64) error {
	return func(v int64) error {
		if v <= 0 {
			return fmt.Errorf("--%s 必须大于 0（当前: %d）", flag, v)
		}
		return nil
	}
}

// positiveDurationValidator 生成正时长校验器。
func positiveDurationValidator(flag string) func(time.Duration) error {
	return func(v time.Duration) error {
		if v <= 0 {
			return fmt.Errorf("--%s 必须大于 0（当前: %s）", flag, v)
		}
		return nil
	}
}

// nonNegativeIntValidator 生成非负整数校验器（0 表示自动计算的 flag 使用）。
func nonNegativeIntValidator(flag string) func(int) error {
	return func(v int) error {
		if v < 0 {
			return fmt.Errorf("--%s 不能为负数（当前: %d）", flag, v)
		}
		return nil
	}
}

// floatRangeValidator 生成浮点范围校验器（闭区间）。
func floatRangeValidator(flag string, min, max float64) func(float64) error {
	return func(v float64) error {
		if v < min || v > max {
			return fmt.Errorf("--%s 必须在 %v~%v 范围内（当前: %v）", flag, min, max, v)
		}
		return nil
	}
}

// positiveFloatValidator 生成正数校验器。
func positiveFloatValidator(flag string) func(float64) error {
	return func(v float64) error {
		if v <= 0 {
			return fmt.Errorf("--%s 必须大于 0（当前: %v）", flag, v)
		}
		return nil
	}
}

// percentRangeValidator 生成百分比字符串校验器（如 "40%"）。
// max 为 0 时不设上限（resize --percent 允许放大，如 200%）。
func percentRangeValidator(flag string, max float64) func(string) error {
	return func(v string) error {
		if v == "" {
			return nil
		}
		if !strings.HasSuffix(v, "%") {
			return fmt.Errorf("--%s 必须使用百分比格式，例如 40%%（当前: %s）", flag, v)
		}
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
		if err != nil {
			return fmt.Errorf("--%s 无法解析百分比: %s", flag, v)
		}
		if parsed <= 0 {
			return fmt.Errorf("--%s 百分比必须大于 0（当前: %s）", flag, v)
		}
		if max > 0 && parsed > max {
			return fmt.Errorf("--%s 百分比必须在 (0,%v] 范围内（当前: %s）", flag, max, v)
		}
		return nil
	}
}
