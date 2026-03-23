package oneclick

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout         = 30 * time.Second
	defaultRetryMaxAttempts    = 3
	defaultRetryInitialBackoff = 200 * time.Millisecond
	defaultRetryMaxBackoff     = 2 * time.Second
	defaultCBFailureThreshold  = 5
	defaultCBCooldown          = 30 * time.Second
)

const (
	defaultIntegrationBaseURL = "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2"
	defaultProductionBaseURL  = "https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2"
)

const (
	// DefaultIntegrationCommerceCode is the public commerce code for Transbank integration testing.
	DefaultIntegrationCommerceCode = "597055555541"
	// DefaultIntegrationAPISecret is the public API secret for Transbank integration testing.
	DefaultIntegrationAPISecret = "579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C"
)

// Environment identifies the target Oneclick API environment.
type Environment string

const (
	// EnvironmentIntegration points to Transbank integration endpoints.
	EnvironmentIntegration Environment = "integration"
	// EnvironmentProduction points to Transbank production endpoints.
	EnvironmentProduction Environment = "production"
)

// IsValid returns true when the environment value is recognized.
func (e Environment) IsValid() bool {
	return e == EnvironmentIntegration || e == EnvironmentProduction
}

// RetryPolicy controls retry behavior on transport and transient gateway errors.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	RetryOnStatus  map[int]struct{}
}

// CircuitBreakerPolicy controls a simple consecutive-failure breaker.
type CircuitBreakerPolicy struct {
	FailureThreshold int
	Cooldown         time.Duration
}

// Logger defines the minimum logging contract used by this SDK.
type Logger interface {
	Printf(format string, args ...interface{})
}

// Clock allows deterministic tests and custom time behavior.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

type noopLogger struct{}

func (noopLogger) Printf(string, ...interface{}) {}

// Hooks provides observability lifecycle callbacks.
// Each outbound API request triggers:
// 1) BeforeRequest
// 2) AfterRequest on success OR OnError on failure.
//
// Operation values emitted include:
// - start_inscription
// - confirm_inscription
// - delete_inscription
// - authorize_transaction
// - transaction_status
// - refund_transaction
// - capture_transaction
// - flow_authorize_charge (FlowService.AuthorizeCharge)
// - flow_reverse_charge (FlowService.ReverseCharge)
//
// request_id is stable per operation and reused across retries.
type Hooks struct {
	BeforeRequest func(ctx context.Context, event RequestEvent)
	AfterRequest  func(ctx context.Context, event ResponseEvent)
	OnError       func(ctx context.Context, event ErrorEvent)
}

// RequestEvent carries request-level metadata.
type RequestEvent struct {
	RequestID   string
	Operation   string
	Method      string
	Path        string
	Attempt     int
	TokenLength int
	StartedAt   time.Time
}

// ResponseEvent carries response-level metadata.
type ResponseEvent struct {
	RequestID    string
	Operation    string
	Method       string
	Path         string
	Attempt      int
	StatusCode   int
	Latency      time.Duration
	ResponseCode int
}

// ErrorEvent carries failure metadata.
type ErrorEvent struct {
	RequestID    string
	Operation    string
	Method       string
	Path         string
	Attempt      int
	StatusCode   int
	ResponseCode int
	Retryable    bool
	Latency      time.Duration
	Err          error
}

// Config centralizes client settings and defaults.
type Config struct {
	Environment    Environment
	CommerceCode   string
	APISecret      string
	baseURL        string
	HTTPClient     *http.Client
	Timeout        time.Duration
	RetryPolicy    RetryPolicy
	CircuitBreaker CircuitBreakerPolicy
	Logger         Logger
	Clock          Clock
	Hooks          Hooks
}

// DefaultConfig returns a production-safe baseline with integration-friendly fallback behavior.
func DefaultConfig() Config {
	return Config{
		Timeout: defaultHTTPTimeout,
		RetryPolicy: RetryPolicy{
			MaxAttempts:    defaultRetryMaxAttempts,
			InitialBackoff: defaultRetryInitialBackoff,
			MaxBackoff:     defaultRetryMaxBackoff,
			RetryOnStatus: map[int]struct{}{
				http.StatusRequestTimeout:      {},
				http.StatusTooEarly:            {},
				http.StatusTooManyRequests:     {},
				http.StatusInternalServerError: {},
				http.StatusBadGateway:          {},
				http.StatusServiceUnavailable:  {},
				http.StatusGatewayTimeout:      {},
			},
		},
		CircuitBreaker: CircuitBreakerPolicy{
			FailureThreshold: defaultCBFailureThreshold,
			Cooldown:         defaultCBCooldown,
		},
		Logger: noopLogger{},
		Clock:  realClock{},
	}
}

