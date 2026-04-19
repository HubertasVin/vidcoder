package main

import (
	"fmt"
	"strconv"
	"strings"
)

func parseScaleMultiplier(raw string) (float64, error) {
	switch {
	case strings.HasPrefix(raw, "/"):
		divisor, err := parsePositiveNumber(raw[1:], "invalid scale divisor")
		if err != nil {
			return 0, err
		}
		return 1 / divisor, nil
	case strings.HasPrefix(raw, "*"):
		return parsePositiveNumber(raw[1:], "invalid scale multiplier")
	case strings.HasPrefix(raw, "x"), strings.HasPrefix(raw, "X"):
		return parsePositiveNumber(raw[1:], "invalid scale multiplier")
	default:
		return parsePositiveNumber(raw, "invalid scale factor")
	}
}

func parsePositiveNumber(raw, context string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s: %s", context, raw)
	}
	return value, nil
}
