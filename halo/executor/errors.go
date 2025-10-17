package executor

import "errors"

var (
	ErrInvalidConnectionString = errors.New("invalid connection string")
	ErrInvalidIsolationLevel   = errors.New("invalid isolation level")
)
