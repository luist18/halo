package errors

import "net/http"

const (
	// InvalidInputErrorType represents errors caused by invalid client input
	InvalidInputErrorType = "InvalidInput"
	// InternalErrorType represents internal server errors
	InternalErrorType = "InternalError"
	// MethodNotAllowedErrorType represents errors for unsupported HTTP methods
	MethodNotAllowedErrorType = "MethodNotAllowedError"
	// RequestEntityTooLargeErrorType represents errors for payloads that exceed size limits
	RequestEntityTooLargeErrorType = "RequestEntityTooLarge"
)

// ProxyError represents an error that occurs within the HTTP proxy with type
// information and HTTP status code
type ProxyError interface {
	error
	Type() string
	StatusCode() int
}

// proxyError implements the ProxyError interface
type proxyError struct {
	type_   string
	message string
	err     error
	Args    []any
}

// Error returns the error message
func (e *proxyError) Error() string {
	return e.message
}

// Unwrap returns the wrapped error if any
func (e *proxyError) Unwrap() error {
	return e.err
}

// Type returns the error type
func (e *proxyError) Type() string {
	return e.type_
}

// StatusCode returns the HTTP status code associated with the error type
func (e *proxyError) StatusCode() int {
	switch e.type_ {
	case InvalidInputErrorType:
		return http.StatusBadRequest
	case InternalErrorType:
		return http.StatusInternalServerError
	case MethodNotAllowedErrorType:
		return http.StatusMethodNotAllowed
	case RequestEntityTooLargeErrorType:
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}

// NewInvalidInputErr creates a new invalid input error with the given message and
// optional arguments
func NewInvalidInputErr(message string, args ...any) ProxyError {
	return &proxyError{
		type_:   InvalidInputErrorType,
		message: message,
		Args:    args,
	}
}

// WrapWithInvalidInputErr wraps an existing error with an invalid input error
func WrapWithInvalidInputErr(err error, message string, args ...any) ProxyError {
	return &proxyError{
		type_:   InvalidInputErrorType,
		message: message,
		err:     err,
		Args:    args,
	}
}

// NewInternalErr creates a new internal error with the given message and
// optional arguments
func NewInternalErr(message string, args ...any) ProxyError {
	return &proxyError{
		type_:   InternalErrorType,
		message: message,
		Args:    args,
	}
}

// WrapWithInternalErr wraps an existing error with an internal error
func WrapWithInternalErr(err error, message string, args ...any) ProxyError {
	return &proxyError{
		type_:   InternalErrorType,
		message: message,
		err:     err,
		Args:    args,
	}
}

// NewMethodNotAllowedErr creates a new method not allowed error with the given
// message and optional arguments
func NewMethodNotAllowedErr(message string, args ...any) ProxyError {
	return &proxyError{
		type_:   MethodNotAllowedErrorType,
		message: message,
		Args:    args,
	}
}

// NewRequestEntityTooLargeErr creates a new request entity too large error with
// the given message and optional arguments
func NewRequestEntityTooLargeErr(message string, args ...any) ProxyError {
	return &proxyError{
		type_:   RequestEntityTooLargeErrorType,
		message: message,
		Args:    args,
	}
}
