package oneclick

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxUsernameLength          = 40
	maxEmailLength             = 100
	maxResponseURLLength       = 255
	maxBuyOrderLength          = 26
	maxTbkUserLength           = 40
	maxCommerceCodeLength      = 12
	maxAuthorizationCodeLength = 6
	maxInstallmentsNumber      = 99
)

type requestContextKey string

const (
	requestIDContextKey         requestContextKey = "oneclick_request_id"
	operationContextOverrideKey requestContextKey = "oneclick_operation_override"
)

// Client is the raw Oneclick API client (1:1 with Transbank endpoints).
type Client struct {
	cfg      Config
	http     *http.Client
	breaker  *circuitBreaker
	metrics  *ClientMetrics
	requestN uint64
}

// OneclickService is kept as a backward-compatible alias.
type OneclickService = Client

// NewClient creates a new raw Oneclick client with defaults + options.
func NewClient(opts ...Option) (*Client, error) {
	cfg := DefaultConfig()
	return NewClientWithConfig(cfg, opts...)
}

// NewClientWithConfig creates a new raw Oneclick client from config + options.
func NewClientWithConfig(cfg Config, opts ...Option) (*Client, error) {
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Client{
		cfg:     cfg,
		http:    cfg.HTTPClient,
		breaker: newCircuitBreaker(cfg.CircuitBreaker, cfg.Clock),
		metrics: &ClientMetrics{},
	}, nil
}

// NewOneclickService creates a Oneclick service preconfigured for integration.
// It uses Transbank public integration credentials and integration base URL.
func NewOneclickService() (*OneclickService, error) {
	cfg := DefaultConfig()
	cfg.Environment = EnvironmentIntegration
	cfg.CommerceCode = ""
	cfg.APISecret = ""
	return NewClientWithConfig(cfg)
}

// NewOneclickServiceFor creates a Oneclick service for the requested environment.
//
// Rules:
// - environment "" is autodetected.
// - integration + empty credentials uses public integration defaults.
// - production requires explicit credentials.
func NewOneclickServiceFor(environment Environment, commerceCode, apiSecret string, httpClient *http.Client) (*OneclickService, error) {
	cfg := DefaultConfig()
	cfg.Environment = environment
	cfg.CommerceCode = strings.TrimSpace(commerceCode)
	cfg.APISecret = strings.TrimSpace(apiSecret)
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}
	return NewClientWithConfig(cfg)
}

func newOneclickServiceWithBaseURL(environment Environment, commerceCode, apiSecret, baseURL string, httpClient *http.Client) (*OneclickService, error) {
	cfg := DefaultConfig()
	cfg.Environment = environment
	cfg.CommerceCode = strings.TrimSpace(commerceCode)
	cfg.APISecret = strings.TrimSpace(apiSecret)
	cfg.baseURL = strings.TrimSpace(baseURL)
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}
	return NewClientWithConfig(cfg)
}

// Config returns a copy of the effective config used by this client.
func (c *Client) Config() Config {
	return c.cfg
}

// CommerceCode returns the configured commerce code.
func (c *Client) CommerceCode() string {
	return c.cfg.CommerceCode
}

// IsIntegrationEnvironment returns true if the service appears configured for integration.
func (c *Client) IsIntegrationEnvironment() bool {
	return c.cfg.Environment == EnvironmentIntegration
}

// IsProduction returns true if the service appears configured for production.
func (c *Client) IsProduction() bool {
	return c.cfg.Environment == EnvironmentProduction
}

// MetricsSnapshot returns current client metrics.
func (c *Client) MetricsSnapshot() MetricsSnapshot {
	return c.metrics.snapshot()
}

