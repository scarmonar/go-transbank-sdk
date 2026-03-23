package oneclick

import "testing"

func TestClassifyResponseCode(t *testing.T) {
	tests := []struct {
		name            string
		code            int
		wantApproved    bool
		wantReason      string
		wantUserMessage string
	}{
		{
			name:            "approved",
			code:            ResponseCodeSuccess,
			wantApproved:    true,
			wantReason:      "approved",
			wantUserMessage: "El pago fue aprobado.",
		},
		{
			name:            "generic error",
			code:            ResponseCodeGenericError,
			wantApproved:    false,
			wantReason:      "generic_error",
			wantUserMessage: "La tarjeta fue rechazada o no fue posible completar la autorización.",
		},
		{
			name:            "user cancelled",
			code:            ResponseCodeUserCancelled,
			wantApproved:    false,
			wantReason:      "user_cancelled",
			wantUserMessage: "El usuario canceló el proceso de pago.",
		},
		{
			name:            "tbk user missing",
			code:            -96,
			wantApproved:    false,
			wantReason:      "tbk_user_not_found",
			wantUserMessage: "No existe un usuario inscrito para esta tarjeta.",
		},
		{
			name:            "daily amount exceeded",
			code:            -97,
			wantApproved:    false,
			wantReason:      "daily_amount_limit_exceeded",
			wantUserMessage: "Se superó el monto diario permitido para esta tarjeta.",
		},
		{
			name:            "amount exceeded",
			code:            -98,
			wantApproved:    false,
			wantReason:      "amount_limit_exceeded",
			wantUserMessage: "Se superó el monto máximo permitido para esta tarjeta.",
		},
		{
			name:            "count exceeded",
			code:            -99,
			wantApproved:    false,
			wantReason:      "daily_payment_count_limit_exceeded",
			wantUserMessage: "Se superó la cantidad diaria permitida de pagos para esta tarjeta.",
		},
		{
			name:            "unknown code falls back to generic safe message",
			code:            -1234,
			wantApproved:    false,
			wantReason:      "unknown_rejection",
			wantUserMessage: defaultSafeErrorMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyResponseCode(tt.code)
			if got.Code != tt.code {
				t.Fatalf("unexpected code: got=%d want=%d", got.Code, tt.code)
			}
			if got.Approved != tt.wantApproved {
				t.Fatalf("unexpected approved: got=%v want=%v", got.Approved, tt.wantApproved)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("unexpected reason: got=%q want=%q", got.Reason, tt.wantReason)
			}
			if got.UserSafeMessage != tt.wantUserMessage {
				t.Fatalf("unexpected user message: got=%q want=%q", got.UserSafeMessage, tt.wantUserMessage)
			}
		})
	}
}
