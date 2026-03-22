# go-transbank-sdk

A pure Go SDK for Transbank Oneclick (non-mall) API integration. Provides simple, idiomatic Go methods for card inscription, authorization, and refunds.

**Status**: Early version (v0.1.0) — Production-ready for Oneclick Non-Mall API v1.2.

## Features

- ✅ Card inscription with automated redirect flow
- ✅ Transaction authorization on registered cards
- ✅ Transaction status queries
- ✅ Reversals/refunds
- ✅ Card deletion/cancellation
- ✅ Proper error handling with typed errors
- ✅ Context cancellation support
- ✅ Support for both integration and production environments
- ✅ Zero external dependencies (stdlib only)

## Installation

```bash
go get github.com/scarmonar/go-transbank-sdk
```

Add to your `go.mod`:

```go
require github.com/scarmonar/go-transbank-sdk v0.1.0
```

## Quick Start

### 1. Create a Service Instance

```go
package main

import (
	"context"
	"fmt"
	"github.com/scarmonar/go-transbank-sdk/oneclick"
)

func main() {
	// Integration environment (testing)
	svc, err := oneclick.NewOneclickService(
		"597055555541", // Parent commerce code
		"579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C", // API secret
		"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		nil, // Use default HTTP client
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Service created. Is Integration: %v\n", svc.IsIntegrationEnvironment())
}
```

### 2. Inscribe a Card

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := svc.CreateInscription(ctx,
	"usuario_123",           // Username
	"user@example.com",      // Email
	"https://myapp.com/return-inscription", // Callback URL after user authorizes
)
if err != nil {
	panic(err)
}

// Redirect user to resp.URLWebpay to authorize card (~$50 CLP test charge)
fmt.Printf("Redirect user to: %s\nToken: %s\n", resp.URLWebpay, resp.Token)
```

### 3. Confirm Inscription (After User Returns)

```go
// Extract TBK_TOKEN from redirect URL parameter
token := r.URL.Query().Get("TBK_TOKEN")

confirmResp, err := svc.ConfirmInscription(ctx, token)
if err != nil {
	panic(err)
}

// $50 CLP test charge is automatically reversed by Transbank
fmt.Printf("Card registered. Tbk User: %s\nCard: %s %s\n",
	confirmResp.TbkUser,
	confirmResp.CardType,
	confirmResp.CardNumber,
)
```

### 4. Make a Charge

```go
resp, err := svc.AuthorizeTransaction(ctx,
	"usuario_123",  // Username (must match inscription)
	"b6bd6ba3-e718-4107-9386-d2b099a8dd42", // tbk_user from confirmation
	"order_592301",  // Unique order ID (for idempotency)
	[]oneclick.TransactionDetail{
		{
			CommerceCode:       "597055555542", // Child commerce code
			BuyOrder:           "order_592301", // Must be unique
			Amount:             50000,          // CLP (no decimals): 50000 = $50k CLP
			InstallmentsNumber: 1,              // No installments: 1
		},
	},
)
if err != nil {
	panic(err)
}

// Check if all details were authorized
for _, detail := range resp.Details {
	if detail.Status == oneclick.TransactionStatusAuthorized {
		fmt.Printf("Authorized: %s\n", detail.AuthorizationCode)
	}
}
```

### 5. Reverse a Charge

```go
refundResp, err := svc.ReverseTransaction(ctx,
	"order_592301",         // Buy order from authorization
	"597055555542",         // Commerce code
	"order_592301",         // Detail buy order
	50000,                  // Amount to refund (CLP)
)
if err != nil {
	panic(err)
}

fmt.Printf("Reversed. Type: %s, Nullified: %d\n", refundResp.Type, refundResp.NullifiedAmount)
```

### 6. Delete a Registered Card

```go
err := svc.DeleteInscription(ctx,
	"b6bd6ba3-e718-4107-9386-d2b099a8dd42", // tbk_user
	"usuario_123", // username
)
if err != nil {
	panic(err)
}

fmt.Println("Card unregistered")
```

## Error Handling

The SDK uses typed errors for better control:

```go
import (
	"github.com/scarmonar/go-transbank-sdk/oneclick"
)

resp, err := svc.ConfirmInscription(ctx, token)

