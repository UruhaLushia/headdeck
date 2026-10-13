package protocol

import "fmt"

func FormatMemory(bytes *uint64) string {
	if bytes == nil {
		return "-"
	}
	value := float64(*bytes)
	for _, unit := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
		if value < 1024 || unit == "TiB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
	}
	return "-"
}