// StartInscription maps to POST /inscriptions.
func (c *Client) StartInscription(ctx context.Context, req InscriptionRequest) (*InscriptionResponse, error) {
	if err := validateUsername(req.Username); err != nil {
		return nil, NewValidationError("invalid username", err)
	}
	if err := validateEmail(req.Email); err != nil {
		return nil, NewValidationError("invalid email", err)
	}
	if err := validateResponseURL(req.ResponseURL); err != nil {
		return nil, NewValidationError("invalid response URL", err)
	}

	normalized := InscriptionRequest{
		Username:    strings.TrimSpace(req.Username),
		Email:       strings.TrimSpace(req.Email),
		ResponseURL: strings.TrimSpace(req.ResponseURL),
	}

	var resp InscriptionResponse
	if err := c.doRequest(ctx, "start_inscription", http.MethodPost, "/inscriptions", normalized, &resp, 0); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ConfirmInscription maps to PUT /inscriptions/{token}.
func (c *Client) ConfirmInscription(ctx context.Context, token string) (*InscriptionConfirmResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewValidationError("invalid token", ErrInvalidToken)
	}

	endpoint := fmt.Sprintf("/inscriptions/%s", url.PathEscape(token))
	var resp InscriptionConfirmResponse
	// Transbank requires an explicit empty JSON body for this endpoint.
	if err := c.doRequest(ctx, "confirm_inscription", http.MethodPut, endpoint, struct{}{}, &resp, len(token)); err != nil {
		return nil, err
	}

	if resp.ResponseCode != ResponseCodeSuccess {
		c.metrics.incConfirmationFailure()
		tbkErr := NewTransbankError(resp.ResponseCode, "inscription confirmation failed", nil)
		return nil, NewGatewayError("inscription confirmation failed", false, tbkErr)
	}

	return &resp, nil
}

// DeleteInscription maps to DELETE /inscriptions.
func (c *Client) DeleteInscription(ctx context.Context, req DeleteInscriptionRequest) error {
	if err := validateTbkUser(req.TbkUser); err != nil {
		return NewValidationError("invalid tbk_user", err)
	}
	if err := validateUsername(req.Username); err != nil {
		return NewValidationError("invalid username", err)
	}

	normalized := DeleteInscriptionRequest{
		TbkUser:  strings.TrimSpace(req.TbkUser),
		Username: strings.TrimSpace(req.Username),
	}

	return c.doRequestRaw(ctx, "delete_inscription", http.MethodDelete, "/inscriptions", normalized, 0)
}

// AuthorizeTransaction maps to POST /transactions.
func (c *Client) AuthorizeTransaction(ctx context.Context, req AuthorizeTransactionRequest) (*AuthorizeTransactionResponse, error) {
	if err := validateUsername(req.Username); err != nil {
		return nil, NewValidationError("invalid username", err)
	}
	if err := validateTbkUser(req.TbkUser); err != nil {
		return nil, NewValidationError("invalid tbk_user", err)
	}
	if err := validateBuyOrder(req.BuyOrder); err != nil {
		return nil, NewValidationError("invalid buy_order", err)
	}

	normalizedDetails, err := normalizeAndValidateDetails(req.Details)
	if err != nil {
		return nil, NewValidationError("invalid transaction details", err)
	}

	normalized := AuthorizeTransactionRequest{
		Username: strings.TrimSpace(req.Username),
		TbkUser:  strings.TrimSpace(req.TbkUser),
		BuyOrder: strings.TrimSpace(req.BuyOrder),
		Details:  normalizedDetails,
	}

	var resp AuthorizeTransactionResponse
	if err := c.doRequest(ctx, "authorize_transaction", http.MethodPost, "/transactions", normalized, &resp, 0); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetTransactionStatus maps to GET /transactions/{buyOrder}.
func (c *Client) GetTransactionStatus(ctx context.Context, buyOrder string) (*AuthorizeTransactionResponse, error) {
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, NewValidationError("invalid buy_order", err)
	}

	endpoint := fmt.Sprintf("/transactions/%s", url.PathEscape(strings.TrimSpace(buyOrder)))
	var resp AuthorizeTransactionResponse
	if err := c.doRequest(ctx, "transaction_status", http.MethodGet, endpoint, nil, &resp, 0); err != nil {
		return nil, err
	}

	return &resp, nil
}

// RefundTransaction maps to POST /transactions/{buyOrder}/refunds.
func (c *Client) RefundTransaction(ctx context.Context, buyOrder string, req RefundRequest) (*RefundResponse, error) {
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, NewValidationError("invalid buy_order", err)
	}
	if err := validateCommerceCode(req.CommerceCode); err != nil {
		return nil, NewValidationError("invalid commerce_code", err)
	}
	if err := validateBuyOrder(req.DetailBuyOrder); err != nil {
		return nil, NewValidationError("invalid detail_buy_order", err)
	}
	if req.Amount <= 0 {
		return nil, NewValidationError("invalid amount", ErrInvalidAmount)
	}

	normalized := RefundRequest{
		CommerceCode:   strings.TrimSpace(req.CommerceCode),
		DetailBuyOrder: strings.TrimSpace(req.DetailBuyOrder),
		Amount:         req.Amount,
	}

	endpoint := fmt.Sprintf("/transactions/%s/refunds", url.PathEscape(strings.TrimSpace(buyOrder)))
	var resp RefundResponse
	if err := c.doRequest(ctx, "refund_transaction", http.MethodPost, endpoint, normalized, &resp, 0); err != nil {
		return nil, err
	}

	return &resp, nil
}

