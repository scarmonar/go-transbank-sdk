package oneclick

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewOneclickService tests the NewOneclickService constructor.
func TestNewOneclickService(t *testing.T) {
	tests := []struct {
		name           string
		commerceCode   string
		apiSecret      string
		baseURL        string
		httpClient     *http.Client
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:         "valid service creation",
			commerceCode: "597055555541",
			apiSecret:    "579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C",
			baseURL:      "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
			httpClient:   nil,
			expectError:  false,
		},
		{
			name:           "missing commerce code",
			commerceCode:   "",
			apiSecret:      "test",
			baseURL:        "https://test.com",
			expectError:    true,
			expectedErrMsg: "invalid commerce code",
		},
		{
			name:           "missing API secret",
			commerceCode:   "597055555541",
			apiSecret:      "",
			baseURL:        "https://test.com",
			expectError:    true,
			expectedErrMsg: "invalid API secret",
		},
		{
			name:           "missing base URL",
			commerceCode:   "597055555541",
			apiSecret:      "test",
			baseURL:        "",
			expectError:    true,
			expectedErrMsg: "invalid base URL",
		},
		{
			name:           "invalid base URL format",
			commerceCode:   "597055555541",
			apiSecret:      "test",
			baseURL:        "ht!tp://invalid [url",
			expectError:    true,
			expectedErrMsg: "parse base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewOneclickService(tt.commerceCode, tt.apiSecret, tt.baseURL, tt.httpClient)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svc == nil {
				t.Fatal("expected non-nil service")
			}
		})
	}
}

// TestIsIntegrationEnvironment tests environment detection.
func TestIsIntegrationEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectInteg bool
	}{
		{
			name:        "integration environment",
			baseURL:     "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
			expectInteg: true,
		},
		{
			name:        "production environment",
			baseURL:     "https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
			expectInteg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := NewOneclickService("597055555541", "secret", tt.baseURL, nil)
			result := svc.IsIntegrationEnvironment()
			if result != tt.expectInteg {
				t.Errorf("IsIntegrationEnvironment() = %v, expected %v", result, tt.expectInteg)
			}
		})
	}
}

// TestCreateInscription tests the CreateInscription method.
func TestCreateInscription(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		email       string
		responseURL string
		expectError bool
	}{
		{
			name:        "valid inscription",
			username:    "usuario_123",
			email:       "user@example.com",
			responseURL: "https://example.com/return",
			expectError: false,
		},
		{
			name:        "missing username",
			username:    "",
			email:       "user@example.com",
			responseURL: "https://example.com/return",
			expectError: true,
		},
		{
			name:        "missing email",
			username:    "usuario_123",
			email:       "",
			responseURL: "https://example.com/return",
			expectError: true,
		},
		{
			name:        "missing response URL",
			username:    "usuario_123",
			email:       "user@example.com",
			responseURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/inscriptions" {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(InscriptionResponse{
					Token:     "test_token_123",
					URLWebpay: "https://webpay.transbank.cl/form",
				})
			}))
			defer server.Close()

			svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := svc.CreateInscription(ctx, tt.username, tt.email, tt.responseURL)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.Token != "test_token_123" {
				t.Errorf("unexpected token: %v", resp.Token)
			}
		})
	}
}

// TestConfirmInscription tests the ConfirmInscription method.
func TestConfirmInscription(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:        "valid confirmation",
			token:       "e128a9c24c0a3cbc09223973327b97c8c474f6b74be509196cce4caf162a016a",
			expectError: false,
		},
		{
			name:        "missing token",
			token:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/inscriptions/") {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(InscriptionConfirmResponse{
					ResponseCode:      0,
					TbkUser:           "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
					AuthorizationCode: "123456",
					CardType:          "Visa",
					CardNumber:        "XXXXXXXXXXXX6623",
				})
			}))
			defer server.Close()

			svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := svc.ConfirmInscription(ctx, tt.token)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.TbkUser != "b6bd6ba3-e718-4107-9386-d2b099a8dd42" {
				t.Errorf("unexpected tbk_user: %v", resp.TbkUser)
			}
		})
	}
}

