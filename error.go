package doccurator

import (
	"errors"
	"fmt"
	"strings"
)

type CommandError struct {
	message string
	cause   error
}

func (e *CommandError) Error() string {
	var msg strings.Builder
	fmt.Fprint(&msg, e.message)
	if e.cause != nil {
		fmt.Fprint(&msg, ": ", e.cause)
	}
	return msg.String()
}

func (e *CommandError) Unwrap() error {
	return e.cause
}

func newCommandError(message string, cause error) *CommandError {
	return &CommandError{message: message, cause: cause}
}

var RecordEmptyContentError = errors.New("content to record is empty")
