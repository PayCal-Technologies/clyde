package clyde

import (
	"fmt"
	"math"
	"strconv"
)

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

func numberInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		if v > int64(maxInt()) || v < int64(minInt()) {
			return 0
		}
		return int(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v || v > float64(maxInt()) || v < float64(minInt()) {
			return 0
		}
		return int(v)
	case jsonNumber:
		n, err := strconv.Atoi(string(v))
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

type jsonNumber string

func formatBytes(size int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	amount := float64(size)
	for _, unit := range units {
		if amount < 1024 || unit == units[len(units)-1] {
			if unit == "B" {
				return fmt.Sprintf("%d B", size)
			}
			return fmt.Sprintf("%.1f %s", amount, unit)
		}
		amount /= 1024
	}
	return fmt.Sprintf("%d B", size)
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}
