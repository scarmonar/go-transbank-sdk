package oneclick

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	idempotencyOpStart           = "start_inscription"
	idempotencyOpConfirm         = "confirm_inscription"
	idempotencyOpAuthorizeCharge = "flow_authorize_charge"
	idempotencyOpReverseCharge   = "flow_reverse_charge"
)

// ReturnURLMode controls URL validation strictness when building return URLs.
type ReturnURLMode string

const (
	// ReturnURLModeStrictHTTPS requires https scheme.
	ReturnURLModeStrictHTTPS ReturnURLMode = "strict_https"
	// ReturnURLModeAllowHTTPLocalhost allows http only for localhost / loopback.
	ReturnURLModeAllowHTTPLocalhost ReturnURLMode = "allow_http_localhost"
)

// FlowStatus identifies current inscription flow state.
type FlowStatus string

const (
	FlowStatusPending   FlowStatus = "pending"
	FlowStatusConfirmed FlowStatus = "confirmed"
)

// RedirectInstruction contains everything needed by frontend/backend to redirect to Webpay.
type RedirectInstruction struct {
	URLWebpay  string            `json:"url_webpay"`
	Token      string            `json:"token"`
	Method     string            `json:"method"`
	FormFields map[string]string `json:"form_fields"`
}

// FlowState stores inscription process context keyed by token.
type FlowState struct {
	Token          string                      `json:"token"`
	Username       string                      `json:"username"`
	Email          string                      `json:"email"`
	ResponseURL    string                      `json:"response_url"`
	URLWebpay      string                      `json:"url_webpay"`
	BusinessID     string                      `json:"business_id,omitempty"`
	SubscriptionID string                      `json:"subscription_id,omitempty"`
	Context        map[string]string           `json:"context,omitempty"`
	Status         FlowStatus                  `json:"status"`
	CreatedAt      string                      `json:"created_at"`
	UpdatedAt      string                      `json:"updated_at"`
	Confirmation   *InscriptionConfirmResponse `json:"confirmation,omitempty"`
}

// StateStore persists flow state by token.
type StateStore interface {
	GetByToken(ctx context.Context, token string) (*FlowState, error)
	SavePending(ctx context.Context, state FlowState) error
	MarkConfirmed(ctx context.Context, token string, confirmation InscriptionConfirmResponse) (*FlowState, error)
}

// IdempotencyRecord stores cached results for idempotent operations.
type IdempotencyRecord struct {
	Operation               string
	Key                     string
	StartResponse           *FlowStartResponse
	ConfirmResponse         *FlowConfirmResponse
	AuthorizeChargeResponse *FlowAuthorizeChargeResponse
	ReverseChargeResponse   *FlowReverseChargeResponse
}

// IdempotencyStore resolves repeated requests by operation + key.
type IdempotencyStore interface {
	Get(ctx context.Context, operation, key string) (*IdempotencyRecord, error)
	Save(ctx context.Context, record IdempotencyRecord) error
}

// FlowStartRequest describes the high-level inscription start input.
type FlowStartRequest struct {
	Username       string
	Email          string
	ResponseURL    string
	BaseURL        string
	ReturnPath     string
	ReturnMode     ReturnURLMode
	BusinessID     string
	SubscriptionID string
	Context        map[string]string
	IdempotencyKey string
}

// FlowStartResponse is returned by FlowService.StartInscription.
type FlowStartResponse struct {
	Token     string              `json:"token"`
	URLWebpay string              `json:"url_webpay"`
	Redirect  RedirectInstruction `json:"redirect"`
	State     FlowState           `json:"state"`
}

// FlowConfirmRequest describes inscription confirmation input.
type FlowConfirmRequest struct {
	Token          string
	IdempotencyKey string
}

// FlowConfirmResponse is returned by FlowService.ConfirmInscription.
type FlowConfirmResponse struct {
	State          FlowState                  `json:"state"`
	Confirmation   InscriptionConfirmResponse `json:"confirmation"`
	BusinessID     string                     `json:"business_id,omitempty"`
	SubscriptionID string                     `json:"subscription_id,omitempty"`
	Context        map[string]string          `json:"context,omitempty"`
}

// FlowAuthorizeChargeRequest describes the high-level authorize input.
// TokenOrTbkUser accepts either:
// - a flow token to resolve tbk_user and username from confirmed state, or
// - a tbk_user value directly.
type FlowAuthorizeChargeRequest struct {
	TokenOrTbkUser string
	Username       string
	BuyOrder       string
	Details        []TransactionDetail
	IdempotencyKey string
}

