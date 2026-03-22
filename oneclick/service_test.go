package oneclick

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOneclickService(t *testing.T) {
	tests := []struct {
		name        string
		commerce    string
		secret      string
		baseURL     string
		expectError bool
	}{
		{
			name:        "valid",
			commerce:    "597055555541",
			secret:      "secret",
			baseURL:     "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
			expectError: false,
		},
		{
			name:        "missing commerce code",
			commerce:    "",
			secret:      "secret",
			baseURL:     "https://test.com",
			expectError: true,
		},
		{
			name:        "missing secret",
			commerce:    "597055555541",
			secret:      "",
			baseURL:     "https://test.com",
			expectError: true,
		},
		{
			name:        "missing base url",
			commerce:    "597055555541",
			secret:      "secret",
			baseURL:     "",
			expectError: true,
		},
		{
			name:        "relative base url is invalid",
			commerce:    "597055555541",
			secret:      "secret",
			baseURL:     "/relative/url",
			expectError: true,
		},
		{
			name:        "malformed base url",
			commerce:    "597055555541",
			secret:      "secret",
			baseURL:     "ht!tp://invalid [url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewOneclickService(tt.commerce, tt.secret, tt.baseURL, nil)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
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

func TestEnvironmentDetection(t *testing.T) {
	integrationSvc, _ := NewOneclickService("597055555541", "secret", "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2", nil)
	if !integrationSvc.IsIntegrationEnvironment() {
		t.Fatal("expected integration environment")
	}
	if integrationSvc.IsProduction() {
		t.Fatal("integration service cannot be production")
	}

	prodSvc, _ := NewOneclickService("597055555541", "secret", "https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2", nil)
	if prodSvc.IsIntegrationEnvironment() {
		t.Fatal("expected production environment")
	}
	if !prodSvc.IsProduction() {
		t.Fatal("expected production flag")
	}
}

func TestStartRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/inscriptions" {
			t.Fatalf("expected /inscriptions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Tbk-Api-Key-Id"); got != "597055555541" {
			t.Fatalf("unexpected commerce header: %s", got)
		}
		if got := r.Header.Get("Tbk-Api-Key-Secret"); got != "secret" {
			t.Fatalf("unexpected secret header: %s", got)
		}

		var req InscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Username != "user_123" || req.Email != "user@example.com" || req.ResponseURL != "https://merchant.com/return" {
			t.Fatalf("unexpected request: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(InscriptionResponse{Token: "token123", URLWebpay: "https://webpay/form"})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	resp, err := svc.Start(context.Background(), "user_123", "user@example.com", "https://merchant.com/return")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token != "token123" {
		t.Fatalf("unexpected token: %s", resp.Token)
	}
}

func TestFinishRequestShapeAndResponseCode(t *testing.T) {
	t.Run("sends empty object body and escapes token", func(t *testing.T) {
		token := "token/with/slash"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %s", r.Method)
			}
			if r.URL.EscapedPath() != "/inscriptions/token%2Fwith%2Fslash" {
				t.Fatalf("unexpected path: %s (escaped: %s)", r.URL.Path, r.URL.EscapedPath())
			}
			body, _ := io.ReadAll(r.Body)
			if strings.TrimSpace(string(body)) != "{}" {
				t.Fatalf("expected empty JSON body {}, got: %q", string(body))
			}

			_ = json.NewEncoder(w).Encode(InscriptionConfirmResponse{
				ResponseCode:      0,
				TbkUser:           "b6bd6ba3-e718-4107-9386-d2b099a8dd42",
				AuthorizationCode: "123456",
				CardType:          "Visa",
				CardNumber:        "XXXXXXXXXXXX6623",
			})
		}))
		defer server.Close()

		svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
		resp, err := svc.Finish(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.TbkUser == "" {
			t.Fatal("expected tbk_user")
		}
	})

	t.Run("non-zero response code returns typed error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(InscriptionConfirmResponse{ResponseCode: -2})
		}))
		defer server.Close()

		svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
		_, err := svc.Finish(context.Background(), "token")
		if err == nil {
			t.Fatal("expected error")
		}
		var tbkErr *TransbankError
		if !errors.As(err, &tbkErr) {
			t.Fatalf("expected TransbankError, got %T", err)
		}
		if tbkErr.Code != -2 {
			t.Fatalf("unexpected code: %d", tbkErr.Code)
		}
	})
}

func TestRemoveRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/inscriptions" {
			t.Fatalf("expected /inscriptions, got %s", r.URL.Path)
		}

		var req DeleteInscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.TbkUser != "tbk-user" || req.Username != "user_123" {
			t.Fatalf("unexpected request: %+v", req)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	if err := svc.Remove(context.Background(), "tbk-user", "user_123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthorizeRequestShapeAndNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/transactions" {
			t.Fatalf("expected /transactions, got %s", r.URL.Path)
		}

		var req AuthorizeTransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.BuyOrder != "mall-order-1" {
			t.Fatalf("unexpected buy_order: %s", req.BuyOrder)
		}
		if len(req.Details) != 1 {
			t.Fatalf("expected one detail, got %d", len(req.Details))
		}
		if req.Details[0].InstallmentsNumber != 0 {
			t.Fatalf("expected installments=0, got %d", req.Details[0].InstallmentsNumber)
		}

		_ = json.NewEncoder(w).Encode(AuthorizeTransactionResponse{
			BuyOrder:        "mall-order-1",
			CardDetail:      CardDetail{CardNumber: "6623"},
			AccountingDate:  "0321",
			SessionID:       "session-123",
			VCI:             "TSY",
			TransactionDate: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
			Details: []TransactionResponseDetail{{
				Amount:             50000,
				Status:             TransactionStatusAuthorized,
				AuthorizationCode:  "1213",
				PaymentTypeCode:    PaymentTypeNormalSale,
				ResponseCode:       0,
				InstallmentsNumber: 0,
				CommerceCode:       "597055555542",
				BuyOrder:           "child-order-1",
				Balance:            50000,
			}},
		})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	resp, err := svc.Authorize(context.Background(), "user_123", "tbk-user", "mall-order-1", []TransactionDetail{{
		CommerceCode:       "597055555542",
		BuyOrder:           "child-order-1",
		Amount:             50000,
		InstallmentsNumber: -7,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SessionID != "session-123" || resp.VCI != "TSY" {
		t.Fatalf("missing extended fields: %+v", resp)
	}
	if got := resp.Details[0].Balance; got != 50000 {
		t.Fatalf("unexpected detail balance: %d", got)
	}
}

func TestStatusEscapesPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/transactions/parent%2Forder" {
			t.Fatalf("unexpected path: %s (escaped: %s)", r.URL.Path, r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(AuthorizeTransactionResponse{BuyOrder: "parent/order"})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	if _, err := svc.Status(context.Background(), "parent/order"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefundRequestShapeAndReversedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/transactions/parent%2Forder/refunds" {
			t.Fatalf("unexpected path: %s (escaped: %s)", r.URL.Path, r.URL.EscapedPath())
		}
		var req RefundRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.CommerceCode != "597055555542" || req.DetailBuyOrder != "child-order" || req.Amount != 50000 {
			t.Fatalf("unexpected request: %+v", req)
		}
		_, _ = io.WriteString(w, `{"type":"REVERSED"}`)
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	resp, err := svc.Refund(context.Background(), "parent/order", "597055555542", "child-order", 50000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != "REVERSED" {
		t.Fatalf("unexpected type: %+v", resp)
	}
}

func TestCaptureRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/transactions/capture" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req CaptureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.CommerceCode != "597055555542" || req.BuyOrder != "child-order" || req.AuthorizationCode != "1213" || req.CaptureAmount != 50000 {
			t.Fatalf("unexpected request: %+v", req)
		}

		_ = json.NewEncoder(w).Encode(CaptureResponse{
			AuthorizationCode: "152759",
			AuthorizationDate: time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC),
			CapturedAmount:    50000,
			ResponseCode:      0,
		})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	resp, err := svc.Capture(context.Background(), "597055555542", "child-order", "1213", 50000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CapturedAmount != 50000 {
		t.Fatalf("unexpected capture response: %+v", resp)
	}
}

func TestCaptureFailureReturnsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error_message":"capture_amount is invalid"}`)
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	_, err := svc.Capture(context.Background(), "597055555542", "child-order", "1213", 50000)
	if err == nil {
		t.Fatal("expected error")
	}

	var tbkErr *TransbankError
	if !errors.As(err, &tbkErr) {
		t.Fatalf("expected TransbankError, got %T", err)
	}
	if tbkErr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected code: %d", tbkErr.Code)
	}
}

func TestErrorMessageParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error_message":"token is required"}`)
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	_, err := svc.Finish(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error")
	}

	var tbkErr *TransbankError
	if !errors.As(err, &tbkErr) {
		t.Fatalf("expected TransbankError, got %T", err)
	}
	if tbkErr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected code: %d", tbkErr.Code)
	}
	if !strings.Contains(tbkErr.Message, "token is required") {
		t.Fatalf("unexpected message: %q", tbkErr.Message)
	}
}

func TestValidationLimitsAndFormats(t *testing.T) {
	svc, _ := NewOneclickService("597055555541", "secret", "https://example.com", nil)

	tooLongUsername := strings.Repeat("a", maxUsernameLength+1)
	_, err := svc.Start(context.Background(), tooLongUsername, "user@example.com", "https://merchant.com/return")
	if err == nil || !errors.Is(err, ErrMaxLengthExceeded) {
		t.Fatalf("expected max length error for username, got: %v", err)
	}

	tooLongEmail := strings.Repeat("a", maxEmailLength-9) + "@mail.test"
	_, err = svc.Start(context.Background(), "user", tooLongEmail, "https://merchant.com/return")
	if err == nil || !errors.Is(err, ErrMaxLengthExceeded) {
		t.Fatalf("expected max length error for email, got: %v", err)
	}

	_, err = svc.Status(context.Background(), "orden-ñ")
	if !errors.Is(err, ErrInvalidBuyOrderFmt) {
		t.Fatalf("expected invalid buy_order format error, got: %v", err)
	}

	_, err = svc.Capture(context.Background(), strings.Repeat("1", maxCommerceCodeLength+1), "child-order", "1213", 1000)
	if err == nil || !errors.Is(err, ErrMaxLengthExceeded) {
		t.Fatalf("expected max length error for commerce_code, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(InscriptionResponse{Token: "test"})
	}))
	defer server.Close()

	svc, _ := NewOneclickService("597055555541", "secret", server.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := svc.Start(ctx, "user", "email@test.com", "https://test.com/return")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected context error, got: %v", err)
	}
}
