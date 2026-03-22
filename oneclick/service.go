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

// OneclickService provides methods to interact with the Transbank Oneclick API.
type OneclickService struct {
	commerceCode string
	apiSecret    string
	baseURL      string
	httpClient   *http.Client
}

// NewOneclickService creates a new Transbank Oneclick service.
// commerceCode: parent commerce code (for example: "597055555541")
// apiSecret: API secret key for authentication
// baseURL: API base URL (integration or production)
// httpClient: HTTP client to use for requests (if nil, a client with 30s timeout is used)
func NewOneclickService(commerceCode, apiSecret, baseURL string, httpClient *http.Client) (*OneclickService, error) {
	if err := validateCommerceCode(commerceCode); err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiSecret) == "" {
		return nil, ErrInvalidAPISecret
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, ErrInvalidBaseURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base URL must be absolute: %w", ErrInvalidBaseURL)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &OneclickService{
		commerceCode: strings.TrimSpace(commerceCode),
		apiSecret:    strings.TrimSpace(apiSecret),
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		httpClient:   httpClient,
	}, nil
}

// CommerceCode returns the configured commerce code.
func (s *OneclickService) CommerceCode() string {
	return s.commerceCode
}

// IsIntegrationEnvironment returns true if the service appears configured for integration.
func (s *OneclickService) IsIntegrationEnvironment() bool {
	base := strings.ToLower(s.baseURL)
	return strings.Contains(base, "int") || strings.Contains(base, "test") || strings.Contains(base, "integration")
}

// IsProduction returns true if the service appears configured for production.
func (s *OneclickService) IsProduction() bool {
	return !s.IsIntegrationEnvironment()
}

