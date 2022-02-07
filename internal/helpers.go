package internal

import "fmt"

func AssertNoError(err error, because string) {
	if err != nil {
		panic(fmt.Errorf("error unexpected because %s: %w", because, err))
	}
}