// CaptureTransaction maps to PUT /transactions/capture.
func (c *Client) CaptureTransaction(ctx context.Context, req CaptureRequest) (*CaptureResponse, error) {
	if err := validateCommerceCode(req.CommerceCode); err != nil {
		return nil, NewValidationError("invalid commerce_code", err)
	}
	if err := validateBuyOrder(req.BuyOrder); err != nil {
		return nil, NewValidationError("invalid buy_order", err)
	}
	if err := validateAuthorizationCode(req.AuthorizationCode); err != nil {
		return nil, NewValidationError("invalid authorization_code", err)
	}
	if req.CaptureAmount <= 0 {
		return nil, NewValidationError("invalid capture_amount", ErrInvalidCaptureAmount)
	}

	normalized := CaptureRequest{
		CommerceCode:      strings.TrimSpace(req.CommerceCode),
		BuyOrder:          strings.TrimSpace(req.BuyOrder),
		CaptureAmount:     req.CaptureAmount,
		AuthorizationCode: strings.TrimSpace(req.AuthorizationCode),
	}

	var resp CaptureResponse
	if err := c.doRequest(ctx, "capture_transaction", http.MethodPut, "/transactions/capture", normalized, &resp, 0); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Backward-compatible wrappers.

// Start initiates a new inscription and returns token + Webpay redirect URL.
func (c *Client) Start(ctx context.Context, username, email, responseURL string) (*InscriptionResponse, error) {
	return c.StartInscription(ctx, InscriptionRequest{
		Username:    username,
		Email:       email,
		ResponseURL: responseURL,
	})
}

// Finish confirms an inscription using the token returned to the commerce callback URL.
func (c *Client) Finish(ctx context.Context, token string) (*InscriptionConfirmResponse, error) {
	return c.ConfirmInscription(ctx, token)
}

// Remove deletes an existing inscription.
func (c *Client) Remove(ctx context.Context, tbkUser, username string) error {
	return c.DeleteInscription(ctx, DeleteInscriptionRequest{
		TbkUser:  tbkUser,
		Username: username,
	})
}

// Authorize authorizes one or many child transactions for an enrolled user.
func (c *Client) Authorize(ctx context.Context, username, tbkUser, buyOrder string, details []TransactionDetail) (*AuthorizeTransactionResponse, error) {
	return c.AuthorizeTransaction(ctx, AuthorizeTransactionRequest{
		Username: username,
		TbkUser:  tbkUser,
		BuyOrder: buyOrder,
		Details:  details,
	})
}

// Status retrieves the status of a previously authorized transaction.
func (c *Client) Status(ctx context.Context, buyOrder string) (*AuthorizeTransactionResponse, error) {
	return c.GetTransactionStatus(ctx, buyOrder)
}

// Refund reverses or nullifies a previously authorized transaction.
func (c *Client) Refund(ctx context.Context, buyOrder, commerceCode, detailBuyOrder string, amount int) (*RefundResponse, error) {
	return c.RefundTransaction(ctx, buyOrder, RefundRequest{
		CommerceCode:   commerceCode,
		DetailBuyOrder: detailBuyOrder,
		Amount:         amount,
	})
}

// Capture performs a deferred capture for a previously authorized child transaction.
func (c *Client) Capture(ctx context.Context, commerceCode, buyOrder, authorizationCode string, captureAmount int) (*CaptureResponse, error) {
	return c.CaptureTransaction(ctx, CaptureRequest{
		CommerceCode:      commerceCode,
		BuyOrder:          buyOrder,
		CaptureAmount:     captureAmount,
		AuthorizationCode: authorizationCode,
	})
}

// doRequest makes an HTTP request to the Transbank API and decodes the JSON response.
func (c *Client) doRequest(ctx context.Context, operation, method, path string, body interface{}, respTarget interface{}, tokenLength int) error {
	operation = operationFromContext(ctx, operation)
	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		requestID = c.nextRequestID()
	}

	if !c.breaker.allow() {
		err := NewTransportError("circuit breaker is open", true, nil)
		c.callOnError(ctx, requestID, operation, method, path, 0, 0, 0, true, 0, err)
		return err
	}

	var lastErr error

	for attempt := 1; attempt <= c.cfg.RetryPolicy.MaxAttempts; attempt++ {
		startedAt := c.cfg.Clock.Now()

		c.callBeforeRequest(ctx, RequestEvent{
			RequestID:   requestID,
			Operation:   operation,
			Method:      method,
			Path:        path,
			Attempt:     attempt,
			TokenLength: tokenLength,
			StartedAt:   startedAt,
		})

		req, err := c.buildRequest(ctx, method, path, body)
		if err != nil {
			wrapped := NewValidationError("build request", err)
			c.callOnError(ctx, requestID, operation, method, path, attempt, 0, 0, false, c.cfg.Clock.Now().Sub(startedAt), wrapped)
			return wrapped
		}

		resp, err := c.http.Do(req)
		if err != nil {
			latency := c.cfg.Clock.Now().Sub(startedAt)
			retryable := isTransportRetryable(err)
			wrapped := NewTransportError("do request", retryable, err)
			c.metrics.record(latency, true, false)
			c.breaker.markFailure()
			c.callOnError(ctx, requestID, operation, method, path, attempt, 0, 0, retryable, latency, wrapped)

			if retryable && attempt < c.cfg.RetryPolicy.MaxAttempts {
				if waitErr := c.waitBeforeRetry(ctx, attempt); waitErr != nil {
					return NewTransportError("retry canceled", false, waitErr)
				}
				lastErr = wrapped
				continue
			}

			return wrapped
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			latency := c.cfg.Clock.Now().Sub(startedAt)
			wrapped := NewTransportError("read response body", true, readErr)
			c.metrics.record(latency, true, false)
			c.breaker.markFailure()
			c.callOnError(ctx, requestID, operation, method, path, attempt, resp.StatusCode, 0, true, latency, wrapped)

			if attempt < c.cfg.RetryPolicy.MaxAttempts {
				if waitErr := c.waitBeforeRetry(ctx, attempt); waitErr != nil {
					return NewTransportError("retry canceled", false, waitErr)
				}
				lastErr = wrapped
				continue
			}

			return wrapped
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			latency := c.cfg.Clock.Now().Sub(startedAt)
			parsedErr, responseCode := c.parseErrorResponse(resp.StatusCode, respBody)
			retryable := c.isStatusRetryable(resp.StatusCode)
			wrapped := NewGatewayError("transbank API error", retryable, parsedErr)
			c.metrics.record(latency, true, operation == "confirm_inscription")
			c.breaker.markFailure()
			c.callOnError(ctx, requestID, operation, method, path, attempt, resp.StatusCode, responseCode, retryable, latency, wrapped)

			if retryable && attempt < c.cfg.RetryPolicy.MaxAttempts {
				if waitErr := c.waitBeforeRetry(ctx, attempt); waitErr != nil {
					return NewTransportError("retry canceled", false, waitErr)
				}
				lastErr = wrapped
				continue
			}

			return wrapped
		}

		if respTarget != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, respTarget); err != nil {
				latency := c.cfg.Clock.Now().Sub(startedAt)
				wrapped := NewGatewayError("unmarshal response", false, err)
				c.metrics.record(latency, true, operation == "confirm_inscription")
				c.breaker.markFailure()
				c.callOnError(ctx, requestID, operation, method, path, attempt, resp.StatusCode, 0, false, latency, wrapped)
				return wrapped
			}
		}

		latency := c.cfg.Clock.Now().Sub(startedAt)
		responseCode := extractResponseCode(respTarget)
		c.metrics.record(latency, false, false)
		c.breaker.markSuccess()
		c.callAfterRequest(ctx, ResponseEvent{
			RequestID:    requestID,
			Operation:    operation,
			Method:       method,
			Path:         path,
			Attempt:      attempt,
			StatusCode:   resp.StatusCode,
			Latency:      latency,
			ResponseCode: responseCode,
		})
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return NewTransportError("request failed after retries", false, nil)
}

