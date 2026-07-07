package core

import "fmt"

// FormatSize renders a byte count in base-1024 units, CLI-compatible:
// bytes get 0 decimals ("512 B"), every larger unit exactly one decimal
// ("1.5 KB", "2.0 GB"). Values >= 1024 TB stay in TB (clamped — the
// sensible fix recommended in porting-notes for the CLI's unguarded index).
func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes) / 1024
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}