// FlowAuthorizeChargeResponse contains normalized authorization data.
type FlowAuthorizeChargeResponse struct {
	BuyOrder        string                      `json:"buy_order"`
	TransactionDate time.Time                   `json:"transaction_date"`
	Details         []TransactionResponseDetail `json:"details"`
	State           *FlowState                  `json:"state,omitempty"`
}

// FlowReverseChargeRequest describes high-level reverse/refund input.
type FlowReverseChargeRequest struct {
	BuyOrder       string
	CommerceCode   string
	DetailBuyOrder string
	Amount         int
	IdempotencyKey string
}

// FlowReverseChargeResponse contains reverse/refund output.
type FlowReverseChargeResponse struct {
	Refund RefundResponse `json:"refund"`
}

// FlowOption customizes FlowService behavior.
type FlowOption func(*FlowService) error

// FlowService orchestrates end-to-end inscription flow with state and idempotency.
type FlowService struct {
	client            *Client
	store             StateStore
	idempotency       IdempotencyStore
	defaultReturnMode ReturnURLMode
}

// NewFlowService creates a high-level flow service.
func NewFlowService(client *Client, store StateStore, opts ...FlowOption) (*FlowService, error) {
	if client == nil {
		return nil, NewValidationError("client cannot be nil", nil)
	}
	if store == nil {
		store = NewInMemoryStateStore()
	}

	service := &FlowService{
		client:            client,
		store:             store,
		idempotency:       NewInMemoryIdempotencyStore(),
		defaultReturnMode: ReturnURLModeStrictHTTPS,
	}

	for _, opt := range opts {
		if err := opt(service); err != nil {
			return nil, err
		}
	}

	return service, nil
}

// WithIdempotencyStore overrides idempotency persistence.
func WithIdempotencyStore(store IdempotencyStore) FlowOption {
	return func(s *FlowService) error {
		if store == nil {
			return NewValidationError("idempotency store cannot be nil", nil)
		}
		s.idempotency = store
		return nil
	}
}

// WithDefaultReturnURLMode overrides return URL generation mode.
func WithDefaultReturnURLMode(mode ReturnURLMode) FlowOption {
	return func(s *FlowService) error {
		s.defaultReturnMode = mode
		return nil
	}
}

// BuildReturnURL builds a secure return URL for inscription callbacks.
func (s *FlowService) BuildReturnURL(baseURL, returnPath string, mode ReturnURLMode) (string, error) {
	return BuildReturnURL(baseURL, returnPath, mode)
}

// BuildReturnURL normalizes and validates callback URLs.
func BuildReturnURL(baseURL, returnPath string, mode ReturnURLMode) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", NewValidationError("base URL is required", ErrInvalidBaseURL)
	}

	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", NewValidationError("base URL must be absolute", ErrInvalidBaseURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.RawQuery = ""
	u.Fragment = ""

	if mode == "" {
		mode = ReturnURLModeStrictHTTPS
	}

	switch mode {
	case ReturnURLModeStrictHTTPS:
		if u.Scheme != "https" {
			return "", NewValidationError("return URL must use https", ErrEmptyResponseURL)
		}
	case ReturnURLModeAllowHTTPLocalhost:
		if u.Scheme == "http" {
			host := u.Hostname()
			if !isLocalhost(host) {
				return "", NewValidationError("http is allowed only for localhost", ErrEmptyResponseURL)
			}
		} else if u.Scheme != "https" {
			return "", NewValidationError("unsupported return URL scheme", ErrEmptyResponseURL)
		}
	default:
		return "", NewValidationError("invalid return URL mode", ErrValidation)
	}

	returnPath = strings.TrimSpace(returnPath)
	if returnPath != "" {
		if strings.Contains(returnPath, "://") {
			return "", NewValidationError("return path must be a path, not an URL", ErrEmptyResponseURL)
		}
		if idx := strings.IndexAny(returnPath, "?#"); idx >= 0 {
			returnPath = returnPath[:idx]
		}
		returnPath = "/" + strings.TrimLeft(returnPath, "/")
		cleanPath := path.Clean(returnPath)
		if cleanPath == "." {
			cleanPath = "/"
		}
		u.Path = cleanPath
	} else {
		if strings.TrimSpace(u.Path) == "" {
			u.Path = "/"
		}
		u.Path = path.Clean("/" + strings.TrimLeft(u.Path, "/"))
	}

	finalURL := u.String()
	if len(finalURL) > maxResponseURLLength {
		return "", NewValidationError("return URL exceeds max length", ErrMaxLengthExceeded)
	}
	if err := validateResponseURL(finalURL); err != nil {
		return "", NewValidationError("invalid return URL", err)
	}

	return finalURL, nil
}

