package oneclick

import "time"

// InscriptionRequest represents a request to create a new inscription.
type InscriptionRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	ResponseURL string `json:"response_url"`
}

// InscriptionResponse represents the response from creating an inscription.
type InscriptionResponse struct {
	Token      string `json:"token"`
	URLWebpay  string `json:"url_webpay"`
	StatusCode int    `json:"status_code,omitempty"`
}

// InscriptionConfirmRequest represents a request to confirm an inscription.
// The token is passed in the URL path, not in the body.
type InscriptionConfirmRequest struct {
	Token string
}

// InscriptionConfirmResponse represents the response from confirming an inscription.
type InscriptionConfirmResponse struct {
	ResponseCode      int    `json:"response_code"`
	TbkUser           string `json:"tbk_user"`
	AuthorizationCode string `json:"authorization_code"`
	CardType          string `json:"card_type"`
	CardNumber        string `json:"card_number"`
}

// DeleteInscriptionRequest represents a request to delete an inscription.
type DeleteInscriptionRequest struct {
	TbkUser  string `json:"tbk_user"`
	Username string `json:"username"`
}

// TransactionDetail represents a single transaction detail in a multi-store transaction.
type TransactionDetail struct {
	CommerceCode       string `json:"commerce_code"`
	BuyOrder           string `json:"buy_order"`
	Amount             int    `json:"amount"`
	InstallmentsNumber int    `json:"installments_number"`
}

// AuthorizeTransactionRequest represents a request to authorize a transaction.
type AuthorizeTransactionRequest struct {
	Username string               `json:"username"`
	TbkUser  string               `json:"tbk_user"`
	BuyOrder string               `json:"buy_order"`
	Details  []TransactionDetail  `json:"details"`
}

// CardDetail represents card information in a transaction response.
type CardDetail struct {
	CardNumber string `json:"card_number"`
}

// TransactionResponseDetail represents a single detail in a transaction response.
type TransactionResponseDetail struct {
	Amount              int    `json:"amount"`
	Status              string `json:"status"`
	AuthorizationCode   string `json:"authorization_code"`
	PaymentTypeCode     string `json:"payment_type_code"`
	ResponseCode        int    `json:"response_code"`
	InstallmentsNumber  int    `json:"installments_number"`
	CommerceCode        string `json:"commerce_code"`
	BuyOrder            string `json:"buy_order"`
}

// AuthorizeTransactionResponse represents the response from authorizing a transaction.
type AuthorizeTransactionResponse struct {
	BuyOrder        string                     `json:"buy_order"`
	CardDetail      CardDetail                 `json:"card_detail"`
	AccountingDate  string                     `json:"accounting_date"`
	TransactionDate time.Time                  `json:"transaction_date"`
	Details         []TransactionResponseDetail `json:"details"`
}

// GetTransactionStatusRequest represents a request to get transaction status.
type GetTransactionStatusRequest struct {
	BuyOrder string
}

// RefundRequest represents a request to refund/reverse a transaction.
type RefundRequest struct {
	CommerceCode    string `json:"commerce_code"`
	DetailBuyOrder  string `json:"detail_buy_order"`
	Amount          int    `json:"amount"`
}

// RefundResponse represents the response from a refund/reverse operation.
type RefundResponse struct {
	Type               string `json:"type"`
	AuthorizationCode  string `json:"authorization_code"`
	AuthorizationDate  time.Time `json:"authorization_date"`
	NullifiedAmount    int    `json:"nullified_amount"`
	Balance            int    `json:"balance"`
	ResponseCode       int    `json:"response_code"`
}

// PaymentTypeCode constants
const (
	PaymentTypeDebit             = "VD"
	PaymentTypePrepaid           = "VP"
	PaymentTypeNormalSale        = "VN"
	PaymentTypeInstallments      = "VC"
	PaymentType3InstallmentsNoInt = "SI"
	PaymentType2InstallmentsNoInt = "S2"
	PaymentTypeNInstallmentsNoInt = "NC"
)

// TransactionStatus constants
const (
	TransactionStatusInitialized        = "INITIALIZED"
	TransactionStatusAuthorized         = "AUTHORIZED"
	TransactionStatusReversed           = "REVERSED"
	TransactionStatusFailed             = "FAILED"
	TransactionStatusNullified          = "NULLIFIED"
	TransactionStatusPartiallyNullified = "PARTIALLY_NULLIFIED"
	TransactionStatusCaptured           = "CAPTURED"
)

// ResponseCode constants
const (
	ResponseCodeSuccess        = 0
	ResponseCodeGenericError   = -1
	ResponseCodeUserCancelled  = -2
)