// TestDeleteInscription tests the DeleteInscription method.
func TestDeleteInscription(t *testing.T) {
	tests := []struct {
		name        string
		tbkUser     string
		username    string
		expectError bool
	}{
		{
			name:        "valid deletion",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			username:    "usuario_123",
			expectError: false,
		},
		{
			name:        "missing tbk_user",
			tbkUser:     "",
			username:    "usuario_123",
			expectError: true,
		},
		{
			name:        "missing username",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			username:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/inscriptions" {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := svc.DeleteInscription(ctx, tt.tbkUser, tt.username)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestAuthorizeTransaction tests the AuthorizeTransaction method.
func TestAuthorizeTransaction(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		tbkUser     string
		buyOrder    string
		details     []TransactionDetail
		expectError bool
	}{
		{
			name:     "valid transaction",
			username: "usuario_123",
			tbkUser:  "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			buyOrder: "415034240",
			details: []TransactionDetail{
				{
					CommerceCode:       "597055555542",
					BuyOrder:           "505479072",
					Amount:             50000,
					InstallmentsNumber: 1,
				},
			},
			expectError: false,
		},
		{
			name:        "missing username",
			username:    "",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			buyOrder:    "415034240",
			details:     []TransactionDetail{{CommerceCode: "597055555542", BuyOrder: "505479072", Amount: 50000, InstallmentsNumber: 1}},
			expectError: true,
		},
		{
			name:        "missing tbk_user",
			username:    "usuario_123",
			tbkUser:     "",
			buyOrder:    "415034240",
			details:     []TransactionDetail{{CommerceCode: "597055555542", BuyOrder: "505479072", Amount: 50000, InstallmentsNumber: 1}},
			expectError: true,
		},
		{
			name:        "missing buy_order",
			username:    "usuario_123",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			buyOrder:    "",
			details:     []TransactionDetail{{CommerceCode: "597055555542", BuyOrder: "505479072", Amount: 50000, InstallmentsNumber: 1}},
			expectError: true,
		},
		{
			name:        "missing details",
			username:    "usuario_123",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			buyOrder:    "415034240",
			details:     []TransactionDetail{},
			expectError: true,
		},
		{
			name:        "invalid amount",
			username:    "usuario_123",
			tbkUser:     "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
			buyOrder:    "415034240",
			details:     []TransactionDetail{{CommerceCode: "597055555542", BuyOrder: "505479072", Amount: 0, InstallmentsNumber: 1}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/transactions" {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(AuthorizeTransactionResponse{
					BuyOrder:       "415034240",
					CardDetail:     CardDetail{CardNumber: "6623"},
					AccountingDate: "0321",
					Details: []TransactionResponseDetail{
						{
							Amount:             50000,
							Status:             TransactionStatusAuthorized,
							AuthorizationCode:  "1213",
							PaymentTypeCode:    PaymentTypeNormalSale,
							ResponseCode:       0,
							InstallmentsNumber: 1,
							CommerceCode:       "597055555542",
							BuyOrder:           "505479072",
						},
					},
				})
			}))
			defer server.Close()

			svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := svc.AuthorizeTransaction(ctx, tt.username, tt.tbkUser, tt.buyOrder, tt.details)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.BuyOrder != "415034240" {
				t.Errorf("unexpected buy_order: %v", resp.BuyOrder)
			}
		})
	}
}

// TestReverseTransaction tests the ReverseTransaction method.
func TestReverseTransaction(t *testing.T) {
	tests := []struct {
		name            string
		buyOrder        string
		commerceCode    string
		detailBuyOrder  string
		amount          int
		expectError     bool
	}{
		{
			name:           "valid reversal",
			buyOrder:       "415034240",
			commerceCode:   "597055555542",
			detailBuyOrder: "505479072",
			amount:         50000,
			expectError:    false,
		},
		{
			name:           "missing buy_order",
			buyOrder:       "",
			commerceCode:   "597055555542",
			detailBuyOrder: "505479072",
			amount:         50000,
			expectError:    true,
		},
		{
			name:           "missing commerce_code",
			buyOrder:       "415034240",
			commerceCode:   "",
			detailBuyOrder: "505479072",
			amount:         50000,
			expectError:    true,
		},
		{
			name:           "invalid amount",
			buyOrder:       "415034240",
			commerceCode:   "597055555542",
			detailBuyOrder: "505479072",
			amount:         0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/transactions/") || !strings.HasSuffix(r.URL.Path, "/refunds") {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(RefundResponse{
					Type:              "REVERSED",
					AuthorizationCode: "1234",
					NullifiedAmount:   50000,
					Balance:           0,
					ResponseCode:      0,
				})
			}))
			defer server.Close()

			svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := svc.ReverseTransaction(ctx, tt.buyOrder, tt.commerceCode, tt.detailBuyOrder, tt.amount)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.Type != "REVERSED" {
				t.Errorf("unexpected type: %v", resp.Type)
			}
		})
	}
}

// TestContextCancellation tests that the service respects context cancellation.
func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		json.NewEncoder(w).Encode(InscriptionResponse{Token: "test"})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := svc.CreateInscription(ctx, "user", "email@test.com", "https://test.com/return")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context deadline error, got: %v", err)
	}
}
