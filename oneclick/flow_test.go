package oneclick

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClientWithOptions(t *testing.T, baseURL string, opts ...Option) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentIntegration
	cfg.CommerceCode = DefaultIntegrationCommerceCode
	cfg.APISecret = "secret"
	cfg.baseURL = baseURL

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

func TestFlowConfirmResponseIncludesExpandedContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/inscriptions":
			_, _ = w.Write([]byte(`{"token":"token-ctx","url_webpay":"https://webpay.example.com/form"}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/inscriptions/token-ctx":
			_, _ = w.Write([]byte(`{"response_code":0,"tbk_user":"tbk-user-ctx","authorization_code":"1213","card_type":"Visa","card_number":"XXXX6623"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	store := NewInMemoryStateStore()
	client := newTestClientWithOptions(t, server.URL)
	flow, err := NewFlowService(client, store)
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	startResp, err := flow.StartInscription(context.Background(), FlowStartRequest{
		Username:       "user_ctx",
		Email:          "ctx@example.com",
		BaseURL:        "https://merchant.cl",
		ReturnPath:     "/return",
		BusinessID:     "biz-ctx",
		SubscriptionID: "sub-ctx",
		Context: map[string]string{
			"tenant": "tenant-1",
		},
	})
	if err != nil {
		t.Fatalf("start flow: %v", err)
	}

	confirmResp, err := flow.ConfirmInscription(context.Background(), FlowConfirmRequest{Token: startResp.Token})
	if err != nil {
		t.Fatalf("confirm flow: %v", err)
	}
	if confirmResp.BusinessID != "biz-ctx" || confirmResp.SubscriptionID != "sub-ctx" {
		t.Fatalf("missing expanded context fields: %+v", confirmResp)
	}
	if confirmResp.Context["tenant"] != "tenant-1" {
		t.Fatalf("unexpected context: %+v", confirmResp.Context)
	}

	confirmResp.Context["tenant"] = "mutated"
	stored, err := store.GetByToken(context.Background(), startResp.Token)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if stored.Context["tenant"] != "tenant-1" {
		t.Fatalf("expected cloned context map, got state=%+v", stored.Context)
	}
}

func TestFlowServiceAuthorizeChargeFromTokenWithIdempotency(t *testing.T) {
	var authorizeCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/inscriptions":
			_, _ = w.Write([]byte(`{"token":"token-charge","url_webpay":"https://webpay.example.com/form"}`))
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/inscriptions/token-charge":
			_, _ = w.Write([]byte(`{"response_code":0,"tbk_user":"tbk-user-charge","authorization_code":"1213","card_type":"Visa","card_number":"XXXX6623"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/transactions":
			atomic.AddInt32(&authorizeCalls, 1)
			var req AuthorizeTransactionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Username != "flow_user" || req.TbkUser != "tbk-user-charge" || req.BuyOrder != "mall-charge-1" {
				t.Fatalf("unexpected authorize payload: %+v", req)
			}
			_ = json.NewEncoder(w).Encode(AuthorizeTransactionResponse{
				BuyOrder:        req.BuyOrder,
				TransactionDate: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
				Details: []TransactionResponseDetail{{
					CommerceCode: "597055555542",
					BuyOrder:     "child-order-1",
					Amount:       1000,
					Status:       TransactionStatusAuthorized,
				}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := newTestClientWithOptions(t, server.URL)
	flow, err := NewFlowService(client, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	startResp, err := flow.StartInscription(context.Background(), FlowStartRequest{
		Username:   "flow_user",
		Email:      "flow@example.com",
		BaseURL:    "https://merchant.cl",
		ReturnPath: "/return",
	})
	if err != nil {
		t.Fatalf("start flow: %v", err)
	}
	if _, err := flow.ConfirmInscription(context.Background(), FlowConfirmRequest{Token: startResp.Token}); err != nil {
		t.Fatalf("confirm flow: %v", err)
	}

	req := FlowAuthorizeChargeRequest{
		TokenOrTbkUser: startResp.Token,
		BuyOrder:       "mall-charge-1",
		Details: []TransactionDetail{{
			CommerceCode: "597055555542",
			BuyOrder:     "child-order-1",
			Amount:       1000,
		}},
		IdempotencyKey: "idem-auth-1",
	}
	resp, err := flow.AuthorizeCharge(context.Background(), req)
	if err != nil {
		t.Fatalf("authorize charge: %v", err)
	}
	if resp.State == nil || resp.State.Token != startResp.Token || resp.BuyOrder != "mall-charge-1" {
		t.Fatalf("unexpected authorize response: %+v", resp)
	}

	cached, err := flow.AuthorizeCharge(context.Background(), req)
	if err != nil {
		t.Fatalf("authorize charge cached: %v", err)
	}
	if cached.BuyOrder != resp.BuyOrder {
		t.Fatalf("unexpected cached response: %+v", cached)
	}
	if got := atomic.LoadInt32(&authorizeCalls); got != 1 {
		t.Fatalf("expected one authorize call, got %d", got)
	}
}

func TestFlowServiceAuthorizeChargeValidationAndGatewayError(t *testing.T) {
	client := newTestClientWithOptions(t, "https://example.com")
	flow, err := NewFlowService(client, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	_, err = flow.AuthorizeCharge(context.Background(), FlowAuthorizeChargeRequest{
		TokenOrTbkUser: "tbk-user-direct",
		BuyOrder:       "mall-order-1",
		Details: []TransactionDetail{{
			CommerceCode: "597055555542",
			BuyOrder:     "child-order-1",
			Amount:       1000,
		}},
	})
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error_message":"upstream unavailable"}`))
	}))
	defer server.Close()

	clientGateway := newTestClientWithOptions(t, server.URL)
	flowGateway, err := NewFlowService(clientGateway, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	_, err = flowGateway.AuthorizeCharge(context.Background(), FlowAuthorizeChargeRequest{
		TokenOrTbkUser: "tbk-user-direct",
		Username:       "user_direct",
		BuyOrder:       "mall-order-1",
		Details: []TransactionDetail{{
			CommerceCode: "597055555542",
			BuyOrder:     "child-order-1",
			Amount:       1000,
		}},
	})
	if err == nil || !errors.Is(err, ErrGateway) {
		t.Fatalf("expected gateway error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestFlowServiceReverseChargeHooksAndTransportError(t *testing.T) {
	var mu sync.Mutex
	seenBefore := map[string]RequestEvent{}
	seenAfter := map[string]ResponseEvent{}

	var reverseCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/transactions/mall-order-1/refunds" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		atomic.AddInt32(&reverseCalls, 1)
		_ = json.NewEncoder(w).Encode(RefundResponse{Type: "REVERSED", ResponseCode: 0})
	}))
	defer server.Close()

	client := newTestClientWithOptions(t, server.URL, WithHooks(Hooks{
		BeforeRequest: func(_ context.Context, event RequestEvent) {
			mu.Lock()
			seenBefore[event.Operation] = event
			mu.Unlock()
		},
		AfterRequest: func(_ context.Context, event ResponseEvent) {
			mu.Lock()
			seenAfter[event.Operation] = event
			mu.Unlock()
		},
	}))
	flow, err := NewFlowService(client, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}

	reverseReq := FlowReverseChargeRequest{
		BuyOrder:       "mall-order-1",
		CommerceCode:   "597055555542",
		DetailBuyOrder: "child-order-1",
		Amount:         1000,
		IdempotencyKey: "idem-reverse-1",
	}
	resp, err := flow.ReverseCharge(context.Background(), reverseReq)
	if err != nil {
		t.Fatalf("reverse charge: %v", err)
	}
	if resp.Refund.Type != "REVERSED" {
		t.Fatalf("unexpected reverse response: %+v", resp)
	}
	if _, err := flow.ReverseCharge(context.Background(), reverseReq); err != nil {
		t.Fatalf("reverse charge cached: %v", err)
	}
	if got := atomic.LoadInt32(&reverseCalls); got != 1 {
		t.Fatalf("expected one reverse call, got %d", got)
	}

	mu.Lock()
	before := seenBefore[idempotencyOpReverseCharge]
	after := seenAfter[idempotencyOpReverseCharge]
	mu.Unlock()
	if before.RequestID == "" || after.RequestID == "" {
		t.Fatalf("expected request ids in flow hooks: before=%+v after=%+v", before, after)
	}
	if before.RequestID != after.RequestID {
		t.Fatalf("expected correlable request IDs, got %s vs %s", before.RequestID, after.RequestID)
	}

	transportClient := newTestClientWithOptions(
		t,
		"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, &net.DNSError{IsTemporary: true}
			}),
		}),
		WithRetryPolicy(RetryPolicy{
			MaxAttempts:    1,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
			RetryOnStatus:  map[int]struct{}{http.StatusBadGateway: {}},
		}),
	)
	transportFlow, err := NewFlowService(transportClient, NewInMemoryStateStore())
	if err != nil {
		t.Fatalf("create flow service: %v", err)
	}
	_, err = transportFlow.ReverseCharge(context.Background(), FlowReverseChargeRequest{
		BuyOrder:       "mall-order-1",
		CommerceCode:   "597055555542",
		DetailBuyOrder: "child-order-1",
		Amount:         1000,
	})
	if err == nil || !errors.Is(err, ErrTransport) {
		t.Fatalf("expected transport error, got %v", err)
	}
}
