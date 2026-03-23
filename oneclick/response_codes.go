package oneclick

// ResponseCodeClassification describes how to interpret a Transbank response_code.
type ResponseCodeClassification struct {
	// Code is the raw Transbank response code.
	Code int
	// Approved reports whether the code represents an approved transaction.
	Approved bool
	// Reason is a stable, machine-readable label for logs and internal handling.
	Reason string
	// UserSafeMessage is a sanitized message suitable for user-facing UI.
	UserSafeMessage string
}

// ClassifyResponseCode translates a Transbank response_code into a stable classification.
//
// The mapping covers the Oneclick-specific codes documented by Transbank plus a
// generic fallback for unknown rejection codes.
func ClassifyResponseCode(code int) ResponseCodeClassification {
	switch code {
	case ResponseCodeSuccess:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        true,
			Reason:          "approved",
			UserSafeMessage: "El pago fue aprobado.",
		}
	case ResponseCodeGenericError:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "generic_error",
			UserSafeMessage: "La tarjeta fue rechazada o no fue posible completar la autorización.",
		}
	case ResponseCodeUserCancelled:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "user_cancelled",
			UserSafeMessage: "El usuario canceló el proceso de pago.",
		}
	case -96:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "tbk_user_not_found",
			UserSafeMessage: "No existe un usuario inscrito para esta tarjeta.",
		}
	case -97:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "daily_amount_limit_exceeded",
			UserSafeMessage: "Se superó el monto diario permitido para esta tarjeta.",
		}
	case -98:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "amount_limit_exceeded",
			UserSafeMessage: "Se superó el monto máximo permitido para esta tarjeta.",
		}
	case -99:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "daily_payment_count_limit_exceeded",
			UserSafeMessage: "Se superó la cantidad diaria permitida de pagos para esta tarjeta.",
		}
	default:
		return ResponseCodeClassification{
			Code:            code,
			Approved:        false,
			Reason:          "unknown_rejection",
			UserSafeMessage: defaultSafeErrorMessage,
		}
	}
}
