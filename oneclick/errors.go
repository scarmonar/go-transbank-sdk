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

// SDKErrorCode identifies high-level SDK failures.
type SDKErrorCode string

const (
	SDKErrorCodeValidation    SDKErrorCode = "validation"
	SDKErrorCodeTransport     SDKErrorCode = "transport"
	SDKErrorCodeGateway       SDKErrorCode = "gateway"
	SDKErrorCodeTokenNotFound SDKErrorCode = "token_not_found"
	SDKErrorCodeFlowState     SDKErrorCode = "flow_state"
)

// SDKError adds a stable code plus retry and user-safe metadata.
type SDKError struct {
	code            SDKErrorCode
	message         string
	retryable       bool
	userSafeMessage string
	err             error
}

func (e *SDKError) Error() string {
	if e.message != "" {
		if e.err != nil {
			return fmt.Sprintf("%s: %v", e.message, e.err)
		}
		return e.message
	}
	if e.err != nil {
		return e.err.Error()
	}
	return string(e.code)
}

func (e *SDKError) Unwrap() error {
	return e.err
}

// Is allows errors.Is(err, ErrValidation) style checks by error code.
func (e *SDKError) Is(target error) bool {
	t, ok := target.(*SDKError)
	if !ok {
		return false
	}
	return e.code == t.code
}

// Code returns the stable, machine-readable code.
func (e *SDKError) Code() string {
	return string(e.code)
}

// Retryable returns true if callers can safely retry the operation.
func (e *SDKError) Retryable() bool {
	return e.retryable
}

// UserSafeMessage returns a sanitized, end-user-safe message.
func (e *SDKError) UserSafeMessage() string {
	if e.userSafeMessage != "" {
		return e.userSafeMessage
	}
	return "No pudimos completar la operación. Intenta nuevamente."
}

func newSDKError(code SDKErrorCode, message string, retryable bool, userSafe string, err error) *SDKError {
	return &SDKError{
		code:            code,
		message:         message,
		retryable:       retryable,
		userSafeMessage: userSafe,
		err:             err,
	}
}

// Canonical typed SDK errors for errors.Is checks.
var (
	ErrValidation    = &SDKError{code: SDKErrorCodeValidation}
	ErrTransport     = &SDKError{code: SDKErrorCodeTransport}
	ErrGateway       = &SDKError{code: SDKErrorCodeGateway}
	ErrTokenNotFound = &SDKError{code: SDKErrorCodeTokenNotFound}
	ErrFlowState     = &SDKError{code: SDKErrorCodeFlowState}
)

// NewValidationError wraps validation failures.
func NewValidationError(message string, err error) *SDKError {
	return newSDKError(
		SDKErrorCodeValidation,
		message,
		false,
		"Revisa los datos ingresados e intenta nuevamente.",
		err,
	)
}

// NewTransportError wraps transport-level failures.
func NewTransportError(message string, retryable bool, err error) *SDKError {
	return newSDKError(
		SDKErrorCodeTransport,
		message,
		retryable,
		"Tenemos problemas de conexión. Intenta nuevamente en unos minutos.",
		err,
	)
}

// NewGatewayError wraps gateway-level failures from Transbank.
func NewGatewayError(message string, retryable bool, err error) *SDKError {
	return newSDKError(
		SDKErrorCodeGateway,
		message,
		retryable,
		"No fue posible procesar el pago en este momento.",
		err,
	)
}

// NewTokenNotFoundError indicates that flow state for the token does not exist.
func NewTokenNotFoundError(token string, err error) *SDKError {
	msg := "token not found"
	if token != "" {
		msg = fmt.Sprintf("token not found: %s", token)
	}
	return newSDKError(
		SDKErrorCodeTokenNotFound,
		msg,
		false,
		"No encontramos el proceso de inscripción asociado al token recibido.",
		err,
	)
}

// NewFlowStateError wraps state persistence or consistency errors.
func NewFlowStateError(message string, retryable bool, err error) *SDKError {
	return newSDKError(
		SDKErrorCodeFlowState,
		message,
		retryable,
		"No pudimos actualizar el estado del proceso. Intenta nuevamente.",
		err,
	)
}

// Common low-level validation errors.
var (
	ErrInvalidCommerceCode  = errors.New("invalid commerce code")
	ErrInvalidAPISecret     = errors.New("invalid API secret")
	ErrInvalidBaseURL       = errors.New("invalid base URL")
	ErrInvalidEnvironment   = errors.New("invalid environment")
	ErrNilHTTPClient        = errors.New("HTTP client cannot be nil")
	ErrInvalidToken         = errors.New("invalid or empty token")
	ErrInvalidBuyOrder      = errors.New("invalid or empty buy order")
	ErrInvalidBuyOrderFmt   = errors.New("buy order contains unsupported characters")
	ErrInvalidUsername      = errors.New("invalid or empty username")
	ErrInvalidEmail         = errors.New("invalid or empty email")
	ErrInvalidTbkUser       = errors.New("invalid or empty tbk_user")
	ErrInvalidAmount        = errors.New("amount must be greater than 0")
	ErrEmptyResponseURL     = errors.New("response_url cannot be empty")
	ErrMissingDetails       = errors.New("transaction details cannot be empty")
	ErrInvalidAuthCode      = errors.New("invalid or empty authorization code")
	ErrInvalidCaptureAmount = errors.New("capture amount must be greater than 0")
	ErrInvalidInstallments  = errors.New("invalid installments number")
	ErrMaxLengthExceeded    = errors.New("field max length exceeded")
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
