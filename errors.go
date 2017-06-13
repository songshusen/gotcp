package gotcp

import (
	"errors"
)

// all error info defined here

var (
	ErrConnExClosed = errors.New("use closed connection ")
	ErrConnExWriteBlocking = errors.New("Write packet blocked")
)