// Start initiates a new inscription and returns token + Webpay redirect URL.
func (s *OneclickService) Start(ctx context.Context, username, email, responseURL string) (*InscriptionResponse, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validateResponseURL(responseURL); err != nil {
		return nil, err
	}

	req := &InscriptionRequest{
		Username:    strings.TrimSpace(username),
		Email:       strings.TrimSpace(email),
		ResponseURL: strings.TrimSpace(responseURL),
	}

	var resp InscriptionResponse
	if err := s.doRequest(ctx, http.MethodPost, "/inscriptions", req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Finish confirms an inscription using the token returned to the commerce callback URL.
func (s *OneclickService) Finish(ctx context.Context, token string) (*InscriptionConfirmResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalidToken
	}

	endpoint := fmt.Sprintf("/inscriptions/%s", url.PathEscape(token))
	var resp InscriptionConfirmResponse
	// Transbank requires an explicit empty JSON body for this endpoint.
	if err := s.doRequest(ctx, http.MethodPut, endpoint, struct{}{}, &resp); err != nil {
		return nil, err
	}

	if resp.ResponseCode != ResponseCodeSuccess {
		return nil, NewTransbankError(resp.ResponseCode, "inscription confirmation failed", nil)
	}

	return &resp, nil
}

// Remove deletes an existing inscription.
func (s *OneclickService) Remove(ctx context.Context, tbkUser, username string) error {
	if err := validateTbkUser(tbkUser); err != nil {
		return err
	}
	if err := validateUsername(username); err != nil {
		return err
	}

	req := &DeleteInscriptionRequest{
		TbkUser:  strings.TrimSpace(tbkUser),
		Username: strings.TrimSpace(username),
	}

	return s.doRequestRaw(ctx, http.MethodDelete, "/inscriptions", req)
}

// Authorize authorizes one or many child transactions for an enrolled user.
func (s *OneclickService) Authorize(ctx context.Context, username, tbkUser, buyOrder string, details []TransactionDetail) (*AuthorizeTransactionResponse, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validateTbkUser(tbkUser); err != nil {
		return nil, err
	}
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, err
	}

	normalizedDetails, err := normalizeAndValidateDetails(details)
	if err != nil {
		return nil, err
	}

	req := &AuthorizeTransactionRequest{
		Username: strings.TrimSpace(username),
		TbkUser:  strings.TrimSpace(tbkUser),
		BuyOrder: strings.TrimSpace(buyOrder),
		Details:  normalizedDetails,
	}

	var resp AuthorizeTransactionResponse
	if err := s.doRequest(ctx, http.MethodPost, "/transactions", req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Status retrieves the status of a previously authorized transaction.
func (s *OneclickService) Status(ctx context.Context, buyOrder string) (*AuthorizeTransactionResponse, error) {
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/transactions/%s", url.PathEscape(strings.TrimSpace(buyOrder)))
	var resp AuthorizeTransactionResponse
	if err := s.doRequest(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Refund reverses or nullifies a previously authorized transaction.
func (s *OneclickService) Refund(ctx context.Context, buyOrder, commerceCode, detailBuyOrder string, amount int) (*RefundResponse, error) {
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, err
	}
	if err := validateCommerceCode(commerceCode); err != nil {
		return nil, err
	}
	if err := validateBuyOrder(detailBuyOrder); err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	req := &RefundRequest{
		CommerceCode:   strings.TrimSpace(commerceCode),
		DetailBuyOrder: strings.TrimSpace(detailBuyOrder),
		Amount:         amount,
	}

	endpoint := fmt.Sprintf("/transactions/%s/refunds", url.PathEscape(strings.TrimSpace(buyOrder)))
	var resp RefundResponse
	if err := s.doRequest(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Capture performs a deferred capture for a previously authorized child transaction.
func (s *OneclickService) Capture(ctx context.Context, commerceCode, buyOrder, authorizationCode string, captureAmount int) (*CaptureResponse, error) {
	if err := validateCommerceCode(commerceCode); err != nil {
		return nil, err
	}
	if err := validateBuyOrder(buyOrder); err != nil {
		return nil, err
	}
	if err := validateAuthorizationCode(authorizationCode); err != nil {
		return nil, err
	}
	if captureAmount <= 0 {
		return nil, ErrInvalidCaptureAmount
	}

	req := &CaptureRequest{
		CommerceCode:      strings.TrimSpace(commerceCode),
		BuyOrder:          strings.TrimSpace(buyOrder),
		CaptureAmount:     captureAmount,
		AuthorizationCode: strings.TrimSpace(authorizationCode),
	}

	var resp CaptureResponse
	if err := s.doRequest(ctx, http.MethodPut, "/transactions/capture", req, &resp); err != nil {
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.parseErrorResponse(resp.StatusCode, respBody)
	}

	if respTarget != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, respTarget); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// doRequestRaw makes an HTTP request without expecting a JSON response body.
func (s *OneclickService) doRequestRaw(ctx context.Context, method, path string, body interface{}) error {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.parseErrorResponse(resp.StatusCode, respBody)
	}

	return nil
}

// buildRequest constructs an HTTP request with proper headers and authentication.
func (s *OneclickService) buildRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	requestURL := s.baseURL + path
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

	req.Header.Set("Tbk-Api-Key-Id", s.commerceCode)
	req.Header.Set("Tbk-Api-Key-Secret", s.apiKeySecret())
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (s *OneclickService) apiKeySecret() string {
	// For Oneclick v1.2, this header is the configured API secret.
	return s.apiSecret
}

// parseErrorResponse parses an error response from Transbank.
func (s *OneclickService) parseErrorResponse(statusCode int, respBody []byte) error {
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
			return NewTransbankErrorWithDetails(*errorResp.ResponseCode, msg, string(respBody), fmt.Errorf("http %d", statusCode))
		}
		if msg != "" {
			return NewTransbankErrorWithDetails(statusCode, msg, string(respBody), fmt.Errorf("http %d", statusCode))
		}
	}

	if len(respBody) > 0 {
		return fmt.Errorf("transbank API error (HTTP %d): %s", statusCode, string(respBody))
	}

	return fmt.Errorf("transbank API error HTTP %d", statusCode)
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
