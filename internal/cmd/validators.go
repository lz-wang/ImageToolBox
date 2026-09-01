package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"imagetoolbox/internal/imageio"
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

// finiteFloat 校验浮点值是有限数：NaN 与任何比较都为 false，
// 仅靠范围判断无法拦截 NaN/Inf。
func finiteFloat(flag string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("--%s 必须是有限数值（当前: %v）", flag, v)
	}
	return nil
}

// floatRangeValidator 生成浮点范围校验器（闭区间）。
func floatRangeValidator(flag string, min, max float64) func(float64) error {
	return func(v float64) error {
		if err := finiteFloat(flag, v); err != nil {
			return err
		}
		if v < min || v > max {
			return fmt.Errorf("--%s 必须在 %v~%v 范围内（当前: %v）", flag, min, max, v)
		}
		return nil
	}
}

// nonNegativeFloatValidator 生成非负浮点校验器。
func nonNegativeFloatValidator(flag string) func(float64) error {
	return func(v float64) error {
		if err := finiteFloat(flag, v); err != nil {
			return err
		}
		if v < 0 {
			return fmt.Errorf("--%s 不能为负数（当前: %v）", flag, v)
		}
		return nil
	}
}

// colorValidator 生成十六进制颜色校验器，规则与 imageio.ParseHexColor
// 一致（#RGB/#RRGGBB/#RRGGBBAA，大小写不敏感；空值走自动选择）。
func colorValidator(flag string) func(string) error {
	return func(v string) error {
		if strings.TrimSpace(v) == "" {
			return nil
		}
		if _, err := imageio.ParseHexColor(v); err != nil {
			return fmt.Errorf("--%s 必须是十六进制颜色，例如 #FFFFFF（当前: %s）", flag, v)
		}
		return nil
	}
}

// positiveFloatValidator 生成正数校验器。
func positiveFloatValidator(flag string) func(float64) error {
	return func(v float64) error {
		if err := finiteFloat(flag, v); err != nil {
			return err
		}
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
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return fmt.Errorf("--%s 百分比必须是有限数值（当前: %s）", flag, v)
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
