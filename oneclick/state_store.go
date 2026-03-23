package oneclick

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrStateStoreNotFound indicates missing token state in the store.
var ErrStateStoreNotFound = errors.New("flow state not found")

// InMemoryStateStore is the default StateStore implementation.
type InMemoryStateStore struct {
	mu      sync.RWMutex
	byToken map[string]FlowState
}

// NewInMemoryStateStore creates an in-memory state store.
func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{
		byToken: make(map[string]FlowState),
	}
}

// GetByToken returns state for the given token.
func (s *InMemoryStateStore) GetByToken(_ context.Context, token string) (*FlowState, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewValidationError("token is required", ErrInvalidToken)
	}

	s.mu.RLock()
	state, ok := s.byToken[token]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrStateStoreNotFound
	}
	cloned := cloneFlowState(state)
	return &cloned, nil
}

// SavePending saves a pending flow state.
func (s *InMemoryStateStore) SavePending(_ context.Context, state FlowState) error {
	token := strings.TrimSpace(state.Token)
	if token == "" {
		return NewValidationError("token is required", ErrInvalidToken)
	}

	state.Token = token
	if state.Status == "" {
		state.Status = FlowStatusPending
	}
	if strings.TrimSpace(state.CreatedAt) == "" {
		now := time.Now().UTC().Format(timeLayout)
		state.CreatedAt = now
		state.UpdatedAt = now
	}

	s.mu.Lock()
	s.byToken[token] = cloneFlowState(state)
	s.mu.Unlock()
	return nil
}

// MarkConfirmed marks token state as confirmed.
func (s *InMemoryStateStore) MarkConfirmed(_ context.Context, token string, confirmation InscriptionConfirmResponse) (*FlowState, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewValidationError("token is required", ErrInvalidToken)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.byToken[token]
	if !ok {
		return nil, ErrStateStoreNotFound
	}

	state.Status = FlowStatusConfirmed
	state.UpdatedAt = time.Now().UTC().Format(timeLayout)
	copied := confirmation
	state.Confirmation = &copied
	state.Token = token

	s.byToken[token] = state

	cloned := cloneFlowState(state)
	return &cloned, nil
}

// RedisKV is the minimal Redis contract required by RedisStateStore.
type RedisKV interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// RedisStateStore stores flow state as JSON in Redis-like key/value stores.
type RedisStateStore struct {
	Client RedisKV
	Prefix string
	TTL    time.Duration
}

// NewRedisStateStore creates a Redis-backed store adapter.
func NewRedisStateStore(client RedisKV, prefix string, ttl time.Duration) (*RedisStateStore, error) {
	if client == nil {
		return nil, NewValidationError("redis client cannot be nil", nil)
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "oneclick:flow"
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &RedisStateStore{Client: client, Prefix: prefix, TTL: ttl}, nil
}

func (s *RedisStateStore) key(token string) string {
	return fmt.Sprintf("%s:%s", s.Prefix, token)
}

// GetByToken returns state for the given token.
func (s *RedisStateStore) GetByToken(ctx context.Context, token string) (*FlowState, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewValidationError("token is required", ErrInvalidToken)
	}

	raw, err := s.Client.Get(ctx, s.key(token))
	if err != nil {
		if looksLikeNotFound(err) {
			return nil, ErrStateStoreNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, ErrStateStoreNotFound
	}

	var state FlowState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, err
	}

	cloned := cloneFlowState(state)
	return &cloned, nil
}

// SavePending saves a pending flow state.
func (s *RedisStateStore) SavePending(ctx context.Context, state FlowState) error {
	token := strings.TrimSpace(state.Token)
	if token == "" {
		return NewValidationError("token is required", ErrInvalidToken)
	}

	if state.Status == "" {
		state.Status = FlowStatusPending
	}
	if strings.TrimSpace(state.CreatedAt) == "" {
		now := time.Now().UTC().Format(timeLayout)
		state.CreatedAt = now
		state.UpdatedAt = now
	}

	body, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return s.Client.Set(ctx, s.key(token), string(body), s.TTL)
}

// MarkConfirmed marks token state as confirmed.
func (s *RedisStateStore) MarkConfirmed(ctx context.Context, token string, confirmation InscriptionConfirmResponse) (*FlowState, error) {
	state, err := s.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	state.Status = FlowStatusConfirmed
	state.UpdatedAt = time.Now().UTC().Format(timeLayout)
	copied := confirmation
	state.Confirmation = &copied

	if err := s.SavePending(ctx, *state); err != nil {
		return nil, err
	}

	cloned := cloneFlowState(*state)
	return &cloned, nil
}

// PostgresStateStore stores flow state in PostgreSQL via database/sql.
type PostgresStateStore struct {
	DB        *sql.DB
	TableName string
}

// NewPostgresStateStore creates a PostgreSQL adapter for StateStore.
func NewPostgresStateStore(db *sql.DB, tableName string) (*PostgresStateStore, error) {
	if db == nil {
		return nil, NewValidationError("postgres DB cannot be nil", nil)
	}
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		tableName = "oneclick_flow_state"
	}
	if !isValidSQLIdentifier(tableName) {
		return nil, NewValidationError("invalid table name", nil)
	}

	return &PostgresStateStore{DB: db, TableName: tableName}, nil
}

