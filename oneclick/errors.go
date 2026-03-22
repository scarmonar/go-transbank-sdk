package oneclick

import (
	"errors"
	"fmt"
)

// TransbankError represents an error response from the Transbank API.
type TransbankError struct {
	Code    int
	Message string
	Details string
	Err     error
}

// Error implements the error interface.
func (e *TransbankError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("transbank error %d: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("transbank error %d: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *TransbankError) Unwrap() error {
	return e.Err
}

// IsUserCancelled returns true if the error represents a user cancellation (code -2).
func (e *TransbankError) IsUserCancelled() bool {
	return e.Code == -2
}

// IsGenericError returns true if the error represents a generic error (code -1).
func (e *TransbankError) IsGenericError() bool {
	return e.Code == -1
}

// Common error constructors
var (
	ErrInvalidCommerceCode = errors.New("invalid commerce code")
	ErrInvalidAPISecret    = errors.New("invalid API secret")
	ErrInvalidBaseURL      = errors.New("invalid base URL")
	ErrNilHTTPClient       = errors.New("HTTP client cannot be nil")
	ErrInvalidToken        = errors.New("invalid or empty token")
	ErrInvalidBuyOrder     = errors.New("invalid or empty buy order")
	ErrInvalidUsername     = errors.New("invalid or empty username")
	ErrInvalidEmail        = errors.New("invalid or empty email")
	ErrInvalidTbkUser      = errors.New("invalid or empty tbk_user")
	ErrInvalidAmount       = errors.New("amount must be greater than 0")
	ErrEmptyResponseURL    = errors.New("response_url cannot be empty")
	ErrMissingDetails      = errors.New("transaction details cannot be empty")
)

// NewTransbankError creates a new TransbankError.
func NewTransbankError(code int, message string, err error) *TransbankError {
	return &TransbankError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewTransbankErrorWithDetails creates a TransbankError with additional details.
func NewTransbankErrorWithDetails(code int, message, details string, err error) *TransbankError {
	return &TransbankError{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}