if err != nil {
	// Check for specific errors
	if errors.Is(err, oneclick.ErrInvalidToken) {
		// Handle missing token
	}

	// Check for Transbank API errors
	var tbkErr *oneclick.TransbankError
	if errors.As(err, &tbkErr) {
		if tbkErr.IsUserCancelled() {
			// User clicked cancel on Transbank form (code -2)
		} else if tbkErr.IsGenericError() {
			// Generic Transbank error (code -1)
		} else {
			// Handle response_code (numerical error)
			fmt.Printf("Error code: %d, Message: %s\n", tbkErr.Code, tbkErr.Message)
		}
	}
}
```

## Request/Response Structs

### Inscription

**Request**: `InscriptionRequest`
```go
type InscriptionRequest struct {
	Username    string // User identifier
	Email       string // User email for notifications
	ResponseURL string // URL to redirect after authorization
}
```

**Response**: `InscriptionResponse`
```go
type InscriptionResponse struct {
	Token     string // Token for confirmation step
	URLWebpay string // Redirect URL for user authorization
}
```

### Confirmation

**Response**: `InscriptionConfirmResponse`
```go
type InscriptionConfirmResponse struct {
	ResponseCode      int    // 0 = success
	TbkUser           string // Unique identifier for registered card
	AuthorizationCode string // Authorization code for this request
	CardType          string // "Visa", "Mastercard", etc.
	CardNumber        string // Masked: "XXXXXXXXXXXX6623"
}
```

### Authorization

**Request**: `AuthorizeTransactionRequest`
```go
type AuthorizeTransactionRequest struct {
	Username string               // Username from inscription
	TbkUser  string               // tbk_user from confirmation
	BuyOrder string               // Unique order ID
	Details  []TransactionDetail  // One or more transactions
}

type TransactionDetail struct {
	CommerceCode       string // Child commerce code
	BuyOrder           string // Must be unique
	Amount             int    // CLP without decimals
	InstallmentsNumber int    // 1 for no installments
}
```

**Response**: `AuthorizeTransactionResponse`
```go
type AuthorizeTransactionResponse struct {
	BuyOrder        string                      // Your order ID
	CardDetail      CardDetail                  // {CardNumber: "...6623"}
	AccountingDate  string                      // "0321" = March 21
	TransactionDate time.Time                   // Response timestamp
	Details         []TransactionResponseDetail // Mirrors details sent
}

type TransactionResponseDetail struct {
	Amount             int    // Charged amount (CLP)
	Status             string // TransactionStatusAuthorized, TransactionStatusFailed, etc.
	AuthorizationCode  string // Code if authorized
	PaymentTypeCode    string // "VN" (normal), "VC" (installments), etc.
	ResponseCode       int    // 0 = success
	InstallmentsNumber int
	CommerceCode       string
	BuyOrder           string
}
```

### Transaction Status

Query transaction status by order ID:

```go
resp, err := svc.GetTransactionStatus(ctx, "order_592301")
// Returns AuthorizeTransactionResponse
```

## Amount Handling (CLP)

Chilean Peso amounts are sent as **integers without decimals**:

| Amount | CLP Value | Code |
|--------|-----------|------|
| `50` | $50 CLP | `50` |
| `50000` | $50.000 CLP | `50000` |
| `100000` | $100.000 CLP | `100000` |

**No decimal point** in the `amount` field.

Conversion example:
```go
amountCLP := float64(5000) // $5k CLP
amountInt := int(amountCLP) // → 5000
```

## Status/Response Codes

### Authorization Status Values

```go
const (
	TransactionStatusInitialized        = "INITIALIZED"
	TransactionStatusAuthorized         = "AUTHORIZED"
	TransactionStatusReversed           = "REVERSED"
	TransactionStatusFailed             = "FAILED"
	TransactionStatusNullified          = "NULLIFIED"
	TransactionStatusPartiallyNullified = "PARTIALLY_NULLIFIED"
	TransactionStatusCaptured           = "CAPTURED"
)
```

### Response Codes

```go
const (
	ResponseCodeSuccess        = 0    // Approved
	ResponseCodeGenericError   = -1   // General failure
	ResponseCodeUserCancelled  = -2   // User clicked cancel
)
```

## Environment Detection

```go
if svc.IsIntegrationEnvironment() {
	fmt.Println("Using sandbox environment")
}

if svc.IsProduction() {
	fmt.Println("Using production environment")
}
```

## Test Credentials (Integration)

```
Parent Commerce Code: 597055555541
Child Commerce Code 1: 597055555542
Child Commerce Code 2: 597055555543
API Secret: 579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C
Base URL: https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2
```

## Production URLs

Change the `baseURL` when deploying to production:

```go
svc, _ := oneclick.NewOneclickService(
	commerceCode,
	apiSecret,
	"https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
	nil,
)
```

## Testing

Run the test suite:

```bash
go test ./oneclick -v -race
```

## Architecture

- **Zero external dependencies**: Uses only stdlib (`net/http`, `encoding/json`, `context`, etc.)
- **Proper context handling**: All blocking operations accept `context.Context`
- **Idiomatic Go**: Follows Go conventions, uses interfaces where appropriate
- **Error wrapping**: Errors use `%w` for proper `errors.Is()` and `errors.As()` support
- **Table-driven tests**: Comprehensive test coverage with subtests

## License

MIT

## References

- [Transbank Developers - Oneclick Documentation](https://github.com/TransbankDevelopers/transbank-developers-docs/blob/master/documentacion/oneclick/README.md)
- [Transbank Oneclick REST API v1.2](https://webpay3g.transbank.cl/webpayserver/oneclick.api.documentation.html)