// BuildRedirectInstruction builds a Transbank POST redirect instruction.
func (s *FlowService) BuildRedirectInstruction(resp *InscriptionResponse) (RedirectInstruction, error) {
	return BuildRedirectInstruction(resp)
}

// BuildRedirectInstruction builds a Transbank POST redirect instruction.
func BuildRedirectInstruction(resp *InscriptionResponse) (RedirectInstruction, error) {
	if resp == nil {
		return RedirectInstruction{}, NewValidationError("inscription response cannot be nil", nil)
	}
	token := strings.TrimSpace(resp.Token)
	urlWebpay := strings.TrimSpace(resp.URLWebpay)
	if token == "" {
		return RedirectInstruction{}, NewValidationError("token is required", ErrInvalidToken)
	}
	if urlWebpay == "" {
		return RedirectInstruction{}, NewValidationError("url_webpay is required", ErrEmptyResponseURL)
	}
	parsedURL, err := url.Parse(urlWebpay)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return RedirectInstruction{}, NewValidationError("url_webpay must be absolute", ErrEmptyResponseURL)
	}

	return RedirectInstruction{
		URLWebpay: urlWebpay,
		Token:     token,
		Method:    httpMethodPost,
		FormFields: map[string]string{
			"TBK_TOKEN": token,
		},
	}, nil
}

const httpMethodPost = "POST"

// StartInscription orchestrates inscription start + state persistence + idempotency.
func (s *FlowService) StartInscription(ctx context.Context, req FlowStartRequest) (*FlowStartResponse, error) {
	ctx = s.client.contextWithRequestMeta(ctx, "")

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		record, err := s.idempotency.Get(ctx, idempotencyOpStart, idempotencyKey)
		if err != nil {
			return nil, NewFlowStateError("idempotency lookup failed", true, err)
		}
		if record != nil && record.StartResponse != nil {
			cached := *record.StartResponse
			cached.State = cloneFlowState(cached.State)
			return &cached, nil
		}
	}

	responseURL := strings.TrimSpace(req.ResponseURL)
	if responseURL == "" {
		mode := req.ReturnMode
		if mode == "" {
			mode = s.defaultReturnMode
		}
		builtURL, err := BuildReturnURL(req.BaseURL, req.ReturnPath, mode)
		if err != nil {
			return nil, err
		}
		responseURL = builtURL
	}

	rawResp, err := s.client.StartInscription(ctx, InscriptionRequest{
		Username:    req.Username,
		Email:       req.Email,
		ResponseURL: responseURL,
	})
	if err != nil {
		return nil, err
	}

	redirect, err := BuildRedirectInstruction(rawResp)
	if err != nil {
		return nil, err
	}

	now := s.client.cfg.Clock.Now().UTC().Format(timeLayout)
	state := FlowState{
		Token:          strings.TrimSpace(rawResp.Token),
		Username:       strings.TrimSpace(req.Username),
		Email:          strings.TrimSpace(req.Email),
		ResponseURL:    responseURL,
		URLWebpay:      strings.TrimSpace(rawResp.URLWebpay),
		BusinessID:     strings.TrimSpace(req.BusinessID),
		SubscriptionID: strings.TrimSpace(req.SubscriptionID),
		Context:        cloneStringMap(req.Context),
		Status:         FlowStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.store.SavePending(ctx, state); err != nil {
		return nil, NewFlowStateError("save pending flow state", true, err)
	}

	response := &FlowStartResponse{
		Token:     state.Token,
		URLWebpay: state.URLWebpay,
		Redirect:  redirect,
		State:     cloneFlowState(state),
	}

	if idempotencyKey != "" {
		err := s.idempotency.Save(ctx, IdempotencyRecord{
			Operation:     idempotencyOpStart,
			Key:           idempotencyKey,
			StartResponse: response,
		})
		if err != nil {
			return nil, NewFlowStateError("save idempotency record", true, err)
		}
	}

	return response, nil
}

