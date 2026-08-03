package clyde

import (
	"fmt"
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
		return int(v)
	case float64:
		return int(v)
	case jsonNumber:
		n, _ := strconv.Atoi(string(v))
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
