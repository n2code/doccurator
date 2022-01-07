package doccinator

import "fmt"

type CommandError struct {
	message string
	cause   error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.cause)
}

func (e *CommandError) Unwrap() error {
	return e.cause
}

func newCommandError(message string, cause error) *CommandError {
	return &CommandError{message: message, cause: cause}
}
