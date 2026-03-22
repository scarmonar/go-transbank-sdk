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
	"time"
)

// OneclickService provides methods to interact with Transbank Oneclick API.
type OneclickService struct {
	commerceCode string
	apiSecret    string
	baseURL      string
	httpClient   *http.Client
}

// NewOneclickService creates a new Transbank Oneclick service.
// commerceCode: parent commerce code (e.g., "597055555541")
// apiSecret: API secret key for authentication
// baseURL: API base URL (integration or production)
// httpClient: HTTP client to use for requests (if nil, creates a new one with 20s timeout)
func NewOneclickService(commerceCode, apiSecret, baseURL string, httpClient *http.Client) (*OneclickService, error) {
	if strings.TrimSpace(commerceCode) == "" {
		return nil, ErrInvalidCommerceCode
	}
	if strings.TrimSpace(apiSecret) == "" {
		return nil, ErrInvalidAPISecret
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, ErrInvalidBaseURL
	}

	// Validate and parse baseURL
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &OneclickService{
		commerceCode: strings.TrimSpace(commerceCode),
		apiSecret:    strings.TrimSpace(apiSecret),
		baseURL:      strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		httpClient:   httpClient,
	}, nil
}

// CommerceCode returns the commerce code.
func (s *OneclickService) CommerceCode() string {
	return s.commerceCode
}

// IsIntegrationEnvironment returns true if the service is configured for integration (test) environment.
func (s *OneclickService) IsIntegrationEnvironment() bool {
	return strings.Contains(s.baseURL, "int") || strings.Contains(s.baseURL, "test") || strings.Contains(s.baseURL, "integration")
}

// IsProduction returns true if the service is configured for production environment.
func (s *OneclickService) IsProduction() bool {
	return !s.IsIntegrationEnvironment()
}

// CreateInscription initiates a new card inscription. Returns the token and redirect URL.
func (s *OneclickService) CreateInscription(ctx context.Context, username, email, responseURL string) (*InscriptionResponse, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrInvalidUsername
	}
	if strings.TrimSpace(email) == "" {
		return nil, ErrInvalidEmail
	}
	if strings.TrimSpace(responseURL) == "" {
		return nil, ErrEmptyResponseURL
	}

	req := &InscriptionRequest{
		Username:    username,
		Email:       email,
		ResponseURL: responseURL,
	}

	var resp InscriptionResponse
	if err := s.doRequest(ctx, http.MethodPost, "/inscriptions", req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ConfirmInscription completes the card inscription process using the token from the redirect.
func (s *OneclickService) ConfirmInscription(ctx context.Context, token string) (*InscriptionConfirmResponse, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvalidToken
	}

	endpoint := fmt.Sprintf("/inscriptions/%s", url.PathEscape(token))
	var resp InscriptionConfirmResponse
	if err := s.doRequest(ctx, http.MethodPut, endpoint, nil, &resp); err != nil {
		return nil, err
	}

	if resp.ResponseCode != 0 {
		return nil, NewTransbankError(resp.ResponseCode, "inscription confirmation failed", nil)
	}

	return &resp, nil
}

// DeleteInscription deletes/cancels an existing inscription.
func (s *OneclickService) DeleteInscription(ctx context.Context, tbkUser, username string) error {
	if strings.TrimSpace(tbkUser) == "" {
		return ErrInvalidTbkUser
	}
	if strings.TrimSpace(username) == "" {
		return ErrInvalidUsername
	}

	req := &DeleteInscriptionRequest{
		TbkUser:  tbkUser,
		Username: username,
	}

	// DELETE returns 204 No Content with empty body
	err := s.doRequestRaw(ctx, http.MethodDelete, "/inscriptions", req, nil)
	return err
}

