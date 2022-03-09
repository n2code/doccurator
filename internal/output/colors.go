package output

import "fmt"

func TerminalFormatAsDim(text string) string {
	return fmt.Sprintf("\x1B[2m%s\x1B[0m", text)
}

func TerminalFormatAsError(text string) string {
	return fmt.Sprintf("\x1B[31m%s\x1B[0m", text)
}
