// Package errs provides the canonical error code types and constructors.
// Error codes are defined in the proto enum alfq.v1.ErrCode (see backend/proto/alfq/v1/errors.proto).
package errs

import (
	"fmt"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Error wraps a proto ErrCode with an optional message.
type Error struct {
	Code    pb.ErrCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// New creates an Error with the given code and message.
func New(code pb.ErrCode, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Wrap creates an Error with a cause.
func Wrap(code pb.ErrCode, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}