// GetByToken returns state for token.
func (s *PostgresStateStore) GetByToken(ctx context.Context, token string) (*FlowState, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, NewValidationError("token is required", ErrInvalidToken)
	}

	query := fmt.Sprintf(`
SELECT token, username, email, response_url, url_webpay, business_id, subscription_id, context_json,
       status, created_at, updated_at, confirmation_json
FROM %s
WHERE token = $1`, s.TableName)

	var (
		state            FlowState
		contextJSON      []byte
		confirmationJSON []byte
	)

	err := s.DB.QueryRowContext(ctx, query, token).Scan(
		&state.Token,
		&state.Username,
		&state.Email,
		&state.ResponseURL,
		&state.URLWebpay,
		&state.BusinessID,
		&state.SubscriptionID,
		&contextJSON,
		&state.Status,
		&state.CreatedAt,
		&state.UpdatedAt,
		&confirmationJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStateStoreNotFound
		}
		return nil, err
	}

	if len(contextJSON) > 0 {
		_ = json.Unmarshal(contextJSON, &state.Context)
	}
	if len(confirmationJSON) > 0 {
		var confirmation InscriptionConfirmResponse
		if err := json.Unmarshal(confirmationJSON, &confirmation); err == nil {
			state.Confirmation = &confirmation
		}
	}

	cloned := cloneFlowState(state)
	return &cloned, nil
}

// SavePending upserts pending state by token.
func (s *PostgresStateStore) SavePending(ctx context.Context, state FlowState) error {
	token := strings.TrimSpace(state.Token)
	if token == "" {
		return NewValidationError("token is required", ErrInvalidToken)
	}

	state.Token = token
	if state.Status == "" {
		state.Status = FlowStatusPending
	}
	if strings.TrimSpace(state.CreatedAt) == "" {
		now := time.Now().UTC().Format(timeLayout)
		state.CreatedAt = now
		state.UpdatedAt = now
	}

	contextJSON, _ := json.Marshal(state.Context)
	confirmationJSON, _ := json.Marshal(state.Confirmation)

	query := fmt.Sprintf(`
INSERT INTO %s (
	token, username, email, response_url, url_webpay, business_id, subscription_id,
	context_json, status, created_at, updated_at, confirmation_json
) VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11, $12
)
ON CONFLICT (token) DO UPDATE SET
	username = EXCLUDED.username,
	email = EXCLUDED.email,
	response_url = EXCLUDED.response_url,
	url_webpay = EXCLUDED.url_webpay,
	business_id = EXCLUDED.business_id,
	subscription_id = EXCLUDED.subscription_id,
	context_json = EXCLUDED.context_json,
	status = EXCLUDED.status,
	updated_at = EXCLUDED.updated_at,
	confirmation_json = EXCLUDED.confirmation_json`, s.TableName)

	_, err := s.DB.ExecContext(ctx, query,
		state.Token,
		state.Username,
		state.Email,
		state.ResponseURL,
		state.URLWebpay,
		state.BusinessID,
		state.SubscriptionID,
		contextJSON,
		state.Status,
		state.CreatedAt,
		state.UpdatedAt,
		confirmationJSON,
	)
	return err
}

// MarkConfirmed updates state confirmation details.
func (s *PostgresStateStore) MarkConfirmed(ctx context.Context, token string, confirmation InscriptionConfirmResponse) (*FlowState, error) {
	state, err := s.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	state.Status = FlowStatusConfirmed
	state.UpdatedAt = time.Now().UTC().Format(timeLayout)
	copied := confirmation
	state.Confirmation = &copied

	if err := s.SavePending(ctx, *state); err != nil {
		return nil, err
	}

	cloned := cloneFlowState(*state)
	return &cloned, nil
}

// InMemoryIdempotencyStore caches idempotency records in memory.
type InMemoryIdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]IdempotencyRecord
}

// NewInMemoryIdempotencyStore creates an in-memory idempotency store.
func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		records: make(map[string]IdempotencyRecord),
	}
}

func idempotencyStoreKey(operation, key string) string {
	return operation + ":" + key
}

// Get returns existing idempotency record.
func (s *InMemoryIdempotencyStore) Get(_ context.Context, operation, key string) (*IdempotencyRecord, error) {
	operation = strings.TrimSpace(operation)
	key = strings.TrimSpace(key)
	if operation == "" || key == "" {
		return nil, nil
	}

	s.mu.RLock()
	record, ok := s.records[idempotencyStoreKey(operation, key)]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	cloned := cloneIdempotencyRecord(record)
	return &cloned, nil
}

// Save stores idempotency record.
func (s *InMemoryIdempotencyStore) Save(_ context.Context, record IdempotencyRecord) error {
	op := strings.TrimSpace(record.Operation)
	key := strings.TrimSpace(record.Key)
	if op == "" || key == "" {
		return NewValidationError("idempotency operation and key are required", nil)
	}

	s.mu.Lock()
	s.records[idempotencyStoreKey(op, key)] = cloneIdempotencyRecord(record)
	s.mu.Unlock()
	return nil
}

func cloneIdempotencyRecord(record IdempotencyRecord) IdempotencyRecord {
	cloned := record
	if record.StartResponse != nil {
		copied := *record.StartResponse
		copied.State = cloneFlowState(copied.State)
		cloned.StartResponse = &copied
	}
	if record.ConfirmResponse != nil {
		copied := *record.ConfirmResponse
		copied.State = cloneFlowState(copied.State)
		cloned.ConfirmResponse = &copied
	}
	return cloned
}

func looksLikeNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "redis: nil")
}

func isValidSQLIdentifier(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
