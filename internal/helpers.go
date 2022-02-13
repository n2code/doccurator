package internal

import (
	"fmt"
	"time"
)

func AssertNoError(err error, because string) {
	if err != nil {
		panic(fmt.Errorf("error unexpected because %s: %w", because, err))
	}
}

var UnixTimestampNow = func() uint64 {
	return uint64(time.Now().Unix())
}
