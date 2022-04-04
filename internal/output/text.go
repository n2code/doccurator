package output

import (
	"fmt"
	"reflect"
	"strings"
)

func Indent(spaces int, multilineText string) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(multilineText, "\n")
	var indented strings.Builder
	for i, line := range lines {
		indented.WriteString(indent)
		indented.WriteString(line)
		if len(lines) > 1 && i < len(lines)-1 {
			indented.WriteRune('\n') //unless last line or only line
		}
	}
	return indented.String()
}

func Plural(countable interface{}, singular string, plural string) string {
	switch c := countable.(type) {

	case int:
		if c != 1 {
			return plural
		}
	default:
		if reflect.ValueOf(c).Len() != 1 {
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