// ConfirmInscription resolves token context, confirms inscription and updates state.
func (s *FlowService) ConfirmInscription(ctx context.Context, req FlowConfirmRequest) (*FlowConfirmResponse, error) {
	ctx = s.client.contextWithRequestMeta(ctx, "")

	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, NewValidationError("token is required", ErrInvalidToken)
	}

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		record, err := s.idempotency.Get(ctx, idempotencyOpConfirm, idempotencyKey)
		if err != nil {
			return nil, NewFlowStateError("idempotency lookup failed", true, err)
		}
		if record != nil && record.ConfirmResponse != nil {
			cached := *record.ConfirmResponse
			cached.State = cloneFlowState(cached.State)
			cached.Context = cloneStringMap(cached.Context)
			return &cached, nil
		}
	}

	state, err := s.store.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrStateStoreNotFound) {
			return nil, NewTokenNotFoundError(token, err)
		}
		return nil, NewFlowStateError("load flow state", true, err)
	}

	if state.Status == FlowStatusConfirmed && state.Confirmation != nil {
		response := flowConfirmResponseFromState(*state)
		return response, nil
	}

	confirmation, err := s.client.ConfirmInscription(ctx, token)
	if err != nil {
		return nil, err
	}

	updated, err := s.store.MarkConfirmed(ctx, token, *confirmation)
	if err != nil {
		if errors.Is(err, ErrStateStoreNotFound) {
			return nil, NewTokenNotFoundError(token, err)
		}
		return nil, NewFlowStateError("mark flow as confirmed", true, err)
	}

	response := flowConfirmResponseFromState(*updated)

	if idempotencyKey != "" {
		err := s.idempotency.Save(ctx, IdempotencyRecord{
			Operation:       idempotencyOpConfirm,
			Key:             idempotencyKey,
			ConfirmResponse: response,
		})
		if err != nil {
			return nil, NewFlowStateError("save idempotency record", true, err)
		}
	}

	return response, nil
}

// AuthorizeCharge resolves identity from flow state when possible and authorizes a charge.
func (s *FlowService) AuthorizeCharge(ctx context.Context, req FlowAuthorizeChargeRequest) (*FlowAuthorizeChargeResponse, error) {
	ctx = s.client.contextWithRequestMeta(ctx, idempotencyOpAuthorizeCharge)

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		record, err := s.idempotency.Get(ctx, idempotencyOpAuthorizeCharge, idempotencyKey)
		if err != nil {
			return nil, NewFlowStateError("idempotency lookup failed", true, err)
		}
		if record != nil && record.AuthorizeChargeResponse != nil {
			cached := *record.AuthorizeChargeResponse
			if cached.State != nil {
				state := cloneFlowState(*cached.State)
				cached.State = &state
			}
			cached.Details = append([]TransactionResponseDetail(nil), cached.Details...)
			return &cached, nil
		}
	}

	username, tbkUser, resolvedState, err := s.resolveChargeIdentity(ctx, req.TokenOrTbkUser, req.Username)
	if err != nil {
		return nil, err
	}

	if err := validateBuyOrder(req.BuyOrder); err != nil {
		return nil, NewValidationError("invalid buy_order", err)
	}
	if _, err := normalizeAndValidateDetails(req.Details); err != nil {
		return nil, NewValidationError("invalid transaction details", err)
	}

	rawResp, err := s.client.AuthorizeTransaction(ctx, AuthorizeTransactionRequest{
		Username: username,
		TbkUser:  tbkUser,
		BuyOrder: req.BuyOrder,
		Details:  req.Details,
	})
	if err != nil {
		return nil, err
	}

	response := &FlowAuthorizeChargeResponse{
		BuyOrder:        rawResp.BuyOrder,
		TransactionDate: rawResp.TransactionDate,
		Details:         append([]TransactionResponseDetail(nil), rawResp.Details...),
	}
	if resolvedState != nil {
		cloned := cloneFlowState(*resolvedState)
		response.State = &cloned
	}

	if idempotencyKey != "" {
		if err := s.idempotency.Save(ctx, IdempotencyRecord{
			Operation:               idempotencyOpAuthorizeCharge,
			Key:                     idempotencyKey,
			AuthorizeChargeResponse: response,
		}); err != nil {
			return nil, NewFlowStateError("save idempotency record", true, err)
		}
	}

	return response, nil
}

