package oneclick

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func newTestClientWithOptions(t *testing.T, baseURL string, opts ...Option) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentIntegration
	cfg.CommerceCode = DefaultIntegrationCommerceCode
	cfg.APISecret = "secret"
	cfg.BaseURL = baseURL

	client, err := NewClientWithConfig(cfg, opts...)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

func TestBuildReturnURL(t *testing.T) {
	t.Run("normalizes and strips query", func(t *testing.T) {
		got, err := BuildReturnURL("HTTPS://Merchant.CL/base?foo=bar", "/billing/confirm?x=1", ReturnURLModeStrictHTTPS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://merchant.cl/billing/confirm" {
			t.Fatalf("unexpected URL: %s", got)
		}
	})

	t.Run("rejects non-https in strict mode", func(t *testing.T) {
		_, err := BuildReturnURL("http://merchant.cl", "/return", ReturnURLModeStrictHTTPS)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("allows localhost http in local mode", func(t *testing.T) {
		got, err := BuildReturnURL("http://localhost:8080", "api/return", ReturnURLModeAllowHTTPLocalhost)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://localhost:8080/api/return" {
			t.Fatalf("unexpected URL: %s", got)
		}
	})
}

func TestBuildRedirectInstruction(t *testing.T) {
	resp := &InscriptionResponse{Token: "token123", URLWebpay: "https://webpay.test/oneclick"}
	instruction, err := BuildRedirectInstruction(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instruction.Method != "POST" {
		t.Fatalf("unexpected method: %s", instruction.Method)
	}
	if instruction.FormFields["TBK_TOKEN"] != "token123" {
		t.Fatalf("unexpected form field value: %+v", instruction.FormFields)
	}
}

func TestFlowServiceStartAndConfirmWithIdempotency(t *testing.T) {
	var startCalls int32
	var confirmCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/inscriptions":
			atomic.AddInt32(&startCalls, 1)
			_, _ = w.Write([]byte(`{"token":"token-abc","url_webpay":"https://webpay.example.com/form"}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/inscriptions/token-abc":
			atomic.AddInt32(&confirmCalls, 1)
			_, _ = w.Write([]byte(`{"response_code":0,"tbk_user":"tbk-user-1","authorization_code":"1213","card_type":"Visa","card_number":"XXXX6623"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := newTestClientWithOptions(t, server.URL)
	store := NewInMemoryStateStore()
	flow, err := NewFlowService(client, store)
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	startResp, err := flow.StartInscription(context.Background(), FlowStartRequest{
		Username:       "user_123",
		Email:          "user@example.com",
		BaseURL:        "https://merchant.cl",
		ReturnPath:     "/billing/confirm",
		BusinessID:     "biz-1",
		SubscriptionID: "sub-1",
		Context: map[string]string{
			"tenant_id": "tenant-1",
		},
		IdempotencyKey: "idem-start-1",
	})
	if err != nil {
		t.Fatalf("start flow: %v", err)
	}
	if startResp.Redirect.Method != "POST" || startResp.Redirect.FormFields["TBK_TOKEN"] == "" {
		t.Fatalf("invalid redirect instruction: %+v", startResp.Redirect)
	}

	startRespCached, err := flow.StartInscription(context.Background(), FlowStartRequest{
		Username:       "user_123",
		Email:          "user@example.com",
		BaseURL:        "https://merchant.cl",
		ReturnPath:     "/billing/confirm",
		IdempotencyKey: "idem-start-1",
	})
	if err != nil {
		t.Fatalf("cached start flow: %v", err)
	}
	if startRespCached.Token != startResp.Token {
		t.Fatalf("expected same token, got %s vs %s", startRespCached.Token, startResp.Token)
	}
	if got := atomic.LoadInt32(&startCalls); got != 1 {
		t.Fatalf("expected one start call, got %d", got)
	}

	confirmResp, err := flow.ConfirmInscription(context.Background(), FlowConfirmRequest{
		Token:          startResp.Token,
		IdempotencyKey: "idem-confirm-1",
	})
	if err != nil {
		t.Fatalf("confirm flow: %v", err)
	}
	if confirmResp.State.Status != FlowStatusConfirmed {
		t.Fatalf("expected confirmed state, got %s", confirmResp.State.Status)
	}
	if confirmResp.Confirmation.TbkUser == "" {
		t.Fatal("expected tbk_user")
	}

	_, err = flow.ConfirmInscription(context.Background(), FlowConfirmRequest{
		Token:          startResp.Token,
		IdempotencyKey: "idem-confirm-1",
	})
	if err != nil {
		t.Fatalf("cached confirm flow: %v", err)
	}
	if got := atomic.LoadInt32(&confirmCalls); got != 1 {
		t.Fatalf("expected one confirm call, got %d", got)
	}
}

func TestFlowServiceConfirmTokenNotFound(t *testing.T) {
	client := newTestClientWithOptions(t, "https://example.com")
	flow, err := NewFlowService(client, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	_, err = flow.ConfirmInscription(context.Background(), FlowConfirmRequest{Token: "missing-token"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestClientHooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"token123","url_webpay":"https://webpay.test/form"}`))
	}))
	defer server.Close()

	var mu sync.Mutex
	var before RequestEvent
	var after ResponseEvent
	var sawBefore bool
	var sawAfter bool

	client := newTestClientWithOptions(t, server.URL, WithHooks(Hooks{
		BeforeRequest: func(_ context.Context, event RequestEvent) {
			mu.Lock()
			before = event
			sawBefore = true
			mu.Unlock()
		},
		AfterRequest: func(_ context.Context, event ResponseEvent) {
			mu.Lock()
			after = event
			sawAfter = true
			mu.Unlock()
		},
	}))

	_, err := client.Start(context.Background(), "user_1", "user@example.com", "https://merchant.cl/return")
	if err != nil {
		t.Fatalf("start request failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !sawBefore || !sawAfter {
		t.Fatalf("expected both hooks to run: before=%v after=%v", sawBefore, sawAfter)
	}
	if before.RequestID == "" || after.RequestID == "" {
		t.Fatalf("expected request IDs in hooks: before=%+v after=%+v", before, after)
	}
	if before.RequestID != after.RequestID {
		t.Fatalf("expected same request ID across hooks: %s != %s", before.RequestID, after.RequestID)
	}
}
