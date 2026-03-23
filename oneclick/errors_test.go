package oneclick

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		wantCode        string
		wantRetryable   bool
		wantUserMessage string
		wantCauseNil    bool
	}{
		{
			name:            "nil error",
			err:             nil,
			wantCode:        "unknown",
			wantRetryable:   false,
			wantUserMessage: defaultSafeErrorMessage,
			wantCauseNil:    true,
		},
		{
			name:            "validation sdk error",
			err:             NewValidationError("invalid input", ErrInvalidEmail),
			wantCode:        "validation",
			wantRetryable:   false,
			wantUserMessage: "Revisa los datos ingresados e intenta nuevamente.",
		},
		{
			name:            "wrapped transport error",
			err:             fmt.Errorf("wrap: %w", NewTransportError("network", true, errors.New("timeout"))),
			wantCode:        "transport",
			wantRetryable:   true,
			wantUserMessage: "Tenemos problemas de conexión. Intenta nuevamente en unos minutos.",
		},
		{
			name:            "joined gateway error",
			err:             errors.Join(errors.New("noise"), NewGatewayError("gateway failed", false, NewTransbankError(500, "upstream", nil))),
			wantCode:        "gateway",
			wantRetryable:   false,
			wantUserMessage: "No fue posible procesar el pago en este momento.",
		},
		{
			name:            "token not found",
			err:             NewTokenNotFoundError("tok123", ErrStateStoreNotFound),
			wantCode:        "token_not_found",
			wantRetryable:   false,
			wantUserMessage: "No encontramos el proceso de inscripción asociado al token recibido.",
		},
		{
			name:            "flow state",
			err:             NewFlowStateError("state update failed", true, errors.New("db unavailable")),
			wantCode:        "flow_state",
			wantRetryable:   true,
			wantUserMessage: "No pudimos actualizar el estado del proceso. Intenta nuevamente.",
		},
		{
			name:            "errors join with sentinel fallback",
			err:             errors.Join(fmt.Errorf("wrapped: %w", ErrValidation), errors.New("other")),
			wantCode:        "validation",
			wantRetryable:   false,
			wantUserMessage: defaultSafeErrorMessage,
		},
		{
			name:            "transbank raw error maps to gateway",
			err:             fmt.Errorf("raw: %w", NewTransbankError(422, "invalid token", nil)),
			wantCode:        "gateway",
			wantRetryable:   false,
			wantUserMessage: defaultSafeErrorMessage,
		},
		{
			name:            "unknown error",
			err:             errors.New("boom"),
			wantCode:        "unknown",
			wantRetryable:   false,
			wantUserMessage: defaultSafeErrorMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got.Code != tt.wantCode {
				t.Fatalf("unexpected code: got=%s want=%s", got.Code, tt.wantCode)
			}
			if got.Retryable != tt.wantRetryable {
				t.Fatalf("unexpected retryable: got=%v want=%v", got.Retryable, tt.wantRetryable)
			}
			if got.UserSafeMessage != tt.wantUserMessage {
				t.Fatalf("unexpected user message: got=%q want=%q", got.UserSafeMessage, tt.wantUserMessage)
			}
			if tt.wantCauseNil {
				if got.Cause != nil {
					t.Fatalf("expected nil cause, got %v", got.Cause)
				}
				return
			}
			if got.Cause == nil {
				t.Fatalf("expected non-nil cause")
			}
		})
	}
}