// doRequestRaw makes an HTTP request without expecting a JSON response body.
func (c *Client) doRequestRaw(ctx context.Context, operation, method, path string, body interface{}, tokenLength int) error {
	return c.doRequest(ctx, operation, method, path, body, nil, tokenLength)
}

// buildRequest constructs an HTTP request with proper headers and authentication.
func (c *Client) buildRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	requestURL := c.cfg.baseURL + path
	var reqBody io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Tbk-Api-Key-Id", c.cfg.CommerceCode)
	req.Header.Set("Tbk-Api-Key-Secret", c.cfg.APISecret)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// parseErrorResponse parses an error response from Transbank.
func (c *Client) parseErrorResponse(statusCode int, respBody []byte) (error, int) {
	var errorResp struct {
		ResponseCode *int   `json:"response_code"`
		Error        string `json:"error"`
		Message      string `json:"message"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(respBody, &errorResp); err == nil {
		msg := firstNonEmpty(errorResp.ErrorMessage, errorResp.Message, errorResp.Error)
		if errorResp.ResponseCode != nil {
			if msg == "" {
				msg = http.StatusText(statusCode)
			}
			return NewTransbankErrorWithDetails(*errorResp.ResponseCode, msg, string(respBody), fmt.Errorf("http %d", statusCode)), *errorResp.ResponseCode
		}
		if msg != "" {
			return NewTransbankErrorWithDetails(statusCode, msg, string(respBody), fmt.Errorf("http %d", statusCode)), statusCode
		}
	}

	if len(respBody) > 0 {
		return fmt.Errorf("transbank API error (HTTP %d): %s", statusCode, string(respBody)), statusCode
	}

	return fmt.Errorf("transbank API error HTTP %d", statusCode), statusCode
}

func (c *Client) waitBeforeRetry(ctx context.Context, attempt int) error {
	backoff := c.cfg.RetryPolicy.InitialBackoff
	for i := 1; i < attempt; i++ {
		if backoff >= c.cfg.RetryPolicy.MaxBackoff {
			break
		}
		backoff = backoff * 2
		if backoff > c.cfg.RetryPolicy.MaxBackoff {
			backoff = c.cfg.RetryPolicy.MaxBackoff
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.cfg.Clock.After(backoff):
		return nil
	}
}

func (c *Client) isStatusRetryable(statusCode int) bool {
	_, ok := c.cfg.RetryPolicy.RetryOnStatus[statusCode]
	return ok
}

func (c *Client) nextRequestID() string {
	n := atomic.AddUint64(&c.requestN, 1)
	return fmt.Sprintf("oc_%d_%d", c.cfg.Clock.Now().UnixNano(), n)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(requestIDContextKey).(string)
	return strings.TrimSpace(value)
}

func operationFromContext(ctx context.Context, fallback string) string {
	if ctx == nil {
		return fallback
	}
	value, _ := ctx.Value(operationContextOverrideKey).(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func withRequestMeta(ctx context.Context, requestID, operation string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	requestID = strings.TrimSpace(requestID)
	if requestID != "" {
		ctx = context.WithValue(ctx, requestIDContextKey, requestID)
	}
	ctx = context.WithValue(ctx, operationContextOverrideKey, strings.TrimSpace(operation))
	return ctx
}

func (c *Client) contextWithRequestMeta(ctx context.Context, operation string) context.Context {
	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		requestID = c.nextRequestID()
	}
	return withRequestMeta(ctx, requestID, operation)
}

func (c *Client) callBeforeRequest(ctx context.Context, event RequestEvent) {
	if c.cfg.Hooks.BeforeRequest != nil {
		c.cfg.Hooks.BeforeRequest(ctx, event)
	}
}

func (c *Client) callAfterRequest(ctx context.Context, event ResponseEvent) {
	if c.cfg.Hooks.AfterRequest != nil {
		c.cfg.Hooks.AfterRequest(ctx, event)
	}
}

func (c *Client) callOnError(ctx context.Context, requestID, operation, method, path string, attempt, statusCode, responseCode int, retryable bool, latency time.Duration, err error) {
	if c.cfg.Hooks.OnError != nil {
		c.cfg.Hooks.OnError(ctx, ErrorEvent{
			RequestID:    requestID,
			Operation:    operation,
			Method:       method,
			Path:         path,
			Attempt:      attempt,
			StatusCode:   statusCode,
			ResponseCode: responseCode,
			Retryable:    retryable,
			Latency:      latency,
			Err:          err,
		})
	}
}

func isTransportRetryable(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "timeout") || strings.Contains(errText, "temporary") || strings.Contains(errText, "connection reset") {
		return true
	}
	return true
}

func extractResponseCode(respTarget interface{}) int {
	switch v := respTarget.(type) {
	case *InscriptionConfirmResponse:
		return v.ResponseCode
	case *CaptureResponse:
		return v.ResponseCode
	case *RefundResponse:
		return v.ResponseCode
	default:
		return 0
	}
}

func normalizeAndValidateDetails(details []TransactionDetail) ([]TransactionDetail, error) {
	if len(details) == 0 {
		return nil, ErrMissingDetails
	}

	normalized := make([]TransactionDetail, len(details))
	for i, detail := range details {
		if err := validateCommerceCode(detail.CommerceCode); err != nil {
			return nil, fmt.Errorf("transaction detail %d: %w", i, err)
		}
		if err := validateBuyOrder(detail.BuyOrder); err != nil {
			return nil, fmt.Errorf("transaction detail %d: %w", i, err)
		}
		if detail.Amount <= 0 {
			return nil, fmt.Errorf("transaction detail %d: %w", i, ErrInvalidAmount)
		}
		if detail.InstallmentsNumber > maxInstallmentsNumber {
			return nil, fmt.Errorf("transaction detail %d: %w", i, ErrInvalidInstallments)
		}

		if detail.InstallmentsNumber < 0 {
			detail.InstallmentsNumber = 0
		}
		normalized[i] = TransactionDetail{
			CommerceCode:       strings.TrimSpace(detail.CommerceCode),
			BuyOrder:           strings.TrimSpace(detail.BuyOrder),
			Amount:             detail.Amount,
			InstallmentsNumber: detail.InstallmentsNumber,
		}
	}

	return normalized, nil
}

func validateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrInvalidUsername
	}
	if len(username) > maxUsernameLength {
		return fmt.Errorf("username: %w", ErrMaxLengthExceeded)
	}
	return nil
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" || !strings.Contains(email, "@") {
		return ErrInvalidEmail
	}
	if len(email) > maxEmailLength {
		return fmt.Errorf("email: %w", ErrMaxLengthExceeded)
	}
	return nil
}

func validateResponseURL(responseURL string) error {
	responseURL = strings.TrimSpace(responseURL)
	if responseURL == "" {
		return ErrEmptyResponseURL
	}
	if len(responseURL) > maxResponseURLLength {
		return fmt.Errorf("response_url: %w", ErrMaxLengthExceeded)
	}

	u, err := url.Parse(responseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrEmptyResponseURL
	}

	return nil
}

func validateBaseURL(baseURL string) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ErrInvalidBaseURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("base URL must be absolute: %w", ErrInvalidBaseURL)
	}

	return nil
}

func validateTbkUser(tbkUser string) error {
	tbkUser = strings.TrimSpace(tbkUser)
	if tbkUser == "" {
		return ErrInvalidTbkUser
	}
	if len(tbkUser) > maxTbkUserLength {
		return fmt.Errorf("tbk_user: %w", ErrMaxLengthExceeded)
	}
	return nil
}

func validateCommerceCode(commerceCode string) error {
	commerceCode = strings.TrimSpace(commerceCode)
	if commerceCode == "" {
		return ErrInvalidCommerceCode
	}
	if len(commerceCode) > maxCommerceCodeLength {
		return fmt.Errorf("commerce_code: %w", ErrMaxLengthExceeded)
	}
	return nil
}

func validateAuthorizationCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrInvalidAuthCode
	}
	if len(code) > maxAuthorizationCodeLength {
		return fmt.Errorf("authorization_code: %w", ErrMaxLengthExceeded)
	}
	return nil
}

func validateBuyOrder(buyOrder string) error {
	buyOrder = strings.TrimSpace(buyOrder)
	if buyOrder == "" {
		return ErrInvalidBuyOrder
	}
	if len(buyOrder) > maxBuyOrderLength {
		return fmt.Errorf("buy_order: %w", ErrMaxLengthExceeded)
	}
	if !isValidBuyOrder(buyOrder) {
		return ErrInvalidBuyOrderFmt
	}
	return nil
}

func isValidBuyOrder(v string) bool {
	const allowedSymbols = "|_=&%.,~:/?[+!@()>-"

	for _, r := range v {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if strings.ContainsRune(allowedSymbols, r) {
			continue
		}
		return false
	}

	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