// ReverseCharge performs a reverse/refund operation.
func (s *FlowService) ReverseCharge(ctx context.Context, req FlowReverseChargeRequest) (*FlowReverseChargeResponse, error) {
	ctx = s.client.contextWithRequestMeta(ctx, idempotencyOpReverseCharge)

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		record, err := s.idempotency.Get(ctx, idempotencyOpReverseCharge, idempotencyKey)
		if err != nil {
			return nil, NewFlowStateError("idempotency lookup failed", true, err)
		}
		if record != nil && record.ReverseChargeResponse != nil {
			cached := *record.ReverseChargeResponse
			return &cached, nil
		}
	}

	if err := validateBuyOrder(req.BuyOrder); err != nil {
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

	rawResp, err := s.client.RefundTransaction(ctx, req.BuyOrder, RefundRequest{
		CommerceCode:   req.CommerceCode,
		DetailBuyOrder: req.DetailBuyOrder,
		Amount:         req.Amount,
	})
	if err != nil {
		return nil, err
	}

	response := &FlowReverseChargeResponse{
		Refund: *rawResp,
	}
	if idempotencyKey != "" {
		if err := s.idempotency.Save(ctx, IdempotencyRecord{
			Operation:             idempotencyOpReverseCharge,
			Key:                   idempotencyKey,
			ReverseChargeResponse: response,
		}); err != nil {
			return nil, NewFlowStateError("save idempotency record", true, err)
		}
	}

	return response, nil
}

func (s *FlowService) resolveChargeIdentity(ctx context.Context, tokenOrTbkUser, username string) (string, string, *FlowState, error) {
	tokenOrTbkUser = strings.TrimSpace(tokenOrTbkUser)
	if tokenOrTbkUser == "" {
		return "", "", nil, NewValidationError("token_or_tbk_user is required", ErrInvalidToken)
	}

	trimmedUsername := strings.TrimSpace(username)
	state, err := s.store.GetByToken(ctx, tokenOrTbkUser)
	if err != nil {
		if !errors.Is(err, ErrStateStoreNotFound) {
			return "", "", nil, NewFlowStateError("load flow state", true, err)
		}

		if trimmedUsername == "" {
			return "", "", nil, NewValidationError("username is required when token_or_tbk_user is a tbk_user value", ErrInvalidUsername)
		}
		if err := validateUsername(trimmedUsername); err != nil {
			return "", "", nil, NewValidationError("invalid username", err)
		}
		if err := validateTbkUser(tokenOrTbkUser); err != nil {
			return "", "", nil, NewValidationError("invalid token_or_tbk_user", err)
		}
		return trimmedUsername, tokenOrTbkUser, nil, nil
	}

	if state.Status != FlowStatusConfirmed || state.Confirmation == nil {
		return "", "", nil, NewFlowStateError("flow state is not confirmed", false, ErrFlowState)
	}

	resolvedUsername := strings.TrimSpace(state.Username)
	if trimmedUsername != "" {
		if err := validateUsername(trimmedUsername); err != nil {
			return "", "", nil, NewValidationError("invalid username", err)
		}
		if resolvedUsername != "" && trimmedUsername != resolvedUsername {
			return "", "", nil, NewValidationError("username does not match confirmed flow state", ErrInvalidUsername)
		}
		resolvedUsername = trimmedUsername
	}
	if resolvedUsername == "" {
		return "", "", nil, NewFlowStateError("confirmed flow state missing username", false, ErrFlowState)
	}
	if err := validateUsername(resolvedUsername); err != nil {
		return "", "", nil, NewValidationError("invalid username", err)
	}

	tbkUser := strings.TrimSpace(state.Confirmation.TbkUser)
	if err := validateTbkUser(tbkUser); err != nil {
		return "", "", nil, NewFlowStateError("confirmed flow state missing tbk_user", false, ErrFlowState)
	}

	cloned := cloneFlowState(*state)
	return resolvedUsername, tbkUser, &cloned, nil
}

func flowConfirmResponseFromState(state FlowState) *FlowConfirmResponse {
	cloned := cloneFlowState(state)
	response := &FlowConfirmResponse{
		State:          cloned,
		BusinessID:     strings.TrimSpace(cloned.BusinessID),
		SubscriptionID: strings.TrimSpace(cloned.SubscriptionID),
		Context:        cloneStringMap(cloned.Context),
	}
	if cloned.Confirmation != nil {
		response.Confirmation = *cloned.Confirmation
	}
	return response
}

func cloneFlowState(state FlowState) FlowState {
	cloned := state
	cloned.Context = cloneStringMap(state.Context)
	if state.Confirmation != nil {
		copied := *state.Confirmation
		cloned.Confirmation = &copied
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return cloned
}

func isLocalhost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"
