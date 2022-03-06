package output

import "fmt"

func TerminalFormatAsDim(text string) string {
	return fmt.Sprintf("\033[2m%s\033[0m", text)
}