// Validate normalizes config values and fails fast when required fields are invalid.
func (c *Config) Validate() error {
	if c == nil {
		return NewValidationError("config is nil", nil)
	}

	c.CommerceCode = strings.TrimSpace(c.CommerceCode)
	c.APISecret = strings.TrimSpace(c.APISecret)

	environment, err := resolveEnvironment(c.Environment)
	if err != nil {
		return NewValidationError("invalid environment", err)
	}

	autodetected, err := autodetectEnvironment(environment, c.CommerceCode, c.APISecret)
	if err != nil {
		return NewValidationError("could not autodetect environment", err)
	}
	c.Environment = autodetected

	if c.Environment == EnvironmentIntegration && c.CommerceCode == "" && c.APISecret == "" {
		c.CommerceCode = DefaultIntegrationCommerceCode
		c.APISecret = DefaultIntegrationAPISecret
	}

	if err := validateCommerceCode(c.CommerceCode); err != nil {
		return NewValidationError("invalid commerce code", err)
	}
	if c.APISecret == "" {
		return NewValidationError("invalid API secret", ErrInvalidAPISecret)
	}

	if c.baseURL == "" {
		c.baseURL = baseURLForEnvironment(c.Environment)
	}
	if err := validateBaseURL(c.baseURL); err != nil {
		return NewValidationError("invalid base URL", err)
	}
	c.baseURL = strings.TrimSuffix(c.baseURL, "/")

	if c.Timeout <= 0 {
		c.Timeout = defaultHTTPTimeout
	}

	if c.RetryPolicy.MaxAttempts <= 0 {
		c.RetryPolicy.MaxAttempts = defaultRetryMaxAttempts
	}
	if c.RetryPolicy.InitialBackoff <= 0 {
		c.RetryPolicy.InitialBackoff = defaultRetryInitialBackoff
	}
	if c.RetryPolicy.MaxBackoff <= 0 {
		c.RetryPolicy.MaxBackoff = defaultRetryMaxBackoff
	}
	if c.RetryPolicy.MaxBackoff < c.RetryPolicy.InitialBackoff {
		c.RetryPolicy.MaxBackoff = c.RetryPolicy.InitialBackoff
	}
	if len(c.RetryPolicy.RetryOnStatus) == 0 {
		c.RetryPolicy = DefaultConfig().RetryPolicy
	}

	if c.CircuitBreaker.FailureThreshold <= 0 {
		c.CircuitBreaker.FailureThreshold = defaultCBFailureThreshold
	}
	if c.CircuitBreaker.Cooldown <= 0 {
		c.CircuitBreaker.Cooldown = defaultCBCooldown
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.Timeout}
	} else if c.HTTPClient.Timeout <= 0 {
		cloned := *c.HTTPClient
		cloned.Timeout = c.Timeout
		c.HTTPClient = &cloned
	}

	if c.Logger == nil {
		c.Logger = noopLogger{}
	}
	if c.Clock == nil {
		c.Clock = realClock{}
	}

	return nil
}

func baseURLForEnvironment(environment Environment) string {
	if environment == EnvironmentProduction {
		return defaultProductionBaseURL
	}
	return defaultIntegrationBaseURL
}

func resolveEnvironment(environment Environment) (Environment, error) {
	if environment == "" {
		return "", nil
	}
	if !environment.IsValid() {
		return "", ErrInvalidEnvironment
	}
	return environment, nil
}

func autodetectEnvironment(explicit Environment, commerceCode, apiSecret string) (Environment, error) {
	if explicit.IsValid() {
		return explicit, nil
	}

	if commerceCode == "" && apiSecret == "" {
		return EnvironmentIntegration, nil
	}

	if commerceCode == DefaultIntegrationCommerceCode && apiSecret == DefaultIntegrationAPISecret {
		return EnvironmentIntegration, nil
	}

	return EnvironmentProduction, nil
}

// Option mutates config values before validation.
type Option func(*Config) error

// WithHTTPClient overrides the HTTP client used by the SDK.
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *Config) error {
		if client == nil {
			return NewValidationError("http client cannot be nil", ErrNilHTTPClient)
		}
		cfg.HTTPClient = client
		return nil
	}
}

// WithLogger overrides the SDK logger.
func WithLogger(logger Logger) Option {
	return func(cfg *Config) error {
		if logger == nil {
			return NewValidationError("logger cannot be nil", nil)
		}
		cfg.Logger = logger
		return nil
	}
}

// WithRetryPolicy overrides retry settings.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(cfg *Config) error {
		cfg.RetryPolicy = policy
		return nil
	}
}

// WithClock overrides the clock implementation.
func WithClock(clock Clock) Option {
	return func(cfg *Config) error {
		if clock == nil {
			return NewValidationError("clock cannot be nil", nil)
		}
		cfg.Clock = clock
		return nil
	}
}

// WithTimeout overrides request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.Timeout = timeout
		return nil
	}
}

// WithHooks registers request lifecycle hooks.
func WithHooks(hooks Hooks) Option {
	return func(cfg *Config) error {
		cfg.Hooks = hooks
		return nil
	}
}

// WithCircuitBreaker overrides circuit-breaker settings.
func WithCircuitBreaker(policy CircuitBreakerPolicy) Option {
	return func(cfg *Config) error {
		cfg.CircuitBreaker = policy
		return nil
	}
}