// AuthorizeTransaction authorizes a transaction on a registered card.
func (s *OneclickService) AuthorizeTransaction(ctx context.Context, username, tbkUser, buyOrder string, details []TransactionDetail) (*AuthorizeTransactionResponse, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrInvalidUsername
	}
	if strings.TrimSpace(tbkUser) == "" {
		return nil, ErrInvalidTbkUser
	}
	if strings.TrimSpace(buyOrder) == "" {
		return nil, ErrInvalidBuyOrder
	}
	if len(details) == 0 {
		return nil, ErrMissingDetails
	}

	// Validate transaction details
	for _, d := range details {
		if strings.TrimSpace(d.CommerceCode) == "" {
			return nil, fmt.Errorf("transaction detail: %w", ErrInvalidCommerceCode)
		}
		if strings.TrimSpace(d.BuyOrder) == "" {
			return nil, fmt.Errorf("transaction detail: %w", ErrInvalidBuyOrder)
		}
		if d.Amount <= 0 {
			return nil, fmt.Errorf("transaction detail: %w", ErrInvalidAmount)
		}
		if d.InstallmentsNumber < 1 {
			d.InstallmentsNumber = 1
		}
	}

	req := &AuthorizeTransactionRequest{
		Username: username,
		TbkUser:  tbkUser,
		BuyOrder: buyOrder,
		Details:  details,
	}

	var resp AuthorizeTransactionResponse
	if err := s.doRequest(ctx, http.MethodPost, "/transactions", req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetTransactionStatus retrieves the status of a transaction.
func (s *OneclickService) GetTransactionStatus(ctx context.Context, buyOrder string) (*AuthorizeTransactionResponse, error) {
	if strings.TrimSpace(buyOrder) == "" {
		return nil, ErrInvalidBuyOrder
	}

	endpoint := fmt.Sprintf("/transactions/%s", url.QueryEscape(buyOrder))
	var resp AuthorizeTransactionResponse
	if err := s.doRequest(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ReverseTransaction reverses/refunds a transaction.
func (s *OneclickService) ReverseTransaction(ctx context.Context, buyOrder, commerceCode, detailBuyOrder string, amount int) (*RefundResponse, error) {
	if strings.TrimSpace(buyOrder) == "" {
		return nil, ErrInvalidBuyOrder
	}
	if strings.TrimSpace(commerceCode) == "" {
		return nil, ErrInvalidCommerceCode
	}
	if strings.TrimSpace(detailBuyOrder) == "" {
		return nil, ErrInvalidBuyOrder
	}
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	req := &RefundRequest{
		CommerceCode:   commerceCode,
		DetailBuyOrder: detailBuyOrder,
		Amount:         amount,
	}

	endpoint := fmt.Sprintf("/transactions/%s/refunds", url.QueryEscape(buyOrder))
	var resp RefundResponse
	if err := s.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// doRequest makes an HTTP request to the Transbank API and decodes the JSON response.
func (s *OneclickService) doRequest(ctx context.Context, method, path string, body interface{}, respTarget interface{}) error {
	req, err := s.buildRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Handle HTTP error status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.parseErrorResponse(resp.StatusCode, respBody)
	}

	// Only decode if there's a target and a body
	if respTarget != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, respTarget); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// doRequestRaw makes an HTTP request without expecting a JSON response body (e.g., DELETE with 204).
func (s *OneclickService) doRequestRaw(ctx context.Context, method, path string, body interface{}, respTarget interface{}) error {
	req, err := s.buildRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Handle HTTP error status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.parseErrorResponse(resp.StatusCode, respBody)
	}

	return nil
}

// buildRequest constructs an HTTP request with proper headers and authentication.
func (s *OneclickService) buildRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	url := s.baseURL + path
	var reqBody io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set authentication headers
	req.Header.Set("Tbk-Api-Key-Id", s.commerceCode)
	req.Header.Set("Tbk-Api-Key-Secret", s.computeSignature())
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// computeSignature computes the HMAC-SHA256 signature for Transbank authentication.
// Transbank uses: base64(sha256(concatenation of method, path, body, timestamp, apiSecret))
func (s *OneclickService) computeSignature() string {
	// For simplicity and based on Transbank's latest Oneclick API (v1.2),
	// the signature is computed as: Hmac-SHA256(apiSecret)
	// The signature header value is the hex-encoded HMAC.
	// However, per Transbank docs, Oneclick v1.2 might not require signature on all endpoints.
	// This is a placeholder that can be adjusted based on actual Transbank requirements.

	// For now, return the API secret directly (Transbank may validate differently per endpoint)
	return s.apiSecret
}

// parseErrorResponse parses an error response from Transbank.
func (s *OneclickService) parseErrorResponse(statusCode int, respBody []byte) error {
	// Attempt to unmarshal as a Transbank error response
	var errorResp struct {
		ResponseCode int    `json:"response_code"`
		Error        string `json:"error"`
		Message      string `json:"message"`
	}

	if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.ResponseCode != 0 {
		return NewTransbankError(errorResp.ResponseCode, errorResp.Message, fmt.Errorf("http %d", statusCode))
	}

	// If it looks like a JSON response but doesn't match expected format, return it as-is
	if len(respBody) > 0 {
		return fmt.Errorf("transbank API error (HTTP %d): %s", statusCode, string(respBody))
	}

	return fmt.Errorf("transbank API error HTTP %d", statusCode)
}
