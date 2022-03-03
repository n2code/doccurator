package output

import (
	"fmt"
)

func Plural(countable interface{}, singular string, plural string) string {
	switch c := countable.(type) {
	case []string:
		if len(c) != 1 {
			return plural
		}
	case int:
		if c != 1 {
			return plural
		}
	}
	return singular
}

func Filesize(i int64) string {
	switch {
	case i >= 1024*1024:
		return fmt.Sprintf("%.1f MiB (%d bytes)", float64(i)/float64(1024*1024), i)
	case i > 1024:
		return fmt.Sprintf("%.0f KiB (%d bytes)", float64(i)/float64(1024), i)
	default:
		return fmt.Sprintf("%d bytes", i)
	}
}
